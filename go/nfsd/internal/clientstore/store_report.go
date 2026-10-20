// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// This file implements the read-only store report that /v1/store serves
// over the persistent client store described in README.md. It reads the
// same records with the same rules as the nfsd fleet, but never writes: it
// does not create the expired marker, remove collectable slots or age GC
// candidates.

// storeFiltered reports whether the query narrows the report to one client.
// A filtered report cannot say whether a staging sidecar belongs to no client
// at all or merely to a client the filter excluded.
func storeFiltered(q inspect.StoreQuery) bool {
	return q.Identity != "" || q.IdentityHash != "" || q.Incarnation != 0 || q.Stateid != nil
}

// inspectFilter echoes the active selector back into the report so that the
// JSON is self-describing. Its keys are the query parameter names.
type inspectFilter struct {
	Identity     string `json:"identity,omitempty"`
	IdentityHex  string `json:"identity_hex,omitempty"`
	IdentityHash string `json:"identity_hash,omitempty"`
	Incarnation  string `json:"incarnation,omitempty"`
	Stateid      string `json:"stateid,omitempty"`
}

func storeFilter(q inspect.StoreQuery) *inspectFilter {
	if !storeFiltered(q) {
		return nil
	}
	f := &inspectFilter{IdentityHash: identityHashText(q.IdentityHash), Incarnation: incarnationText(q.Incarnation)}
	f.Identity, f.IdentityHex = inspectOpaque([]byte(q.Identity))
	if q.Stateid != nil {
		f.Stateid = stateidText(nfs4.StateID(*q.Stateid))
	}
	return f
}

type inspectReport struct {
	Now          time.Time `json:"now"`
	LeaseTime    string    `json:"lease_time"`
	ClientsDirID string    `json:"clients_dir_id"`
	// Filter is the selection the report was narrowed to, or nil. A filtered
	// report's Staging holds only the selected clients' sidecars.
	Filter         *inspectFilter         `json:"filter,omitempty"`
	Summary        inspectSummary         `json:"summary"`
	Identities     []inspectIdentity      `json:"identities"`
	StagingDir     string                 `json:"staging_dir,omitempty"`
	StagingSummary *inspectStagingSummary `json:"staging_summary,omitempty"`
	Staging        []*inspectStagingEntry `json:"staging,omitempty"`
	// Quarantined names the files nfsd moved aside because their metadata
	// sidecar could not be decoded, relative to StagingDir.
	Quarantined []string `json:"quarantined,omitempty"`
	// Problems are defects in the store or the staging directory, counted in
	// Summary.Problems.
	Problems []string `json:"problems,omitempty"`
	// Notes explain an empty or narrowed report, such as a filter which
	// selected a client that is not in the store. A note is an answer to the
	// question asked, not a defect, so notes are not counted as problems.
	Notes []string `json:"notes,omitempty"`
}

type inspectSummary struct {
	Identities   int `json:"identities"`
	Incarnations int `json:"incarnations"`
	Active       int `json:"active"`
	Expired      int `json:"expired"`
	Unleased     int `json:"unleased"`
	Pending      int `json:"pending"`
	Replaced     int `json:"replaced"`
	Unreachable  int `json:"unreachable"`
	OpenMarkers  int `json:"open_markers"`
	StaleOpens   int `json:"stale_open_markers"`
	// Problems counts every defect anywhere in the report. Expired leases,
	// stale opens, unreachable incarnations, collectable slots and retired
	// staging entries are ordinary lifecycle states, not defects, and are not
	// counted here.
	Problems int `json:"problems"`
}

type inspectStagingSummary struct {
	Entries  int `json:"entries"`
	Complete int `json:"complete"`
	// Retired entries hold acknowledged data for a client whose lease
	// expired.
	Retired   int `json:"retired"`
	DataFiles int `json:"data_files"`
	Sidecars  int `json:"sidecars"`
	// OtherClients counts the sidecars suppressed because the report is
	// filtered to one client.
	OtherClients   int   `json:"other_clients"`
	DataBytes      int64 `json:"data_bytes"`
	AllocatedBytes int64 `json:"allocated_bytes"`
}

// inspectIdentity names its fields after the query parameters which select
// it: IdentityHash is identity-hash and Identity is identity. Name is the
// directory's raw name, which is the hash when the store is well formed and
// is kept as evidence when it is not. DirectoryID is the identity
// directory's own inode, which is not an incarnation.
type inspectIdentity struct {
	Name         string `json:"name"`
	IdentityHash string `json:"identity_hash,omitempty"`
	DirectoryID  string `json:"directory_id"`
	Identity     string `json:"identity,omitempty"`
	// IdentityHex carries the identity when it is not printable text.
	IdentityHex  string               `json:"identity_hex,omitempty"`
	Confirmed    *inspectPointer      `json:"confirmed,omitempty"`
	Pending      *inspectPointer      `json:"pending,omitempty"`
	Incarnations []inspectIncarnation `json:"incarnations"`
	Temps        []inspectTemp        `json:"temps,omitempty"`
	Problems     []string             `json:"problems,omitempty"`
}

type inspectPointer struct {
	Target   string `json:"target"`
	TargetID string `json:"target_id,omitempty"`
	Dangling bool   `json:"dangling,omitempty"`
	Problem  string `json:"problem,omitempty"`
}

type inspectIncarnation struct {
	Name        string `json:"name"`
	Incarnation string `json:"incarnation"`
	// Roles is any of confirmed, pending and reboot-target. An incarnation
	// without a role is unreachable and will be collected.
	Roles []string `json:"roles"`
	// LeaseState is live, expired or none.
	LeaseState string `json:"lease_state"`
	// Status is ACTIVE, EXPIRED, UNLEASED, PENDING, REPLACED or
	// UNREACHABLE.
	Status string `json:"status"`
	// Usable reports whether stateids of this incarnation are accepted: the
	// incarnation is confirmed and its lease is live.
	Usable        bool                   `json:"usable"`
	Record        *inspectRecord         `json:"client,omitempty"`
	Update        *inspectRecord         `json:"update,omitempty"`
	Reboot        *inspectPointer        `json:"reboot,omitempty"`
	Leases        []inspectSlot          `json:"leases,omitempty"`
	Confirming    []inspectSlot          `json:"confirming,omitempty"`
	ExpiredMarker bool                   `json:"expired_marker"`
	GCAfter       *time.Time             `json:"gc_after,omitempty"`
	Opens         []inspectOpen          `json:"opens,omitempty"`
	Temps         []inspectTemp          `json:"temps,omitempty"`
	Unknown       []string               `json:"unknown,omitempty"`
	Problems      []string               `json:"problems,omitempty"`
	Staging       []*inspectStagingEntry `json:"staging,omitempty"`
}

type inspectRecord struct {
	Identity    string `json:"identity,omitempty"`
	IdentityHex string `json:"identity_hex,omitempty"`
	Verifier    string `json:"verifier"`
	Confirm     string `json:"confirm"`
	Principal   string `json:"principal"`
}

type inspectSlot struct {
	Name    string    `json:"name"`
	NfsdID  string    `json:"nfsd_id"`
	Expires time.Time `json:"expires"`
	Live    bool      `json:"live"`
	// Collectable slots have been expired for a full extra lease and are
	// removed by the next scan of this client.
	Collectable bool `json:"collectable"`
}

type inspectOpen struct {
	StateID string               `json:"stateid"`
	Epoch   string               `json:"epoch"`
	Active  bool                 `json:"active"`
	Staging *inspectStagingEntry `json:"staging,omitempty"`
}

type inspectTemp struct {
	Name  string    `json:"name"`
	Mtime time.Time `json:"mtime"`
	// Collectable temporaries are older than one lease.
	Collectable bool `json:"collectable"`
}

type inspectStagingEntry struct {
	FileID         string     `json:"file_id"`
	DataPresent    bool       `json:"data_present"`
	MetaPresent    bool       `json:"meta_present"`
	Size           int64      `json:"size"`
	AllocatedBytes int64      `json:"allocated_bytes"`
	DataMtime      *time.Time `json:"data_mtime,omitempty"`
	CheckpointTime *time.Time `json:"checkpoint_time,omitempty"`
	SidecarVersion string     `json:"sidecar_version,omitempty"`
	DirID          string     `json:"dir_id,omitempty"`
	FileName       string     `json:"file_name,omitempty"`
	Cookie         string     `json:"cookie,omitempty"`
	// RecoveryKey is the client identity, boot verifier and principal which
	// must match for a rebooted client to reclaim this data.
	RecoveryKey string `json:"recovery_key,omitempty"`
	StateID     string `json:"stateid,omitempty"`
	Incarnation string `json:"incarnation,omitempty"`
	OpenOwner   string `json:"open_owner,omitempty"`
	// OpenOwnerHex carries the open owner when it is not printable text.
	OpenOwnerHex string `json:"open_owner_hex,omitempty"`
	OwnerKnown   bool   `json:"owner_known"`
	ReadOnly     bool   `json:"read_only"`
	// Retired marks data kept after the owning lease expired, so that the
	// client can reclaim it after a reboot.
	Retired  bool `json:"retired"`
	Unlinked bool `json:"unlinked"`
	Guarded  bool `json:"guarded"`
	// Exclusive marks an EXCLUSIVE4 create; Verifier is the client's create
	// verifier, which a retry must match to reuse this writer.
	Exclusive       bool               `json:"exclusive,omitempty"`
	Verifier        string             `json:"verifier,omitempty"`
	BaseID          string             `json:"base_id,omitempty"`
	BaseSize        uint64             `json:"base_size"`
	CheckpointSize  uint64             `json:"checkpoint_size"`
	Dirty           []inspectByteRange `json:"dirty,omitempty"`
	MetadataChanged bool               `json:"metadata_changed"`
	Change          uint64             `json:"change,omitempty"`
	Mtime           *time.Time         `json:"mtime,omitempty"`
	Atime           *time.Time         `json:"atime,omitempty"`
	Ctime           *time.Time         `json:"ctime,omitempty"`
	Problems        []string           `json:"problems,omitempty"`
	openMatched     bool
	reported        bool
	metaValid       bool
	// clientID is the sidecar's owner, for resolving a stateid selector to
	// its identity directory; Incarnation is its text.
	clientID vfs.InodeID
}

type inspectByteRange struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

const (
	inspectStatusActive      = "ACTIVE"
	inspectStatusExpired     = "EXPIRED"
	inspectStatusUnleased    = "UNLEASED"
	inspectStatusPending     = "PENDING"
	inspectStatusReplaced    = "REPLACED"
	inspectStatusUnreachable = "UNREACHABLE"
)

// OpenReader returns a Store for inspection. Unlike
// NewStore it does not create the store directories or a process
// identity.
func OpenReader(fs vfs.TernVFS) (*Store, error) {
	nfsID, err := fs.Lookup(fs.RootID(), vfs.NFSDirName)
	if err != nil {
		return nil, fmt.Errorf("client store: lookup /%s: %w", vfs.NFSDirName, err)
	}
	clientsID, err := fs.Lookup(nfsID, "clients")
	if err != nil {
		return nil, fmt.Errorf(
			"client store: lookup /%s/clients: %w", vfs.NFSDirName, err)
	}
	return NewStoreAt(fs, nfsID, clientsID)
}

// Inspect builds a report of the client store without modifying it. Every
// lease and collection decision is taken against a single instant so that the
// report is internally consistent. stagingDir is the server's own staging
// directory, or empty when the sidecars were not asked for; the query never
// names a path.
func (cs *Store) Inspect(q inspect.StoreQuery, stagingDir string) (*inspectReport, error) {
	now := cs.Now()
	report := &inspectReport{
		Now:          now,
		LeaseTime:    LeaseTime.String(),
		ClientsDirID: inodeText(cs.DirID),
		Filter:       storeFilter(q),
		Identities:   []inspectIdentity{},
		StagingDir:   stagingDir,
	}

	var staging []*inspectStagingEntry
	if stagingDir != "" {
		var err error
		if staging, err = report.readStaging(stagingDir); err != nil {
			return nil, err
		}
	}

	identityIDs, err := cs.inspectSelectIdentities(q, staging, report)
	if err != nil {
		return nil, err
	}
	for _, entry := range identityIDs {
		identity, err := cs.inspectIdentity(entry, staging, now)
		if err != nil {
			return nil, err
		}
		if q.Stateid != nil {
			identity.Incarnations = inspectKeepStateID(identity.Incarnations, stateidText(nfs4.StateID(*q.Stateid)))
			if len(identity.Incarnations) == 0 {
				continue
			}
		}
		for _, inc := range identity.Incarnations {
			for _, open := range inc.Opens {
				if open.Staging != nil {
					open.Staging.reported = true
				}
			}
			for _, entry := range inc.Staging {
				entry.reported = true
			}
		}
		report.Identities = append(report.Identities, identity)
	}
	sort.Slice(report.Identities, func(i, j int) bool {
		return report.Identities[i].Name < report.Identities[j].Name
	})
	if q.Stateid != nil && len(report.Identities) == 0 {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"%s has no open marker in the store", stateidText(nfs4.StateID(*q.Stateid))))
	}
	// Only an unfiltered report can claim a sidecar belongs to no client: a
	// filtered one has not looked at the other identities. Count the rest
	// instead of listing another client's files under a misleading heading.
	for _, entry := range staging {
		if entry.reported {
			continue
		}
		if storeFiltered(q) {
			report.StagingSummary.OtherClients++
			continue
		}
		report.Staging = append(report.Staging, entry)
	}
	report.summarize()
	return report, nil
}

// inspectKeepStateID keeps the incarnations which own the stateid, either as
// an open marker or as a staging sidecar left behind by one.
func inspectKeepStateID(
	incarnations []inspectIncarnation,
	stateID string,
) []inspectIncarnation {
	var kept []inspectIncarnation
	for _, inc := range incarnations {
		if slices.ContainsFunc(inc.Opens, func(o inspectOpen) bool {
			return o.StateID == stateID
		}) || slices.ContainsFunc(inc.Staging, func(e *inspectStagingEntry) bool {
			return e.StateID == stateID
		}) {
			kept = append(kept, inc)
		}
	}
	return kept
}

// inspectSelectIdentities resolves the query to identity directories. The
// store is addressed by its raw directory names: the identity hash as hex,
// never the typed text. An incarnation, or a stateid with a staging sidecar,
// resolves through the parent directory; a stateid without one requires a
// scan of every identity.
func (cs *Store) inspectSelectIdentities(
	q inspect.StoreQuery,
	staging []*inspectStagingEntry,
	report *inspectReport,
) ([]vfs.DirEntry, error) {
	switch {
	case q.Identity != "" || q.IdentityHash != "":
		name := q.IdentityHash
		if q.Identity != "" {
			name = clientIdentityKey([]byte(q.Identity))
		}
		id, err := cs.fs.Lookup(cs.DirID, name)
		if errors.Is(err, os.ErrNotExist) {
			report.Notes = append(report.Notes,
				fmt.Sprintf("identity-hash:%s is not registered", name))
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []vfs.DirEntry{{Name: name, ID: id}}, nil
	case q.Incarnation != 0:
		return cs.inspectIdentityForIncarnation(vfs.InodeID(q.Incarnation), report)
	case q.Stateid != nil:
		// A sidecar names the incarnation directly. Without one there is
		// nothing to resolve against, so fall through and scan every identity
		// for an open marker with this stateid.
		want := stateidText(nfs4.StateID(*q.Stateid))
		for _, entry := range staging {
			if entry.StateID == want && entry.clientID != 0 {
				return cs.inspectIdentityForIncarnation(entry.clientID, report)
			}
		}
	}
	return cs.entries(cs.DirID)
}

// inspectIdentityForIncarnation resolves an incarnation to its identity
// directory. Anything that goes wrong here is about the argument, not the
// store, so it is reported as a note rather than a problem. The most likely
// mistake is pasting the identity directory inode, which the report prints
// next to the incarnation, so that case gets its own message.
func (cs *Store) inspectIdentityForIncarnation(
	incarnation vfs.InodeID,
	report *inspectReport,
) ([]vfs.DirEntry, error) {
	note := func(format string, args ...any) ([]vfs.DirEntry, error) {
		report.Notes = append(report.Notes, fmt.Sprintf(format, args...))
		return nil, nil
	}
	identityID, err := cs.fs.LookupParent(incarnation)
	if errors.Is(err, os.ErrNotExist) {
		return note("%s has no incarnation directory; it is stale or collected",
			incarnationText(uint64(incarnation)))
	}
	if err != nil {
		return nil, err
	}
	if identityID == cs.DirID {
		return note("%s is an identity directory, not an incarnation; "+
			"select it with identity-hash", inodeText(incarnation))
	}
	name, err := cs.inspectChildName(cs.DirID, identityID)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return note("%s is not under /%s/clients",
			incarnationText(uint64(incarnation)), vfs.NFSDirName)
	}
	return []vfs.DirEntry{{Name: name, ID: identityID}}, nil
}

func (cs *Store) inspectChildName(
	dirID vfs.InodeID,
	childID vfs.InodeID,
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

func (cs *Store) inspectIdentity(
	dir vfs.DirEntry,
	staging []*inspectStagingEntry,
	now time.Time,
) (inspectIdentity, error) {
	identity := inspectIdentity{
		Name:         dir.Name,
		IdentityHash: identityHashText(dir.Name),
		DirectoryID:  inodeText(dir.ID),
	}
	// The directory name is the identity hash. A name that is not one is
	// kept as evidence; the hash field stays absent rather than invented.
	if identity.IdentityHash == "" {
		identity.Problems = append(identity.Problems,
			fmt.Sprintf("identity entry %q is not a 64-digit hex hash", dir.Name))
	}
	if !IsDirectoryInodeID(dir.ID) {
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

	// Roles are keyed by the incarnation's text, as the pointers and
	// incarnations report it.
	roles := make(map[string][]string)
	pointer := func(name string) (*inspectPointer, error) {
		return cs.inspectPointer(dir.ID, name, false)
	}
	if identity.Confirmed, err = pointer(ConfirmedName); err != nil {
		return identity, err
	}
	if identity.Pending, err = pointer(pendingName); err != nil {
		return identity, err
	}
	if p := identity.Confirmed; p != nil && p.TargetID != "" {
		roles[p.TargetID] = append(roles[p.TargetID], "confirmed")
	}
	if p := identity.Pending; p != nil && p.TargetID != "" {
		roles[p.TargetID] = append(roles[p.TargetID], "pending")
	}

	var incarnations []inspectIncarnation
	for _, entry := range entries {
		switch {
		case entry.Name == ConfirmedName || entry.Name == pendingName:
		case strings.HasPrefix(entry.Name, TempPrefix):
			temp, err := cs.inspectTemp(entry, now)
			if err != nil {
				return identity, err
			}
			identity.Temps = append(identity.Temps, temp)
		case isIncarnationName(entry.Name) && IsDirectoryInodeID(entry.ID):
			inc, err := cs.inspectIncarnation(entry, staging, now)
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
		if _, primary := roles[inc.Incarnation]; !primary {
			continue
		}
		if inc.Reboot != nil && inc.Reboot.TargetID != "" {
			roles[inc.Reboot.TargetID] = append(roles[inc.Reboot.TargetID], "reboot-target")
		}
	}
	sort.Slice(incarnations, func(i, j int) bool {
		return incarnations[i].Name < incarnations[j].Name
	})
	for i := range incarnations {
		inc := &incarnations[i]
		inc.Roles = roles[inc.Incarnation]
		if inc.Roles == nil {
			inc.Roles = []string{}
		}
		inc.Status = inspectIncarnationStatus(inc.Roles, inc.LeaseState)
		inc.Usable = inc.Status == inspectStatusActive
		for j := range inc.Opens {
			inc.Opens[j].Active = inc.Usable
		}
		// Every record under one hash holds the same identity, so the first
		// readable one in name order will do.
		if identity.Identity == "" && identity.IdentityHex == "" && inc.Record != nil {
			identity.Identity = inc.Record.Identity
			identity.IdentityHex = inc.Record.IdentityHex
		}
	}
	identity.Incarnations = incarnations
	if identity.Incarnations == nil {
		identity.Incarnations = []inspectIncarnation{}
	}
	return identity, nil
}

func inspectIncarnationStatus(roles []string, leaseState string) string {
	hasRole := func(want string) bool { return slices.Contains(roles, want) }
	if hasRole("confirmed") {
		switch leaseState {
		case "live":
			return inspectStatusActive
		case "expired":
			return inspectStatusExpired
		default:
			return inspectStatusUnleased
		}
	}
	if hasRole("pending") {
		return inspectStatusPending
	}
	if hasRole("reboot-target") {
		return inspectStatusReplaced
	}
	return inspectStatusUnreachable
}

// summarize counts the lifecycle states and every problem in the report. A
// filtered report only inspects the selected clients, so its problem count
// covers those clients and the staging directory, not the whole store.
func (r *inspectReport) summarize() {
	summary := inspectSummary{Identities: len(r.Identities)}
	summary.Problems = len(r.Problems) + len(r.Quarantined)
	countPointer := func(p *inspectPointer) {
		if p != nil && (p.Dangling || p.Problem != "") {
			summary.Problems++
		}
	}
	countStaging := func(entry *inspectStagingEntry) {
		if entry != nil {
			summary.Problems += len(entry.Problems)
		}
	}
	for _, entry := range r.Staging {
		countStaging(entry)
	}
	for _, identity := range r.Identities {
		summary.Problems += len(identity.Problems)
		countPointer(identity.Confirmed)
		countPointer(identity.Pending)
		for _, inc := range identity.Incarnations {
			summary.Problems += len(inc.Problems) + len(inc.Unknown)
			countPointer(inc.Reboot)
			for _, entry := range inc.Staging {
				countStaging(entry)
			}
			summary.Incarnations++
			switch inc.Status {
			case inspectStatusActive:
				summary.Active++
			case inspectStatusExpired:
				summary.Expired++
			case inspectStatusUnleased:
				summary.Unleased++
			case inspectStatusPending:
				summary.Pending++
			case inspectStatusReplaced:
				summary.Replaced++
			case inspectStatusUnreachable:
				summary.Unreachable++
			}
			summary.OpenMarkers += len(inc.Opens)
			for _, open := range inc.Opens {
				if !open.Active {
					summary.StaleOpens++
				}
				countStaging(open.Staging)
			}
		}
	}
	r.Summary = summary
}

func (cs *Store) inspectPointer(
	dirID vfs.InodeID,
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
	p.TargetID = incarnationText(uint64(id))
	return p, nil
}

func (cs *Store) inspectTemp(
	entry vfs.DirEntry,
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
	temp.Collectable = !info.Mtime.After(now.Add(-LeaseTime))
	return temp, nil
}

func (cs *Store) inspectIncarnation(
	dir vfs.DirEntry,
	staging []*inspectStagingEntry,
	now time.Time,
) (inspectIncarnation, error) {
	inc := inspectIncarnation{
		Name:        dir.Name,
		Incarnation: incarnationText(uint64(dir.ID)),
		LeaseState:  "none",
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
	leaseFound := false
	leaseLive := false
	for _, entry := range entries {
		switch {
		case entry.Name == ClientRecordName || entry.Name == updateName:
			record, err := cs.inspectRecord(entry)
			if err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
			}
			if entry.Name == ClientRecordName {
				inc.Record = record
			} else {
				inc.Update = record
			}
		case entry.Name == RebootName:
			inc.Reboot, err = cs.inspectPointer(dir.ID, RebootName, true)
			if err != nil {
				return inc, err
			}
		case entry.Name == ExpiredName:
			inc.ExpiredMarker = true
		case entry.Name == GCCandidateName:
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
			slot, ok, err := cs.inspectSlot(entry, LeasePrefix, now)
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
		case isConfirmingName(entry.Name):
			slot, ok, err := cs.inspectSlot(entry, confirmingPrefix, now)
			if err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
			}
			if ok {
				inc.Confirming = append(inc.Confirming, slot)
			}
		case IsActiveOpenName(entry.Name):
			// The marker suffix is the raw stateid hex. One that does not
			// decode is kept by name under unknown, not given a made-up id.
			raw, err := hex.DecodeString(strings.TrimPrefix(entry.Name, activeOpenPrefix))
			if err != nil || len(raw) != len(nfs4.StateID{}) {
				inc.Unknown = append(inc.Unknown, entry.Name)
				continue
			}
			inc.Opens = append(inc.Opens, inspectOpenMarker(nfs4.StateID(raw), inc.Incarnation, staging))
		case strings.HasPrefix(entry.Name, TempPrefix):
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
		if entry.Incarnation == inc.Incarnation && !entry.openMatched {
			inc.Staging = append(inc.Staging, entry)
		}
	}
	sort.Slice(inc.Opens, func(i, j int) bool {
		return inc.Opens[i].StateID < inc.Opens[j].StateID
	})
	return inc, nil
}

// inspectRecord decodes a client or update record. An unreadable record is a
// defect in the store, so the error is returned for the caller to report.
func (cs *Store) inspectRecord(entry vfs.DirEntry) (*inspectRecord, error) {
	var record durableClientRecord
	if err := cs.readJSONFile(entry.ID, entry.Name, &record); err != nil {
		return nil, err
	}
	out := &inspectRecord{
		Verifier:  hex.EncodeToString(record.Verifier),
		Confirm:   hex.EncodeToString(record.Confirm),
		Principal: formatPrincipal(record.principal()),
	}
	out.Identity, out.IdentityHex = inspectOpaque(record.ID)
	return out, nil
}

func (cs *Store) inspectSlot(
	entry vfs.DirEntry,
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
			now.UnixNano()-LeaseTime.Nanoseconds(),
	}, true, nil
}

func inspectOpenMarker(
	stateID nfs4.StateID,
	incarnation string,
	staging []*inspectStagingEntry,
) inspectOpen {
	open := inspectOpen{StateID: stateidText(stateID), Epoch: hex.EncodeToString(stateID[:4])}
	for _, entry := range staging {
		if !entry.openMatched && entry.StateID == open.StateID &&
			(entry.Incarnation == "" || entry.Incarnation == incarnation) {
			entry.openMatched = true
			open.Staging = entry
			break
		}
	}
	return open
}

// readStaging loads local staging data and sidecar files without modifying
// them, fills in the report's staging summary, quarantine list and top-level
// problems, and returns the entries for joining to open markers. It reports
// incomplete pairs because nfsd recovery starts from .staging files. The
// quarantine directory holds the checkpoints recovery moved aside because
// their checkpoint is invalid, guarded, or both unlinked and retired.
func (r *inspectReport) readStaging(dir string) ([]*inspectStagingEntry, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("staging directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("staging path %q is not a directory", dir)
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read staging directory %q: %w", dir, err)
	}
	summary := &inspectStagingSummary{}
	r.StagingSummary = summary
	problem := func(format string, args ...any) {
		r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
	}

	byID := make(map[uint64]*inspectStagingEntry)
	entryFor := func(id uint64) *inspectStagingEntry {
		entry := byID[id]
		if entry == nil {
			entry = &inspectStagingEntry{FileID: inodeText(vfs.InodeID(id))}
			byID[id] = entry
		}
		return entry
	}
	// nfsd names its files with %016x, and its recovery scan looks up the
	// sidecar of a .staging file under exactly that name. Accepting a shorter
	// spelling here would report a pair as complete which nfsd would in fact
	// quarantine.
	parseName := func(name, suffix string) (uint64, bool) {
		base := strings.TrimSuffix(name, suffix)
		if len(base) != 16 {
			return 0, false
		}
		id, err := strconv.ParseUint(base, 16, 64)
		return id, err == nil
	}

	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		path := filepath.Join(dir, name)
		if dirEntry.IsDir() {
			if name == staging.QuarantineDirName {
				found, err := readStagingQuarantine(dir, path)
				if err != nil {
					problem("%v", err)
					continue
				}
				r.Quarantined = append(r.Quarantined, found...)
				continue
			}
			if name == staging.AliasesDirName {
				continue // The server's publication alias log.
			}
			if name == staging.SessionsDirName {
				continue // The server's session files.
			}
			problem("%s: unexpected directory", path)
			continue
		}
		if strings.HasPrefix(name, ".") &&
			strings.Contains(name, ".meta.tmp-") {
			problem("%s: temporary metadata file present", path)
			continue
		}
		switch {
		case strings.HasSuffix(name, ".staging"):
			id, ok := parseName(name, ".staging")
			if !ok {
				problem("%s: unexpected staging file name", path)
				continue
			}
			entry := entryFor(id)
			entry.DataPresent = true
			summary.DataFiles++
			fileInfo, err := dirEntry.Info()
			if err != nil {
				entry.Problems = append(entry.Problems,
					"cannot stat staging data: "+err.Error())
				continue
			}
			entry.Size = fileInfo.Size()
			entry.DataMtime = inspectTime(fileInfo.ModTime())
			if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
				entry.AllocatedBytes = stat.Blocks * 512
			}
			summary.DataBytes += entry.Size
			summary.AllocatedBytes += entry.AllocatedBytes

		case strings.HasSuffix(name, ".meta"):
			id, ok := parseName(name, ".meta")
			if !ok {
				problem("%s: unexpected sidecar name", path)
				continue
			}
			entry := entryFor(id)
			entry.MetaPresent = true
			summary.Sidecars++
			if fileInfo, err := dirEntry.Info(); err != nil {
				entry.Problems = append(entry.Problems,
					"cannot stat metadata sidecar: "+err.Error())
			} else {
				entry.CheckpointTime = inspectTime(fileInfo.ModTime())
			}
			meta, err := staging.LoadMeta(path)
			if err != nil {
				entry.Problems = append(entry.Problems,
					"cannot read metadata sidecar: "+err.Error())
				continue
			}
			entry.setMeta(meta)

		default:
			problem("%s: unexpected staging directory entry", path)
		}
	}

	ids := make([]uint64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	result := make([]*inspectStagingEntry, 0, len(ids))
	for _, id := range ids {
		entry := byID[id]
		switch {
		case !entry.DataPresent:
			entry.Problems = append(entry.Problems,
				"staging data file is missing")
		case !entry.MetaPresent:
			entry.Problems = append(entry.Problems,
				"metadata sidecar is missing")
		}
		if entry.DataPresent && entry.MetaPresent && entry.metaValid {
			summary.Complete++
		}
		if entry.Retired {
			summary.Retired++
		}
		result = append(result, entry)
	}
	summary.Entries = len(result)
	slices.Sort(r.Quarantined)
	return result, nil
}

// readStagingQuarantine lists the files nfsd moved aside, as paths relative to
// the staging directory.
func readStagingQuarantine(stagingDir string, dir string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(stagingDir, path)
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read quarantine directory %q: %w", dir, err)
	}
	return found, nil
}

func (entry *inspectStagingEntry) setMeta(meta staging.Meta) {
	entry.metaValid = true
	entry.SidecarVersion = inspectSidecarVersion(meta.Version)
	entry.DirID = inodeText(meta.DirID)
	entry.FileName = meta.FileName
	entry.Cookie = hex.EncodeToString(meta.TernCookie[:])
	if meta.RecoveryKey != ([32]byte{}) {
		entry.RecoveryKey = hex.EncodeToString(meta.RecoveryKey[:])
	}
	entry.StateID = stateidText(meta.NFSStateID)
	entry.clientID = vfs.InodeID(meta.ClientID)
	entry.Incarnation = incarnationText(meta.ClientID)
	entry.OpenOwner, entry.OpenOwnerHex = inspectOpaque([]byte(meta.OpenOwner))
	entry.OwnerKnown = meta.OwnerKnown
	entry.ReadOnly = meta.ReadOnly
	entry.Retired = meta.Retired
	entry.Unlinked = meta.Unlinked
	entry.Guarded = meta.Guarded
	entry.Exclusive = meta.Exclusive
	if meta.Exclusive {
		entry.Verifier = hex.EncodeToString(meta.Verifier[:])
	}
	entry.BaseID = inodeText(meta.BaseID)
	entry.BaseSize = meta.BaseSize
	entry.CheckpointSize = meta.Size
	for _, dirty := range meta.Dirty {
		entry.Dirty = append(entry.Dirty, inspectByteRange{
			Start: dirty.Start,
			End:   dirty.End,
		})
	}
	entry.MetadataChanged = meta.MetadataChanged
	entry.Change = meta.Attrs.Change
	entry.Mtime = inspectTime(meta.Attrs.Mtime)
	entry.Atime = inspectTime(meta.Attrs.Atime)
	entry.Ctime = inspectTime(meta.Attrs.Ctime)
}

// inspectSidecarVersion names the on-disk metadata format. This is the
// staging sidecar layout version, not an NFS protocol version.
func inspectSidecarVersion(version uint8) string {
	return fmt.Sprintf("v%d", version)
}

func inspectTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// inspectOpaque renders client-supplied opaque bytes such as a EXCHANGE_ID
// identity or an open owner. Clients send printable text in practice but
// nothing requires it, so anything else is reported as hex rather than
// mangled into replacement runes by the JSON encoder. The returned text is
// the raw value, so it can be passed straight back as the identity selector.
func inspectOpaque(value []byte) (text string, hexText string) {
	if len(value) == 0 {
		return "", ""
	}
	text = string(value)
	if !utf8.ValidString(text) || strings.ContainsFunc(text, func(r rune) bool {
		return !unicode.IsPrint(r)
	}) {
		return "", hex.EncodeToString(value)
	}
	return text, ""
}

// formatPrincipal decodes the normalized principal stored by
// rpcRequest.principal. AUTH_SYS bodies hold uid, gid and auxiliary gids.
func formatPrincipal(p RPCPrincipal) string {
	body := []byte(p.Body)
	switch p.Flavor {
	case nfs4.AUTH_NONE:
		return "AUTH_NONE"
	case nfs4.AUTH_SYS:
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
	return fmt.Sprintf("flavor %d %s", p.Flavor, hex.EncodeToString(body))
}
