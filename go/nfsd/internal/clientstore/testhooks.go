// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"

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

// CollectStaleForClient and collectStale run a collection pass and discard
// the recheck flag. Production code uses CollectStaleForClientResult.
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

// RefresherForTest returns the record of a client in a Store without a
// server, kept by a
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

// LeaseTemp reports a refresher lease temp: the publication of this
// process's lease uses ConstructFile and LinkFile like a file publication,
// so fixtures that fault or count file publications skip these.
func LeaseTemp(name string) bool { return strings.HasPrefix(name, TempPrefix) }

var errInjectedConfirm = errors.New("injected confirmation failure")

// writeLeaseTemp marks the refresher's lease temps, which LinkFile creates.
// No fault is ever set for it.
const writeLeaseTemp WriteID = 99

// LockForTest holds the attempt's lock, as a settle does.
func (a *ConfirmAttempt) LockForTest() { a.mu.Lock() }

// UnlockForTest releases LockForTest.
func (a *ConfirmAttempt) UnlockForTest() { a.mu.Unlock() }

// RefresherWorkerForTest is RefresherForTest with server hooks, so the
// record has a worker that wakes are delivered to.
func RefresherWorkerForTest(t testingT, fs *PassCountVFS) (*Store, uint64, *LeaseRefresher) {
	t.Helper()
	cs, clientID, r, _ := RefresherForTest(t, fs)
	fs.LeaseName = cs.LeaseName
	// Real time: no worker runs yet, so the field can still be set.
	cs.Now = time.Now
	var need atomic.Bool
	need.Store(true)
	cs.LeaseNeed = func(uint64) bool { return need.Load() }
	cs.LeaseSessions = func(uint64) bool { return true }
	t.Cleanup(cs.StopLeaseWorkers)
	return cs, clientID, r
}
