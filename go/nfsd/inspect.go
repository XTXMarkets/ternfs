// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// This file implements a read-only inspector over the persistent client
// store described in README.md. It reads the same records with the same
// rules as the nfsd fleet, but never writes: it does not create the expired
// marker, remove collectable slots or age GC candidates.

// inspectOptions selects which part of the client store to report.
type inspectOptions struct {
	// Identity is the raw SETCLIENTID identity. Its SHA-256 names the
	// identity directory.
	Identity []byte
	// IdentityHash is the hexadecimal identity directory name.
	IdentityHash string
	// ClientID is an incarnation directory inode.
	ClientID uint64
	// StateID limits the report to the incarnation which owns the marker.
	StateID *StateID
	// StagingDir is the local staging directory of this nfsd host. Its
	// sidecars are joined to open markers.
	StagingDir string
}

type inspectReport struct {
	Now          time.Time             `json:"now"`
	LeaseTime    time.Duration         `json:"lease_time"`
	ClientsDirID inspectInode          `json:"clients_dir_id"`
	Identities   []inspectIdentity     `json:"identities"`
	Staging      []inspectStagingEntry `json:"staging,omitempty"`
	Problems     []string              `json:"problems,omitempty"`
}

type inspectIdentity struct {
	Hash         string               `json:"hash"`
	ID           inspectInode         `json:"id"`
	Identity     []byte               `json:"identity,omitempty"`
	Confirmed    *inspectPointer      `json:"confirmed,omitempty"`
	Pending      *inspectPointer      `json:"pending,omitempty"`
	Incarnations []inspectIncarnation `json:"incarnations"`
	Temps        []inspectTemp        `json:"temps,omitempty"`
	Problems     []string             `json:"problems,omitempty"`
}

type inspectPointer struct {
	Target   string       `json:"target"`
	TargetID inspectInode `json:"target_id,omitempty"`
	Dangling bool         `json:"dangling,omitempty"`
	Problem  string       `json:"problem,omitempty"`
}

type inspectIncarnation struct {
	Name     string       `json:"name"`
	ClientID inspectInode `json:"clientid"`
	// Roles is any of confirmed, pending and reboot-target. An incarnation
	// without a role is unreachable and will be collected.
	Roles []string `json:"roles"`
	// LeaseState is live, expired or none.
	LeaseState string `json:"lease_state"`
	// Usable reports whether stateids of this clientid are accepted: the
	// incarnation is confirmed and its lease is live.
	Usable        bool                  `json:"usable"`
	Record        *inspectRecord        `json:"client,omitempty"`
	Update        *inspectRecord        `json:"update,omitempty"`
	Reboot        *inspectPointer       `json:"reboot,omitempty"`
	Leases        []inspectSlot         `json:"leases,omitempty"`
	Confirming    []inspectSlot         `json:"confirming,omitempty"`
	ExpiredMarker bool                  `json:"expired_marker"`
	GCAfter       *time.Time            `json:"gc_after,omitempty"`
	Opens         []inspectOpen         `json:"opens,omitempty"`
	Temps         []inspectTemp         `json:"temps,omitempty"`
	Unknown       []string              `json:"unknown,omitempty"`
	Problems      []string              `json:"problems,omitempty"`
	Staging       []inspectStagingEntry `json:"staging,omitempty"`
}

type inspectRecord struct {
	Identity  []byte `json:"identity,omitempty"`
	Verifier  string `json:"verifier"`
	Confirm   string `json:"confirm"`
	Principal string `json:"principal"`
	Callback  string `json:"callback"`
}

type inspectSlot struct {
	Name    string    `json:"name"`
	NfsdID  string    `json:"nfsd_id"`
	Expires time.Time `json:"expires"`
	Live    bool      `json:"live"`
	// Collectable slots have been expired for a full extra lease and are
	// removed by the next scan.
	Collectable bool `json:"collectable"`
}

type inspectOpen struct {
	StateID string               `json:"stateid"`
	Epoch   string               `json:"epoch"`
	Staging *inspectStagingEntry `json:"staging,omitempty"`
}

type inspectTemp struct {
	Name  string    `json:"name"`
	Mtime time.Time `json:"mtime"`
	// Collectable temporaries are older than one lease.
	Collectable bool `json:"collectable"`
}

type inspectStagingEntry struct {
	FileID   inspectInode `json:"file_id"`
	DirID    inspectInode `json:"dir_id"`
	FileName string       `json:"file_name"`
	StateID  string       `json:"stateid"`
	ClientID inspectInode `json:"clientid"`
	Size     int64        `json:"size"`
	// Matched is set once the entry has been joined to an open marker.
	Matched bool `json:"-"`
}

// openClientStoreReader returns a ClientStore for inspection. Unlike
// NewClientStore it does not create the store directories or a process
// identity.
func openClientStoreReader(fs TernVFS) (*ClientStore, error) {
	nfsID, err := fs.Lookup(fs.RootID(), nfsDirName)
	if err != nil {
		return nil, fmt.Errorf("client store: lookup /%s: %w", nfsDirName, err)
	}
	clientsID, err := fs.Lookup(nfsID, "clients")
	if err != nil {
		return nil, fmt.Errorf(
			"client store: lookup /%s/clients: %w", nfsDirName, err)
	}
	return &ClientStore{
		fs:            fs,
		nfsDirID:      nfsID,
		dirID:         clientsID,
		now:           time.Now,
		identityLocks: make(map[InodeID]*clientIdentityLock),
	}, nil
}

// Inspect builds a report of the client store without modifying it.
func (cs *ClientStore) Inspect(opts inspectOptions) (*inspectReport, error) {
	report := &inspectReport{
		Now:          cs.now(),
		LeaseTime:    nfsLeaseTime,
		ClientsDirID: inspectInode(cs.dirID),
		Identities:   []inspectIdentity{},
	}

	var staging []inspectStagingEntry
	if opts.StagingDir != "" {
		var err error
		var problems []string
		staging, problems, err = readStagingSidecars(opts.StagingDir)
		report.Problems = append(report.Problems, problems...)
		if err != nil {
			return nil, err
		}
	}

	identityIDs, err := cs.inspectSelectIdentities(opts, staging, report)
	if err != nil {
		return nil, err
	}
	for _, entry := range identityIDs {
		identity, err := cs.inspectIdentity(entry, staging)
		if err != nil {
			return nil, err
		}
		if opts.StateID != nil {
			want := activeOpenName(*opts.StateID)
			var kept []inspectIncarnation
			for _, inc := range identity.Incarnations {
				for _, open := range inc.Opens {
					if activeOpenPrefix+open.StateID == want {
						kept = append(kept, inc)
						break
					}
				}
			}
			if len(kept) == 0 {
				continue
			}
			identity.Incarnations = kept
		}
		report.Identities = append(report.Identities, identity)
	}
	sort.Slice(report.Identities, func(i, j int) bool {
		return report.Identities[i].Hash < report.Identities[j].Hash
	})
	for _, entry := range staging {
		if !entry.Matched {
			report.Staging = append(report.Staging, entry)
		}
	}
	return report, nil
}

// inspectSelectIdentities resolves the options to identity directories. A
// clientid or a stateid with a staging sidecar resolve through the parent
// directory; a stateid without one requires a scan of every identity.
func (cs *ClientStore) inspectSelectIdentities(
	opts inspectOptions,
	staging []inspectStagingEntry,
	report *inspectReport,
) ([]DirEntry, error) {
	switch {
	case opts.Identity != nil || opts.IdentityHash != "":
		name := opts.IdentityHash
		if opts.Identity != nil {
			name = clientIdentityKey(opts.Identity)
		}
		id, err := cs.fs.Lookup(cs.dirID, name)
		if errors.Is(err, os.ErrNotExist) {
			report.Problems = append(report.Problems,
				fmt.Sprintf("identity %s is not registered", name))
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []DirEntry{{Name: name, ID: id}}, nil
	case opts.ClientID != 0:
		return cs.inspectIdentityForClientID(InodeID(opts.ClientID), report)
	case opts.StateID != nil:
		want := hex.EncodeToString(opts.StateID[:])
		for _, entry := range staging {
			if entry.StateID == want && entry.ClientID != 0 {
				return cs.inspectIdentityForClientID(
					InodeID(entry.ClientID), report)
			}
		}
	}
	return cs.entries(cs.dirID)
}

func (cs *ClientStore) inspectIdentityForClientID(
	clientID InodeID,
	report *inspectReport,
) ([]DirEntry, error) {
	if !isDirectoryInodeID(clientID) {
		report.Problems = append(report.Problems, fmt.Sprintf(
			"clientid %s is not a directory inode", inodeHex(clientID)))
		return nil, nil
	}
	identityID, err := cs.fs.LookupParent(clientID)
	if errors.Is(err, os.ErrNotExist) {
		report.Problems = append(report.Problems, fmt.Sprintf(
			"clientid %s has no incarnation directory; it is stale or collected",
			inodeHex(clientID)))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if identityID == cs.dirID || identityID == cs.nfsDirID ||
		identityID == cs.fs.RootID() {
		report.Problems = append(report.Problems, fmt.Sprintf(
			"clientid %s is not an incarnation directory", inodeHex(clientID)))
		return nil, nil
	}
	name, err := cs.inspectChildName(cs.dirID, identityID)
	if err != nil {
		return nil, err
	}
	if name == "" {
		report.Problems = append(report.Problems, fmt.Sprintf(
			"clientid %s belongs to directory %s, which is not under /%s/clients",
			inodeHex(clientID), inodeHex(identityID), nfsDirName))
		return nil, nil
	}
	return []DirEntry{{Name: name, ID: identityID}}, nil
}

func (cs *ClientStore) inspectChildName(
	dirID InodeID,
	childID InodeID,
) (string, error) {
	entries, err := cs.entries(dirID)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.ID == childID {
			return entry.Name, nil
		}
	}
	return "", nil
}

func (cs *ClientStore) inspectIdentity(
	dir DirEntry,
	staging []inspectStagingEntry,
) (inspectIdentity, error) {
	identity := inspectIdentity{Hash: dir.Name, ID: inspectInode(dir.ID)}
	if !isDirectoryInodeID(dir.ID) {
		identity.Problems = append(identity.Problems,
			"identity entry is not a directory")
		return identity, nil
	}
	entries, err := cs.entries(dir.ID)
	if errors.Is(err, os.ErrNotExist) {
		identity.Problems = append(identity.Problems,
			"identity directory disappeared during inspection")
		return identity, nil
	}
	if err != nil {
		return identity, err
	}
	now := cs.now()

	roles := make(map[InodeID][]string)
	pointer := func(name string) (*inspectPointer, error) {
		return cs.inspectPointer(dir.ID, name, false)
	}
	if identity.Confirmed, err = pointer(confirmedName); err != nil {
		return identity, err
	}
	if identity.Pending, err = pointer(pendingName); err != nil {
		return identity, err
	}
	if p := identity.Confirmed; p != nil && p.TargetID != 0 {
		roles[InodeID(p.TargetID)] = append(roles[InodeID(p.TargetID)], "confirmed")
	}
	if p := identity.Pending; p != nil && p.TargetID != 0 {
		roles[InodeID(p.TargetID)] = append(roles[InodeID(p.TargetID)], "pending")
	}

	var incarnations []inspectIncarnation
	for _, entry := range entries {
		switch {
		case entry.Name == confirmedName || entry.Name == pendingName:
		case strings.HasPrefix(entry.Name, tempPrefix):
			temp, err := cs.inspectTemp(entry, now)
			if err != nil {
				return identity, err
			}
			identity.Temps = append(identity.Temps, temp)
		case isIncarnationName(entry.Name) && isDirectoryInodeID(entry.ID):
			inc, err := cs.inspectIncarnation(dir.ID, entry, staging)
			if err != nil {
				return identity, err
			}
			incarnations = append(incarnations, inc)
		default:
			identity.Problems = append(identity.Problems,
				fmt.Sprintf("unexpected entry %q", entry.Name))
		}
	}

	// Reboot targets of the primary incarnations stay reachable until the
	// confirmation has purged their state.
	for _, inc := range incarnations {
		if _, primary := roles[InodeID(inc.ClientID)]; !primary {
			continue
		}
		if inc.Reboot != nil && inc.Reboot.TargetID != 0 {
			targetID := InodeID(inc.Reboot.TargetID)
			roles[targetID] = append(roles[targetID], "reboot-target")
		}
	}
	for i := range incarnations {
		inc := &incarnations[i]
		inc.Roles = roles[InodeID(inc.ClientID)]
		if inc.Roles == nil {
			inc.Roles = []string{}
		}
		confirmed := false
		for _, role := range inc.Roles {
			confirmed = confirmed || role == "confirmed"
		}
		inc.Usable = confirmed && inc.LeaseState == "live"
		if inc.Record != nil && len(inc.Record.Identity) != 0 {
			identity.Identity = inc.Record.Identity
		}
	}
	sort.Slice(incarnations, func(i, j int) bool {
		return incarnations[i].Name < incarnations[j].Name
	})
	identity.Incarnations = incarnations
	if identity.Incarnations == nil {
		identity.Incarnations = []inspectIncarnation{}
	}
	return identity, nil
}

func (cs *ClientStore) inspectPointer(
	dirID InodeID,
	name string,
	parentRelative bool,
) (*inspectPointer, error) {
	fileID, err := cs.fs.Lookup(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	target, err := cs.fs.Readlink(fileID)
	if err != nil {
		return &inspectPointer{Target: "<unreadable: " + err.Error() + ">",
			Dangling: true, Problem: "cannot read pointer"}, nil
	}
	p := &inspectPointer{Target: target}
	targetName := target
	targetDirID := dirID
	if parentRelative {
		if !strings.HasPrefix(target, "../") {
			p.Dangling = true
			p.Problem = "pointer must name ../i.<hex>"
			return p, nil
		}
		targetName = strings.TrimPrefix(target, "../")
		targetDirID, err = cs.fs.LookupParent(dirID)
		if err != nil {
			p.Dangling = true
			p.Problem = "cannot find pointer parent: " + err.Error()
			return p, nil
		}
	} else if strings.HasPrefix(target, "../") {
		p.Dangling = true
		p.Problem = "pointer must name bare i.<hex>"
		return p, nil
	}
	if !isIncarnationName(targetName) {
		p.Dangling = true
		p.Problem = "invalid incarnation pointer"
		return p, nil
	}
	id, err := cs.fs.Lookup(targetDirID, targetName)
	if errors.Is(err, os.ErrNotExist) {
		p.Dangling = true
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	p.TargetID = inspectInode(id)
	return p, nil
}

func (cs *ClientStore) inspectTemp(
	entry DirEntry,
	now time.Time,
) (inspectTemp, error) {
	temp := inspectTemp{Name: entry.Name}
	info, err := cs.fs.Stat(entry.ID)
	if errors.Is(err, os.ErrNotExist) {
		return temp, nil
	}
	if err != nil {
		return temp, err
	}
	temp.Mtime = info.Mtime
	temp.Collectable = !info.Mtime.After(now.Add(-nfsLeaseTime))
	return temp, nil
}

func (cs *ClientStore) inspectIncarnation(
	identityID InodeID,
	dir DirEntry,
	staging []inspectStagingEntry,
) (inspectIncarnation, error) {
	inc := inspectIncarnation{
		Name:       dir.Name,
		ClientID:   inspectInode(dir.ID),
		LeaseState: "none",
	}
	entries, err := cs.entries(dir.ID)
	if errors.Is(err, os.ErrNotExist) {
		inc.Problems = append(inc.Problems,
			"incarnation directory disappeared during inspection")
		return inc, nil
	}
	if err != nil {
		return inc, err
	}
	now := cs.now()
	leaseFound := false
	leaseLive := false
	for _, entry := range entries {
		switch {
		case entry.Name == clientRecordName || entry.Name == updateName:
			record, err := cs.inspectRecord(entry)
			if err != nil {
				return inc, err
			}
			if entry.Name == clientRecordName {
				inc.Record = record
			} else {
				inc.Update = record
			}
		case entry.Name == rebootName:
			inc.Reboot, err = cs.inspectPointer(dir.ID, rebootName, true)
			if err != nil {
				return inc, err
			}
		case entry.Name == expiredName:
			inc.ExpiredMarker = true
		case entry.Name == gcCandidateName:
			var candidate durableGCCandidate
			if err := cs.readJSONFile(
				entry.ID, entry.Name, &candidate,
			); err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
			}
			after := time.Unix(0, candidate.CollectAfterUnixNano)
			inc.GCAfter = &after
		case isLeaseName(entry.Name):
			slot, ok, err := cs.inspectSlot(entry, leasePrefix, now)
			if err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
			}
			if !ok {
				continue
			}
			leaseFound = true
			leaseLive = leaseLive || slot.Live
			inc.Leases = append(inc.Leases, slot)
		case strings.HasPrefix(entry.Name, confirmingPrefix) &&
			len(entry.Name) > len(confirmingPrefix):
			slot, ok, err := cs.inspectSlot(entry, confirmingPrefix, now)
			if err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
			}
			if ok {
				inc.Confirming = append(inc.Confirming, slot)
			}
		case isActiveOpenName(entry.Name):
			inc.Opens = append(inc.Opens, inspectOpenMarker(
				strings.TrimPrefix(entry.Name, activeOpenPrefix), staging))
		case strings.HasPrefix(entry.Name, tempPrefix):
			temp, err := cs.inspectTemp(entry, now)
			if err != nil {
				return inc, err
			}
			inc.Temps = append(inc.Temps, temp)
		default:
			inc.Unknown = append(inc.Unknown, entry.Name)
		}
	}
	switch {
	case inc.ExpiredMarker:
		inc.LeaseState = "expired"
	case leaseLive:
		inc.LeaseState = "live"
	case leaseFound:
		inc.LeaseState = "expired"
	}
	if inc.Record == nil {
		inc.Problems = append(inc.Problems, "no client record")
	}
	for _, entry := range staging {
		if entry.ClientID == inc.ClientID && !entry.Matched {
			entry.Matched = true
			inc.Staging = append(inc.Staging, entry)
		}
	}
	sort.Slice(inc.Opens, func(i, j int) bool {
		return inc.Opens[i].StateID < inc.Opens[j].StateID
	})
	return inc, nil
}

func (cs *ClientStore) inspectRecord(entry DirEntry) (*inspectRecord, error) {
	var record durableClientRecord
	if err := cs.readJSONFile(entry.ID, entry.Name, &record); err != nil {
		return &inspectRecord{Principal: "<unreadable: " + err.Error() + ">"}, nil
	}
	return &inspectRecord{
		Identity:  record.ID,
		Verifier:  hex.EncodeToString(record.Verifier),
		Confirm:   hex.EncodeToString(record.Confirm),
		Principal: formatPrincipal(record.principal()),
		Callback:  formatCallback(record.NetID, record.Addr),
	}, nil
}

func (cs *ClientStore) inspectSlot(
	entry DirEntry,
	prefix string,
	now time.Time,
) (inspectSlot, bool, error) {
	lease, err := cs.readLease(entry.ID)
	if errors.Is(err, os.ErrNotExist) {
		return inspectSlot{}, false, nil
	}
	if err != nil {
		return inspectSlot{}, false, fmt.Errorf("%s: %w", entry.Name, err)
	}
	expires := time.Unix(0, lease.ExpiresUnixNano)
	return inspectSlot{
		Name:    entry.Name,
		NfsdID:  strings.TrimPrefix(entry.Name, prefix),
		Expires: expires,
		Live:    lease.ExpiresUnixNano > now.UnixNano(),
		Collectable: lease.ExpiresUnixNano <=
			now.UnixNano()-nfsLeaseTime.Nanoseconds(),
	}, true, nil
}

func inspectOpenMarker(stateID string, staging []inspectStagingEntry) inspectOpen {
	open := inspectOpen{StateID: stateID}
	if len(stateID) >= 8 {
		open.Epoch = stateID[:8]
	}
	for i := range staging {
		if staging[i].StateID == stateID {
			staging[i].Matched = true
			entry := staging[i]
			open.Staging = &entry
			break
		}
	}
	return open
}

// readStagingSidecars loads every .meta sidecar in a local staging
// directory.
func readStagingSidecars(dir string) ([]inspectStagingEntry, []string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.meta"))
	if err != nil {
		return nil, nil, err
	}
	var result []inspectStagingEntry
	var problems []string
	for _, metaPath := range paths {
		meta, err := loadStagingMeta(metaPath)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", metaPath, err))
			continue
		}
		// Sidecars are named by LocalStagingStore as %016x.meta.
		base := strings.TrimSuffix(filepath.Base(metaPath), ".meta")
		fileID, err := strconv.ParseUint(base, 16, 64)
		if err != nil {
			problems = append(problems,
				fmt.Sprintf("%s: unexpected sidecar name", metaPath))
			continue
		}
		entry := inspectStagingEntry{
			FileID:   inspectInode(fileID),
			DirID:    inspectInode(meta.DirID),
			FileName: meta.FileName,
			StateID:  hex.EncodeToString(meta.NFSStateID[:]),
			ClientID: inspectInode(meta.ClientID),
		}
		if info, err := os.Stat(filepath.Join(dir, base+".staging")); err == nil {
			entry.Size = info.Size()
		}
		result = append(result, entry)
	}
	return result, problems, nil
}

func formatClientIdentity(id []byte) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("%q", id)
}

// formatPrincipal decodes the normalized principal stored by
// rpcRequest.principal. AUTH_SYS bodies hold uid, gid and auxiliary gids.
func formatPrincipal(p rpcPrincipal) string {
	body := []byte(p.body)
	switch p.flavor {
	case authNone:
		return "AUTH_NONE"
	case authSys:
		if len(body) < 12 {
			return "AUTH_SYS " + hex.EncodeToString(body)
		}
		uid := binary.BigEndian.Uint32(body[0:4])
		gid := binary.BigEndian.Uint32(body[4:8])
		n := int(binary.BigEndian.Uint32(body[8:12]))
		gids := make([]string, 0, n)
		for i := 0; i < n && 12+4*i+4 <= len(body); i++ {
			gids = append(gids, fmt.Sprint(
				binary.BigEndian.Uint32(body[12+4*i:16+4*i])))
		}
		return fmt.Sprintf("AUTH_SYS uid=%d gid=%d groups=[%s]",
			uid, gid, strings.Join(gids, ","))
	}
	return fmt.Sprintf("flavor %d %s", p.flavor, hex.EncodeToString(body))
}

// formatCallback turns an RFC 5665 universal address such as
// 10.1.2.3.8.1 into 10.1.2.3:2049.
func formatCallback(netid string, addr string) string {
	parts := strings.Split(addr, ".")
	if (netid == "tcp" || netid == "udp") && len(parts) == 6 {
		var octets [6]int
		ok := true
		for i, part := range parts {
			if _, err := fmt.Sscanf(part, "%d", &octets[i]); err != nil {
				ok = false
				break
			}
		}
		if ok {
			return fmt.Sprintf("%s %d.%d.%d.%d:%d", netid,
				octets[0], octets[1], octets[2], octets[3],
				octets[4]<<8|octets[5])
		}
	}
	return netid + " " + addr
}

// WriteText renders the report for a terminal.
func (r *inspectReport) WriteText(w io.Writer) {
	rel := func(t time.Time) string {
		d := t.Sub(r.Now).Round(time.Second)
		if d >= 0 {
			return fmt.Sprintf("%s (in %s)", t.Format(time.RFC3339), d)
		}
		return fmt.Sprintf("%s (%s ago)", t.Format(time.RFC3339), -d)
	}
	fmt.Fprintf(w, "client store /%s/clients %s at %s, lease %s\n",
		nfsDirName, inodeHex(r.ClientsDirID), r.Now.Format(time.RFC3339), r.LeaseTime)
	for _, problem := range r.Problems {
		fmt.Fprintf(w, "problem: %s\n", problem)
	}
	if len(r.Identities) == 0 {
		fmt.Fprintf(w, "no client identities\n")
	}
	for _, identity := range r.Identities {
		fmt.Fprintf(w, "\nidentity %s %s\n", identity.Hash, inodeHex(identity.ID))
		if len(identity.Identity) != 0 {
			fmt.Fprintf(w, "  id        %s\n", formatClientIdentity(identity.Identity))
		}
		writePointer := func(name string, p *inspectPointer) {
			if p == nil {
				fmt.Fprintf(w, "  %-9s (none)\n", name)
				return
			}
			fmt.Fprintf(w, "  %-9s -> %s", name, p.Target)
			if p.Dangling {
				fmt.Fprintf(w, " DANGLING")
			} else {
				fmt.Fprintf(w, " clientid %s", inodeHex(p.TargetID))
			}
			if p.Problem != "" {
				fmt.Fprintf(w, " (%s)", p.Problem)
			}
			fmt.Fprintln(w)
		}
		writePointer("confirmed", identity.Confirmed)
		writePointer("pending", identity.Pending)
		for _, problem := range identity.Problems {
			fmt.Fprintf(w, "  problem: %s\n", problem)
		}
		for _, temp := range identity.Temps {
			fmt.Fprintf(w, "  temp %s mtime %s%s\n", temp.Name, rel(temp.Mtime),
				collectableSuffix(temp.Collectable))
		}
		for _, inc := range identity.Incarnations {
			roles := strings.Join(inc.Roles, ",")
			if roles == "" {
				roles = "unreachable"
			}
			state := "NOT USABLE"
			if inc.Usable {
				state = "USABLE"
			} else if strings.Contains(roles, "confirmed") && inc.LeaseState == "none" {
				state = "AWAITING FIRST RENEW"
			}
			fmt.Fprintf(w, "\n  incarnation %s clientid %s [%s, lease %s] %s\n",
				inc.Name, inodeHex(inc.ClientID), roles, inc.LeaseState, state)
			writeRecord := func(name string, rec *inspectRecord) {
				if rec == nil {
					return
				}
				fmt.Fprintf(w, "    %-10s", name)
				if len(rec.Identity) != 0 {
					fmt.Fprintf(w, " id %s", formatClientIdentity(rec.Identity))
				}
				fmt.Fprintf(w, " verifier %s confirm %s\n", rec.Verifier, rec.Confirm)
				fmt.Fprintf(w, "    %-10s principal %s callback %s\n", "",
					rec.Principal, rec.Callback)
			}
			writeRecord("client", inc.Record)
			writeRecord("update", inc.Update)
			if inc.Reboot != nil {
				fmt.Fprintf(w, "    reboot     -> %s", inc.Reboot.Target)
				if inc.Reboot.Dangling {
					fmt.Fprintf(w, " DANGLING")
				} else {
					fmt.Fprintf(w, " clientid %s (purge in progress)",
						inodeHex(inc.Reboot.TargetID))
				}
				fmt.Fprintln(w)
			}
			for _, slot := range inc.Leases {
				fmt.Fprintf(w, "    lease      nfsd %s expires %s %s%s\n",
					slot.NfsdID, rel(slot.Expires), liveWord(slot.Live),
					collectableSuffix(slot.Collectable))
			}
			for _, slot := range inc.Confirming {
				fmt.Fprintf(w, "    confirming nfsd %s expires %s %s%s\n",
					slot.NfsdID, rel(slot.Expires), liveWord(slot.Live),
					collectableSuffix(slot.Collectable))
			}
			if inc.ExpiredMarker {
				fmt.Fprintf(w, "    expired    marker present; clientid cannot renew\n")
			}
			if inc.GCAfter != nil {
				fmt.Fprintf(w, "    gc         unreachable, collect after %s\n",
					rel(*inc.GCAfter))
			}
			for _, open := range inc.Opens {
				fmt.Fprintf(w, "    open       stateid %s epoch %s", open.StateID, open.Epoch)
				if open.Staging != nil {
					fmt.Fprintf(w, " staging file %s -> dir %s name %q (%d bytes)",
						inodeHex(open.Staging.FileID), inodeHex(open.Staging.DirID),
						open.Staging.FileName, open.Staging.Size)
				}
				fmt.Fprintln(w)
			}
			for _, entry := range inc.Staging {
				fmt.Fprintf(w, "    staging    file %s stateid %s -> dir %s name %q (%d bytes) NO OPEN MARKER\n",
					inodeHex(entry.FileID), entry.StateID, inodeHex(entry.DirID),
					entry.FileName, entry.Size)
			}
			for _, temp := range inc.Temps {
				fmt.Fprintf(w, "    temp       %s mtime %s%s\n", temp.Name,
					rel(temp.Mtime), collectableSuffix(temp.Collectable))
			}
			for _, name := range inc.Unknown {
				fmt.Fprintf(w, "    unknown    %s\n", name)
			}
			for _, problem := range inc.Problems {
				fmt.Fprintf(w, "    problem: %s\n", problem)
			}
		}
	}
	if len(r.Staging) > 0 {
		fmt.Fprintf(w, "\nstaging files not matched to a reported client\n")
		for _, entry := range r.Staging {
			fmt.Fprintf(w, "  file %s stateid %s clientid %s -> dir %s name %q (%d bytes)\n",
				inodeHex(entry.FileID), entry.StateID, inodeHex(entry.ClientID),
				inodeHex(entry.DirID), entry.FileName, entry.Size)
		}
	}
}

func (r *inspectReport) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func liveWord(live bool) string {
	if live {
		return "LIVE"
	}
	return "EXPIRED"
}

func collectableSuffix(collectable bool) string {
	if collectable {
		return " (collectable)"
	}
	return ""
}

// inodeHex formats an inode ID the way terncli and the TernFS logs do.
func inodeHex[T ~uint64](id T) string {
	return fmt.Sprintf("0x%016x", uint64(id))
}

// inspectInode keeps inspector JSON formatting local to the report.
type inspectInode InodeID

func (id inspectInode) MarshalJSON() ([]byte, error) {
	return json.Marshal(inodeHex(id))
}
