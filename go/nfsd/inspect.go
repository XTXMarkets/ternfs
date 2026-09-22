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
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

// This file implements a read-only inspector over the persistent client
// store described in README.md. It reads the same records with the same
// rules as the nfsd fleet, but never writes: it does not create the expired
// marker, remove collectable slots or age GC candidates.

// inspectOptions selects which part of the client store to report. Each
// field is set by the command line flag of the same name, and the report
// labels the values it prints with those same names.
type inspectOptions struct {
	// Identity is the raw SETCLIENTID identity (-identity). Its SHA-256 names
	// the identity directory.
	Identity []byte
	// IdentityHash is the hexadecimal identity directory name
	// (-identity-hash).
	IdentityHash string
	// ClientID is an incarnation directory inode (-clientid).
	ClientID uint64
	// StateID limits the report to the incarnation which owns the marker
	// (-stateid).
	StateID *StateID
	// StagingDir is the local staging directory of this nfsd host
	// (-staging). Its sidecars are joined to open markers.
	StagingDir string
}

// filtered reports whether the options narrow the report to one client. A
// filtered report cannot say whether a staging sidecar belongs to no client
// at all or merely to a client the filter excluded.
func (o inspectOptions) filtered() bool {
	return len(o.Identity) != 0 || o.IdentityHash != "" ||
		o.ClientID != 0 || o.StateID != nil
}

// inspectFilter echoes the active filter back into the report so that JSON
// output is self-describing. Its keys are the command line flag names.
type inspectFilter struct {
	Identity     string `json:"identity,omitempty"`
	IdentityHex  string `json:"identity_hex,omitempty"`
	IdentityHash string `json:"identity_hash,omitempty"`
	ClientID     string `json:"clientid,omitempty"`
	StateID      string `json:"stateid,omitempty"`
}

func (o inspectOptions) filter() *inspectFilter {
	if !o.filtered() {
		return nil
	}
	f := &inspectFilter{IdentityHash: o.IdentityHash}
	f.Identity, f.IdentityHex = inspectOpaque(o.Identity)
	if o.ClientID != 0 {
		f.ClientID = inodeHex(o.ClientID)
	}
	if o.StateID != nil {
		f.StateID = hex.EncodeToString(o.StateID[:])
	}
	return f
}

type inspectReport struct {
	Now          time.Time    `json:"now"`
	LeaseTime    string       `json:"lease_time"`
	ClientsDirID inspectInode `json:"clients_dir_id"`
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
	// Problems are defects in the store or the staging directory. They set the
	// exit status; see inspectExitProblems.
	Problems []string `json:"problems,omitempty"`
	// Notes explain an empty or narrowed report, such as a filter which
	// selected a client that is not in the store. A note is an answer to the
	// question asked, not a defect, so notes do not affect the exit status.
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

// inspectIdentity names its fields after the flags which select it:
// IdentityHash is -identity-hash and Identity is -identity. DirectoryID is
// the identity directory's own inode, which is not a clientid.
type inspectIdentity struct {
	IdentityHash string       `json:"identity_hash"`
	DirectoryID  inspectInode `json:"directory_id"`
	Identity     string       `json:"identity,omitempty"`
	// IdentityHex carries the identity when it is not printable text.
	IdentityHex  string               `json:"identity_hex,omitempty"`
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
	// Status is ACTIVE, EXPIRED, UNLEASED, PENDING, REPLACED or
	// UNREACHABLE.
	Status string `json:"status"`
	// Usable reports whether stateids of this clientid are accepted: the
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
	Callback    string `json:"callback"`
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
	FileID         inspectInode `json:"file_id"`
	DataPresent    bool         `json:"data_present"`
	MetaPresent    bool         `json:"meta_present"`
	Size           int64        `json:"size"`
	AllocatedBytes int64        `json:"allocated_bytes"`
	DataMtime      *time.Time   `json:"data_mtime,omitempty"`
	CheckpointTime *time.Time   `json:"checkpoint_time,omitempty"`
	// SidecarVersion is the on-disk metadata format, "v4" or "legacy". It is
	// not an NFS protocol version.
	SidecarVersion string       `json:"sidecar_version,omitempty"`
	DirID          inspectInode `json:"dir_id"`
	FileName       string       `json:"file_name,omitempty"`
	Cookie         string       `json:"cookie,omitempty"`
	// RecoveryKey is the client identity, boot verifier and principal which
	// must match for a rebooted client to reclaim this data.
	RecoveryKey string       `json:"recovery_key,omitempty"`
	StateID     string       `json:"stateid,omitempty"`
	ClientID    inspectInode `json:"clientid"`
	OpenOwner   string       `json:"open_owner,omitempty"`
	// OpenOwnerHex carries the open owner when it is not printable text.
	OpenOwnerHex string `json:"open_owner_hex,omitempty"`
	OwnerKnown   bool   `json:"owner_known"`
	ReadOnly     bool   `json:"read_only"`
	// Retired marks data kept after the owning lease expired, so that the
	// client can reclaim it after a reboot.
	Retired         bool               `json:"retired"`
	BaseID          inspectInode       `json:"base_id"`
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
	return newClientStoreAt(fs, nfsID, clientsID)
}

// Inspect builds a report of the client store without modifying it. Every
// lease and collection decision is taken against a single instant so that the
// report is internally consistent.
func (cs *ClientStore) Inspect(opts inspectOptions) (*inspectReport, error) {
	now := cs.now()
	report := &inspectReport{
		Now:          now,
		LeaseTime:    nfsLeaseTime.String(),
		ClientsDirID: inspectInode(cs.dirID),
		Filter:       opts.filter(),
		Identities:   []inspectIdentity{},
		StagingDir:   opts.StagingDir,
	}

	var staging []*inspectStagingEntry
	if opts.StagingDir != "" {
		var err error
		if staging, err = report.readStaging(opts.StagingDir); err != nil {
			return nil, err
		}
	}

	identityIDs, err := cs.inspectSelectIdentities(opts, staging, report)
	if err != nil {
		return nil, err
	}
	for _, entry := range identityIDs {
		identity, err := cs.inspectIdentity(entry, staging, now)
		if err != nil {
			return nil, err
		}
		if opts.StateID != nil {
			identity.Incarnations = inspectKeepStateID(
				identity.Incarnations, hex.EncodeToString(opts.StateID[:]))
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
		return report.Identities[i].IdentityHash < report.Identities[j].IdentityHash
	})
	if opts.StateID != nil && len(report.Identities) == 0 {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"stateid %s has no open marker in the store",
			hex.EncodeToString(opts.StateID[:])))
	}
	// Only an unfiltered report can claim a sidecar belongs to no client: a
	// filtered one has not looked at the other identities. Count the rest
	// instead of listing another client's files under a misleading heading.
	for _, entry := range staging {
		if entry.reported {
			continue
		}
		if opts.filtered() {
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

// inspectSelectIdentities resolves the options to identity directories. A
// clientid or a stateid with a staging sidecar resolve through the parent
// directory; a stateid without one requires a scan of every identity.
func (cs *ClientStore) inspectSelectIdentities(
	opts inspectOptions,
	staging []*inspectStagingEntry,
	report *inspectReport,
) ([]DirEntry, error) {
	switch {
	case len(opts.Identity) != 0 || opts.IdentityHash != "":
		name := opts.IdentityHash
		if len(opts.Identity) != 0 {
			name = clientIdentityKey(opts.Identity)
		}
		id, err := cs.fs.Lookup(cs.dirID, name)
		if errors.Is(err, os.ErrNotExist) {
			report.Notes = append(report.Notes,
				fmt.Sprintf("identity-hash %s is not registered", name))
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []DirEntry{{Name: name, ID: id}}, nil
	case opts.ClientID != 0:
		return cs.inspectIdentityForClientID(InodeID(opts.ClientID), report)
	case opts.StateID != nil:
		// A sidecar names the clientid directly. Without one there is nothing
		// to resolve against, so fall through and scan every identity for an
		// open marker with this stateid.
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

// inspectIdentityForClientID resolves a clientid to its identity directory.
// Anything that goes wrong here is about the argument, not the store, so it
// is reported as a note rather than a problem. The most likely mistake is
// pasting the identity directory inode, which the report prints next to the
// clientid, so that case gets its own message.
func (cs *ClientStore) inspectIdentityForClientID(
	clientID InodeID,
	report *inspectReport,
) ([]DirEntry, error) {
	note := func(format string, args ...any) ([]DirEntry, error) {
		report.Notes = append(report.Notes, fmt.Sprintf(format, args...))
		return nil, nil
	}
	identityID, err := cs.fs.LookupParent(clientID)
	if errors.Is(err, os.ErrNotExist) {
		return note("clientid %s has no incarnation directory; "+
			"it is stale or collected", inodeHex(clientID))
	}
	if err != nil {
		return nil, err
	}
	if identityID == cs.dirID {
		return note("%s is an identity directory, not a clientid; "+
			"select it with -identity-hash", inodeHex(clientID))
	}
	name, err := cs.inspectChildName(cs.dirID, identityID)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return note("clientid %s is not under /%s/clients",
			inodeHex(clientID), nfsDirName)
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
	staging []*inspectStagingEntry,
	now time.Time,
) (inspectIdentity, error) {
	identity := inspectIdentity{
		IdentityHash: dir.Name,
		DirectoryID:  inspectInode(dir.ID),
	}
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
		if _, primary := roles[InodeID(inc.ClientID)]; !primary {
			continue
		}
		if inc.Reboot != nil && inc.Reboot.TargetID != 0 {
			targetID := InodeID(inc.Reboot.TargetID)
			roles[targetID] = append(roles[targetID], "reboot-target")
		}
	}
	sort.Slice(incarnations, func(i, j int) bool {
		return incarnations[i].Name < incarnations[j].Name
	})
	for i := range incarnations {
		inc := &incarnations[i]
		inc.Roles = roles[InodeID(inc.ClientID)]
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
	dir DirEntry,
	staging []*inspectStagingEntry,
	now time.Time,
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
	leaseFound := false
	leaseLive := false
	for _, entry := range entries {
		switch {
		case entry.Name == clientRecordName || entry.Name == updateName:
			record, err := cs.inspectRecord(entry)
			if err != nil {
				inc.Problems = append(inc.Problems, err.Error())
				continue
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
		case isConfirmingName(entry.Name):
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
				strings.TrimPrefix(entry.Name, activeOpenPrefix),
				inspectInode(dir.ID), staging))
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
		if entry.ClientID == inc.ClientID && !entry.openMatched {
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
func (cs *ClientStore) inspectRecord(entry DirEntry) (*inspectRecord, error) {
	var record durableClientRecord
	if err := cs.readJSONFile(entry.ID, entry.Name, &record); err != nil {
		return nil, err
	}
	out := &inspectRecord{
		Verifier:  hex.EncodeToString(record.Verifier),
		Confirm:   hex.EncodeToString(record.Confirm),
		Principal: formatPrincipal(record.principal()),
		Callback:  formatCallback(record.NetID, record.Addr),
	}
	out.Identity, out.IdentityHex = inspectOpaque(record.ID)
	return out, nil
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

func inspectOpenMarker(
	stateID string,
	clientID inspectInode,
	staging []*inspectStagingEntry,
) inspectOpen {
	open := inspectOpen{StateID: stateID}
	if len(stateID) >= 8 {
		open.Epoch = stateID[:8]
	}
	for _, entry := range staging {
		if !entry.openMatched && entry.StateID == stateID &&
			(entry.ClientID == 0 || entry.ClientID == clientID) {
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
// their sidecar would not decode.
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
			entry = &inspectStagingEntry{FileID: inspectInode(id)}
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
			if name == stagingQuarantineDirName {
				found, err := readStagingQuarantine(dir, path)
				if err != nil {
					problem("%v", err)
					continue
				}
				r.Quarantined = append(r.Quarantined, found...)
				continue
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
			meta, err := loadStagingMeta(path)
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

func (entry *inspectStagingEntry) setMeta(meta StagingMeta) {
	entry.metaValid = true
	entry.SidecarVersion = inspectSidecarVersion(meta.version)
	entry.DirID = inspectInode(meta.DirID)
	entry.FileName = meta.FileName
	entry.Cookie = hex.EncodeToString(meta.TernCookie[:])
	if meta.RecoveryKey != ([32]byte{}) {
		entry.RecoveryKey = hex.EncodeToString(meta.RecoveryKey[:])
	}
	entry.StateID = hex.EncodeToString(meta.NFSStateID[:])
	entry.ClientID = inspectInode(meta.ClientID)
	entry.OpenOwner, entry.OpenOwnerHex = inspectOpaque([]byte(meta.OpenOwner))
	entry.OwnerKnown = meta.OwnerKnown
	entry.ReadOnly = meta.ReadOnly
	entry.Retired = meta.Retired
	entry.BaseID = inspectInode(meta.BaseID)
	entry.BaseSize = meta.BaseSize
	entry.CheckpointSize = meta.Size
	for _, dirty := range meta.Dirty {
		entry.Dirty = append(entry.Dirty, inspectByteRange{
			Start: dirty.start,
			End:   dirty.end,
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
	if version == 0 {
		return "legacy"
	}
	return fmt.Sprintf("v%d", version)
}

func inspectTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// inspectOpaque renders client-supplied opaque bytes such as a SETCLIENTID
// identity or an open owner. Clients send printable text in practice but
// nothing requires it, so anything else is reported as hex rather than
// mangled into replacement runes by the JSON encoder. The returned text is
// the raw value, so it can be passed straight back to -identity.
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
	if (netid != "tcp" && netid != "udp") || len(parts) != 6 {
		return netid + " " + addr
	}
	var octets [6]int
	for i, part := range parts {
		// strconv rejects trailing junk, which Sscanf would silently accept.
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || value > 255 {
			return netid + " " + addr
		}
		octets[i] = value
	}
	return fmt.Sprintf("%s %d.%d.%d.%d:%d", netid,
		octets[0], octets[1], octets[2], octets[3],
		octets[4]<<8|octets[5])
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
	fmt.Fprintf(w,
		"summary identities %d incarnations %d: ACTIVE %d EXPIRED %d "+
			"UNLEASED %d PENDING %d REPLACED %d UNREACHABLE %d; "+
			"open markers %d, stale %d; problems %d\n",
		r.Summary.Identities, r.Summary.Incarnations,
		r.Summary.Active, r.Summary.Expired, r.Summary.Unleased,
		r.Summary.Pending, r.Summary.Replaced, r.Summary.Unreachable,
		r.Summary.OpenMarkers, r.Summary.StaleOpens, r.Summary.Problems)
	if summary := r.StagingSummary; summary != nil {
		fmt.Fprintf(w,
			"staging %q: entries %d complete %d retired %d data files %d "+
				"sidecars %d, %d bytes (%d allocated)\n",
			r.StagingDir, summary.Entries, summary.Complete, summary.Retired,
			summary.DataFiles, summary.Sidecars, summary.DataBytes,
			summary.AllocatedBytes)
		if summary.OtherClients > 0 {
			fmt.Fprintf(w,
				"staging: %d sidecars belong to clients outside this filter; "+
					"rerun without a filter to list them\n",
				summary.OtherClients)
		}
	}
	for _, problem := range r.Problems {
		fmt.Fprintf(w, "problem: %s\n", problem)
	}
	for _, note := range r.Notes {
		fmt.Fprintf(w, "note: %s\n", note)
	}
	if len(r.Identities) == 0 {
		fmt.Fprintf(w, "no client identities\n")
	}
	// Every label below is spelled like the flag which selects that value, so
	// that anything in the report can be pasted back onto the command line.
	for _, identity := range r.Identities {
		fmt.Fprintf(w, "\nidentity-hash %s\n", identity.IdentityHash)
		fmt.Fprintf(w, "  %-9s %s\n", "directory", inodeHex(identity.DirectoryID))
		if identity.Identity != "" {
			fmt.Fprintf(w, "  %-9s %q\n", "identity", identity.Identity)
		}
		if identity.IdentityHex != "" {
			fmt.Fprintf(w, "  %-9s %s (not printable)\n",
				"identity", identity.IdentityHex)
		}
		fmt.Fprintf(w, "  %-9s %s\n", "confirmed", pointerText(identity.Confirmed))
		fmt.Fprintf(w, "  %-9s %s\n", "pending", pointerText(identity.Pending))
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
			fmt.Fprintf(w, "\n  incarnation %s clientid %s [%s, lease %s] %s\n",
				inc.Name, inodeHex(inc.ClientID), roles, inc.LeaseState,
				inc.Status)
			writeRecord := func(name string, rec *inspectRecord) {
				if rec == nil {
					return
				}
				fmt.Fprintf(w, "    %-10s", name)
				if rec.Identity != "" {
					fmt.Fprintf(w, " identity %q", rec.Identity)
				}
				if rec.IdentityHex != "" {
					fmt.Fprintf(w, " identity %s (not printable)", rec.IdentityHex)
				}
				fmt.Fprintf(w, " verifier %s confirm %s\n", rec.Verifier, rec.Confirm)
				fmt.Fprintf(w, "    %-10s principal %s callback %s\n", "",
					rec.Principal, rec.Callback)
			}
			writeRecord("client", inc.Record)
			writeRecord("update", inc.Update)
			if inc.Reboot != nil {
				fmt.Fprintf(w, "    reboot     %s; purge in progress\n",
					pointerText(inc.Reboot))
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
				state := "STALE"
				if open.Active {
					state = inspectStatusActive
				}
				fmt.Fprintf(w, "    open       stateid %s epoch %s %s\n",
					open.StateID, open.Epoch, state)
				if open.Staging != nil {
					writeInspectStagingEntry(w, "      ", open.Staging, rel)
				}
			}
			for _, entry := range inc.Staging {
				fmt.Fprintf(w, "    staging    NO OPEN MARKER\n")
				writeInspectStagingEntry(w, "      ", entry, rel)
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
		fmt.Fprintf(w, "\nstaging files with no client in the store\n")
		for _, entry := range r.Staging {
			writeInspectStagingEntry(w, "  ", entry, rel)
		}
	}
	if len(r.Quarantined) > 0 {
		fmt.Fprintf(w,
			"\nquarantined checkpoints (sidecar would not decode; "+
				"nfsd will not recover these)\n")
		for _, name := range r.Quarantined {
			fmt.Fprintf(w, "  %s\n", name)
		}
	}
	// Problems are scattered through a long report, so repeat the count where
	// it will be read: at the end.
	if r.Summary.Problems > 0 {
		fmt.Fprintf(w, "\nproblems found: %d\n", r.Summary.Problems)
	}
}

// pointerText renders a confirmed, pending or reboot pointer. The clientid
// comes first because that is what the reader pastes into -clientid; the
// symlink target is kept for matching against a directory listing.
func pointerText(p *inspectPointer) string {
	if p == nil {
		return "(none)"
	}
	var s string
	if p.Dangling {
		s = "-> " + p.Target + " DANGLING"
	} else {
		s = fmt.Sprintf("clientid %s (%s)", inodeHex(p.TargetID), p.Target)
	}
	if p.Problem != "" {
		s += " (" + p.Problem + ")"
	}
	return s
}

func writeInspectStagingEntry(
	w io.Writer,
	indent string,
	entry *inspectStagingEntry,
	rel func(time.Time) string,
) {
	fmt.Fprintf(w, "%sfile %s", indent, inodeHex(entry.FileID))
	if entry.DataPresent {
		fmt.Fprintf(w, " data %d bytes (%d allocated)",
			entry.Size, entry.AllocatedBytes)
		if entry.DataMtime != nil {
			fmt.Fprintf(w, " modified %s", rel(*entry.DataMtime))
		}
	} else {
		fmt.Fprint(w, " data MISSING")
	}
	switch {
	case !entry.MetaPresent:
		fmt.Fprint(w, " sidecar MISSING")
	case entry.SidecarVersion == "":
		fmt.Fprint(w, " sidecar UNREADABLE")
	default:
		fmt.Fprintf(w, " sidecar %s", entry.SidecarVersion)
	}
	fmt.Fprintln(w)

	if entry.metaValid {
		fmt.Fprintf(w, "%starget dir %s name %q; clientid %s stateid %s\n",
			indent, inodeHex(entry.DirID), entry.FileName,
			inodeHex(entry.ClientID), entry.StateID)
		access := "write"
		if entry.ReadOnly {
			access = "read-only"
		}
		owner := "(not recorded)"
		switch {
		case entry.OpenOwnerHex != "":
			owner = entry.OpenOwnerHex + " (not printable)"
		case entry.OwnerKnown:
			owner = fmt.Sprintf("%q", entry.OpenOwner)
		}
		fmt.Fprintf(w, "%sowner %s access %s", indent, owner, access)
		if entry.Retired {
			fmt.Fprint(w, " RETIRED (lease expired, held for reclaim)")
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w,
			"%sbase %s size %d; checkpoint size %d dirty %s\n",
			indent, inodeHex(entry.BaseID), entry.BaseSize,
			entry.CheckpointSize, formatInspectRanges(entry.Dirty))
		if entry.Change != 0 {
			fmt.Fprintf(w, "%sattrs change %d mtime %s atime %s ctime %s",
				indent, entry.Change, inspectTimeText(entry.Mtime),
				inspectTimeText(entry.Atime), inspectTimeText(entry.Ctime))
		} else {
			fmt.Fprintf(w, "%sattrs (no change counter recorded)", indent)
		}
		if entry.MetadataChanged {
			fmt.Fprint(w, " METADATA CHANGED")
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%scookie %s", indent, entry.Cookie)
		if entry.RecoveryKey != "" {
			fmt.Fprintf(w, " recovery-key %s", entry.RecoveryKey)
		}
		if entry.CheckpointTime != nil {
			fmt.Fprintf(w, "; checkpoint %s", rel(*entry.CheckpointTime))
		}
		fmt.Fprintln(w)
	}
	for _, problem := range entry.Problems {
		fmt.Fprintf(w, "%sproblem: %s\n", indent, problem)
	}
}

func formatInspectRanges(ranges []inspectByteRange) string {
	if len(ranges) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		parts = append(parts, fmt.Sprintf("[%d,%d)", r.Start, r.End))
	}
	return strings.Join(parts, ",")
}

func inspectTimeText(value *time.Time) string {
	if value == nil {
		return "(not recorded)"
	}
	return value.Format(time.RFC3339Nano)
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
