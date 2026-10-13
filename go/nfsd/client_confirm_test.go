// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

type confirmFault uint8

const (
	faultNone confirmFault = iota
	faultPreSend
	faultAppliedErr   // the rename applies, then reports an error
	faultNotApplied   // the rename reports an error without applying
	faultFenceErr     // not applied, and the fence fails
	faultAppliedFail  // applied with an error, and the fence fails
	faultAppliedPanic // the rename applies, then panics
	faultFencePanic   // not applied; the fence removes the temp, then panics
)

var errInjectedConfirm = errors.New("injected confirmation failure")

type edgeKey struct {
	dir  InodeID
	name string
}

// confirmFaultVFS gives temps creation times as the remote backend does:
// LookupEdge returns the edge's own time, and RenameEdge and RemoveEdge of a
// temp require it, answering EDGE_NOT_FOUND or MISMATCHING_CREATION_TIME
// when the edge is gone. Faults apply per write, recognised by the temp it
// creates: a file is A, a "../" symlink is B, any other symlink is D.
type confirmFaultVFS struct {
	TernVFS
	mu         sync.Mutex
	next       uint64
	creation   map[edgeKey]uint64
	kinds      map[edgeKey]writeID
	armed      bool
	faults     map[writeID]confirmFault
	hooks      map[writeID]func() // run inside the faulted call
	failLookup bool               // the confirmed read-back fails
	panicRead  bool               // the confirmed read-back panics, once
	failRemove InodeID            // one Remove in this directory fails
	// onReaddir and onRemove run once, before the first Readdir or Remove
	// in their directory.
	onReaddir, onRemove map[InodeID]func()
	fences              int // RemoveEdge calls on armed temps
	// lookupHook runs once, before the skip+1th Lookup of name in dir.
	lookupHook struct {
		dir  InodeID
		name string
		skip int
		fn   func()
	}
	// failLeaseRemove fails one removal of a lease slot in this directory.
	failLeaseRemove InodeID
	// panicRemove panics once, at the skip+1th Remove in dir.
	panicRemove struct {
		dir  InodeID
		skip int
	}
	// createHook runs once, before CreateFile of name in dir.
	createHook struct {
		dir  InodeID
		name string
		fn   func()
	}
}

func (fs *confirmFaultVFS) once(hooks map[InodeID]func(), dir InodeID) {
	fs.mu.Lock()
	hook := hooks[dir]
	delete(hooks, dir)
	fs.mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (fs *confirmFaultVFS) Readdir(dir InodeID, cursor uint64) ([]DirEntry, uint64, error) {
	fs.once(fs.onReaddir, dir)
	return fs.TernVFS.Readdir(dir, cursor)
}

func newConfirmFaultVFS(t *testing.T) *confirmFaultVFS {
	return &confirmFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		creation: make(map[edgeKey]uint64), kinds: make(map[edgeKey]writeID),
		faults: make(map[writeID]confirmFault), hooks: make(map[writeID]func()),
		onReaddir: make(map[InodeID]func()), onRemove: make(map[InodeID]func())}
}

func (fs *confirmFaultVFS) record(dir InodeID, name string, kind writeID) {
	if !strings.HasPrefix(name, tempPrefix) {
		return
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.next++
	key := edgeKey{dir, name}
	fs.creation[key] = fs.next
	if fs.armed {
		fs.kinds[key] = kind
	}
}

// fault returns the write's fault and hook, if it is one of the armed temps.
func (fs *confirmFaultVFS) fault(dir InodeID, name string) (confirmFault, func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	kind, ok := fs.kinds[edgeKey{dir, name}]
	if !ok {
		return faultNone, nil
	}
	return fs.faults[kind], fs.hooks[kind]
}

func (fs *confirmFaultVFS) CreateFile(dir InodeID, name string, data io.Reader) (InodeID, error) {
	fs.mu.Lock()
	var hook func()
	if h := &fs.createHook; h.fn != nil && h.dir == dir && h.name == name {
		hook, h.fn = h.fn, nil
	}
	fs.mu.Unlock()
	if hook != nil {
		hook()
	}
	id, err := fs.TernVFS.CreateFile(dir, name, data)
	if err == nil {
		fs.record(dir, name, writeA)
	}
	return id, err
}

// writeLeaseTemp marks the refresher's lease temps, which LinkFile creates.
// No fault is ever set for it.
const writeLeaseTemp writeID = 99

func (fs *confirmFaultVFS) LinkFile(id InodeID, cookie Cookie, dir InodeID, name string, data io.Reader) error {
	if err := fs.TernVFS.LinkFile(id, cookie, dir, name, data); err != nil {
		return err
	}
	fs.record(dir, name, writeLeaseTemp)
	return nil
}

func (fs *confirmFaultVFS) Symlink(dir InodeID, name, target string) (InodeID, error) {
	id, err := fs.TernVFS.Symlink(dir, name, target)
	if err == nil {
		kind := writeD
		if strings.HasPrefix(target, "../") {
			kind = writeB
		}
		fs.record(dir, name, kind)
	}
	return id, err
}

func (fs *confirmFaultVFS) Lookup(dir InodeID, name string) (InodeID, error) {
	fs.mu.Lock()
	fail := fs.failLookup && name == confirmedName
	panicking := fs.panicRead && name == confirmedName
	if panicking {
		fs.panicRead = false
	}
	var hook func()
	if h := &fs.lookupHook; h.fn != nil && h.dir == dir && h.name == name {
		if h.skip == 0 {
			hook, h.fn = h.fn, nil
		} else {
			h.skip--
		}
	}
	fs.mu.Unlock()
	if hook != nil {
		hook()
	}
	if panicking {
		panic("injected read-back panic")
	}
	if fail {
		return 0, errInjectedConfirm
	}
	return fs.TernVFS.Lookup(dir, name)
}

func (fs *confirmFaultVFS) Remove(dir InodeID, name string) error {
	fs.once(fs.onRemove, dir)
	fs.mu.Lock()
	fail := fs.failRemove == dir
	if fail {
		fs.failRemove = 0
	}
	if fs.failLeaseRemove == dir && strings.HasPrefix(name, leasePrefix) {
		fs.failLeaseRemove, fail = 0, true
	}
	panicking := false
	if p := &fs.panicRemove; p.dir != 0 && p.dir == dir {
		if p.skip == 0 {
			p.dir, panicking = 0, true
		} else {
			p.skip--
		}
	}
	fs.mu.Unlock()
	if panicking {
		panic("injected panic in Remove")
	}
	if fail {
		return errInjectedConfirm
	}
	return fs.TernVFS.Remove(dir, name)
}

// LookupEdge is overridden itself: LocalTernVFS.LookupEdge calls its own
// Lookup and has no creation times.
func (fs *confirmFaultVFS) LookupEdge(dir InodeID, name string) (Edge, error) {
	fault, hook := fs.fault(dir, name)
	if fault == faultPreSend {
		if hook != nil {
			hook()
		}
		return Edge{}, errInjectedConfirm
	}
	id, err := fs.TernVFS.Lookup(dir, name)
	if err != nil {
		return Edge{}, err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return Edge{ID: id, CreationTime: fs.creation[edgeKey{dir, name}]}, nil
}

// checkEdge reports the remote backend's error for a stale temp edge.
func (fs *confirmFaultVFS) checkEdge(dir InodeID, name string, edge Edge) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if !strings.HasPrefix(name, tempPrefix) {
		return nil
	}
	created, ok := fs.creation[edgeKey{dir, name}]
	switch {
	case !ok:
		return msgs.EDGE_NOT_FOUND
	case created != edge.CreationTime:
		return msgs.MISMATCHING_CREATION_TIME
	}
	return nil
}

func (fs *confirmFaultVFS) forget(dir InodeID, name string) {
	fs.mu.Lock()
	delete(fs.creation, edgeKey{dir, name})
	fs.mu.Unlock()
}

func (fs *confirmFaultVFS) RenameEdge(src InodeID, name string, edge Edge, dst InodeID, newName string) error {
	if err := fs.checkEdge(src, name, edge); err != nil {
		return err
	}
	fault, hook := fs.fault(src, name)
	switch fault {
	case faultNotApplied, faultFenceErr, faultFencePanic:
		if hook != nil {
			hook()
		}
		return errInjectedConfirm
	}
	if err := fs.TernVFS.RenameEdge(src, name, edge, dst, newName); err != nil {
		return err
	}
	fs.forget(src, name)
	switch fault {
	case faultAppliedErr, faultAppliedFail:
		if hook != nil {
			hook()
		}
		return errInjectedConfirm
	case faultAppliedPanic:
		panic("injected panic after rename")
	}
	return nil
}

func (fs *confirmFaultVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	fault, _ := fs.fault(dir, name)
	fs.mu.Lock()
	if _, armed := fs.kinds[edgeKey{dir, name}]; armed {
		fs.fences++
	}
	fs.mu.Unlock()
	if fault == faultFenceErr || fault == faultAppliedFail {
		return errInjectedConfirm
	}
	if err := fs.checkEdge(dir, name, edge); err != nil {
		return err
	}
	if err := fs.TernVFS.RemoveEdge(dir, name, edge); err != nil {
		return err
	}
	fs.forget(dir, name)
	if fault == faultFencePanic {
		panic("injected panic after fence")
	}
	return nil
}

func (fs *confirmFaultVFS) set(w writeID, fault confirmFault, hook func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.faults[w], fs.hooks[w] = fault, hook
}

type confirmFixture struct {
	fs                *confirmFaultVFS
	local             TernVFS
	store             *ClientStore
	identity, p, n, m InodeID
	confirm           [8]byte
	target            confirmTarget
}

// newConfirmFixture builds a pending N, and with withP a confirmed P that
// has a lease and an open marker for the purge to remove.
func newConfirmFixture(t *testing.T, withP bool) *confirmFixture {
	t.Helper()
	fs := newConfirmFaultVFS(t)
	f := &confirmFixture{fs: fs, local: fs.TernVFS}
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	f.store = store
	if withP {
		p, confirm, err := store.SetClientID([8]byte{1}, []byte("client"), clientOwner{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.ConfirmClientID(p, confirm, rpcPrincipal{}); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkOpen(p, StateID{1}); err != nil {
			t.Fatal(err)
		}
		f.p = InodeID(p)
	}
	n, confirm, err := store.SetClientID([8]byte{2}, []byte("client"), clientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	f.n, f.confirm = InodeID(n), confirm
	if f.identity, err = f.local.LookupParent(f.n); err != nil {
		t.Fatal(err)
	}
	if f.m, err = store.newIncarnation(f.identity); err != nil {
		t.Fatal(err)
	}
	if f.target, err = store.PrepareConfirmation(n, confirm); err != nil {
		t.Fatal(err)
	}
	if f.target.n != f.n || f.target.p != f.p || (f.target.sp != 0) != withP {
		t.Fatalf("target %+v", f.target)
	}
	return f
}

// point replaces identity/confirmed, through the base VFS, with a new
// symlink to incarnation id, or removes it when id is zero.
func (f *confirmFixture) point(t *testing.T, id InodeID) {
	t.Helper()
	if id == 0 {
		if err := f.local.Remove(f.identity, confirmedName); err != nil {
			t.Fatal(err)
		}
		return
	}
	name, found, err := f.store.incarnationName(f.identity, id)
	if err != nil || !found {
		t.Fatalf("incarnation name %v", err)
	}
	if _, err := f.local.Symlink(f.identity, "t.hook", name); err != nil {
		t.Fatal(err)
	}
	if err := f.local.Rename(f.identity, "t.hook", f.identity, confirmedName); err != nil {
		t.Fatal(err)
	}
}

func (f *confirmFixture) commit(t *testing.T) *confirmAttempt {
	t.Helper()
	f.fs.mu.Lock()
	f.fs.armed = true
	f.fs.mu.Unlock()
	return f.store.CommitConfirmation(f.target, f.confirm)
}

func (f *confirmFixture) confirmedIs(t *testing.T) InodeID {
	t.Helper()
	id, found, err := f.store.readIdentitySymlink(f.identity, confirmedName)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return 0
	}
	return id
}

func confirmResultName(r confirmResult) string {
	return [...]string{"unresolved", "committed", "rolled back", "conservative"}[r]
}

// Each write's faults with an unchanged predecessor. Only D can commit, and
// only on proof; an unfenceable write stays unresolved until a later
// settle succeeds.
func TestConfirmationWriteFaults(t *testing.T) {
	names := map[writeID]string{writeA: "A", writeB: "B", writeD: "D"}
	for _, w := range []writeID{writeA, writeB, writeD} {
		for _, c := range []struct {
			name             string
			fault            confirmFault
			submitted        bool
			want, afterClear confirmResult
			outcome          writeOutcome
		}{
			{"pre-send", faultPreSend, false, confirmRolledBack, confirmRolledBack, writeNever},
			{"applied with an error", faultAppliedErr, true, confirmRolledBack, confirmRolledBack, writeMaybe},
			{"not applied", faultNotApplied, true, confirmRolledBack, confirmRolledBack, writeNever},
			{"fence error", faultFenceErr, true, confirmUnresolved, confirmRolledBack, writeNever},
			{"applied, fence error", faultAppliedFail, true, confirmUnresolved, confirmRolledBack, writeMaybe},
		} {
			t.Run(names[w]+"/"+c.name, func(t *testing.T) {
				f := newConfirmFixture(t, true)
				f.fs.set(w, c.fault, nil)
				want, afterClear := c.want, c.afterClear
				if w == writeD && (c.fault == faultAppliedErr || c.fault == faultAppliedFail) {
					// D applied: the read-back finds N through D's inode.
					want, afterClear = confirmCommitted, confirmCommitted
					if c.fault == faultAppliedFail {
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
				for later := w + 1; later <= writeD; later++ {
					if a.writes[later] != nil {
						t.Fatalf("write %s attempted after %s failed", names[later], names[w])
					}
				}
				f.fs.set(w, faultNone, nil)
				if got := f.store.SettleConfirmation(a); got != afterClear {
					t.Fatalf("settled %s, want %s", confirmResultName(got), confirmResultName(afterClear))
				}
				if rec.outcome() != c.outcome {
					t.Fatalf("write outcome %d, want %d", rec.outcome(), c.outcome)
				}
				wantConfirmed := f.p
				if afterClear == confirmCommitted {
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
		fault confirmFault
		to    func(f *confirmFixture) InodeID
		want  confirmResult
	}{
		{"never applied, P through another symlink", faultNotApplied, func(f *confirmFixture) InodeID { return f.p }, confirmConservative},
		{"never applied, another incarnation M", faultNotApplied, func(f *confirmFixture) InodeID { return f.m }, confirmConservative},
		{"never applied, absent", faultNotApplied, func(f *confirmFixture) InodeID { return 0 }, confirmConservative},
		{"never applied, N through another symlink", faultNotApplied, func(f *confirmFixture) InodeID { return f.n }, confirmCommitted},
		{"edge gone, P", faultAppliedErr, func(f *confirmFixture) InodeID { return f.p }, confirmConservative},
		{"edge gone, M", faultAppliedErr, func(f *confirmFixture) InodeID { return f.m }, confirmConservative},
		{"edge gone, absent", faultAppliedErr, func(f *confirmFixture) InodeID { return 0 }, confirmConservative},
		{"edge gone, N through another symlink", faultAppliedErr, func(f *confirmFixture) InodeID { return f.n }, confirmCommitted},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.set(writeD, c.fault, func() { f.point(t, c.to(f)) })
			if got := f.commit(t).Result(); got != c.want {
				t.Fatalf("result %s, want %s", confirmResultName(got), confirmResultName(c.want))
			}
		})
	}
	t.Run("read error", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		f.fs.set(writeD, faultNotApplied, func() { f.fs.failLookup = true })
		a := f.commit(t)
		if a.Result() != confirmUnresolved {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		f.fs.mu.Lock()
		f.fs.failLookup = false
		f.fs.mu.Unlock()
		if got := f.store.SettleConfirmation(a); got != confirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
	t.Run("success", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		if a.Result() != confirmCommitted || a.observed.file != a.writes[writeD].inode {
			t.Fatalf("result %s, observed %+v", confirmResultName(a.Result()), a.observed)
		}
	})
}

// A first confirmation has no predecessor: no B, and a D that never applied
// observes an absent pointer, which the approved table treats as
// conservative, not as a rollback.
func TestConfirmationWithoutPredecessor(t *testing.T) {
	f := newConfirmFixture(t, false)
	f.fs.set(writeD, faultNotApplied, nil)
	a := f.commit(t)
	if a.writes[writeB] != nil || a.Result() != confirmConservative {
		t.Fatalf("B %+v, result %s", a.writes[writeB], confirmResultName(a.Result()))
	}
	f = newConfirmFixture(t, false)
	a = f.commit(t)
	if a.Result() != confirmCommitted || f.confirmedIs(t) != f.n {
		t.Fatalf("result %s", confirmResultName(a.Result()))
	}
	if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 {
		t.Fatalf("finish %d %v", replaced, err)
	}
}

func pMarker(t *testing.T, f *confirmFixture) bool {
	t.Helper()
	_, err := f.local.Lookup(f.p, activeOpenName(StateID{1}))
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
		for _, name := range []string{rebootName, f.store.confirmingName} {
			if _, err := f.local.Lookup(f.n, name); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s kept: %v", name, err)
			}
		}
	})
	t.Run("B never applied", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		// Another process commits N while B cannot be sent.
		f.fs.set(writeB, faultPreSend, func() { f.point(t, f.n) })
		a := f.commit(t)
		if a.Result() != confirmCommitted {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
			t.Fatalf("finish %d %v purged without B", replaced, err)
		}
	})
	t.Run("reboot names another incarnation", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		if err := f.local.Remove(f.n, rebootName); err != nil {
			t.Fatal(err)
		}
		name, _, _ := f.store.incarnationName(f.identity, f.m)
		if _, err := f.local.Symlink(f.n, rebootName, "../"+name); err != nil {
			t.Fatal(err)
		}
		if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
			t.Fatalf("finish %d %v purged P for another reboot target", replaced, err)
		}
	})
	t.Run("purge failure is retried", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		a := f.commit(t)
		f.fs.mu.Lock()
		f.fs.failRemove = f.p
		f.fs.mu.Unlock()
		if replaced, err := f.store.FinishReplacement(a); err == nil || replaced != 0 {
			t.Fatalf("finish %d %v", replaced, err)
		}
		if _, err := f.local.Lookup(f.n, rebootName); err != nil {
			t.Fatalf("reboot evidence lost: %v", err)
		}
		if replaced, err := f.store.ResumeReplacementPurge(uint64(f.n)); err != nil || replaced != uint64(f.p) || pMarker(t, f) {
			t.Fatalf("retry %d %v", replaced, err)
		}
	})
}

// pointReboot replaces N/reboot, through the base VFS, with a new symlink to
// incarnation id.
func (f *confirmFixture) pointReboot(t *testing.T, id InodeID) {
	t.Helper()
	name, found, err := f.store.incarnationName(f.identity, id)
	if err != nil || !found {
		t.Fatalf("incarnation name %v", err)
	}
	if _, err := f.local.Symlink(f.n, "t.hook", "../"+name); err != nil {
		t.Fatal(err)
	}
	if err := f.local.Rename(f.n, "t.hook", f.n, rebootName); err != nil {
		t.Fatal(err)
	}
}

func markerIn(t *testing.T, fs TernVFS, dir InodeID, stateID StateID) bool {
	t.Helper()
	_, err := fs.Lookup(dir, activeOpenName(stateID))
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
				if _, err := f.local.CreateFile(f.m, activeOpenName(StateID{2}), nil); err != nil {
					t.Fatal(err)
				}
				a := f.commit(t)
				if a.Result() != confirmCommitted {
					t.Fatalf("result %s", confirmResultName(a.Result()))
				}
				if retry {
					// The first purge fails at P's first removal; the retry
					// runs with the same proof.
					f.fs.mu.Lock()
					f.fs.failRemove = f.p
					f.fs.mu.Unlock()
					if _, err := f.store.FinishReplacement(a); err == nil {
						t.Fatal("injected purge failure ignored")
					}
				}
				hooks := f.fs.onReaddir
				if when == "during purge" {
					hooks = f.fs.onRemove
				}
				f.fs.mu.Lock()
				hooks[f.p] = func() { f.pointReboot(t, f.m) }
				f.fs.mu.Unlock()
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
				if markerIn(t, f.local, f.p, StateID{1}) {
					t.Fatal("P not purged")
				}
				if !markerIn(t, f.local, f.m, StateID{2}) {
					t.Fatal("M purged")
				}
				target, found, err := f.store.readRebootSymlink(f.n)
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
	names := map[writeID]string{writeA: "A", writeB: "B", writeD: "D"}
	for _, w := range []writeID{writeA, writeB, writeD} {
		t.Run(names[w], func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.set(w, faultAppliedPanic, nil)
			a := f.commit(t)
			if a.Result() != confirmUnresolved || a.panicked == nil {
				t.Fatalf("result %s, panic %v", confirmResultName(a.Result()), a.panicked)
			}
			rec := a.writes[w]
			if rec == nil || !rec.submitted || rec.err != errOutcomeUnknown || rec.inode == 0 {
				t.Fatalf("write record %+v", rec)
			}
			for later := w + 1; later <= writeD; later++ {
				if a.writes[later] != nil {
					t.Fatalf("write %s attempted after a panic", names[later])
				}
			}
			want := confirmRolledBack
			if w == writeD {
				want = confirmCommitted
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
		f.fs.set(writeD, faultFencePanic, nil)
		a := f.commit(t)
		if a.Result() != confirmUnresolved || a.panicked == nil {
			t.Fatalf("result %s", confirmResultName(a.Result()))
		}
		if d := a.writes[writeD]; d.fence != fenceFailed || d.fenceErr != errOutcomeUnknown {
			t.Fatalf("fence record %+v", d)
		}
		f.fs.set(writeD, faultNone, nil)
		if got := f.store.SettleConfirmation(a); got != confirmConservative {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
	t.Run("read-back", func(t *testing.T) {
		f := newConfirmFixture(t, true)
		f.fs.set(writeD, faultFenceErr, nil)
		a := f.commit(t)
		if a.Result() != confirmUnresolved {
			t.Fatal(confirmResultName(a.Result()))
		}
		f.fs.set(writeD, faultNone, nil)
		f.fs.mu.Lock()
		f.fs.panicRead = true
		f.fs.mu.Unlock()
		if got := f.store.SettleConfirmation(a); got != confirmUnresolved || a.panicked == nil {
			t.Fatalf("settled %s after read-back panic", confirmResultName(got))
		}
		if d := a.writes[writeD]; d.fence != fenceNotApplied {
			t.Fatalf("fence record %+v", d)
		}
		if got := f.store.SettleConfirmation(a); got != confirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	})
}

// Concurrent settles of one attempt are serialized: the first fences and
// decides, the rest return its result without fencing or reading again.
func TestConfirmationSettleSerialized(t *testing.T) {
	f := newConfirmFixture(t, true)
	f.fs.set(writeD, faultFenceErr, nil)
	a := f.commit(t)
	if a.Result() != confirmUnresolved {
		t.Fatal(confirmResultName(a.Result()))
	}
	f.fs.set(writeD, faultNone, nil)
	f.fs.mu.Lock()
	before := f.fs.fences
	f.fs.mu.Unlock()
	results := make(chan confirmResult, 8)
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
		if got != confirmRolledBack {
			t.Fatalf("settled %s", confirmResultName(got))
		}
	}
	f.fs.mu.Lock()
	fences := f.fs.fences - before
	f.fs.mu.Unlock()
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
		want   confirmResult
	}{
		{"P through another symlink", confirmConservative},
		{"another incarnation", confirmConservative},
		{"absent", confirmConservative},
		{"N", confirmCommitted},
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
			if !errors.Is(a.preWrite, errPredecessorChanged) || a.Result() != c.want || a.writes != [3]*edgeWrite{} {
				t.Fatalf("commit %v %s %+v", a.preWrite, confirmResultName(a.Result()), a.writes)
			}
			if c.want == confirmCommitted {
				if replaced, err := f.store.FinishReplacement(a); err != nil || replaced != 0 || !pMarker(t, f) {
					t.Fatalf("committed elsewhere purged P: %d %v", replaced, err)
				}
			}
			for _, name := range []string{f.store.confirmingName, rebootName} {
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
	local := NewLocalTernVFS(t.TempDir())
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
	if _, _, err := f.store.SetClientID([8]byte{3}, []byte("client"), clientOwner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PrepareConfirmation(uint64(f.n), f.confirm); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("prepare %v", err)
	}
	a := f.store.CommitConfirmation(f.target, f.confirm)
	if nfs4.ErrorCode(a.preWrite) != nfs4.NFS4ERR_STALE_CLIENTID || a.Result() != confirmRolledBack || a.writes != [3]*edgeWrite{} {
		t.Fatalf("commit %v %s", a.preWrite, confirmResultName(a.Result()))
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
		want   confirmResult
	}{
		{"unchanged", false, confirmRolledBack},
		{"changed by another process", true, confirmConservative},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newConfirmFixture(t, true)
			f.fs.mu.Lock()
			f.fs.failLookup = true
			f.fs.mu.Unlock()
			a := f.store.CommitConfirmation(f.target, f.confirm)
			if a.Result() != confirmUnresolved || a.preWrite == nil || a.writes != [3]*edgeWrite{} {
				t.Fatalf("commit %v %s", a.preWrite, confirmResultName(a.Result()))
			}
			// Still unreadable: still unresolved.
			if got := f.store.SettleConfirmation(a); got != confirmUnresolved {
				t.Fatalf("settled %s while unreadable", confirmResultName(got))
			}
			if c.change {
				f.point(t, f.m)
			}
			f.fs.mu.Lock()
			f.fs.failLookup = false
			f.fs.mu.Unlock()
			if got := f.store.SettleConfirmation(a); got != c.want {
				t.Fatalf("settled %s, want %s", confirmResultName(got), confirmResultName(c.want))
			}
			if f.present(t, f.n, f.store.confirmingName) || !pMarker(t, f) {
				t.Fatal("durable state changed")
			}
		})
	}
}

func (f *confirmFixture) present(t *testing.T, dir InodeID, name string) bool {
	t.Helper()
	_, err := f.local.Lookup(dir, name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}
