// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// testingT is the part of testing.T the hooks use, so that this package
// does not import testing.
type testingT interface {
	Cleanup(func())
	Fatal(args ...any)
	Helper()
	TempDir() string
}

type ConfirmFault uint8

const (
	FaultNone ConfirmFault = iota
	faultPreSend
	faultAppliedErr   // the rename applies, then reports an error
	FaultNotApplied   // the rename reports an error without applying
	FaultFenceErr     // not applied, and the fence fails
	FaultAppliedFail  // applied with an error, and the fence fails
	FaultAppliedPanic // the rename applies, then panics
	faultFencePanic   // not applied; the fence removes the temp, then panics
)

// ConfirmFaultVFS gives temps creation times as the remote backend does:
// LookupEdge returns the edge's own time, and RenameEdge and RemoveEdge of a
// temp require it, answering EDGE_NOT_FOUND or MISMATCHING_CREATION_TIME
// when the edge is gone. Faults apply per write, recognised by the temp it
// creates: a file is A, a "../" symlink is B, any other symlink is D.
type ConfirmFaultVFS struct {
	vfs.TernVFS
	Mu         sync.Mutex
	next       uint64
	creation   map[edgeKey]uint64
	kinds      map[edgeKey]WriteID
	Armed      bool
	faults     map[WriteID]ConfirmFault
	hooks      map[WriteID]func() // run inside the faulted call
	FailLookup bool               // the confirmed read-back fails
	panicRead  bool               // the confirmed read-back panics, once
	failRemove vfs.InodeID        // one Remove in this directory fails
	// onReaddir and onRemove run once, before the first Readdir or Remove
	// in their directory.
	OnReaddir, onRemove map[vfs.InodeID]func()
	fences              int // RemoveEdge calls on armed temps
	// lookupHook runs once, before the skip+1th Lookup of name in dir.
	LookupHook struct {
		Dir  vfs.InodeID
		Name string
		Skip int
		Fn   func()
	}
	// failLeaseRemove fails one removal of a lease slot in this directory.
	FailLeaseRemove vfs.InodeID
	// panicRemove panics once, at the skip+1th Remove in dir.
	PanicRemove struct {
		Dir  vfs.InodeID
		Skip int
	}
	// createHook runs once, before CreateFile of name in dir.
	CreateHook struct {
		Dir  vfs.InodeID
		Name string
		Fn   func()
	}
}

func (fs *ConfirmFaultVFS) once(hooks map[vfs.InodeID]func(), dir vfs.InodeID) {
	fs.Mu.Lock()
	hook := hooks[dir]
	delete(hooks, dir)
	fs.Mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (fs *ConfirmFaultVFS) Readdir(dir vfs.InodeID, cursor uint64) ([]vfs.DirEntry, uint64, error) {
	fs.once(fs.OnReaddir, dir)
	return fs.TernVFS.Readdir(dir, cursor)
}

func NewConfirmFaultVFS(t testingT) *ConfirmFaultVFS {
	return &ConfirmFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		creation: make(map[edgeKey]uint64), kinds: make(map[edgeKey]WriteID),
		faults: make(map[WriteID]ConfirmFault), hooks: make(map[WriteID]func()),
		OnReaddir: make(map[vfs.InodeID]func()), onRemove: make(map[vfs.InodeID]func())}
}

func (fs *ConfirmFaultVFS) record(dir vfs.InodeID, name string, kind WriteID) {
	if !strings.HasPrefix(name, TempPrefix) {
		return
	}
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	fs.next++
	key := edgeKey{dir, name}
	fs.creation[key] = fs.next
	if fs.Armed {
		fs.kinds[key] = kind
	}
}

// fault returns the write's fault and hook, if it is one of the armed temps.
func (fs *ConfirmFaultVFS) fault(dir vfs.InodeID, name string) (ConfirmFault, func()) {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	kind, ok := fs.kinds[edgeKey{dir, name}]
	if !ok {
		return FaultNone, nil
	}
	return fs.faults[kind], fs.hooks[kind]
}

func (fs *ConfirmFaultVFS) CreateFile(dir vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	fs.Mu.Lock()
	var hook func()
	if h := &fs.CreateHook; h.Fn != nil && h.Dir == dir && h.Name == name {
		hook, h.Fn = h.Fn, nil
	}
	fs.Mu.Unlock()
	if hook != nil {
		hook()
	}
	id, err := fs.TernVFS.CreateFile(dir, name, data)
	if err == nil {
		fs.record(dir, name, writeA)
	}
	return id, err
}

func (fs *ConfirmFaultVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	if err := fs.TernVFS.LinkFile(id, cookie, dir, name, data); err != nil {
		return err
	}
	fs.record(dir, name, writeLeaseTemp)
	return nil
}

func (fs *ConfirmFaultVFS) Symlink(dir vfs.InodeID, name, target string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Symlink(dir, name, target)
	if err == nil {
		kind := WriteD
		if strings.HasPrefix(target, "../") {
			kind = writeB
		}
		fs.record(dir, name, kind)
	}
	return id, err
}

func (fs *ConfirmFaultVFS) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	fs.Mu.Lock()
	fail := fs.FailLookup && name == ConfirmedName
	panicking := fs.panicRead && name == ConfirmedName
	if panicking {
		fs.panicRead = false
	}
	var hook func()
	if h := &fs.LookupHook; h.Fn != nil && h.Dir == dir && h.Name == name {
		if h.Skip == 0 {
			hook, h.Fn = h.Fn, nil
		} else {
			h.Skip--
		}
	}
	fs.Mu.Unlock()
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

func (fs *ConfirmFaultVFS) Remove(dir vfs.InodeID, name string) error {
	fs.once(fs.onRemove, dir)
	fs.Mu.Lock()
	fail := fs.failRemove == dir
	if fail {
		fs.failRemove = 0
	}
	if fs.FailLeaseRemove == dir && strings.HasPrefix(name, LeasePrefix) {
		fs.FailLeaseRemove, fail = 0, true
	}
	panicking := false
	if p := &fs.PanicRemove; p.Dir != 0 && p.Dir == dir {
		if p.Skip == 0 {
			p.Dir, panicking = 0, true
		} else {
			p.Skip--
		}
	}
	fs.Mu.Unlock()
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
func (fs *ConfirmFaultVFS) LookupEdge(dir vfs.InodeID, name string) (vfs.Edge, error) {
	fault, hook := fs.fault(dir, name)
	if fault == faultPreSend {
		if hook != nil {
			hook()
		}
		return vfs.Edge{}, errInjectedConfirm
	}
	id, err := fs.TernVFS.Lookup(dir, name)
	if err != nil {
		return vfs.Edge{}, err
	}
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	return vfs.Edge{ID: id, CreationTime: fs.creation[edgeKey{dir, name}]}, nil
}

// checkEdge reports the remote backend's error for a stale temp edge.
func (fs *ConfirmFaultVFS) checkEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	if !strings.HasPrefix(name, TempPrefix) {
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

func (fs *ConfirmFaultVFS) forget(dir vfs.InodeID, name string) {
	fs.Mu.Lock()
	delete(fs.creation, edgeKey{dir, name})
	fs.Mu.Unlock()
}

func (fs *ConfirmFaultVFS) RenameEdge(src vfs.InodeID, name string, edge vfs.Edge, dst vfs.InodeID, newName string) error {
	if err := fs.checkEdge(src, name, edge); err != nil {
		return err
	}
	fault, hook := fs.fault(src, name)
	switch fault {
	case FaultNotApplied, FaultFenceErr, faultFencePanic:
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
	case faultAppliedErr, FaultAppliedFail:
		if hook != nil {
			hook()
		}
		return errInjectedConfirm
	case FaultAppliedPanic:
		panic("injected panic after rename")
	}
	return nil
}

func (fs *ConfirmFaultVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	fault, _ := fs.fault(dir, name)
	fs.Mu.Lock()
	if _, armed := fs.kinds[edgeKey{dir, name}]; armed {
		fs.fences++
	}
	fs.Mu.Unlock()
	if fault == FaultFenceErr || fault == FaultAppliedFail {
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

func (fs *ConfirmFaultVFS) Set(w WriteID, fault ConfirmFault, hook func()) {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	fs.faults[w], fs.hooks[w] = fault, hook
}

// collectStaleForClient and collectStale run a collection pass and discard
// the recheck flag. Production code uses collectStaleForClientResult.
func (cs *Store) CollectStaleForClient(clientID vfs.InodeID) error {
	_, err := cs.CollectStaleForClientResult(clientID)
	return err
}

func (cs *Store) collectStale(identityID vfs.InodeID) error {
	_, err := cs.collectStaleResult(identityID)
	return err
}

func NewConfirmedStoreClient(
	t testingT,
	fs vfs.TernVFS,
	identity []byte,
	verifier [8]byte,
	owner ClientOwner,
) (*Store, uint64) {
	t.Helper()
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	clientID, confirm, err := store.SetClientID(
		verifier, identity, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	return store, clientID
}

type CountingVFS struct {
	vfs.TernVFS

	mu           sync.Mutex
	lookup       int
	lookupParent int
	readdir      int
	stat         int
	read         int
	readlink     int
	CreateFiles  int
	Renames      int
}

func (fs *CountingVFS) Lookup(dirID vfs.InodeID, name string) (vfs.InodeID, error) {
	fs.mu.Lock()
	fs.lookup++
	fs.mu.Unlock()
	return fs.TernVFS.Lookup(dirID, name)
}

func (fs *CountingVFS) LookupParent(id vfs.InodeID) (vfs.InodeID, error) {
	fs.mu.Lock()
	fs.lookupParent++
	fs.mu.Unlock()
	return fs.TernVFS.LookupParent(id)
}

func (fs *CountingVFS) Readdir(
	dirID vfs.InodeID,
	startHash uint64,
) ([]vfs.DirEntry, uint64, error) {
	fs.mu.Lock()
	fs.readdir++
	fs.mu.Unlock()
	return fs.TernVFS.Readdir(dirID, startHash)
}

func (fs *CountingVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	fs.mu.Lock()
	fs.stat++
	fs.mu.Unlock()
	return fs.TernVFS.Stat(id)
}

func (fs *CountingVFS) Read(
	id vfs.InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	fs.mu.Lock()
	fs.read++
	fs.mu.Unlock()
	return fs.TernVFS.Read(id, offset, dest)
}

func (fs *CountingVFS) Readlink(id vfs.InodeID) (string, error) {
	fs.mu.Lock()
	fs.readlink++
	fs.mu.Unlock()
	return fs.TernVFS.Readlink(id)
}

func (fs *CountingVFS) CreateFile(
	dirID vfs.InodeID,
	name string,
	data io.Reader,
) (vfs.InodeID, error) {
	fs.mu.Lock()
	fs.CreateFiles++
	fs.mu.Unlock()
	return fs.TernVFS.CreateFile(dirID, name, data)
}

func (fs *CountingVFS) Rename(
	srcDirID vfs.InodeID,
	srcName string,
	dstDirID vfs.InodeID,
	dstName string,
) error {
	fs.mu.Lock()
	fs.Renames++
	fs.mu.Unlock()
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

// The refresher's lease publication counts as a create (its LINK) and a
// rename.
func (fs *CountingVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dirID vfs.InodeID, name string, data io.Reader) error {
	fs.mu.Lock()
	fs.CreateFiles++
	fs.mu.Unlock()
	return fs.TernVFS.LinkFile(id, cookie, dirID, name, data)
}

func (fs *CountingVFS) RenameEdge(srcDirID vfs.InodeID, srcName string, edge vfs.Edge, dstDirID vfs.InodeID, dstName string) error {
	fs.mu.Lock()
	fs.Renames++
	fs.mu.Unlock()
	return fs.TernVFS.RenameEdge(srcDirID, srcName, edge, dstDirID, dstName)
}

func (fs *CountingVFS) Reset() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.lookup = 0
	fs.lookupParent = 0
	fs.readdir = 0
	fs.stat = 0
	fs.read = 0
	fs.readlink = 0
	fs.CreateFiles = 0
	fs.Renames = 0
}

type RemoveFailureVFS struct {
	vfs.TernVFS
	mu        sync.Mutex
	Name      string
	Remaining int
}

var ErrInjectedRemove = errors.New("injected remove failure")

func (fs *RemoveFailureVFS) Remove(dirID vfs.InodeID, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if name == fs.Name && fs.Remaining > 0 {
		fs.Remaining--
		return ErrInjectedRemove
	}
	return fs.TernVFS.Remove(dirID, name)
}

type LookupFailureVFS struct {
	vfs.TernVFS
	Mu             sync.Mutex
	Name           string
	Remaining      int
	PanicRemaining int
}

var ErrInjectedLookup = errors.New("injected lookup failure")

func (fs *LookupFailureVFS) Lookup(
	dirID vfs.InodeID,
	name string,
) (vfs.InodeID, error) {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	if name == fs.Name && fs.PanicRemaining > 0 {
		fs.PanicRemaining--
		panic("injected lookup panic")
	}
	if name == fs.Name && fs.Remaining > 0 {
		fs.Remaining--
		return 0, ErrInjectedLookup
	}
	return fs.TernVFS.Lookup(dirID, name)
}

type BlockingRenameVFS struct {
	vfs.TernVFS
	DstName string
	Entered chan struct{}
	Release chan struct{}
	once    sync.Once
}

func (fs *BlockingRenameVFS) Rename(
	srcDirID vfs.InodeID,
	srcName string,
	dstDirID vfs.InodeID,
	dstName string,
) error {
	if dstName == fs.DstName {
		fs.once.Do(func() { close(fs.Entered) })
		<-fs.Release
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func (fs *BlockingRenameVFS) RenameEdge(
	srcDirID vfs.InodeID,
	srcName string,
	edge vfs.Edge,
	dstDirID vfs.InodeID,
	dstName string,
) error {
	if dstName == fs.DstName {
		fs.once.Do(func() { close(fs.Entered) })
		<-fs.Release
	}
	return fs.TernVFS.RenameEdge(srcDirID, srcName, edge, dstDirID, dstName)
}

var ErrInjectedLease = errors.New("injected lease fault")

// refresherFaultVFS controls the refresher's temp protocol. A failed LINK
// is held, unapplied, until applyLate links it, as a backend that applied
// it after the error would.
type RefresherFaultVFS struct {
	vfs.TernVFS
	Mu          sync.Mutex
	constructFn func() error
	linkFail    bool
	renameFail  int
	fenceErr    error
	Constructs  int
	links       []string
	scraps      int
	late        []lateLink
}

func (fs *RefresherFaultVFS) ConstructFile(dir vfs.InodeID) (vfs.InodeID, vfs.Cookie, error) {
	fs.Mu.Lock()
	fn := fs.constructFn
	fs.Constructs++
	fs.Mu.Unlock()
	if fn != nil {
		if err := fn(); err != nil {
			return 0, vfs.Cookie{}, err
		}
	}
	return fs.TernVFS.ConstructFile(dir)
}

func (fs *RefresherFaultVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	fs.Mu.Lock()
	fs.links = append(fs.links, name)
	fail := fs.linkFail && LeaseTemp(name)
	fs.Mu.Unlock()
	if fail {
		body, _ := io.ReadAll(data)
		fs.Mu.Lock()
		fs.late = append(fs.late, lateLink{id, cookie, dir, name, body})
		fs.Mu.Unlock()
		return ErrInjectedLease
	}
	return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
}

func (fs *RefresherFaultVFS) ScrapFile(id vfs.InodeID, cookie vfs.Cookie) error {
	fs.Mu.Lock()
	fs.scraps++
	fs.Mu.Unlock()
	return fs.TernVFS.ScrapFile(id, cookie)
}

func (fs *RefresherFaultVFS) RenameEdge(src vfs.InodeID, old string, edge vfs.Edge, dst vfs.InodeID, name string) error {
	fs.Mu.Lock()
	fail := fs.renameFail > 0 && LeaseTemp(old)
	if fail {
		fs.renameFail--
	}
	fs.Mu.Unlock()
	if fail {
		return ErrInjectedLease
	}
	return fs.TernVFS.RenameEdge(src, old, edge, dst, name)
}

func (fs *RefresherFaultVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	fs.Mu.Lock()
	err := fs.fenceErr
	fs.Mu.Unlock()
	if err != nil && LeaseTemp(name) {
		return err
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}

func (fs *RefresherFaultVFS) applyLate(t testingT) {
	t.Helper()
	fs.Mu.Lock()
	late := fs.late
	fs.late = nil
	fs.Mu.Unlock()
	for _, l := range late {
		if err := fs.TernVFS.LinkFile(l.id, l.cookie, l.dir, l.name, bytes.NewReader(l.data)); err != nil {
			t.Fatal(err)
		}
	}
}

func (fs *RefresherFaultVFS) Set(fn func()) {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	fn()
}

// RefresherForTest returns a store-only client's record, kept by a
// reservation, and a clock the test moves.
func RefresherForTest(t testingT, fs vfs.TernVFS) (*Store, uint64, *LeaseRefresher, *time.Time) {
	t.Helper()
	store, clientID := NewConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, ClientOwner{})
	clock := time.Unix(1000, 0)
	store.Now = func() time.Time { return clock }
	release, ok := store.ReserveLeaseRecord(clientID)
	if !ok {
		t.Fatal("no record credit")
	}
	t.Cleanup(release)
	store.RefreshMu.Lock()
	r := store.Refreshers[vfs.InodeID(clientID)]
	store.RefreshMu.Unlock()
	return store, clientID, r, &clock
}

func RecordImage(r *LeaseRefresher) (attempt bool, orphans int, proven int64, bootstrap bool) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.Attempt != nil, len(r.orphans), r.Proven, r.bootstrap
}

// trapVFS blocks every backend call while armed.
type TrapVFS struct {
	vfs.TernVFS
	Armed   bool
	Mu      sync.Mutex
	Calls   []string
	Release chan struct{}
}

func (fs *TrapVFS) trap(name string) {
	fs.Mu.Lock()
	armed := fs.Armed
	if armed {
		fs.Calls = append(fs.Calls, name)
	}
	fs.Mu.Unlock()
	if armed {
		<-fs.Release
	}
}

func (fs *TrapVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	fs.trap("Stat")
	return fs.TernVFS.Stat(id)
}

func (fs *TrapVFS) Lookup(d vfs.InodeID, n string) (vfs.InodeID, error) {
	fs.trap("Lookup")
	return fs.TernVFS.Lookup(d, n)
}

func (fs *TrapVFS) LookupParent(id vfs.InodeID) (vfs.InodeID, error) {
	fs.trap("LookupParent")
	return fs.TernVFS.LookupParent(id)
}

func (fs *TrapVFS) Readdir(d vfs.InodeID, h uint64) ([]vfs.DirEntry, uint64, error) {
	fs.trap("Readdir")
	return fs.TernVFS.Readdir(d, h)
}

func (fs *TrapVFS) ReadAll(id vfs.InodeID) ([]byte, error) {
	fs.trap("ReadAll")
	return fs.TernVFS.ReadAll(id)
}

func (fs *TrapVFS) Readlink(id vfs.InodeID) (string, error) {
	fs.trap("Readlink")
	return fs.TernVFS.Readlink(id)
}

func (fs *TrapVFS) LookupEdge(d vfs.InodeID, n string) (vfs.Edge, error) {
	fs.trap("LookupEdge")
	return fs.TernVFS.LookupEdge(d, n)
}

// LeaseTemp reports a refresher lease temp: the publication of this
// process's lease uses ConstructFile and LinkFile like a file publication,
// so fixtures that fault or count file publications skip these.
func LeaseTemp(name string) bool { return strings.HasPrefix(name, TempPrefix) }

type edgeKey struct {
	dir  vfs.InodeID
	name string
}

var errInjectedConfirm = errors.New("injected confirmation failure")

type lateLink struct {
	id     vfs.InodeID
	cookie vfs.Cookie
	dir    vfs.InodeID
	name   string
	data   []byte
}

// writeLeaseTemp marks the refresher's lease temps, which LinkFile creates.
// No fault is ever set for it.
const writeLeaseTemp WriteID = 99
