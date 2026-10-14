// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

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

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/byterange"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
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
	fs         vfs.TernVFS
	first      *Store
	second     *Store
	now        time.Time
	identity   []byte
	clientID   uint64
	replacedID uint64
	stateID    nfs4.StateID
	pendingID  uint64
	expiredID  uint64
	// identityDirID is the identity directory inode of clientID. It is not a
	// clientid, and the report must not label it as one.
	identityDirID vfs.InodeID
	stagingDir    string
}

func newInspectFixture(t *testing.T) *inspectFixture {
	t.Helper()
	f := &inspectFixture{
		fs:       vfs.NewLocalTernVFS(t.TempDir()),
		now:      time.Unix(1_700_000_000, 0),
		identity: []byte("Linux NFSv4.1 host-a/10.0.0.1"),
	}
	var err error
	f.first, err = NewStore(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	f.second, err = NewStore(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	f.first.Now = func() time.Time { return f.now }
	f.second.Now = func() time.Time { return f.now }

	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: authSysBody(1000, 100, 100, 4)},
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
	f.identityDirID, err = f.fs.LookupParent(vfs.InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	f.stateID = nfs4.StateID{0xab, 0xcd, 0xef, 0x01, 0, 0, 0, 0, 0, 0, 0, 7}
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
	// expired, by 10s; the second slot of identity A is live.
	f.now = f.now.Add(LeaseWriteTerm - 40*time.Second)

	f.stagingDir = t.TempDir()
	uploadMeta := filepath.Join(f.stagingDir, "0000000000003039.meta")
	if err := staging.SaveMeta(uploadMeta, staging.Meta{
		DirID:           f.fs.RootID(),
		FileName:        "upload.bin",
		TernCookie:      vfs.Cookie{1, 2, 3, 4, 5, 6, 7, 8},
		NFSStateID:      f.stateID,
		ClientID:        f.clientID,
		OpenOwner:       "owner-a",
		OwnerKnown:      true,
		BaseID:          vfs.MakeInodeID(vfs.InodeTypeFile, 123),
		BaseSize:        8192,
		Size:            4096,
		Dirty:           byterange.Set{{Start: 512, End: 1024}},
		MetadataChanged: true,
		Attrs: vfs.NodeInfo{
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
	if err := staging.SaveMeta(orphanMeta, staging.Meta{
		DirID:      f.fs.RootID(),
		FileName:   "orphan.bin",
		NFSStateID: nfs4.StateID{9},
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
	store *Store,
	identity []byte,
	verifier [8]byte,
	owner ClientOwner,
) uint64 {
	t.Helper()
	clientID, confirm, err := store.SetClientID(verifier, identity, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(clientID, confirm, owner.Principal); err != nil {
		t.Fatal(err)
	}
	return clientID
}

func (f *inspectFixture) reader(t *testing.T) *Store {
	t.Helper()
	store, err := OpenReader(f.fs)
	if err != nil {
		t.Fatal(err)
	}
	store.Now = func() time.Time { return f.now }
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
			if inc.Incarnation == incarnationText(clientID) {
				return identity, inc
			}
		}
	}
	t.Fatalf("clientid %#x not in report", clientID)
	return inspectIdentity{}, inspectIncarnation{}
}

func TestInspectReportsClientState(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, f.stagingDir)
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
	if identity.IdentityHash != identityHashText(clientIdentityKey(f.identity)) {
		t.Fatalf("identity hash = %q", identity.IdentityHash)
	}
	if identity.Identity != "Linux NFSv4.1 host-a/10.0.0.1" ||
		identity.IdentityHex != "" {
		t.Fatalf("identity = %q hex = %q", identity.Identity, identity.IdentityHex)
	}
	if identity.Confirmed == nil ||
		identity.Confirmed.TargetID != incarnationText(f.clientID) {
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
			if slot.NfsdID != f.second.NfsdID {
				t.Fatalf("live slot belongs to %s, want %s", slot.NfsdID, f.second.NfsdID)
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
	if current.Record.Verifier != hex.EncodeToString([]byte{2, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("verifier = %s", current.Record.Verifier)
	}
	if len(current.Opens) != 1 {
		t.Fatalf("opens = %+v", current.Opens)
	}
	open := current.Opens[0]
	if open.StateID != stateidText(f.stateID) || open.Epoch != "abcdef01" {
		t.Fatalf("open = %+v", open)
	}
	if open.Staging == nil || open.Staging.FileID != inodeText(0x3039) ||
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
	if open.Staging.SidecarVersion != "v8" ||
		open.Staging.BaseID != inodeText(vfs.MakeInodeID(vfs.InodeTypeFile, 123)) ||
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
	f.now = f.now.Add(10 * LeaseTime)
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, f.stagingDir)
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
}

// An unreadable client record is a defect and must be counted, not tucked into
// the principal field where nothing looks for it.
func TestInspectCountsUnreadableRecord(t *testing.T) {
	f := newInspectFixture(t)
	if err := f.fs.Remove(vfs.InodeID(f.clientID), ClientRecordName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(vfs.InodeID(f.clientID), ClientRecordName, "nowhere"); err != nil {
		t.Fatal(err)
	}
	report, err := f.reader(t).Inspect(inspect.StoreQuery{Incarnation: f.clientID}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, current := findIncarnation(t, report, f.clientID)
	if current.Record != nil || len(current.Problems) == 0 ||
		report.Summary.Problems == 0 {
		t.Fatalf("unreadable record not reported: %+v", current)
	}
}

// Defects anywhere in the report are counted, so that summary.problems is
// meaningful without reading the rest of the report.
func TestInspectCountsProblems(t *testing.T) {
	f := newInspectFixture(t)

	// A dangling confirmed pointer.
	identityID, err := f.fs.LookupParent(vfs.InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.fs.Remove(identityID, ConfirmedName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(
		identityID, ConfirmedName, "i.ffffffffffffffffffffffffffffffff",
	); err != nil {
		t.Fatal(err)
	}
	// An unrecognized entry in an incarnation directory.
	if _, err := f.fs.Symlink(vfs.InodeID(f.clientID), "junk", "elsewhere"); err != nil {
		t.Fatal(err)
	}
	// A staging data file with no sidecar.
	if err := os.WriteFile(
		filepath.Join(f.stagingDir, "0000000000009999.staging"),
		[]byte("orphaned"), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, f.stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Problems != 3 {
		t.Fatalf("problems = %d, want 3:\n%+v", report.Summary.Problems, report)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"problems": 3`)) {
		t.Fatalf("JSON lacks the problem count:\n%s", out)
	}
}

func TestInspectDoesNotModifyStore(t *testing.T) {
	f := newInspectFixture(t)
	// Move far enough that every slot of the expired identity is collectable
	// and the replaced incarnation would be a GC candidate.
	f.now = f.now.Add(10 * LeaseTime)

	before := listTree(t, f.fs, f.first.DirID, "")
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, "")
	if err != nil {
		t.Fatal(err)
	}
	after := listTree(t, f.fs, f.first.DirID, "")
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
	if _, err := f.fs.Lookup(vfs.InodeID(f.expiredID), ExpiredName); !errors.Is(err, os.ErrNotExist) {
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
		report, err := reader.Inspect(inspect.StoreQuery{Incarnation: f.clientID}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 2 {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	// A clientid the store has collected is a note, not a problem: asking
	// whether a client is still around and being told it is not must not
	// count as a defect.
	t.Run("stale clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspect.StoreQuery{
			Incarnation: uint64(vfs.MakeInodeID(vfs.InodeTypeDir, 0x123456)),
		}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			len(report.Problems) != 0 || report.Summary.Problems != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
	// Giving the identity directory inode as clientid is the likely mistake,
	// so the note names the selector to use instead. The store is not
	// broken, so it is not a problem.
	t.Run("identity directory as clientid", func(t *testing.T) {
		report, err := reader.Inspect(inspect.StoreQuery{Incarnation: uint64(f.identityDirID)}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || report.Summary.Problems != 0 ||
			len(report.Notes) != 1 ||
			!strings.Contains(report.Notes[0], "select it with identity-hash") {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("identity", func(t *testing.T) {
		report, err := reader.Inspect(inspect.StoreQuery{Identity: string(f.identity)}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			report.Identities[0].IdentityHash != identityHashText(clientIdentityKey(f.identity)) {
			t.Fatalf("report = %+v", report.Identities)
		}
		report, err = reader.Inspect(inspect.StoreQuery{Identity: "nobody"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			report.Summary.Problems != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("identity hash", func(t *testing.T) {
		report, err := reader.Inspect(inspect.StoreQuery{IdentityHash: clientIdentityKey([]byte("pending-client"))}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 1 {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	t.Run("stateid scan", func(t *testing.T) {
		sid := [12]byte(f.stateID)
		report, err := reader.Inspect(inspect.StoreQuery{Stateid: &sid}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 || len(report.Identities[0].Incarnations) != 1 ||
			report.Identities[0].Incarnations[0].Incarnation != incarnationText(f.clientID) {
			t.Fatalf("report = %+v", report.Identities)
		}
		unknown := [12]byte{1}
		report, err = reader.Inspect(inspect.StoreQuery{Stateid: &unknown}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 0 || len(report.Notes) != 1 ||
			report.Summary.Problems != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
	t.Run("stateid via staging sidecar", func(t *testing.T) {
		sid := [12]byte(f.stateID)
		report, err := reader.Inspect(inspect.StoreQuery{Stateid: &sid}, f.stagingDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Identities) != 1 ||
			report.Identities[0].Incarnations[0].Opens[0].Staging == nil {
			t.Fatalf("report = %+v", report.Identities)
		}
	})
	t.Run("stateid with staging but no open marker", func(t *testing.T) {
		sid := [12]byte{9}
		report, err := reader.Inspect(inspect.StoreQuery{Stateid: &sid}, f.stagingDir)
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
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, f.stagingDir)
	if err != nil {
		t.Fatal(err)
	}

	if s := report.Summary; s.Identities != 3 || s.Incarnations != 4 || s.Active != 1 ||
		s.Expired != 1 || s.Pending != 1 || s.Unreachable != 1 || s.OpenMarkers != 1 || s.StaleOpens != 0 {
		t.Fatalf("summary = %+v", s)
	}
	// A staged file with no client in the store is listed once, at the top
	// level, and not again under any identity.
	orphans := 0
	for _, entry := range report.Staging {
		if entry.FileName == "orphan.bin" {
			orphans++
		}
	}
	for _, identity := range report.Identities {
		for _, inc := range identity.Incarnations {
			for _, entry := range inc.Staging {
				if entry.FileName == "orphan.bin" {
					orphans++
				}
			}
		}
	}
	if orphans != 1 {
		t.Fatalf("orphan staging reported %d times", orphans)
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Opaque client bytes are plain strings, not base64, so that they can be
	// read and pasted back into the matching selector.
	if !bytes.Contains(out, []byte(`"open_owner": "owner-a"`)) ||
		!bytes.Contains(out, []byte(`"status": "ACTIVE"`)) {
		t.Fatalf("JSON lacks staging owner or lifecycle status:\n%s", out)
	}
	if bytes.Contains(out, []byte("b3duZXItYQ==")) {
		t.Fatalf("JSON still base64-encodes the open owner:\n%s", out)
	}
	var decoded struct {
		Identities []struct {
			Incarnations []struct {
				Incarnation string `json:"incarnation"`
				Usable      bool   `json:"usable"`
				Status      string `json:"status"`
			} `json:"incarnations"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, identity := range decoded.Identities {
		for _, inc := range identity.Incarnations {
			if inc.Incarnation == incarnationText(f.clientID) && inc.Usable &&
				inc.Status == inspectStatusActive {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("JSON lacks usable %s:\n%s", incarnationText(f.clientID), out)
	}
	// The JSON keys are the selector names, and the values are what those
	// selectors accept: identity takes identity, identity-hash takes
	// identity_hash.
	var identityJSON struct {
		Identities []struct {
			IdentityHash string `json:"identity_hash"`
			DirectoryID  string `json:"directory_id"`
			Identity     string `json:"identity"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(out, &identityJSON); err != nil {
		t.Fatal(err)
	}
	for _, identity := range identityJSON.Identities {
		if identity.Identity != string(f.identity) {
			continue
		}
		if identity.IdentityHash != identityHashText(clientIdentityKey(f.identity)) {
			t.Fatalf("identity_hash = %q", identity.IdentityHash)
		}
		if identity.DirectoryID != inodeText(f.identityDirID) {
			t.Fatalf("directory_id = %q", identity.DirectoryID)
		}
		return
	}
	t.Fatalf("JSON lacks identity %q as a plain string:\n%s", f.identity, out)
}

func TestInspectJSONKeepsAnEmptyIdentityListAnArray(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspect.StoreQuery{
		Incarnation: uint64(vfs.MakeInodeID(vfs.InodeTypeDir, 0x123456)),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Identities []json.RawMessage `json:"identities"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Identities == nil {
		t.Fatalf("identities = null:\n%s", out)
	}
}

func TestInspectReportsMalformedStagingAndPointers(t *testing.T) {
	f := newInspectFixture(t)
	if err := staging.SaveMeta(filepath.Join(f.stagingDir, "not-an-inode.meta"),
		staging.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.stagingDir, "broken.meta"),
		[]byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	identityID, err := f.fs.LookupParent(vfs.InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	name, err := f.first.requireIncarnationName(identityID, vfs.InodeID(f.clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.fs.Remove(identityID, ConfirmedName); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fs.Symlink(identityID, ConfirmedName, "../"+name); err != nil {
		t.Fatal(err)
	}

	// A directory under clients whose name is not a hash is kept by its raw
	// name as a problem, with no typed hash invented for it.
	if _, err := f.fs.Mkdir(f.first.DirID, "not-a-hash"); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, f.stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Problems) != 2 {
		t.Fatalf("staging problems = %v", report.Problems)
	}
	found := 0
	for _, identity := range report.Identities {
		switch {
		case identity.DirectoryID == inodeText(identityID):
			if identity.Confirmed == nil || !identity.Confirmed.Dangling ||
				identity.Confirmed.Problem == "" {
				t.Fatalf("confirmed pointer = %+v", identity.Confirmed)
			}
		case identity.Name == "not-a-hash":
			if identity.IdentityHash != "" || len(identity.Problems) != 1 ||
				!strings.Contains(identity.Problems[0], "not a 64-digit hex hash") {
				t.Fatalf("malformed identity = %+v", identity)
			}
		default:
			continue
		}
		found++
	}
	if found != 2 {
		t.Fatalf("found %d of the dangling and malformed identities in %+v", found, report.Identities)
	}
}

// A confirmed client that has never recorded a lease is UNLEASED, not
// EXPIRED: it can still acquire its first lease.
func TestInspectReportsConfirmedClientWithoutLease(t *testing.T) {
	f := newInspectFixture(t)
	id := f.register(t, f.first, []byte("awaiting-renew"), [8]byte{5}, ClientOwner{})
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, inc := findIncarnation(t, report, id)
	if inc.Status != inspectStatusUnleased || inc.LeaseState != "none" {
		t.Fatalf("unleased incarnation = %+v", inc)
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
	f.now = f.now.Add(LeaseTime)
	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, "")
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
}

func TestInspectReportsStagingIntegrity(t *testing.T) {
	f := newInspectFixture(t)
	dir := t.TempDir()

	dataOnly := filepath.Join(dir, "0000000000000001.staging")
	if err := os.WriteFile(dataOnly, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	metaOnly := filepath.Join(dir, "0000000000000002.meta")
	if err := staging.SaveMeta(metaOnly, staging.Meta{
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
	if err := staging.SaveMeta(completeMeta, staging.Meta{
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
	if err := staging.SaveMeta(filepath.Join(dir, "5.meta"),
		staging.Meta{DirID: f.fs.RootID(), FileName: "short-name"}); err != nil {
		t.Fatal(err)
	}
	quarantine := filepath.Join(dir, staging.QuarantineDirName, "0000000000000006-abc")
	if err := os.MkdirAll(quarantine, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quarantine, "0000000000000006.staging"),
		[]byte("quarantined"), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := f.reader(t).Inspect(inspect.StoreQuery{}, dir)
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
		report.Quarantined[0] != filepath.Join(staging.QuarantineDirName,
			"0000000000000006-abc", "0000000000000006.staging") {
		t.Fatalf("quarantined = %v", report.Quarantined)
	}
	if len(report.Staging) != 4 {
		t.Fatalf("unmatched staging = %+v", report.Staging)
	}
	byID := make(map[string]*inspectStagingEntry)
	for _, entry := range report.Staging {
		byID[entry.FileID] = entry
	}
	if got := byID[inodeText(1)]; got == nil || len(got.Problems) != 1 ||
		got.Problems[0] != "metadata sidecar is missing" {
		t.Fatalf("data-only entry = %+v", got)
	}
	if got := byID[inodeText(2)]; got == nil || len(got.Problems) != 1 ||
		got.Problems[0] != "staging data file is missing" {
		t.Fatalf("meta-only entry = %+v", got)
	}
	if got := byID[inodeText(3)]; got == nil || len(got.Problems) != 1 ||
		!strings.Contains(got.Problems[0], "cannot read metadata sidecar") {
		t.Fatalf("malformed entry = %+v", got)
	}
	if got := byID[inodeText(4)]; got == nil || got.SidecarVersion != "v8" ||
		!got.Exclusive || got.Verifier != "0102030000000000" ||
		!got.ReadOnly || !got.Retired || !got.Unlinked || !got.Guarded ||
		!strings.HasPrefix(got.RecoveryKey, "aabb") ||
		len(got.Problems) != 0 {
		t.Fatalf("complete entry = %+v", got)
	}
	if got := byID[inodeText(5)]; got != nil {
		t.Fatalf("short-named pair was accepted: %+v", got)
	}
	// An entry whose sidecar is missing or unreadable has no directory to
	// report, so dir_id is omitted rather than a zero id invented; the two
	// entries with a readable sidecar keep theirs.
	out, err := json.Marshal(report.Staging)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatal(err)
	}
	hasDir := map[string]bool{inodeText(2): true, inodeText(4): true}
	for _, e := range entries {
		var fileID string
		if err := json.Unmarshal(e["file_id"], &fileID); err != nil {
			t.Fatal(err)
		}
		if _, ok := e["dir_id"]; ok != hasDir[fileID] {
			t.Fatalf("%s: dir_id present = %v, want %v", fileID, ok, hasDir[fileID])
		}
	}
}

// A filtered report cannot tell an orphaned sidecar from one belonging to a
// client the filter excluded, so it must count them rather than list them
// under a heading which claims they have no client.
func TestInspectFilteredReportCountsOtherClientsStaging(t *testing.T) {
	f := newInspectFixture(t)
	report, err := f.reader(t).Inspect(inspect.StoreQuery{IdentityHash: clientIdentityKey([]byte("pending-client"))}, f.stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.Filter == nil ||
		report.Filter.IdentityHash != identityHashText(clientIdentityKey([]byte("pending-client"))) {
		t.Fatalf("filter = %+v", report.Filter)
	}
	if len(report.Staging) != 0 {
		t.Fatalf("filtered report listed other clients' staging: %+v", report.Staging)
	}
	if report.StagingSummary.OtherClients != 2 {
		t.Fatalf("staging summary = %+v", report.StagingSummary)
	}
	// Nothing of the other clients' files leaks into the filtered report.
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("upload.bin")) || bytes.Contains(out, []byte("orphan.bin")) {
		t.Fatalf("filtered report leaked other clients' files:\n%s", out)
	}
}

func TestInspectRejectsInvalidStagingPath(t *testing.T) {
	f := newInspectFixture(t)
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := f.reader(t).Inspect(inspect.StoreQuery{}, missing); err == nil || !strings.Contains(err.Error(), "staging directory") {
		t.Fatalf("missing staging path error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.reader(t).Inspect(inspect.StoreQuery{}, path); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("file staging path error = %v", err)
	}
}

func TestInspectWithoutStore(t *testing.T) {
	if _, err := OpenReader(vfs.NewLocalTernVFS(t.TempDir())); err == nil {
		t.Fatal("reader opened a filesystem without a client store")
	}
}

func TestFormatPrincipal(t *testing.T) {
	if got := formatPrincipal(RPCPrincipal{Flavor: nfs4.AUTH_NONE}); got != "AUTH_NONE" {
		t.Fatalf("AUTH_NONE = %s", got)
	}
	if got := formatPrincipal(RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "ab"}); got != "AUTH_SYS 6162" {
		t.Fatalf("short AUTH_SYS = %s", got)
	}
	if got := formatPrincipal(RPCPrincipal{Flavor: 6, Body: "\x01"}); got != "flavor 6 01" {
		t.Fatalf("unknown flavor = %s", got)
	}
}

func listTree(t *testing.T, fs vfs.TernVFS, dirID vfs.InodeID, prefix string) []string {
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
			if IsDirectoryInodeID(entry.ID) {
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
