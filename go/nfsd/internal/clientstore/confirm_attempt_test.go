// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type confirmFixture struct {
	fs                *ConfirmFaultVFS
	local             vfs.TernVFS
	store             *Store
	identity, p, n, m vfs.InodeID
	confirm           [8]byte
	target            confirmTarget
}

// newConfirmFixture builds a pending N, and with withP a confirmed P that
// has a lease and an open marker for the purge to remove.
func newConfirmFixture(t *testing.T, withP bool) *confirmFixture {
	t.Helper()
	fs := NewConfirmFaultVFS(t)
	f := &confirmFixture{fs: fs, local: fs.TernVFS}
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	f.store = store
	if withP {
		p, confirm, err := store.SetClientID([8]byte{1}, []byte("client"), ClientOwner{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.ConfirmClientID(p, confirm, RPCPrincipal{}); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkOpen(p, nfs4.StateID{1}); err != nil {
			t.Fatal(err)
		}
		f.p = vfs.InodeID(p)
	}
	n, confirm, err := store.SetClientID([8]byte{2}, []byte("client"), ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	f.n, f.confirm = vfs.InodeID(n), confirm
	if f.identity, err = f.local.LookupParent(f.n); err != nil {
		t.Fatal(err)
	}
	if f.m, err = store.NewIncarnation(f.identity); err != nil {
		t.Fatal(err)
	}
	if f.target, err = store.PrepareConfirmation(n, confirm); err != nil {
		t.Fatal(err)
	}
	if f.target.N != f.n || f.target.P != f.p || (f.target.sp != 0) != withP {
		t.Fatalf("target %+v", f.target)
	}
	return f
}

// point replaces identity/confirmed, through the base VFS, with a new
// symlink to incarnation id, or removes it when id is zero.
func (f *confirmFixture) point(t *testing.T, id vfs.InodeID) {
	t.Helper()
	if id == 0 {
		if err := f.local.Remove(f.identity, ConfirmedName); err != nil {
			t.Fatal(err)
		}
		return
	}
	name, found, err := f.store.IncarnationName(f.identity, id)
	if err != nil || !found {
		t.Fatalf("incarnation name %v", err)
	}
	if _, err := f.local.Symlink(f.identity, "t.hook", name); err != nil {
		t.Fatal(err)
	}
	if err := f.local.Rename(f.identity, "t.hook", f.identity, ConfirmedName); err != nil {
		t.Fatal(err)
	}
}

func (f *confirmFixture) commit(t *testing.T) *ConfirmAttempt {
	t.Helper()
	f.fs.Mu.Lock()
	f.fs.Armed = true
	f.fs.Mu.Unlock()
	return f.store.CommitConfirmation(f.target, f.confirm)
}

func (f *confirmFixture) confirmedIs(t *testing.T) vfs.InodeID {
	t.Helper()
	id, found, err := f.store.ReadIdentitySymlink(f.identity, ConfirmedName)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return 0
	}
	return id
}

// Each write's faults with an unchanged predecessor. Only D can commit, and
// only on proof; an unfenceable write stays unresolved until a later
// settle succeeds.
func TestConfirmationWriteFaults(t *testing.T) {
	names := map[WriteID]string{writeA: "A", writeB: "B", WriteD: "D"}
	for _, w := range []WriteID{writeA, writeB, WriteD} {
		for _, c := range []struct {
			name             string
			fault            ConfirmFault
			submitted        bool
			want, afterClear ConfirmResult
			outcome          writeOutcome
		}{
			{"pre-send", faultPreSend, false, ConfirmRolledBack, ConfirmRolledBack, writeNever},
			{"applied with an error", faultAppliedErr, true, ConfirmRolledBack, ConfirmRolledBack, writeMaybe},
			{"not applied", FaultNotApplied, true, ConfirmRolledBack, ConfirmRolledBack, writeNever},
			{"fence error", FaultFenceErr, true, confirmUnresolved, ConfirmRolledBack, writeNever},
			{"applied, fence error", FaultAppliedFail, true, confirmUnresolved, ConfirmRolledBack, writeMaybe},
		} {
			t.Run(names[w]+"/"+c.name, func(t *testing.T) {
				f := newConfirmFixture(t, true)
				f.fs.Set(w, c.fault, nil)
				want, afterClear := c.want, c.afterClear
				if w == WriteD && (c.fault == faultAppliedErr || c.fault == FaultAppliedFail) {
					// D applied: the read-back finds N through D's inode.
					want, afterClear = ConfirmCommitted, ConfirmCommitted
					if c.fault == FaultAppliedFail {
						want = confirmUnresolved
					}
				}
				a := f.commit(t)
				if a.Result() != want {
					t.Fatalf("result %s, want %s", confirmResultName(a.Result()), confirmResultName(want))
				}
				rec := a.writes[w]
				if rec == nil || rec.submitted != c.submitted {
					t.Fatalf("attempt record %+v", rec)
				}
				for later := w + 1; later <= WriteD; later++ {
					if a.writes[later] != nil {
						t.Fatalf("write %s attempted after %s failed", names[later], names[w])
					}
				}
				f.fs.Set(w, FaultNone, nil)
				if got := f.store.SettleConfirmation(a); got != afterClear {
					t.Fatalf("settled %s, want %s", confirmResultName(got), confirmResultName(afterClear))
				}
				if rec.outcome() != c.outcome {
					t.Fatalf("write outcome %d, want %d", rec.outcome(), c.outcome)
				}
				wantConfirmed := f.p
				if afterClear == ConfirmCommitted {
					wantConfirmed = f.n
				}
				if got := f.confirmedIs(t); got != wantConfirmed {
					t.Fatalf("confirmed %d, want %d", got, wantConfirmed)
				}
			})
		}
	}
}

// The read-back table when D's rename reports an error and something else
// has changed the confirmed pointer.
func TestConfirmationObservation(t *testing.T) {
	for _, c := range []struct {
		name  string
		fault ConfirmFault
		to    func(f *confirmFixture) vfs.InodeID
		want  ConfirmResult
	}{
		{"never applied, P through another symlink", FaultNotApplied, func(f *confirmFixture) vfs.InodeID { return f.p }, ConfirmConservative},
		{"never applied, another incarnation M", FaultNotApplied, func(f *confirmFixture) vfs.InodeID { return f.m }, ConfirmConservative},
		{"never applied, absent", FaultNotApplied, func(f *confirmFixture) vfs.InodeID { return 0 }, ConfirmConservative},
		{"never applied, N through another symlink", FaultNotApplied, func(f *confirmFixture) vfs.InodeID { return f.n }, ConfirmCommitted},
		{"edge gone, P", faultAppliedErr, func(f *confirmFixture) vfs.InodeID { return f.p }, ConfirmConservative},
		{"edge gone, M", faultAppliedErr, func(f *confirmFixture) vfs.InodeID { return f.m }, ConfirmConservative},
		{"edge gone, absent", faultAppliedErr, func(f *confirmFixture) vfs.InodeID { return 0 }, ConfirmConservative},
		{"edge gone, N through another symlink", faultAppliedErr, func(f *confirmFixture) vfs.InodeID { return f.n }, ConfirmCommitted},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.Set(WriteD, c.fault, func() { f.point(t, c.to(f)) })
			if got := f.commit(t).Result(); got != c.want {
				t.Fatalf("result %s, want %s", confirmResultName(got), confirmResultName(c.want))
			}
		})
	}
	t.Run("read error", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		f.fs.Set(WriteD, FaultNotApplied, func() { f.fs.FailLookup = true })
		a := f.commit(t)
		if a.Result() != confirmUnresolved {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		f.fs.Mu.Lock()
		f.fs.FailLookup = false
		f.fs.Mu.Unlock()
		if got := f.store.SettleConfirmation(a); got != ConfirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
	t.Run("success", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		if a.Result() != ConfirmCommitted || a.observed.file != a.writes[WriteD].inode {
			t.Fatalf("result %s, observed %+v", confirmResultName(a.Result()), a.observed)
		}
	})
}

// A first confirmation has no predecessor: no B, and a D that never applied
// observes an absent pointer, which the approved table treats as
// conservative, not as a rollback.
func TestConfirmationWithoutPredecessor(t *testing.T) {
	f := newConfirmFixture(t, false)
	f.fs.Set(WriteD, FaultNotApplied, nil)
	a := f.commit(t)
	if a.writes[writeB] != nil || a.Result() != ConfirmConservative {
		t.Fatalf("B %+v, result %s", a.writes[writeB], confirmResultName(a.Result()))
	}
	f = newConfirmFixture(t, false)
	a = f.commit(t)
	if a.Result() != ConfirmCommitted || f.confirmedIs(t) != f.n {
		t.Fatalf("result %s", confirmResultName(a.Result()))
	}
	if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 {
		t.Fatalf("finish %d %v", replaced, err)
	}
}

func pMarker(t *testing.T, f *confirmFixture) bool {
	t.Helper()
	_, err := f.local.Lookup(f.p, ActiveOpenName(nfs4.StateID{1}))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// FinishReplacement purges P only after a commit, when B applied (or may
// have) and N's reboot symlink still names P.
func TestConfirmationFinishReplacement(t *testing.T) {
	t.Run("purges P", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		replaced, err := f.store.FinishReplacement(a)
		if err != nil || replaced != uint64(f.p) || pMarker(t, f) {
			t.Fatalf("finish %d %v, P marker kept %t", replaced, err, pMarker(t, f))
		}
		for _, name := range []string{RebootName, f.store.ConfirmingName} {
			if _, err := f.local.Lookup(f.n, name); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s kept: %v", name, err)
			}
		}
	})
	t.Run("B never applied", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		// Another process commits N while B cannot be sent.
		f.fs.Set(writeB, faultPreSend, func() { f.point(t, f.n) })
		a := f.commit(t)
		if a.Result() != ConfirmCommitted {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
			t.Fatalf("finish %d %v purged without B", replaced, err)
		}
	})
	t.Run("reboot names another incarnation", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		if err := f.local.Remove(f.n, RebootName); err != nil {
			t.Fatal(err)
		}
		name, _, _ := f.store.IncarnationName(f.identity, f.m)
		if _, err := f.local.Symlink(f.n, RebootName, "../"+name); err != nil {
			t.Fatal(err)
		}
		if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
			t.Fatalf("finish %d %v purged P for another reboot target", replaced, err)
		}
	})
	t.Run("purge failure is retried", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		f.fs.Mu.Lock()
		f.fs.failRemove = f.p
		f.fs.Mu.Unlock()
		if replaced, err := f.store.FinishReplacement(a); err == nil || replaced != 0 {
			t.Fatalf("finish %d %v", replaced, err)
		}
		if _, err := f.local.Lookup(f.n, RebootName); err != nil {
			t.Fatalf("reboot evidence lost: %v", err)
		}
		if replaced, err := f.store.ResumeReplacementPurge(uint64(f.n)); err != nil || replaced != uint64(f.p) || pMarker(t, f) {
			t.Fatalf("retry %d %v", replaced, err)
		}
	})
}

// pointReboot replaces N/reboot, through the base VFS, with a new symlink to
// incarnation id.
func (f *confirmFixture) pointReboot(t *testing.T, id vfs.InodeID) {
	t.Helper()
	name, found, err := f.store.IncarnationName(f.identity, id)
	if err != nil || !found {
		t.Fatalf("incarnation name %v", err)
	}
	if _, err := f.local.Symlink(f.n, "t.hook", "../"+name); err != nil {
		t.Fatal(err)
	}
	if err := f.local.Rename(f.n, "t.hook", f.n, RebootName); err != nil {
		t.Fatal(err)
	}
}

func markerIn(t *testing.T, fs vfs.TernVFS, dir vfs.InodeID, stateID nfs4.StateID) bool {
	t.Helper()
	_, err := fs.Lookup(dir, ActiveOpenName(stateID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// The purge is bound to the checked P and to the exact reboot edge that
// named it. Another process replacing N/reboot with a pointer to M after the
// check, before or during the purge, does not purge M, and its newer reboot
// symlink is left in place.
func TestConfirmationPurgeBoundToCheckedReboot(t *testing.T) {
	for _, when := range []string{"between check and purge", "during purge"} {
		for _, retry := range []bool{false, true} {
			name := when
			if retry {
				name += "/retry"
			}
			t.Run(name, func(t *testing.T) {
				f := newConfirmFixture(t, true)
				if _, err := f.local.CreateFile(f.m, ActiveOpenName(nfs4.StateID{2}), nil); err != nil {
					t.Fatal(err)
				}
				a := f.commit(t)
				if a.Result() != ConfirmCommitted {
					t.Fatalf("result %s", confirmResultName(a.Result()))
				}
				if retry {
					// The first purge fails at P's first removal; the retry
					// runs with the same proof.
					f.fs.Mu.Lock()
					f.fs.failRemove = f.p
					f.fs.Mu.Unlock()
					if _, err := f.store.FinishReplacement(a); err == nil {
						t.Fatal("injected purge failure ignored")
					}
				}
				hooks := f.fs.OnReaddir
				if when == "during purge" {
					hooks = f.fs.onRemove
				}
				f.fs.Mu.Lock()
				hooks[f.p] = func() { f.pointReboot(t, f.m) }
				f.fs.Mu.Unlock()
				var replaced uint64
				var err error
				if retry {
					replaced, err = f.store.ResumeReplacementPurge(uint64(f.n))
				} else {
					replaced, err = f.store.FinishReplacement(a)
				}
				if err != nil || replaced != uint64(f.p) {
					t.Fatalf("purge %d %v, want P", replaced, err)
				}
				if markerIn(t, f.local, f.p, nfs4.StateID{1}) {
					t.Fatal("P not purged")
				}
				if !markerIn(t, f.local, f.m, nfs4.StateID{2}) {
					t.Fatal("M purged")
				}
				target, found, err := f.store.ReadRebootSymlink(f.n)
				if err != nil || !found || target != f.m {
					t.Fatalf("newer reboot symlink removed: %d %t %v", target, found, err)
				}
			})
		}
	}
}

// A rename that applies and then panics keeps its write record, submitted
// with an unknown outcome. The attempt is unresolved until a settle fences
// it, and the evidence decides: A or B applied without D rolls back; D
// applied commits.
func TestConfirmationAppliedThenPanic(t *testing.T) {
	names := map[WriteID]string{writeA: "A", writeB: "B", WriteD: "D"}
	for _, w := range []WriteID{writeA, writeB, WriteD} {
		t.Run(names[w], func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.Set(w, FaultAppliedPanic, nil)
			a := f.commit(t)
			if a.Result() != confirmUnresolved || a.Panicked == nil {
				t.Fatalf("result %s, panic %v", confirmResultName(a.Result()), a.Panicked)
			}
			rec := a.writes[w]
			if rec == nil || !rec.submitted || rec.err != errOutcomeUnknown || rec.inode == 0 {
				t.Fatalf("write record %+v", rec)
			}
			for later := w + 1; later <= WriteD; later++ {
				if a.writes[later] != nil {
					t.Fatalf("write %s attempted after a panic", names[later])
				}
			}
			want := ConfirmRolledBack
			if w == WriteD {
				want = ConfirmCommitted
			}
			if got := f.store.SettleConfirmation(a); got != want {
				t.Fatalf("settled %s, want %s", confirmResultName(got), confirmResultName(want))
			}
			if rec.outcome() != writeMaybe {
				t.Fatalf("outcome %d", rec.outcome())
			}
		})
	}
}

// A panic while settling leaves the attempt unresolved. A fence that removed
// its temp before panicking is then found edge gone, which may have applied:
// with confirmed still P, that is conservative, never a rollback. A panic in
// the read-back leaves the fences settled and only the read to repeat.
func TestConfirmationSettlementPanic(t *testing.T) {
	t.Run("fence", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		f.fs.Set(WriteD, faultFencePanic, nil)
		a := f.commit(t)
		if a.Result() != confirmUnresolved || a.Panicked == nil {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		if d := a.writes[WriteD]; d.fence != fenceFailed || d.fenceErr != errOutcomeUnknown {
			t.Fatalf("fence record %+v", d)
		}
		f.fs.Set(WriteD, FaultNone, nil)
		if got := f.store.SettleConfirmation(a); got != ConfirmConservative {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
	t.Run("read-back", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		f.fs.Set(WriteD, FaultFenceErr, nil)
		a := f.commit(t)
		if a.Result() != confirmUnresolved {
			t.Fatal(confirmResultName(a.Result()))
		}
		f.fs.Set(WriteD, FaultNone, nil)
		f.fs.Mu.Lock()
		f.fs.panicRead = true
		f.fs.Mu.Unlock()
		if got := f.store.SettleConfirmation(a); got != confirmUnresolved || a.Panicked == nil {
			t.Fatalf("settled %s after read-back panic", confirmResultName(got))
		}
		if d := a.writes[WriteD]; d.fence != fenceNotApplied {
			t.Fatalf("fence record %+v", d)
		}
		if got := f.store.SettleConfirmation(a); got != ConfirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
}

// Concurrent settles of one attempt are serialized: the first fences and
// decides, the rest return its result without fencing or reading again.
func TestConfirmationSettleSerialized(t *testing.T) {
	f := newConfirmFixture(t, true)
	f.fs.Set(WriteD, FaultFenceErr, nil)
	a := f.commit(t)
	if a.Result() != confirmUnresolved {
		t.Fatal(confirmResultName(a.Result()))
	}
	f.fs.Set(WriteD, FaultNone, nil)
	f.fs.Mu.Lock()
	before := f.fs.fences
	f.fs.Mu.Unlock()
	results := make(chan ConfirmResult, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- f.store.SettleConfirmation(a)
		}()
	}
	wg.Wait()
	close(results)
	for got := range results {
		if got != ConfirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	}
	f.fs.Mu.Lock()
	fences := f.fs.fences - before
	f.fs.Mu.Unlock()
	if fences != 1 {
		t.Fatalf("%d fences, want 1", fences)
	}
}

// CommitConfirmation revalidates the prepared P and Sp under the exclusive
// identity lock, after the caller has closed P. A change writes nothing, and
// the read-back classifies the attempt: never a rollback, and a pointer
// naming N commits without purge authority.
func TestConfirmationRevalidatesPredecessor(t *testing.T) {
	for _, c := range []struct {
		change string
		want   ConfirmResult
	}{
		{"P through another symlink", ConfirmConservative},
		{"another incarnation", ConfirmConservative},
		{"absent", ConfirmConservative},
		{"N", ConfirmCommitted},
	} {
		t.Run(c.change, func(t *testing.T) {
			f := newConfirmFixture(t, true)
			switch c.change {
			case "P through another symlink":
				f.point(t, f.p)
			case "another incarnation":
				f.point(t, f.m)
			case "N":
				f.point(t, f.n)
			default:
				f.point(t, 0)
			}
			before := f.confirmedIs(t)
			a := f.store.CommitConfirmation(f.target, f.confirm)
			if !errors.Is(a.PreWrite, errPredecessorChanged) || a.Result() != c.want || a.writes != [3]*edgeWrite{} {
				t.Fatalf("commit %v %s %+v", a.PreWrite, confirmResultName(a.Result()), a.writes)
			}
			if c.want == ConfirmCommitted {
				if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
					t.Fatalf("committed elsewhere purged P: %d %v", replaced, err)
				}
			}
			for _, name := range []string{f.store.ConfirmingName, RebootName} {
				if _, err := f.local.Lookup(f.n, name); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s written: %v", name, err)
				}
			}
			if f.confirmedIs(t) != before {
				t.Fatal("confirmed pointer changed")
			}
		})
	}
}

// A fence counts the remote backend's edge-gone errors and the local VFS's
// inode-pinned not-found as edge gone; anything else leaves it unresolved.
func TestConfirmationEdgeGoneErrors(t *testing.T) {
	for _, err := range []error{os.ErrNotExist, msgs.EDGE_NOT_FOUND, msgs.NAME_NOT_FOUND, msgs.MISMATCHING_CREATION_TIME} {
		if !isEdgeGone(err) {
			t.Fatalf("%v is not edge gone", err)
		}
	}
	for _, err := range []error{errInjectedConfirm, msgs.TIMEOUT, msgs.EDGE_IS_LOCKED} {
		if isEdgeGone(err) {
			t.Fatalf("%v counted as edge gone", err)
		}
	}
	// The local VFS's own answer to fencing a temp that was renamed away.
	local := vfs.NewLocalTernVFS(t.TempDir())
	id, err := local.CreateFile(local.RootID(), "t.x", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	edge, err := local.LookupEdge(local.RootID(), "t.x")
	if err != nil || edge.ID != id {
		t.Fatal(err)
	}
	if err := local.RenameEdge(local.RootID(), "t.x", edge, local.RootID(), "x"); err != nil {
		t.Fatal(err)
	}
	if err := local.RemoveEdge(local.RootID(), "t.x", edge); !isEdgeGone(err) {
		t.Fatalf("local fence after rename: %v", err)
	}
}

// An N that is no longer pending writes nothing. PrepareConfirmation refuses
// it; CommitConfirmation's read-back still finds P through Sp, the approved
// proof that P is unchanged, so the attempt rolls back.
func TestConfirmationRequiresPending(t *testing.T) {
	f := newConfirmFixture(t, true)
	if _, _, err := f.store.SetClientID([8]byte{3}, []byte("client"), ClientOwner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PrepareConfirmation(uint64(f.n), f.confirm); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("prepare %v", err)
	}
	a := f.store.CommitConfirmation(f.target, f.confirm)
	if nfs4.ErrorCode(a.PreWrite) != nfs4.NFS4ERR_STALE_CLIENTID || a.Result() != ConfirmRolledBack || a.writes != [3]*edgeWrite{} {
		t.Fatalf("commit %v %s", a.PreWrite, confirmResultName(a.Result()))
	}
	if f.confirmedIs(t) != f.p {
		t.Fatal("confirmed pointer changed")
	}
}

// A read error before any write gives no evidence about P: the attempt is
// unresolved. Its settlement classifies whatever the pointer then shows:
// unchanged P rolls back, a pointer another process changed is conservative.
func TestConfirmationStorePreWriteReadError(t *testing.T) {
	for _, c := range []struct {
		name   string
		change bool
		want   ConfirmResult
	}{
		{"unchanged", false, ConfirmRolledBack},
		{"changed by another process", true, ConfirmConservative},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.Mu.Lock()
			f.fs.FailLookup = true
			f.fs.Mu.Unlock()
			a := f.store.CommitConfirmation(f.target, f.confirm)
			if a.Result() != confirmUnresolved || a.PreWrite == nil || a.writes != [3]*edgeWrite{} {
				t.Fatalf("commit %v %s", a.PreWrite, confirmResultName(a.Result()))
			}
			// Still unreadable: still unresolved.
			if got := f.store.SettleConfirmation(a); got != confirmUnresolved {
				t.Fatalf("settled %s while unreadable", confirmResultName(got))
			}
			if c.change {
				f.point(t, f.m)
			}
			f.fs.Mu.Lock()
			f.fs.FailLookup = false
			f.fs.Mu.Unlock()
			if got := f.store.SettleConfirmation(a); got != c.want {
				t.Fatalf("settled %s, want %s", confirmResultName(got), confirmResultName(c.want))
			}
			if f.present(t, f.n, f.store.ConfirmingName) || !pMarker(t, f) {
				t.Fatal("durable state changed")
			}
		})
	}
}

func (f *confirmFixture) present(t *testing.T, dir vfs.InodeID, name string) bool {
	t.Helper()
	_, err := f.local.Lookup(dir, name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// A confirmation stuck at a failed fence reports the fence's error, so the
// cause of an unresolved attempt is visible; a later settle that fences
// clears it.
func TestConfirmationSnapshotFenceError(t *testing.T) {
	f := newConfirmFixture(t, true)
	f.fs.Set(WriteD, FaultFenceErr, nil)
	a := f.commit(t)
	if a.Result() != confirmUnresolved {
		t.Fatal(confirmResultName(a.Result()))
	}
	confirmed := func(snap *inspect.Attempt) inspect.ConfirmWrite {
		t.Helper()
		for _, w := range snap.Writes {
			if w.Name == "confirmed" {
				return w
			}
		}
		t.Fatalf("no confirmed write in %+v", snap)
		return inspect.ConfirmWrite{}
	}
	snap := a.Snapshot()
	if snap.Locked || snap.Result != inspect.ConfirmUnresolved {
		t.Fatalf("attempt = %+v", snap)
	}
	w := confirmed(snap)
	if w.Fence != inspect.FenceFailed || w.FenceError == "" || w.Outcome != inspect.WriteUnsettled {
		t.Fatalf("confirmed write = %+v", w)
	}
	if want := a.writes[WriteD].fenceErr.Error(); w.FenceError != want {
		t.Fatalf("fence_error %q, want %q", w.FenceError, want)
	}
	f.fs.Set(WriteD, FaultNone, nil)
	if got := f.store.SettleConfirmation(a); got != ConfirmRolledBack {
		t.Fatalf("settled %s", confirmResultName(got))
	}
	w = confirmed(a.Snapshot())
	if w.Fence != inspect.FenceNotApplied || w.FenceError != "" || w.Outcome != inspect.WriteNever {
		t.Fatalf("settled write = %+v", w)
	}
}
