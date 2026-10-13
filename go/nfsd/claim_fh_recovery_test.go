// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sidecar"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// claimHookVFS runs a hook once, after the next Lookup of name returns. A
// writable CLAIM_FH on a cold name cache first looks the name up in its
// lockless preselection, so the hook runs before the pathname lock.
type claimHookVFS struct {
	vfs.TernVFS
	mu    sync.Mutex
	name  string
	after func()
}

func (fs *claimHookVFS) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Lookup(dir, name)
	fs.mu.Lock()
	var hook func()
	if fs.after != nil && name == fs.name {
		hook, fs.after = fs.after, nil
	}
	fs.mu.Unlock()
	if hook != nil {
		hook()
	}
	return id, err
}

func (fs *claimHookVFS) afterLookup(name string, hook func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.name, fs.after = name, hook
}

// claimFHRestart is a staged writer left by one nfsd process, and a second
// process over the same store and staging directory, with a cold name cache
// and a new session for the same client.
type claimFHRestart struct {
	root        string
	fs          *claimHookVFS
	srv         *Server
	staging     *LocalStagingStore
	conn        net.Conn
	xid         uint32
	wire        uint64
	oldWire     uint64 // the first process's clientid
	incarnation uint64 // the writer's incarnation, from the first process
	name, owner string
	fh          []byte // the handle the client holds: H
	stagingID   vfs.InodeID
	data        []byte
	healthyFH   []byte // a closed file with no staging
}

// newClaimFHRestart leaves a writer of name with data staged. A replacement
// writer opens an existing file; otherwise the OPEN creates it. now, if
// set, is the second process's clock.
func newClaimFHRestart(t *testing.T, name string, replacement bool, now func() time.Time) *claimFHRestart {
	t.Helper()
	f := newRestartedWriter(t, name, replacement, now)
	f.wire = setupClient(t, f.conn, &f.xid)
	if _, ok := f.srv.names.lookup(f.handleID(t)); ok {
		t.Fatal("name cache is not cold")
	}
	return f
}

// newRestartedWriter is newClaimFHRestart before the client has registered
// with the second process: its connection has no session.
func newRestartedWriter(t *testing.T, name string, replacement bool, now func() time.Time) *claimFHRestart {
	t.Helper()
	f := &claimFHRestart{root: t.TempDir(), name: name, owner: "test-owner-" + name,
		data: []byte("staged before restart")}
	f.fs = &claimHookVFS{TernVFS: vfs.NewLocalTernVFS(f.root)}
	stagingDir := t.TempDir()
	firstStaging, err := NewLocalStagingStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewServer(f.fs, firstStaging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stopFirst := serveTestServer(t, first)
	conn := dial(t, addr)
	xid := uint32(1)
	wire := setupClient(t, conn, &xid)
	f.oldWire = wire
	f.incarnation = incarnationForTest(t, first, wire)
	healthy, healthyFH := openCreateFile(t, conn, &xid, wire, "healthy")
	writeFileAt(t, conn, &xid, healthyFH, healthy, 0, []byte("closed"))
	closeFile(t, conn, &xid, healthyFH, healthy)
	f.healthyFH = lookupFH(t, conn, &xid, "healthy")
	var stateid [16]byte
	if replacement {
		if err := os.WriteFile(filepath.Join(f.root, name), []byte("published base"), 0o644); err != nil {
			t.Fatal(err)
		}
		stateid, f.fh = openWriteFile(t, conn, &xid, wire, name)
	} else {
		stateid, f.fh = openCreateFile(t, conn, &xid, wire, name)
	}
	writeFileAt(t, conn, &xid, f.fh, stateid, 0, f.data)
	f.stagingID = stagingIDForTest(t, firstStaging, stateid)
	conn.Close()
	stopFirst()
	closeLocalStagingFiles(t, firstStaging)

	if f.staging, err = NewLocalStagingStore(stagingDir, nil); err != nil {
		t.Fatal(err)
	}
	if f.srv, err = NewServer(f.fs, f.staging, nil); err != nil {
		t.Fatal(err)
	}
	if now != nil {
		f.srv.clients.setClock(now)
	}
	addr, stopSecond := serveTestServer(t, f.srv)
	f.conn = dial(t, addr)
	t.Cleanup(func() {
		f.conn.Close()
		stopSecond()
	})
	f.xid = 100
	return f
}

func (f *claimFHRestart) handleID(t *testing.T) vfs.InodeID {
	t.Helper()
	id, ok := vfs.FHToInodeID(f.fh)
	if !ok {
		t.Fatal("bad handle")
	}
	return id
}

func (f *claimFHRestart) claimFH(t *testing.T, owner string) (uint32, [16]byte, []byte) {
	t.Helper()
	return openByHandle(t, f.conn, &f.xid, f.wire, owner, f.fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
}

func (f *claimFHRestart) meta(t *testing.T) sidecar.Meta {
	t.Helper()
	meta, ok := f.staging.GetMeta(f.stagingID)
	if !ok {
		t.Fatal("staging gone")
	}
	return meta
}

// assertNothingAdopted checks that a refused CLAIM_FH opened nothing and left
// the staging as it was.
func (f *claimFHRestart) assertNothingAdopted(t *testing.T, before sidecar.Meta) {
	t.Helper()
	if states := f.srv.opens.clientStates(incarnationForTest(t, f.srv, f.wire)); len(states) != 0 {
		t.Fatalf("refused CLAIM_FH left %d opens", len(states))
	}
	if meta, ok := f.staging.GetMeta(f.stagingID); ok &&
		(meta.NFSStateID != before.NFSStateID || meta.ClientID != before.ClientID) {
		t.Fatal("refused CLAIM_FH rebound the staging")
	}
}

// oldEpochStateID is a stateid an earlier process could have issued.
func oldEpochStateID(srv *Server) nfs4.StateID {
	sid := srv.opens.newStateID()
	binary.BigEndian.PutUint32(sid[0:4], binary.BigEndian.Uint32(sid[0:4])+1)
	return sid
}

// injectWriter adds another recoverable writer of the same name, incarnation
// and owner, as an earlier process could have left.
func (f *claimFHRestart) injectWriter(t *testing.T) {
	t.Helper()
	id, cookie, err := f.fs.TernVFS.ConstructFile(f.fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	meta := f.meta(t)
	if _, err := f.staging.Create(id, sidecar.Meta{DirID: meta.DirID, FileName: meta.FileName,
		TernCookie: cookie, NFSStateID: oldEpochStateID(f.srv), ClientID: meta.ClientID,
		OwnerKnown: true, OpenOwner: meta.OpenOwner, RecoveryKey: meta.RecoveryKey}); err != nil {
		t.Fatal(err)
	}
}

// A staged writer is recovered by a writable CLAIM_FH of the handle the
// client already holds, on a cold name cache, for a new file and for a
// replacement. The reply keeps the handle, the name cache learns it, and
// CLOSE publishes the data written before and after the restart.
func TestClaimFHRecoversStagedWriter(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		name := "new file"
		if replacement {
			name = "replacement"
		}
		t.Run(name, func(t *testing.T) {
			f := newClaimFHRestart(t, "file", replacement, nil)
			old := f.meta(t).NFSStateID
			status, sid, fh := f.claimFH(t, f.owner)
			if status != nfs4.NFS4_OK || !bytes.Equal(fh, f.fh) {
				t.Fatalf("CLAIM_FH = %s", nfs4.Nfsstat4Name(status))
			}
			if meta := f.meta(t); meta.NFSStateID == old || meta.ClientID != incarnationForTest(t, f.srv, f.wire) {
				t.Fatal("staging not rebound to the new open")
			}
			if target, ok := f.srv.names.lookup(f.handleID(t)); !ok || target.name != f.name {
				t.Fatal("name cache not refreshed")
			}
			more := []byte(" and after")
			writeFileAt(t, f.conn, &f.xid, f.fh, sid, uint64(len(f.data)), more)
			closeFile(t, f.conn, &f.xid, f.fh, sid)
			want := append(append([]byte(nil), f.data...), more...)
			if got, _ := readFileData(t, f.conn, &f.xid, lookupFH(t, f.conn, &f.xid, f.name), 0, 128); !bytes.Equal(got, want) {
				t.Fatalf("published %q, want %q", got, want)
			}
		})
	}
}

// After the writer's lease has expired, the client's next incarnation
// recovers it through CLAIM_FH by RecoveryKey.
func TestClaimFHRecoversAfterExpiry(t *testing.T) {
	later := func() time.Time { return time.Now().Add(3 * nfsLeaseTime) }
	f := newClaimFHRestart(t, "file", true, later)
	if n := incarnationForTest(t, f.srv, f.wire); n == f.incarnation {
		t.Fatal("expired incarnation was adopted")
	}
	status, sid, _ := f.claimFH(t, f.owner)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CLAIM_FH after expiry = %s", nfs4.Nfsstat4Name(status))
	}
	if f.meta(t).ClientID == f.incarnation {
		t.Fatal("staging still owned by the expired incarnation")
	}
	closeFile(t, f.conn, &f.xid, f.fh, sid)
	if got, _ := readFileData(t, f.conn, &f.xid, lookupFH(t, f.conn, &f.xid, f.name), 0, 128); !bytes.Equal(got, f.data) {
		t.Fatalf("published %q", got)
	}
}

// Refusals that need no race: each gives its status and adopts nothing.
func TestClaimFHRecoveryRefusals(t *testing.T) {
	t.Run("wrong owner", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", false, nil)
		before := f.meta(t)
		if status, _, _ := f.claimFH(t, "another-owner"); status != nfs4.NFS4ERR_STALE {
			t.Fatalf("CLAIM_FH by another owner = %s", nfs4.Nfsstat4Name(status))
		}
		f.assertNothingAdopted(t, before)
	})
	t.Run("wrong incarnation", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", false, nil)
		before := f.meta(t)
		other := dial(t, f.conn.RemoteAddr().String())
		defer other.Close()
		otherWire, confirm := requestClientID(t, other, &f.xid, "other-client", [8]byte{2})
		if status := confirmClientID(t, other, &f.xid, otherWire, confirm); status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		if status, _, _ := openByHandle(t, other, &f.xid, otherWire, f.owner, f.fh, nfs4.OPEN4_SHARE_ACCESS_BOTH); status != nfs4.NFS4ERR_STALE {
			t.Fatalf("CLAIM_FH by another client = %s", nfs4.Nfsstat4Name(status))
		}
		f.assertNothingAdopted(t, before)
	})
	t.Run("renamed away", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", true, nil)
		before := f.meta(t)
		if err := os.Rename(filepath.Join(f.root, f.name), filepath.Join(f.root, "moved")); err != nil {
			t.Fatal(err)
		}
		if status, _, _ := f.claimFH(t, f.owner); status != nfs4.NFS4ERR_STALE {
			t.Fatalf("CLAIM_FH of a renamed-away target = %s", nfs4.Nfsstat4Name(status))
		}
		f.assertNothingAdopted(t, before)
	})
	t.Run("replaced", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", true, nil)
		before := f.meta(t)
		replaceLocalFile(t, f.root, f.name)
		if status, _, _ := f.claimFH(t, f.owner); status != nfs4.NFS4ERR_STALE {
			t.Fatalf("CLAIM_FH of a replaced target = %s", nfs4.Nfsstat4Name(status))
		}
		f.assertNothingAdopted(t, before)
	})
	t.Run("two candidates", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", false, nil)
		before := f.meta(t)
		f.injectWriter(t)
		if status, _, _ := f.claimFH(t, f.owner); status != nfs4.NFS4ERR_EXPIRED {
			t.Fatalf("CLAIM_FH with two candidates = %s", nfs4.Nfsstat4Name(status))
		}
		f.assertNothingAdopted(t, before)
	})
	t.Run("healthy handle", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", false, nil)
		if status, _, _ := openByHandle(t, f.conn, &f.xid, f.wire, "healthy-owner", f.healthyFH, nfs4.OPEN4_SHARE_ACCESS_BOTH); status != nfs4.NFS4ERR_STALE {
			t.Fatalf("writable CLAIM_FH of a healthy cold handle = %s", nfs4.Nfsstat4Name(status))
		}
	})
	t.Run("CREATE", func(t *testing.T) {
		f := newClaimFHRestart(t, "file", false, nil)
		before := f.meta(t)
		res := sendCompound(t, f.conn, f.xid, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(f.fh).Finish())
			w.Resume(pw.Finish())
			ow := w.AppendArgarray_Open()
			ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
			ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
			ownerW := ow.StartOwner().SetClientid(f.wire)
			ow.Resume(ownerW.SetOwner([]byte(f.owner)).Finish())
			chw := ow.SetOpenhow_Create()
			faw := chw.SetValue_Unchecked4()
			bmW := faw.StartAttrmask()
			buf := bmW.Finish()
			faw.Resume(buf)
			alW := faw.StartAttrVals()
			buf = alW.SetData(nil).Finish()
			faw.Resume(buf)
			buf = faw.Finish()
			chw.Resume(buf)
			buf = chw.Finish()
			ow.Resume(buf)
			ow.SetClaim_Fh()
			w.Resume(ow.Finish())
		})
		f.xid++
		if res.Status() != nfs4.NFS4ERR_INVAL {
			t.Fatalf("CLAIM_FH with CREATE = %s", nfs4.Nfsstat4Name(res.Status()))
		}
		f.assertNothingAdopted(t, before)
	})
}

// replaceLocalFile links a new inode at name.
func replaceLocalFile(t *testing.T, root, name string) {
	t.Helper()
	tmp := filepath.Join(root, ".replacement")
	if err := os.WriteFile(tmp, []byte("someone else's"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
}

// Changes after the lockless preselection and before the pathname lock: the
// selection is checked again under the lock and anything other than the
// same staging, still at a name holding H, gives STALE with nothing
// adopted.
func TestClaimFHRecoveryRevalidation(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(t *testing.T, f *claimFHRestart)
		gone   bool // the change removes the staging
	}{
		{name: "target replaced", change: func(t *testing.T, f *claimFHRestart) {
			replaceLocalFile(t, f.root, f.name)
		}},
		{name: "target renamed away", change: func(t *testing.T, f *claimFHRestart) {
			if err := os.Rename(filepath.Join(f.root, f.name), filepath.Join(f.root, "moved")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "staging rebound", change: func(t *testing.T, f *claimFHRestart) {
			meta := f.meta(t)
			if err := f.staging.Rebind(f.stagingID, meta.ClientID, oldEpochStateID(f.srv)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "staging removed", gone: true, change: func(t *testing.T, f *claimFHRestart) {
			f.staging.Remove(f.stagingID)
		}},
		{name: "staging failed", gone: true, change: func(t *testing.T, f *claimFHRestart) {
			if err := f.staging.Quarantine(f.stagingID); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "another staging selectable", change: func(t *testing.T, f *claimFHRestart) {
			f.injectWriter(t)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newClaimFHRestart(t, "file", true, nil)
			ran := false
			var changed sidecar.Meta // the staging as the change left it
			f.fs.afterLookup(f.name, func() {
				ran = true
				c.change(t, f)
				if !c.gone {
					changed = f.meta(t)
				}
			})
			if status, _, _ := f.claimFH(t, f.owner); status != nfs4.NFS4ERR_STALE {
				t.Fatalf("CLAIM_FH = %s, want STALE", nfs4.Nfsstat4Name(status))
			}
			if !ran {
				t.Fatal("the change did not run between preselection and the lock")
			}
			if c.gone {
				if states := f.srv.opens.clientStates(incarnationForTest(t, f.srv, f.wire)); len(states) != 0 {
					t.Fatalf("refused CLAIM_FH left %d opens", len(states))
				}
				return
			}
			f.assertNothingAdopted(t, changed)
		})
	}
}
