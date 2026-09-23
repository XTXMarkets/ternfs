// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
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
	// identityDirID is the identity directory inode of clientID. It is not a
	// clientid, and the report must not label it as one.
	identityDirID InodeID
	stagingDir    string
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
	f.identityDirID, err = f.fs.LookupParent(InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
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
	uploadMeta := filepath.Join(f.stagingDir, "0000000000003039.meta")
	if err := saveStagingMeta(uploadMeta, StagingMeta{
		DirID:           f.fs.RootID(),
		FileName:        "upload.bin",
		TernCookie:      Cookie{1, 2, 3, 4, 5, 6, 7, 8},
		NFSStateID:      f.stateID,
		ClientID:        f.clientID,
		OpenOwner:       "owner-a",
		OwnerKnown:      true,
		BaseID:          MakeInodeID(InodeTypeFile, 123),
		BaseSize:        8192,
		Size:            4096,
		Dirty:           byteRangeSet{{start: 512, end: 1024}},
		MetadataChanged: true,
		Attrs: NodeInfo{
			Mtime:  f.now.Add(-3 * time.Second),
			Atime:  f.now.Add(-2 * time.Second),
			Ctime:  f.now.Add(-time.Second),
			Change: 99,
		},
	}); err != nil {
		t.Fatal(err)
	}
	uploadData := filepath.Join(f.stagingDir, "0000000000003039.staging")
	if err := os.WriteFile(uploadData,
		bytes.Repeat([]byte{0}, 4096), 0600); err != nil {
		t.Fatal(err)
	}
	orphanMeta := filepath.Join(f.stagingDir, "4000000000010932.meta")
	if err := saveStagingMeta(orphanMeta, StagingMeta{
		DirID:      f.fs.RootID(),
		FileName:   "orphan.bin",
		NFSStateID: StateID{9},
		ClientID:   f.clientID,
		Size:       512,
	}); err != nil {
		t.Fatal(err)
	}
	orphanData := filepath.Join(f.stagingDir, "4000000000010932.staging")
	if err := os.WriteFile(orphanData, bytes.Repeat([]byte{1}, 512), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{uploadMeta, uploadData, orphanMeta, orphanData} {
		if err := os.Chtimes(path, f.now, f.now); err != nil {
			t.Fatal(err)
		}
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
	if identity.IdentityHash != clientIdentityKey(f.identity) {
		t.Fatalf("identity hash = %s", identity.IdentityHash)
	}
	if identity.Identity != "Linux NFSv4.0 host-a/10.0.0.1" ||
		identity.IdentityHex != "" {
		t.Fatalf("identity = %q hex = %q", identity.Identity, identity.IdentityHex)
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
	if len(report.Staging) != 0 {
		t.Fatalf("unmatched staging = %+v", report.Staging)
	}
	if report.StagingSummary == nil || report.StagingSummary.Entries != 2 ||
		report.StagingSummary.Complete != 2 {
		t.Fatalf("staging summary = %+v", report.StagingSummary)
	}
	if open.Staging.SidecarVersion != "v6" ||
		open.Staging.BaseID != inspectInode(MakeInodeID(InodeTypeFile, 123)) ||
		open.Staging.BaseSize != 8192 || open.Staging.CheckpointSize != 4096 ||
		len(open.Staging.Dirty) != 1 || !open.Staging.MetadataChanged ||
		open.Staging.OpenOwner != "owner-a" || !open.Staging.OwnerKnown {
		t.Fatalf("open staging metadata = %+v", open.Staging)
	}

	_, replaced := findIncarnation(t, report, f.replacedID)
	if len(replaced.Roles) != 0 || replaced.Usable ||
		replaced.Status != inspectStatusUnreachable {
		t.Fatalf("replaced incarnation = %+v", replaced)
	}

	pendingIdentity, pending := findIncarnation(t, report, f.pendingID)
	if pendingIdentity.Confirmed != nil {
		t.Fatalf("pending identity has confirmed pointer %+v", pendingIdentity.Confirmed)
	}
	if strings.Join(pending.Roles, ",") != "pending" || pending.LeaseState != "none" ||
		pending.Usable || pending.Status != inspectStatusPending {
		t.Fatalf("pending incarnation = %+v", pending)
	}

	_, expired := findIncarnation(t, report, f.expiredID)
	if expired.LeaseState != "expired" || expired.Usable || expired.ExpiredMarker ||
		expired.Status != inspectStatusExpired {
		t.Fatalf("expired incarnation = %+v", expired)
	}
	if current.Status != inspectStatusActive {
		t.Fatalf("current status = %s", current.Status)
	}
	if report.Summary != (inspectSummary{
		Identities: 3, Incarnations: 4, Active: 1, Expired: 1,
		Pending: 1, Unreachable: 1, OpenMarkers: 1,
	}) {
		t.Fatalf("summary = %+v", report.Summary)
	}
}

// A healthy store must report no problems even though it holds expired leases,
// an unreachable incarnation and a staged file with no open marker. A check
// which fired on ordinary lifecycle states would fire constantly.
func TestInspectCountsNoProblemsForOrdinaryLifecycleStates(t *testing.T) {
	f := newInspectFixture(t)
	f.now = f.now.Add(10 * nfsLeaseTime)
	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: f.stagingDir})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Expired == 0 || report.Summary.Unreachable == 0 ||
		report.Summary.StaleOpens == 0 {
		t.Fatalf("fixture no longer exercises the lifecycle states: %+v",
			report.Summary)
	}
	if report.Summary.Problems != 0 {
		t.Fatalf("summary = %+v problems = %v", report.Summary, report.Problems)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if !strings.Contains(text.String(), "problems 0") ||
		strings.Contains(text.String(), "problems found") {
		t.Fatalf("text report claims problems:\n%s", text.String())
	}
}

// An unreadable client record is a defect and must be counted, not tucked into
// the principal field where nothing looks for it.
func TestInspectCountsUnreadableRecord(t *testing.T) {
	f := newInspectFixture(t)
	if err := f.fs.Remove(InodeID(f.clientID), clientRecordName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(InodeID(f.clientID), clientRecordName, "nowhere"); err != nil {
		t.Fatal(err)
	}
	report, err := f.reader(t).Inspect(inspectOptions{ClientID: f.clientID})
	if err != nil {
		t.Fatal(err)
	}
	_, current := findIncarnation(t, report, f.clientID)
	if current.Record != nil || len(current.Problems) == 0 ||
		report.Summary.Problems == 0 {
		t.Fatalf("unreadable record not reported: %+v", current)
	}
}

// Defects anywhere in the report are counted, so that the exit status is
// meaningful without parsing the output.
func TestInspectCountsProblems(t *testing.T) {
	f := newInspectFixture(t)

	// A dangling confirmed pointer.
	identityID, err := f.fs.LookupParent(InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.fs.Remove(identityID, confirmedName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(
		identityID, confirmedName, "i.ffffffffffffffffffffffffffffffff",
	); err != nil {
		t.Fatal(err)
	}
	// An unrecognized entry in an incarnation directory.
	if _, err := f.fs.Symlink(InodeID(f.clientID), "junk", "elsewhere"); err != nil {
		t.Fatal(err)
	}
	// A staging data file with no sidecar.
	if err := os.WriteFile(
		filepath.Join(f.stagingDir, "0000000000009999.staging"),
		[]byte("orphaned"), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: f.stagingDir})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Problems != 3 {
		t.Fatalf("problems = %d, want 3:\n%+v", report.Summary.Problems, report)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if !strings.Contains(text.String(), "problems 3") ||
		!strings.Contains(text.String(), "\nproblems found: 3\n") {
		t.Fatalf("text report lacks the problem count:\n%s", text.String())
	}

	var out bytes.Buffer
	if err := report.WriteJSON(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"problems": 3`)) {
		t.Fatalf("JSON lacks the problem count:\n%s", out.String())
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
	// A clientid the store has collected is a note, not a problem: asking
	// whether a client is still around and being told it is not must not make
	// a monitoring check fire.
	t.Run("stale clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{
			ClientID: uint64(MakeInodeID(InodeTypeDir, 0x123456)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			len(report.Problems) != 0 || report.Summary.Problems != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
	// Pasting the identity directory inode into -clientid is the likely
	// mistake, so the note names the flag to use instead. The store is not
	// broken, so it is not a problem.
	t.Run("identity directory as clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{
			ClientID: uint64(f.identityDirID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || report.Summary.Problems != 0 ||
			len(report.Notes) != 1 ||
			!strings.Contains(report.Notes[0], "-identity-hash") {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("identity", func(t *testing.T) {
		report, err := reader.Inspect(inspectOptions{Identity: f.identity})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			report.Identities[0].IdentityHash != clientIdentityKey(f.identity) {
			t.Fatalf("report = %+v", report.Identities)
		}
		report, err = reader.Inspect(inspectOptions{Identity: []byte("nobody")})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			report.Summary.Problems != 0 {
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
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			report.Summary.Problems != 0 {
			t.Fatalf("report = %+v", report)
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
	t.Run("stateid with staging but no open marker", func(t *testing.T) {
		sid := StateID{9}
		report, err := reader.Inspect(inspectOptions{
			StateID: &sid, StagingDir: f.stagingDir,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			len(report.Identities[0].Incarnations) != 1 ||
			len(report.Identities[0].Incarnations[0].Staging) != 1 ||
			report.Identities[0].Incarnations[0].Staging[0].FileName !=
				"orphan.bin" {
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
		"summary identities 3 incarnations 4: ACTIVE 1 EXPIRED 1 " +
			"UNLEASED 0 PENDING 1 REPLACED 0 UNREACHABLE 1; " +
			"open markers 1, stale 0",
		// Every label matches the flag which selects it.
		"identity-hash " + clientIdentityKey(f.identity),
		"identity  \"Linux NFSv4.0 host-a/10.0.0.1\"",
		"directory " + inodeHex(InodeID(f.identityDirID)),
		"confirmed clientid " + inodeHex(InodeID(f.clientID)),
		"[confirmed,pending, lease live] ACTIVE",
		"[unreachable, lease none] UNREACHABLE",
		"[pending, lease none] PENDING",
		"[confirmed,pending, lease expired] EXPIRED",
		"open       stateid " + hex.EncodeToString(f.stateID[:]) +
			" epoch abcdef01 ACTIVE",
		"file 0x0000000000003039 data 4096 bytes",
		"target dir " + inodeHex(f.fs.RootID()) + " name \"upload.bin\"",
		"base " + inodeHex(MakeInodeID(InodeTypeFile, 123)) +
			" size 8192; checkpoint size 4096 dirty [512,1024)",
		"attrs change 99",
		"owner \"owner-a\" access write",
		"NO OPEN MARKER",
		"client     identity \"Linux NFSv4.0 host-a/10.0.0.1\" verifier ",
		"principal AUTH_SYS uid=1000 gid=100 groups=[100,4] callback tcp 10.0.0.1:2049",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, text.String())
		}
	}
	// The identity directory inode is not a clientid and must not be labelled
	// as one: pasting it into -clientid does not work.
	if strings.Contains(text.String(),
		"clientid "+inodeHex(InodeID(f.identityDirID))) {
		t.Errorf("identity directory inode labelled as a clientid:\n%s", text.String())
	}
	if count := strings.Count(text.String(), "orphan.bin"); count != 1 {
		t.Fatalf("orphan staging reported %d times:\n%s", count, text.String())
	}

	var out bytes.Buffer
	if err := report.WriteJSON(&out); err != nil {
		t.Fatal(err)
	}
	// Opaque client bytes are plain strings, not base64, so that they can be
	// read and pasted back into the matching flag.
	if !bytes.Contains(out.Bytes(), []byte(`"open_owner": "owner-a"`)) ||
		!bytes.Contains(out.Bytes(), []byte(`"status": "ACTIVE"`)) {
		t.Fatalf("JSON lacks staging owner or lifecycle status:\n%s", out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("b3duZXItYQ==")) {
		t.Fatalf("JSON still base64-encodes the open owner:\n%s", out.String())
	}
	var decoded struct {
		Identities []struct {
			Incarnations []struct {
				ClientID string `json:"clientid"`
				Usable   bool   `json:"usable"`
				Status   string `json:"status"`
			} `json:"incarnations"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, identity := range decoded.Identities {
		for _, inc := range identity.Incarnations {
			if inc.ClientID == inodeHex(InodeID(f.clientID)) && inc.Usable &&
				inc.Status == inspectStatusActive {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("JSON lacks usable clientid %s:\n%s", inodeHex(InodeID(f.clientID)), out.String())
	}
	// The JSON keys are the flag names, and the values are what those flags
	// accept: -identity takes identity, -identity-hash takes identity_hash.
	var identityJSON struct {
		Identities []struct {
			IdentityHash string `json:"identity_hash"`
			DirectoryID  string `json:"directory_id"`
			Identity     string `json:"identity"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out.Bytes(), &identityJSON); err != nil {
		t.Fatal(err)
	}
	for _, identity := range identityJSON.Identities {
		if identity.Identity != string(f.identity) {
			continue
		}
		if identity.IdentityHash != clientIdentityKey(f.identity) {
			t.Fatalf("identity_hash = %q", identity.IdentityHash)
		}
		if identity.DirectoryID != inodeHex(InodeID(f.identityDirID)) {
			t.Fatalf("directory_id = %q", identity.DirectoryID)
		}
		return
	}
	t.Fatalf("JSON lacks identity %q as a plain string:\n%s", f.identity, out.String())
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
		if identity.DirectoryID != inspectInode(identityID) {
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
	if !strings.Contains(text.String(), "[confirmed,pending, lease none] UNLEASED") {
		t.Fatalf("text report lacks unleased state:\n%s", text.String())
	}
}

func TestInspectIncarnationStatus(t *testing.T) {
	tests := []struct {
		name       string
		roles      []string
		leaseState string
		want       string
	}{
		{name: "active", roles: []string{"confirmed"}, leaseState: "live",
			want: inspectStatusActive},
		{name: "expired", roles: []string{"confirmed"}, leaseState: "expired",
			want: inspectStatusExpired},
		{name: "unleased", roles: []string{"confirmed"}, leaseState: "none",
			want: inspectStatusUnleased},
		{name: "pending", roles: []string{"pending"}, leaseState: "none",
			want: inspectStatusPending},
		{name: "replaced", roles: []string{"reboot-target"}, leaseState: "live",
			want: inspectStatusReplaced},
		{name: "unreachable", leaseState: "none",
			want: inspectStatusUnreachable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := inspectIncarnationStatus(
				test.roles, test.leaseState,
			); got != test.want {
				t.Fatalf("status = %s, want %s", got, test.want)
			}
		})
	}
}

func TestInspectMarksOpenUnderExpiredLeaseStale(t *testing.T) {
	f := newInspectFixture(t)
	f.now = f.now.Add(nfsLeaseTime)
	report, err := f.reader(t).Inspect(inspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, current := findIncarnation(t, report, f.clientID)
	if current.Status != inspectStatusExpired || len(current.Opens) != 1 ||
		current.Opens[0].Active {
		t.Fatalf("expired open = %+v", current)
	}
	if report.Summary.StaleOpens != 1 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if !strings.Contains(text.String(),
		"open       stateid "+hex.EncodeToString(f.stateID[:])+
			" epoch abcdef01 STALE") {
		t.Fatalf("text report lacks stale open:\n%s", text.String())
	}
}

func TestInspectReportsStagingIntegrity(t *testing.T) {
	f := newInspectFixture(t)
	dir := t.TempDir()

	dataOnly := filepath.Join(dir, "0000000000000001.staging")
	if err := os.WriteFile(dataOnly, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	metaOnly := filepath.Join(dir, "0000000000000002.meta")
	if err := saveStagingMeta(metaOnly, StagingMeta{
		DirID: f.fs.RootID(), FileName: "meta-only",
	}); err != nil {
		t.Fatal(err)
	}
	malformedData := filepath.Join(dir, "0000000000000003.staging")
	if err := os.WriteFile(malformedData, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	malformedMeta := filepath.Join(dir, "0000000000000003.meta")
	if err := os.WriteFile(malformedMeta, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	completeData := filepath.Join(dir, "0000000000000004.staging")
	if err := os.WriteFile(completeData, []byte("complete"), 0600); err != nil {
		t.Fatal(err)
	}
	completeMeta := filepath.Join(dir, "0000000000000004.meta")
	if err := saveStagingMeta(completeMeta, StagingMeta{
		DirID: f.fs.RootID(), FileName: "exclusive",
		Exclusive: true, Verifier: [8]byte{1, 2, 3},
		ReadOnly: true, Retired: true, Unlinked: true, Guarded: true, Size: 8,
		RecoveryKey: [32]byte{0xaa, 0xbb},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".0004.meta.tmp-left"),
		[]byte("temporary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "unexpected"), 0700); err != nil {
		t.Fatal(err)
	}
	// nfsd names its files with %016x and looks the sidecar up under exactly
	// that name, so a short spelling is not a recoverable pair.
	if err := os.WriteFile(filepath.Join(dir, "5.staging"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveStagingMeta(filepath.Join(dir, "5.meta"),
		StagingMeta{DirID: f.fs.RootID(), FileName: "short-name"}); err != nil {
		t.Fatal(err)
	}
	quarantine := filepath.Join(dir, stagingQuarantineDirName, "0000000000000006-abc")
	if err := os.MkdirAll(quarantine, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quarantine, "0000000000000006.staging"),
		[]byte("quarantined"), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspectOptions{StagingDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.StagingSummary == nil ||
		report.StagingSummary.Entries != 4 ||
		report.StagingSummary.Complete != 1 ||
		report.StagingSummary.Retired != 1 ||
		report.StagingSummary.DataFiles != 3 ||
		report.StagingSummary.Sidecars != 3 ||
		report.StagingSummary.OtherClients != 0 ||
		report.StagingSummary.DataBytes != 16 {
		t.Fatalf("staging summary = %+v", report.StagingSummary)
	}
	// The metadata temporary, the unexpected directory, and both halves of the
	// short-named pair.
	if len(report.Problems) != 4 {
		t.Fatalf("problems = %v", report.Problems)
	}
	if len(report.Quarantined) != 1 ||
		report.Quarantined[0] != filepath.Join(stagingQuarantineDirName,
			"0000000000000006-abc", "0000000000000006.staging") {
		t.Fatalf("quarantined = %v", report.Quarantined)
	}
	if len(report.Staging) != 4 {
		t.Fatalf("unmatched staging = %+v", report.Staging)
	}
	byID := make(map[inspectInode]*inspectStagingEntry)
	for _, entry := range report.Staging {
		byID[entry.FileID] = entry
	}
	if got := byID[1]; got == nil || len(got.Problems) != 1 ||
		got.Problems[0] != "metadata sidecar is missing" {
		t.Fatalf("data-only entry = %+v", got)
	}
	if got := byID[2]; got == nil || len(got.Problems) != 1 ||
		got.Problems[0] != "staging data file is missing" {
		t.Fatalf("meta-only entry = %+v", got)
	}
	if got := byID[3]; got == nil || len(got.Problems) != 1 ||
		!strings.Contains(got.Problems[0], "cannot read metadata sidecar") {
		t.Fatalf("malformed entry = %+v", got)
	}
	if got := byID[4]; got == nil || got.SidecarVersion != "v6" ||
		!got.Exclusive || got.Verifier != "0102030000000000" ||
		!got.ReadOnly || !got.Retired || !got.Unlinked || !got.Guarded ||
		!strings.HasPrefix(got.RecoveryKey, "aabb") ||
		len(got.Problems) != 0 {
		t.Fatalf("complete entry = %+v", got)
	}
	if got := byID[5]; got != nil {
		t.Fatalf("short-named pair was accepted: %+v", got)
	}

	var text bytes.Buffer
	report.WriteText(&text)
	for _, want := range []string{
		"sidecar v6",
		"unlinked guarded",
		"EXCLUSIVE4 verifier 0102030000000000",
		"RETIRED (lease expired, held for reclaim)",
		"quarantined checkpoints",
		"attrs (no change counter recorded)",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, text.String())
		}
	}
	if strings.Contains(text.String(), "sidecar NFS") {
		t.Errorf("sidecar version still rendered as an NFS version:\n%s", text.String())
	}
}

// A filtered report cannot tell an orphaned sidecar from one belonging to a
// client the filter excluded, so it must count them rather than list them
// under a heading which claims they have no client.
func TestInspectFilteredReportCountsOtherClientsStaging(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspectOptions{
		IdentityHash: clientIdentityKey([]byte("pending-client")),
		StagingDir:   f.stagingDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Filter == nil ||
		report.Filter.IdentityHash != clientIdentityKey([]byte("pending-client")) {
		t.Fatalf("filter = %+v", report.Filter)
	}
	if len(report.Staging) != 0 {
		t.Fatalf("filtered report listed other clients' staging: %+v", report.Staging)
	}
	if report.StagingSummary.OtherClients != 2 {
		t.Fatalf("staging summary = %+v", report.StagingSummary)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if strings.Contains(text.String(), "upload.bin") ||
		strings.Contains(text.String(), "orphan.bin") {
		t.Fatalf("filtered text report leaked other clients' files:\n%s", text.String())
	}
	if !strings.Contains(text.String(),
		"2 sidecars belong to clients outside this filter") {
		t.Fatalf("filtered text report lacks the suppression note:\n%s", text.String())
	}
}

func TestInspectRejectsInvalidStagingPath(t *testing.T) {
	f := newInspectFixture(t)
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := f.reader(t).Inspect(inspectOptions{
		StagingDir: missing,
	}); err == nil || !strings.Contains(err.Error(), "staging directory") {
		t.Fatalf("missing staging path error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.reader(t).Inspect(inspectOptions{
		StagingDir: path,
	}); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("file staging path error = %v", err)
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
		{name: "file inode as clientid", clientID: "0x4000000000000001", wantErr: true},
		{name: "short identity hash", identityHash: "abcd", wantErr: true},
		{name: "invalid identity hash", identityHash: strings.Repeat("g", 64), wantErr: true},
		{name: "two filters", identity: "client", clientID: "0x2000000000000001", wantErr: true},
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
	// Sscanf used to accept trailing junk and out-of-range octets, formatting
	// garbage as if it were a clean address.
	for _, addr := range []string{"10.1.2.3junk.8.1", "10.1.2.999.8.1",
		"10.1.2.-3.8.1", "10.1.2..8.1"} {
		if got := formatCallback("tcp", addr); got != "tcp "+addr {
			t.Errorf("formatCallback(tcp, %q) = %s, want the raw address", addr, got)
		}
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
