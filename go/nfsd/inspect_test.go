// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func authSysBody(uid uint32, gid uint32, groups ...uint32) string {
	body := binary.BigEndian.AppendUint32(nil, uid)
	body = binary.BigEndian.AppendUint32(body, gid)
	body = binary.BigEndian.AppendUint32(body, uint32(len(groups)))
	for _, g := range groups {
		body = binary.BigEndian.AppendUint32(body, g)
	}
	return string(body)
}

// inspectFixture holds a client store with two nfsd processes and three
// client identities in different states.
type inspectFixture struct {
	fs         TernVFS
	first      *ClientStore
	second     *ClientStore
	now        time.Time
	identity   []byte
	clientID   uint64
	replacedID uint64
	stateID    StateID
	pendingID  uint64
	expiredID  uint64
	stagingDir string
}

func newInspectFixture(t *testing.T) *inspectFixture {
	t.Helper()
	f := &inspectFixture{
		fs:       NewLocalTernVFS(t.TempDir()),
		now:      time.Unix(1_700_000_000, 0),
		identity: []byte("Linux NFSv4.0 host-a/10.0.0.1"),
	}
	var err error
	f.first, err = NewClientStore(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	f.second, err = NewClientStore(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	f.first.now = func() time.Time { return f.now }
	f.second.now = func() time.Time { return f.now }

	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: authSysBody(1000, 100, 100, 4)},
		netid:     "tcp",
		addr:      "10.0.0.1.8.1",
	}

	// Identity C: confirmed with a lease which expires before the report is
	// taken. Nothing observes the expiry, so there is no expired marker.
	f.expiredID = f.register(t, f.second, []byte("expired-client"), [8]byte{4}, owner)
	if err := f.second.Renew(f.expiredID); err != nil {
		t.Fatal(err)
	}

	// Identity A: a confirmed incarnation which replaced an earlier one, with
	// one open and lease slots from both nfsd processes.
	f.replacedID = f.register(t, f.first, f.identity, [8]byte{1}, owner)
	f.clientID = f.register(t, f.first, f.identity, [8]byte{2}, owner)
	if f.clientID == f.replacedID {
		t.Fatal("new verifier did not create a new incarnation")
	}
	f.stateID = StateID{0xab, 0xcd, 0xef, 0x01, 0, 0, 0, 0, 0, 0, 0, 7}
	if err := f.first.MarkOpen(f.clientID, f.stateID); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(50 * time.Second)
	if err := f.second.Renew(f.clientID); err != nil {
		t.Fatal(err)
	}

	// Identity B: registered but never confirmed.
	f.pendingID, _, err = f.first.SetClientID(
		[8]byte{3}, []byte("pending-client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	// The first slot of identity A and the only slot of identity C are now
	// expired; the second slot of identity A is live.
	f.now = f.now.Add(50 * time.Second)

	f.stagingDir = t.TempDir()
	if err := saveStagingMeta(filepath.Join(f.stagingDir, "0000000000003039.meta"), StagingMeta{
		DirID:      f.fs.RootID(),
		FileName:   "upload.bin",
		NFSStateID: f.stateID,
		ClientID:   f.clientID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.stagingDir, "0000000000003039.staging"),
		bytes.Repeat([]byte{0}, 4096), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveStagingMeta(filepath.Join(f.stagingDir, "4000000000010932.meta"), StagingMeta{
		DirID:      f.fs.RootID(),
		FileName:   "orphan.bin",
		NFSStateID: StateID{9},
		ClientID:   f.clientID,
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *inspectFixture) register(
	t *testing.T,
	store *ClientStore,
	identity []byte,
	verifier [8]byte,
	owner clientOwner,
) uint64 {
	t.Helper()
	clientID, confirm, err := store.SetClientID(verifier, identity, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(clientID, confirm, owner.principal); err != nil {
		t.Fatal(err)
	}
	return clientID
}

func (f *inspectFixture) reader(t *testing.T) *ClientStore {
	t.Helper()
	store, err := openClientStoreReader(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return f.now }
	return store
}

func findIncarnation(
	t *testing.T,
	report *inspectReport,
	clientID uint64,
) (inspectIdentity, inspectIncarnation) {
	t.Helper()
	for _, identity := range report.Identities {
		for _, inc := range identity.Incarnations {
			if uint64(inc.ClientID) == clientID {
				return identity, inc
			}
		}
	}
	t.Fatalf("clientid %#x not in report", clientID)
	return inspectIdentity{}, inspectIncarnation{}
}

func TestInspectReportsClientState(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: f.stagingDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Identities) != 3 {
		t.Fatalf("identities = %d, want 3", len(report.Identities))
	}
	if len(report.Problems) != 0 {
		t.Fatalf("problems = %v", report.Problems)
	}

	identity, current := findIncarnation(t, report, f.clientID)
	if identity.Hash != clientIdentityKey(f.identity) {
		t.Fatalf("identity hash = %s", identity.Hash)
	}
	if string(identity.Identity) != "Linux NFSv4.0 host-a/10.0.0.1" {
		t.Fatalf("identity = %q", identity.Identity)
	}
	if identity.Confirmed == nil ||
		identity.Confirmed.TargetID != inspectInode(f.clientID) {
		t.Fatalf("confirmed pointer = %+v", identity.Confirmed)
	}
	if strings.Join(current.Roles, ",") != "confirmed,pending" {
		t.Fatalf("roles = %v", current.Roles)
	}
	if current.LeaseState != "live" || !current.Usable {
		t.Fatalf("lease state = %s usable = %v", current.LeaseState, current.Usable)
	}
	if len(current.Leases) != 2 {
		t.Fatalf("lease slots = %+v", current.Leases)
	}
	liveSlots := 0
	for _, slot := range current.Leases {
		if slot.Live {
			liveSlots++
			if slot.NfsdID != f.second.nfsdID {
				t.Fatalf("live slot belongs to %s, want %s", slot.NfsdID, f.second.nfsdID)
			}
		} else if slot.Collectable {
			t.Fatalf("slot expired 10s ago reported collectable: %+v", slot)
		}
	}
	if liveSlots != 1 {
		t.Fatalf("live slots = %d", liveSlots)
	}
	if current.Record == nil {
		t.Fatal("no client record")
	}
	if current.Record.Principal != "AUTH_SYS uid=1000 gid=100 groups=[100,4]" {
		t.Fatalf("principal = %s", current.Record.Principal)
	}
	if current.Record.Callback != "tcp 10.0.0.1:2049" {
		t.Fatalf("callback = %s", current.Record.Callback)
	}
	if current.Record.Verifier != hex.EncodeToString([]byte{2, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("verifier = %s", current.Record.Verifier)
	}
	if len(current.Opens) != 1 {
		t.Fatalf("opens = %+v", current.Opens)
	}
	open := current.Opens[0]
	if open.StateID != hex.EncodeToString(f.stateID[:]) || open.Epoch != "abcdef01" {
		t.Fatalf("open = %+v", open)
	}
	if open.Staging == nil || open.Staging.FileID != 0x3039 ||
		open.Staging.FileName != "upload.bin" || open.Staging.Size != 4096 {
		t.Fatalf("open staging = %+v", open.Staging)
	}
	if len(current.Staging) != 1 || current.Staging[0].FileName != "orphan.bin" {
		t.Fatalf("unmatched client staging = %+v", current.Staging)
	}
	if len(report.Staging) != 1 || report.Staging[0].FileName != "orphan.bin" {
		t.Fatalf("unmatched staging = %+v", report.Staging)
	}

	_, replaced := findIncarnation(t, report, f.replacedID)
	if len(replaced.Roles) != 0 || replaced.Usable {
		t.Fatalf("replaced incarnation = %+v", replaced)
	}

	pendingIdentity, pending := findIncarnation(t, report, f.pendingID)
	if pendingIdentity.Confirmed != nil {
		t.Fatalf("pending identity has confirmed pointer %+v", pendingIdentity.Confirmed)
	}
	if strings.Join(pending.Roles, ",") != "pending" || pending.LeaseState != "none" ||
		pending.Usable {
		t.Fatalf("pending incarnation = %+v", pending)
	}

	_, expired := findIncarnation(t, report, f.expiredID)
	if expired.LeaseState != "expired" || expired.Usable || expired.ExpiredMarker {
		t.Fatalf("expired incarnation = %+v", expired)
	}
}

func TestInspectDoesNotModifyStore(t *testing.T) {
	f := newInspectFixture(t)
	// Move far enough that every slot of the expired identity is collectable
	// and the replaced incarnation would be a GC candidate.
	f.now = f.now.Add(10 * nfsLeaseTime)

	before := listTree(t, f.fs, f.first.dirID, "")
	report, err := f.reader(t).Inspect(inspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	after := listTree(t, f.fs, f.first.dirID, "")
	if !equalStrings(before, after) {
		t.Fatalf("inspect modified the store:\nbefore %v\nafter  %v", before, after)
	}

	_, expired := findIncarnation(t, report, f.expiredID)
	if expired.ExpiredMarker {
		t.Fatal("inspect created the expired marker")
	}
	if len(expired.Leases) != 1 || !expired.Leases[0].Collectable {
		t.Fatalf("expired leases = %+v", expired.Leases)
	}
	if _, err := f.fs.Lookup(InodeID(f.expiredID), expiredName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired marker lookup = %v", err)
	}
	_, replaced := findIncarnation(t, report, f.replacedID)
	if replaced.GCAfter != nil {
		t.Fatal("inspect created a GC candidate")
	}
}

func TestInspectFilters(t *testing.T) {
	f := newInspectFixture(t)
	reader := f.reader(t)

	t.Run("clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{ClientID: f.clientID})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 2 {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	t.Run("stale clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{
			ClientID: uint64(MakeInodeID(InodeTypeDir, 0x123456)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Problems) != 1 {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("identity", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{Identity: f.identity})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			report.Identities[0].Hash != clientIdentityKey(f.identity) {
			t.Fatalf("report = %+v", report.Identities)
		}
		report, err = reader.Inspect(inspectOptions{Identity: []byte("nobody")})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Problems) != 1 {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("identity hash", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{
			IdentityHash: clientIdentityKey([]byte("pending-client")),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 1 {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	t.Run("stateid scan", func(t *testing.T) {
		sid := f.stateID
		report, err := reader.Inspect(inspectOptions{StateID: &sid})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 1 ||
			report.Identities[0].Incarnations[0].ClientID != inspectInode(f.clientID) {
			t.Fatalf("report = %+v", report.Identities)
		}
		unknown := StateID{1}
		report, err = reader.Inspect(inspectOptions{StateID: &unknown})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	t.Run("stateid via staging sidecar", func(t *testing.T) {
		sid := f.stateID
		report, err := reader.Inspect(inspectOptions{
			StateID: &sid, StagingDir: f.stagingDir,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			report.Identities[0].Incarnations[0].Opens[0].Staging == nil {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
}

func TestInspectOutput(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: f.stagingDir})
	if err != nil {
		t.Fatal(err)
	}

	var text bytes.Buffer
	report.WriteText(&text)
	for _, want := range []string{
		"id        \"Linux NFSv4.0 host-a/10.0.0.1\"",
		"[confirmed,pending, lease live] USABLE",
		"[unreachable, lease none] NOT USABLE",
		"[pending, lease none] NOT USABLE",
		"[confirmed,pending, lease expired] NOT USABLE",
		"open       stateid " + hex.EncodeToString(f.stateID[:]) + " epoch abcdef01 staging file 0x0000000000003039",
		"name \"upload.bin\" (4096 bytes)",
		"NO OPEN MARKER",
		"principal AUTH_SYS uid=1000 gid=100 groups=[100,4] callback tcp 10.0.0.1:2049",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, text.String())
		}
	}

	var out bytes.Buffer
	if err := report.WriteJSON(&out); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Identities []struct {
			Incarnations []struct {
				ClientID string `json:"clientid"`
				Usable   bool   `json:"usable"`
			} `json:"incarnations"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, identity := range decoded.Identities {
		for _, inc := range identity.Incarnations {
			if inc.ClientID == inodeHex(InodeID(f.clientID)) && inc.Usable {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("JSON lacks usable clientid %s:\n%s", inodeHex(InodeID(f.clientID)), out.String())
	}
	var identityJSON struct {
		Identities []struct {
			Identity string `json:"identity"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out.Bytes(), &identityJSON); err != nil {
		t.Fatal(err)
	}
	wantIdentity := base64.StdEncoding.EncodeToString(f.identity)
	for _, identity := range identityJSON.Identities {
		if identity.Identity == wantIdentity {
			return
		}
	}
	t.Fatalf("JSON lacks base64 identity %q:\n%s", wantIdentity, out.String())
}

func TestInspectJSONKeepsAnEmptyIdentityListAnArray(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspectOptions{
		ClientID: uint64(MakeInodeID(InodeTypeDir, 0x123456)),
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := report.WriteJSON(&out); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Identities []json.RawMessage `json:"identities"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Identities == nil {
		t.Fatalf("identities = null:\n%s", out.String())
	}
}

func TestInspectReportsMalformedStagingAndPointers(t *testing.T) {
	f := newInspectFixture(t)
	if err := saveStagingMeta(filepath.Join(f.stagingDir, "not-an-inode.meta"),
		StagingMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.stagingDir, "broken.meta"),
		[]byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	identityID, err := f.fs.LookupParent(InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	name, err := f.first.requireIncarnationName(identityID, InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.fs.Remove(identityID, confirmedName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(identityID, confirmedName, "../"+name); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: f.stagingDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Problems) != 2 {
		t.Fatalf("staging problems = %v", report.Problems)
	}
	for _, identity := range report.Identities {
		if identity.ID != inspectInode(identityID) {
			continue
		}
		if identity.Confirmed == nil || !identity.Confirmed.Dangling ||
			identity.Confirmed.Problem == "" {
			t.Fatalf("confirmed pointer = %+v", identity.Confirmed)
		}
		return
	}
	t.Fatalf("identity %s not found", inodeHex(identityID))
}

func TestInspectTextDescribesConfirmedClientWithoutLease(t *testing.T) {
	f := newInspectFixture(t)
	f.register(t, f.first, []byte("awaiting-renew"), [8]byte{5}, clientOwner{})
	report, err := f.reader(t).Inspect(inspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if !strings.Contains(text.String(), "AWAITING FIRST RENEW") {
		t.Fatalf("text report lacks awaiting-first-renew state:\n%s", text.String())
	}
}

func TestParseInspectOptions(t *testing.T) {
	hash := strings.Repeat("a", 64)
	tests := []struct {
		name                                      string
		identity, identityHash, clientID, stateID string
		wantErr                                   bool
	}{
		{name: "no filter"},
		{name: "identity", identity: "client"},
		{name: "identity hash", identityHash: hash},
		{name: "uppercase identity hash", identityHash: strings.ToUpper(hash)},
		{name: "clientid", clientID: "0x2000000000000001"},
		{name: "stateid", stateID: strings.Repeat("0", 24)},
		{name: "zero clientid", clientID: "0", wantErr: true},
		{name: "short identity hash", identityHash: "abcd", wantErr: true},
		{name: "invalid identity hash", identityHash: strings.Repeat("g", 64), wantErr: true},
		{name: "two filters", identity: "client", clientID: "1", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, err := parseInspectOptions(
				test.identity, test.identityHash, test.clientID, test.stateID)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseInspectOptions() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && test.identityHash != "" &&
				opts.IdentityHash != strings.ToLower(test.identityHash) {
				t.Fatalf("identity hash = %q", opts.IdentityHash)
			}
		})
	}
}

func TestInspectWithoutStore(t *testing.T) {
	if _, err := openClientStoreReader(NewLocalTernVFS(t.TempDir())); err == nil {
		t.Fatal("reader opened a filesystem without a client store")
	}
}

func TestFormatPrincipalAndCallback(t *testing.T) {
	if got := formatPrincipal(rpcPrincipal{flavor: authNone}); got != "AUTH_NONE" {
		t.Fatalf("AUTH_NONE = %s", got)
	}
	if got := formatPrincipal(rpcPrincipal{flavor: authSys, body: "ab"}); got != "AUTH_SYS 6162" {
		t.Fatalf("short AUTH_SYS = %s", got)
	}
	if got := formatPrincipal(rpcPrincipal{flavor: 6, body: "\x01"}); got != "flavor 6 01" {
		t.Fatalf("unknown flavor = %s", got)
	}
	if got := formatCallback("tcp", "192.168.1.20.8.1"); got != "tcp 192.168.1.20:2049" {
		t.Fatalf("callback = %s", got)
	}
	if got := formatCallback("tcp6", "::1.8.1"); got != "tcp6 ::1.8.1" {
		t.Fatalf("callback = %s", got)
	}
}

func listTree(t *testing.T, fs TernVFS, dirID InodeID, prefix string) []string {
	t.Helper()
	var result []string
	var cursor uint64
	for {
		entries, next, err := fs.Readdir(dirID, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := prefix + entry.Name
			result = append(result, name)
			if isDirectoryInodeID(entry.ID) {
				result = append(result, listTree(t, fs, entry.ID, name+"/")...)
			}
		}
		if next == 0 || next == cursor {
			return result
		}
		cursor = next
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
