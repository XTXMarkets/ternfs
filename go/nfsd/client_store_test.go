// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func newConfirmedStoreClient(
	t *testing.T,
	fs TernVFS,
	identity []byte,
	verifier [8]byte,
	owner clientOwner,
) (*ClientStore, uint64) {
	t.Helper()
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	clientID, confirm, err := store.SetClientID(
		verifier, identity, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	return store, clientID
}

func TestClientStoreErrToNFS(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want uint32
	}{
		{"NFS status", nfsError(NFS4ERR_EXPIRED), NFS4ERR_EXPIRED},
		{"missing object", fmt.Errorf("lookup: %w", os.ErrNotExist),
			NFS4ERR_RESOURCE},
		{"storage failure", errors.New("storage failed"), NFS4ERR_RESOURCE},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := clientStoreErrToNFS(test.err); got != test.want {
				t.Fatalf("status = %s, want %s",
					Nfsstat4Name(got), Nfsstat4Name(test.want))
			}
		})
	}
}

func TestClientStorePrunesIdentityLocks(t *testing.T) {
	store, err := NewClientStore(NewLocalTernVFS(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	unlock := store.lockIdentity(42)
	unlock()
	store.identityLocksMu.Lock()
	defer store.identityLocksMu.Unlock()
	if len(store.identityLocks) != 0 {
		t.Fatalf("idle identity locks = %d, want 0",
			len(store.identityLocks))
	}
}

func TestClientStoreDurableState(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	first, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}

	owner1 := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner-1"},
		netid:     "tcp",
		addr:      "127.0.0.1.1.2",
	}
	owner2 := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner-2"},
		netid:     "tcp",
		addr:      "127.0.0.2.1.2",
	}
	verifier := [8]byte{1}
	base := time.Unix(1000, 0)
	first.now = func() time.Time { return base }
	second.now = func() time.Time { return base }

	var clientID uint64
	var confirm [8]byte
	t.Run("registration and callback update", func(t *testing.T) {
		if _, err := first.ConfirmClientID(
			0, [8]byte{}, rpcPrincipal{},
		); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
			t.Fatalf("zero clientid confirmation error = %v, want NFS4ERR_STALE_CLIENTID", err)
		}
		staleID, staleConfirm, err := first.SetClientID(
			verifier, []byte("client"), owner1)
		if err != nil {
			t.Fatal(err)
		}
		clientID, confirm, err = first.SetClientID(
			verifier, []byte("client"), owner1)
		if err != nil {
			t.Fatal(err)
		}
		if clientID == staleID {
			t.Fatal("replaced pending SETCLIENTID reused clientid")
		}
		if confirm == staleConfirm {
			t.Fatal("repeated pending SETCLIENTID reused the previous confirm token")
		}
		if _, err := fs.Stat(InodeID(staleID)); err != nil {
			t.Fatalf("superseded pending incarnation %d was removed synchronously: %v",
				staleID, err)
		}
		if _, err := second.ConfirmClientID(
			clientID, staleConfirm, owner1.principal,
		); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
			t.Fatalf("replaced confirmation error = %v, want NFS4ERR_STALE_CLIENTID", err)
		}
		if _, err := second.ConfirmClientID(
			clientID, confirm, owner1.principal,
		); err != nil {
			t.Fatal(err)
		}
		confirmed, err := first.IsConfirmed(clientID)
		if err != nil {
			t.Fatal(err)
		}
		if !confirmed {
			t.Fatal("confirmed client is not visible to another ClientStore")
		}

		updateID, updateConfirm, err := second.SetClientID(
			verifier, []byte("client"), owner1)
		if err != nil {
			t.Fatal(err)
		}
		if updateID != clientID || updateConfirm == confirm {
			t.Fatal("callback update did not retain clientid with a new token")
		}
		if _, err := first.ConfirmClientID(
			clientID, confirm, owner1.principal,
		); err != nil {
			t.Fatalf("previous confirmed token should remain replayable: %v", err)
		}
		if _, err := first.ConfirmClientID(
			clientID, updateConfirm, owner1.principal,
		); err != nil {
			t.Fatal(err)
		}
	})

	var stateID StateID
	t.Run("ownership takeover and expiry", func(t *testing.T) {
		stateID[0] = 1
		if err := first.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := second.SetClientID(
			[8]byte{2}, []byte("client"), owner2,
		); err == nil {
			t.Fatal("different-verifier owner conflict with active state was accepted")
		} else {
			var inUse clientInUseError
			if !errors.As(err, &inUse) || inUse.owner != owner1 {
				t.Fatalf("different-verifier owner conflict = %v", err)
			}
		}
		if _, _, err := second.SetClientID(
			verifier, []byte("client"), owner2,
		); err == nil {
			t.Fatal("conflicting owner with active state was accepted")
		} else {
			var inUse clientInUseError
			if !errors.As(err, &inUse) || inUse.owner != owner1 {
				t.Fatalf("active owner conflict = %v", err)
			}
		}
		if err := first.RemoveOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		takeoverID, takeoverConfirm, err := second.SetClientID(
			verifier, []byte("client"), owner2)
		if err != nil {
			t.Fatalf("inactive owner takeover failed: %v", err)
		}
		if takeoverID == clientID {
			t.Fatal("inactive owner takeover reused clientid")
		}
		if _, err := first.ConfirmClientID(
			takeoverID, takeoverConfirm, owner2.principal,
		); err != nil {
			t.Fatal(err)
		}
		clientID = takeoverID

		second.now = func() time.Time { return base.Add(nfsLeaseTime + time.Second) }
		stateID[0] = 2
		if err := first.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		reregisterID, _, err := second.SetClientID(
			verifier, []byte("client"), owner1,
		)
		if err != nil {
			t.Fatalf("expired active-open marker blocked takeover: %v", err)
		}
		if reregisterID == clientID {
			t.Fatal("registration after lease expiry reused clientid")
		}
		active, err := second.HasOpen(clientID, stateID)
		if err != nil {
			t.Fatal(err)
		}
		if active {
			t.Fatal("expired active-open marker still reported as active")
		}
	})

	t.Run("reboot and collection", func(t *testing.T) {
		second.now = func() time.Time { return base }
		stateID[0] = 3
		if err := second.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		rebootID, rebootConfirm, err := first.SetClientID(
			[8]byte{2}, []byte("client"), owner2)
		if err != nil {
			t.Fatal(err)
		}
		if rebootID == clientID {
			t.Fatal("client reboot reused clientid")
		}
		replacedID, err := second.ConfirmClientID(
			rebootID, rebootConfirm, owner2.principal)
		if err != nil {
			t.Fatal(err)
		}
		if replacedID != clientID {
			t.Fatalf("reboot replaced clientid %d, want %d", replacedID, clientID)
		}
		active, err := first.HasOpen(clientID, stateID)
		if err != nil {
			t.Fatal(err)
		}
		if active {
			t.Fatal("reboot did not purge active-open record visible to another store")
		}
		if err := first.Renew(clientID); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
			t.Fatalf("old client RENEW error = %v, want NFS4ERR_STALE_CLIENTID", err)
		}
		if err := first.Renew(rebootID); err != nil {
			t.Fatalf("new client RENEW failed: %v", err)
		}

		if err := first.collectStaleForClient(InodeID(rebootID)); err != nil {
			t.Fatal(err)
		}
		first.now = func() time.Time { return base.Add(clientGCGrace + time.Second) }
		if err := first.collectStaleForClient(InodeID(rebootID)); err != nil {
			t.Fatal(err)
		}

		identityID, err := fs.LookupParent(InodeID(rebootID))
		if err != nil {
			t.Fatal(err)
		}
		entries, _, err := fs.Readdir(identityID, 0)
		if err != nil {
			t.Fatal(err)
		}
		incarnations := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, incarnationPrefix) {
				incarnations++
			}
		}
		if incarnations != 1 {
			t.Fatalf("incarnation directories after reboot = %d, want 1 (superseded incarnation leaked)",
				incarnations)
		}
	})
}

func TestClientStoreConfirmationReplayAndUpdateRejection(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	otherPrincipal := rpcPrincipal{flavor: authSys, body: "other"}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.SetClientID(
		[8]byte{2}, []byte("client"), owner,
	); err != nil {
		t.Fatal(err)
	}
	if replaced, err := store.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatalf("confirmed SETCLIENTID_CONFIRM replay failed: %v", err)
	} else if replaced != 0 {
		t.Fatalf("confirmed replay replaced clientid %d", replaced)
	}

	updateID, updateConfirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if updateID != clientID {
		t.Fatalf("callback update clientid = %d, want %d",
			updateID, clientID)
	}
	wrongConfirm := updateConfirm
	wrongConfirm[0] ^= 0xff
	if _, err := store.ConfirmClientID(
		clientID, wrongConfirm, owner.principal,
	); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("wrong callback-update token error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if _, err := store.ConfirmClientID(
		clientID, updateConfirm, otherPrincipal,
	); nfsErrCode(err) != NFS4ERR_CLID_INUSE {
		t.Fatalf("callback-update principal error = %v, want NFS4ERR_CLID_INUSE",
			err)
	}
	if _, err := store.ConfirmClientID(
		clientID, updateConfirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
}

func TestClientStoreLeaseSlotsDoNotShortenEachOther(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	first, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.now = func() time.Time { return base }
	second.now = func() time.Time { return base }
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}
	clientID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	var stateID StateID
	stateID[0] = 1
	if err := first.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	secondRenewAt := base.Add(nfsLeaseRenewAfter)
	second.now = func() time.Time { return secondRenewAt }
	if err := second.Renew(clientID); err != nil {
		t.Fatal(err)
	}
	secondExpiry := secondRenewAt.Add(nfsLeaseTime)
	firstRenewAt := secondExpiry.Add(-time.Second)
	first.now = func() time.Time { return firstRenewAt }
	if err := first.Renew(clientID); err != nil {
		t.Fatal(err)
	}
	firstExpiry := firstRenewAt.Add(nfsLeaseTime)

	entries, _, err := fs.Readdir(InodeID(clientID), 0)
	if err != nil {
		t.Fatal(err)
	}
	leases := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, leasePrefix) {
			leases++
		}
	}
	if leases != 2 {
		t.Fatalf("lease slots = %d, want 2", leases)
	}

	second.now = func() time.Time { return secondExpiry.Add(time.Second) }
	live, _, err := second.leaseStatus(InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Fatal("shorter lease slot hid a longer lease")
	}
	second.now = func() time.Time { return firstExpiry.Add(time.Second) }
	live, _, err = second.leaseStatus(InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Fatal("expired lease slots kept an open marker active")
	}
}

func TestClientStoreRemovesExpiredProcessSlots(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	base := time.Unix(1000, 0)
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	first, clientID := newConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	first.now = func() time.Time { return base }
	if err := first.MarkOpen(clientID, StateID{1}); err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second.now = func() time.Time { return base }
	if err := second.beginConfirmation(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	observer, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	observer.now = func() time.Time {
		return base.Add(nfsLeaseTime + time.Second)
	}
	if _, _, err := observer.leaseStatus(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := observer.hasLiveConfirmation(
		InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{first.leaseName, second.confirmingName} {
		if _, err := fs.Lookup(InodeID(clientID), name); err != nil {
			t.Fatalf("process slot %q removed without cleanup grace: %v",
				name, err)
		}
	}
	collector, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector.now = func() time.Time {
		return base.Add(2*nfsLeaseTime + time.Second)
	}
	if _, _, err := collector.leaseStatus(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.hasLiveConfirmation(
		InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{first.leaseName, second.confirmingName} {
		if _, err := fs.Lookup(
			InodeID(clientID), name,
		); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired process slot %q remains: %v", name, err)
		}
	}
}

type countingVFS struct {
	TernVFS

	mu           sync.Mutex
	lookup       int
	lookupParent int
	readdir      int
	stat         int
	read         int
	readlink     int
	createFile   int
	rename       int
}

func (fs *countingVFS) Lookup(dirID InodeID, name string) (InodeID, error) {
	fs.mu.Lock()
	fs.lookup++
	fs.mu.Unlock()
	return fs.TernVFS.Lookup(dirID, name)
}

func (fs *countingVFS) LookupParent(id InodeID) (InodeID, error) {
	fs.mu.Lock()
	fs.lookupParent++
	fs.mu.Unlock()
	return fs.TernVFS.LookupParent(id)
}

func (fs *countingVFS) Readdir(
	dirID InodeID,
	startHash uint64,
) ([]DirEntry, uint64, error) {
	fs.mu.Lock()
	fs.readdir++
	fs.mu.Unlock()
	return fs.TernVFS.Readdir(dirID, startHash)
}

func (fs *countingVFS) Stat(id InodeID) (NodeInfo, error) {
	fs.mu.Lock()
	fs.stat++
	fs.mu.Unlock()
	return fs.TernVFS.Stat(id)
}

func (fs *countingVFS) Read(
	id InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	fs.mu.Lock()
	fs.read++
	fs.mu.Unlock()
	return fs.TernVFS.Read(id, offset, dest)
}

func (fs *countingVFS) Readlink(id InodeID) (string, error) {
	fs.mu.Lock()
	fs.readlink++
	fs.mu.Unlock()
	return fs.TernVFS.Readlink(id)
}

func (fs *countingVFS) CreateFile(
	dirID InodeID,
	name string,
	data io.Reader,
) (InodeID, error) {
	fs.mu.Lock()
	fs.createFile++
	fs.mu.Unlock()
	return fs.TernVFS.CreateFile(dirID, name, data)
}

func (fs *countingVFS) Rename(
	srcDirID InodeID,
	srcName string,
	dstDirID InodeID,
	dstName string,
) error {
	fs.mu.Lock()
	fs.rename++
	fs.mu.Unlock()
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func (fs *countingVFS) reset() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.lookup = 0
	fs.lookupParent = 0
	fs.readdir = 0
	fs.stat = 0
	fs.read = 0
	fs.readlink = 0
	fs.createFile = 0
	fs.rename = 0
}

func TestClientStoreIsConfirmedUsesCachedLocation(t *testing.T) {
	fs := &countingVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	store, clientID := newConfirmedStoreClient(
		t,
		fs,
		[]byte("client"),
		[8]byte{1},
		clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}},
	)
	if err := store.MarkOpen(clientID, StateID{1}); err != nil {
		t.Fatal(err)
	}
	fs.reset()
	confirmed, err := store.IsConfirmed(clientID)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Fatal("cached client was not confirmed")
	}
	if fs.lookup != 1 || fs.lookupParent != 0 || fs.readdir != 0 ||
		fs.stat != 0 || fs.read != 0 || fs.readlink != 0 {
		t.Fatalf("cached confirmation calls: lookup=%d parent=%d readdir=%d stat=%d read=%d readlink=%d",
			fs.lookup, fs.lookupParent, fs.readdir, fs.stat, fs.read,
			fs.readlink)
	}
}

func TestClientStoreHasOpenFastPathAndImplicitRenewal(t *testing.T) {
	fs := &countingVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.now = func() time.Time { return base }
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	stateID := StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	store.now = func() time.Time { return base.Add(time.Second) }
	fs.reset()
	active, err := store.HasOpen(clientID, stateID)
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("fresh open was not active")
	}
	if fs.lookup != 1 || fs.lookupParent != 0 || fs.readdir != 0 ||
		fs.stat != 0 || fs.read != 0 || fs.readlink != 0 ||
		fs.createFile != 0 || fs.rename != 0 {
		t.Fatalf("fast HasOpen calls: lookup=%d parent=%d readdir=%d stat=%d read=%d readlink=%d create=%d rename=%d",
			fs.lookup, fs.lookupParent, fs.readdir, fs.stat, fs.read,
			fs.readlink, fs.createFile, fs.rename)
	}

	for _, elapsed := range []time.Duration{
		nfsLeaseRenewAfter + time.Second,
		2 * (nfsLeaseRenewAfter + time.Second),
		3 * (nfsLeaseRenewAfter + time.Second),
		4 * (nfsLeaseRenewAfter + time.Second),
	} {
		store.now = func() time.Time { return base.Add(elapsed) }
		fs.reset()
		active, err = store.HasOpen(clientID, stateID)
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			t.Fatalf("active traffic did not renew lease at %s", elapsed)
		}
		if fs.readdir != 0 || fs.stat != 0 || fs.read != 0 ||
			fs.readlink != 0 {
			t.Fatalf("implicit renewal used slow metadata path at %s: readdir=%d stat=%d read=%d readlink=%d",
				elapsed, fs.readdir, fs.stat, fs.read, fs.readlink)
		}
		if fs.createFile != 1 || fs.rename != 1 {
			t.Fatalf("implicit renewal writes at %s: create=%d rename=%d",
				elapsed, fs.createFile, fs.rename)
		}
	}
}

func TestClientStoreImplicitRenewalRechecksReboot(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	hookFS := &renameHookVFS{TernVFS: baseFS}
	first, err := NewClientStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.now = func() time.Time { return base }
	second.now = func() time.Time { return base }
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	oldID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmClientID(
		oldID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	stateID := StateID{1}
	if err := first.MarkOpen(oldID, stateID); err != nil {
		t.Fatal(err)
	}
	newID, confirm, err := second.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	var confirmErr error
	hookFS.dstName = first.leaseName
	hookFS.hook = func() {
		_, confirmErr = second.ConfirmClientID(
			newID, confirm, owner.principal)
	}
	first.now = func() time.Time {
		return base.Add(nfsLeaseRenewAfter + time.Second)
	}
	active, err := first.HasOpen(oldID, stateID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmErr != nil {
		t.Fatalf("reboot confirmation failed: %v", confirmErr)
	}
	if active {
		t.Fatal("implicit renewal resurrected state after reboot confirmation")
	}
	if _, err := baseFS.Lookup(
		InodeID(oldID), first.leaseName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("implicit renewal left a stale lease: %v", err)
	}
	if _, err := baseFS.Lookup(
		InodeID(oldID), activeOpenName(stateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reboot confirmation left a stale open marker: %v", err)
	}
}

func TestClientStoreRejectsClientIDOutsideStateDirectory(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := fs.Mkdir(fs.RootID(), "user")
	if err != nil {
		t.Fatal(err)
	}
	incarnationID, err := fs.Mkdir(
		parentID, incarnationPrefix+strings.Repeat("0", 32))
	if err != nil {
		t.Fatal(err)
	}
	confirm := [8]byte{1}
	record := newDurableClientRecord(
		[8]byte{1}, confirm, clientOwner{})
	if _, err := store.createJSON(
		incarnationID, clientRecordName, record,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.replaceIdentitySymlink(
		parentID, pendingName, incarnationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		uint64(incarnationID), confirm, rpcPrincipal{},
	); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid confirmation error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	stateID := StateID{1}
	if err := store.MarkOpen(
		uint64(incarnationID), stateID,
	); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid MarkOpen error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if _, err := fs.Lookup(
		incarnationID, activeOpenName(stateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("forged clientid created an open marker: %v", err)
	}
	if err := store.RemoveOpen(
		uint64(incarnationID), stateID,
	); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid RemoveOpen error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if err := store.Renew(
		uint64(incarnationID),
	); nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid Renew error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if active, err := store.HasOpen(
		uint64(incarnationID), stateID,
	); err != nil {
		t.Fatalf("forged clientid HasOpen error = %v, want nil", err)
	} else if active {
		t.Fatal("forged clientid reported an active open")
	}
}

type removeFailureVFS struct {
	TernVFS
	mu        sync.Mutex
	name      string
	remaining int
}

var errInjectedRemove = errors.New("injected remove failure")

func (fs *removeFailureVFS) Remove(dirID InodeID, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if name == fs.name && fs.remaining > 0 {
		fs.remaining--
		return errInjectedRemove
	}
	return fs.TernVFS.Remove(dirID, name)
}

type lookupFailureVFS struct {
	TernVFS
	mu             sync.Mutex
	name           string
	remaining      int
	panicRemaining int
}

var errInjectedLookup = errors.New("injected lookup failure")

func (fs *lookupFailureVFS) Lookup(
	dirID InodeID,
	name string,
) (InodeID, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if name == fs.name && fs.panicRemaining > 0 {
		fs.panicRemaining--
		panic("injected lookup panic")
	}
	if name == fs.name && fs.remaining > 0 {
		fs.remaining--
		return 0, errInjectedLookup
	}
	return fs.TernVFS.Lookup(dirID, name)
}

type readAllHookVFS struct {
	TernVFS
	id   InodeID
	once sync.Once
	hook func()
}

func (fs *readAllHookVFS) ReadAll(id InodeID) ([]byte, error) {
	if id == fs.id {
		fs.once.Do(fs.hook)
	}
	return fs.TernVFS.ReadAll(id)
}

type renameHookVFS struct {
	TernVFS
	dstName string
	once    sync.Once
	hook    func()
}

type beforeRenameHookVFS struct {
	TernVFS
	dstName string
	once    sync.Once
	hook    func()
}

type blockingRenameVFS struct {
	TernVFS
	dstName string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type renameFailureVFS struct {
	TernVFS
	dstName   string
	remaining int
}

var errInjectedRename = errors.New("injected rename failure")

func (fs *renameFailureVFS) Rename(
	srcDirID InodeID,
	srcName string,
	dstDirID InodeID,
	dstName string,
) error {
	if dstName == fs.dstName && fs.remaining > 0 {
		fs.remaining--
		return errInjectedRename
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func (fs *renameHookVFS) Rename(
	srcDirID InodeID,
	srcName string,
	dstDirID InodeID,
	dstName string,
) error {
	if err := fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName); err != nil {
		return err
	}
	if dstName == fs.dstName {
		fs.once.Do(fs.hook)
	}
	return nil
}

func (fs *beforeRenameHookVFS) Rename(
	srcDirID InodeID,
	srcName string,
	dstDirID InodeID,
	dstName string,
) error {
	if dstName == fs.dstName {
		fs.once.Do(fs.hook)
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func (fs *blockingRenameVFS) Rename(
	srcDirID InodeID,
	srcName string,
	dstDirID InodeID,
	dstName string,
) error {
	if dstName == fs.dstName {
		fs.once.Do(func() { close(fs.entered) })
		<-fs.release
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func TestClientStoreConfirmationRechecksPending(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	hookFS := &renameHookVFS{TernVFS: baseFS}
	first, err := NewClientStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}
	oldID, oldConfirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	var newID uint64
	var newConfirm [8]byte
	hookFS.dstName = first.confirmingName
	hookFS.hook = func() {
		newID, newConfirm, err = second.SetClientID(
			[8]byte{2}, []byte("client"), owner)
	}
	_, confirmErr := first.ConfirmClientID(
		oldID, oldConfirm, owner.principal)
	if err != nil {
		t.Fatal(err)
	}
	if nfsErrCode(confirmErr) != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("superseded confirmation error = %v, want NFS4ERR_STALE_CLIENTID",
			confirmErr)
	}
	if _, err := second.ConfirmClientID(
		newID, newConfirm, owner.principal,
	); err != nil {
		t.Fatalf("replacement confirmation failed: %v", err)
	}
	if confirmed, err := first.IsConfirmed(oldID); err != nil {
		t.Fatal(err)
	} else if confirmed {
		t.Fatal("superseded confirmation replaced the confirmed pointer")
	}
}

func TestClientStoreMarkOpenKeepsExistingMarkerOnRenewFailure(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	fs := &renameFailureVFS{TernVFS: baseFS}
	base := time.Unix(1000, 0)
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	store, clientID := newConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.now = func() time.Time { return base }
	stateID := StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	store.now = func() time.Time {
		return base.Add(nfsLeaseRenewAfter + time.Second)
	}
	fs.dstName = store.leaseName
	fs.remaining = 1
	if _, err := baseFS.Lookup(
		InodeID(clientID), activeOpenName(stateID),
	); err != nil {
		t.Fatalf("initial marker is missing: %v", err)
	}
	if err := store.ensureMarker(
		InodeID(clientID), activeOpenName(stateID)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOpen(
		clientID, stateID,
	); !errors.Is(err, errInjectedRename) {
		t.Fatalf("MarkOpen error = %v, want injected renewal failure", err)
	}
	if _, err := baseFS.Lookup(
		InodeID(clientID), activeOpenName(stateID),
	); err != nil {
		t.Fatalf("renewal failure removed existing marker: %v", err)
	}

	newStateID := StateID{2}
	fs.remaining = 1
	if err := store.MarkOpen(
		clientID, newStateID,
	); !errors.Is(err, errInjectedRename) {
		t.Fatalf("new MarkOpen error = %v, want injected renewal failure", err)
	}
	if _, err := baseFS.Lookup(
		InodeID(clientID), activeOpenName(newStateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed renewal created a new marker: %v", err)
	}
}

func TestClientStoreLiveSlotsSkipsRemovedEntry(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	fs := &readAllHookVFS{TernVFS: baseFS}
	base := time.Unix(1000, 0)
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	store, clientID := newConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.now = func() time.Time { return base }
	if err := store.MarkOpen(clientID, StateID{1}); err != nil {
		t.Fatal(err)
	}
	leaseID, err := baseFS.Lookup(InodeID(clientID), store.leaseName)
	if err != nil {
		t.Fatal(err)
	}
	var hookErr error
	fs.id = leaseID
	fs.hook = func() {
		hookErr = baseFS.Remove(InodeID(clientID), store.leaseName)
	}

	live, found, err := store.liveSlots(InodeID(clientID), leasePrefix)
	if err != nil {
		t.Fatalf("raced slot scan failed: %v", err)
	}
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if live || found {
		t.Fatalf("raced slot scan = live %t, found %t; want false, false",
			live, found)
	}
	if _, err := store.replaceJSON(
		InodeID(clientID),
		store.leaseName,
		durableLease{
			ExpiresUnixNano: base.Add(nfsLeaseTime).UnixNano(),
		},
	); err != nil {
		t.Fatal(err)
	}
	if live, err := store.hasLiveLease(InodeID(clientID)); err != nil {
		t.Fatal(err)
	} else if !live {
		t.Fatal("replacement lease slot was not live on the next scan")
	}
}

func TestClientStoreRenewalDoesNotBlockActiveState(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	fs := &blockingRenameVFS{
		TernVFS: baseFS,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	base := time.Unix(1000, 0)
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	store, clientID := newConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.now = func() time.Time { return base }
	stateID := StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	store.now = func() time.Time {
		return base.Add(nfsLeaseRenewAfter + time.Second)
	}
	fs.dstName = store.leaseName
	renewed := make(chan error, 1)
	go func() {
		renewed <- store.Renew(clientID)
	}()
	<-fs.entered

	activeResult := make(chan bool, 1)
	go func() {
		active, err := store.HasOpen(clientID, stateID)
		activeResult <- err == nil && active
	}()
	select {
	case active := <-activeResult:
		if !active {
			t.Fatal("active state failed while renewal was in flight")
		}
	case <-time.After(time.Second):
		close(fs.release)
		t.Fatal("active state blocked behind lease renewal")
	}
	close(fs.release)
	if err := <-renewed; err != nil {
		t.Fatal(err)
	}
}

func TestClientStoreConfirmLeavesPendingUntilReplacement(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	store, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	identityID, err := fs.LookupParent(InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	if pendingID, found, err := store.readIdentitySymlink(
		identityID, pendingName,
	); err != nil {
		t.Fatal(err)
	} else if !found || uint64(pendingID) != clientID {
		t.Fatalf("confirmed pending pointer = %d, %t; want %d, true",
			pendingID, found, clientID)
	}
	replacementID, _, err := store.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if pendingID, found, err := store.readIdentitySymlink(
		identityID, pendingName,
	); err != nil {
		t.Fatal(err)
	} else if !found || uint64(pendingID) != replacementID {
		t.Fatalf("replacement pending pointer = %d, %t; want %d, true",
			pendingID, found, replacementID)
	}
}

func TestClientStoreGCDoesNotRemoveInFlightTemporaryEntry(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	hookFS := &beforeRenameHookVFS{
		TernVFS: baseFS,
		dstName: pendingName,
	}
	first, err := NewClientStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	identityID, err := first.identityDir([]byte("client"))
	if err != nil {
		t.Fatal(err)
	}
	incarnationID, err := first.newIncarnation(identityID)
	if err != nil {
		t.Fatal(err)
	}
	var collectErr error
	hookFS.hook = func() {
		collectErr = second.collectStale(identityID)
	}

	if _, err := first.replaceIdentitySymlink(
		identityID, pendingName, incarnationID,
	); err != nil {
		t.Fatalf("rename after concurrent collection failed: %v", err)
	}
	if collectErr != nil {
		t.Fatalf("concurrent collection failed: %v", collectErr)
	}
	if got, found, err := first.readIdentitySymlink(
		identityID, pendingName,
	); err != nil {
		t.Fatal(err)
	} else if !found || got != incarnationID {
		t.Fatalf("pending pointer = %d, %t; want %d, true",
			got, found, incarnationID)
	}
}

func TestClientStoreGCCollectsTemporaryAndHalfCreatedEntries(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	owner := clientOwner{
		principal: rpcPrincipal{flavor: authSys, body: "owner"},
	}
	store, clientID := newConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	identityID, err := fs.LookupParent(InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	tempName := tempPrefix + "orphan"
	tempID, err := fs.CreateFile(identityID, tempName, nil)
	if err != nil {
		t.Fatal(err)
	}
	incarnationTempName := tempPrefix + "incarnation-orphan"
	incarnationTempID, err := fs.CreateFile(
		InodeID(clientID), incarnationTempName, nil)
	if err != nil {
		t.Fatal(err)
	}
	halfCreated, err := fs.Mkdir(
		identityID, incarnationPrefix+strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.now = func() time.Time { return base }
	oldTemp := base.Add(-nfsLeaseTime - time.Second)
	if err := fs.SetTime(tempID, &oldTemp, nil); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetTime(incarnationTempID, &oldTemp, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(
		identityID, tempName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary entry remains after GC: %v", err)
	}
	if _, err := fs.Lookup(
		InodeID(clientID), incarnationTempName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incarnation temporary entry remains after GC: %v", err)
	}
	store.now = func() time.Time {
		return base.Add(clientGCGrace + time.Second)
	}
	if err := store.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(halfCreated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("half-created incarnation remains after GC: %v", err)
	}
}

func TestClientStoreGCHonorsConfirmationClaims(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	first, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.now = func() time.Time { return base }
	second.now = func() time.Time { return base.Add(30 * time.Second) }
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}
	oldID, _, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	err = first.beginConfirmation(InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}
	err = second.beginConfirmation(InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}
	clientID, _, err := first.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	collector.now = func() time.Time { return base.Add(100 * time.Second) }
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(InodeID(oldID), gcCandidateName); !os.IsNotExist(err) {
		t.Fatalf("live confirmation claim did not protect incarnation: %v", err)
	}

	collector.now = func() time.Time { return base.Add(121 * time.Second) }
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(InodeID(oldID), gcCandidateName); err != nil {
		t.Fatalf("expired confirmation claims still protected incarnation: %v", err)
	}
}

// TestClientStoreGCRechecksConfirmationBeforeDeleting covers a confirmation
// claim that appears only after an incarnation has already aged into a GC
// candidate: collectStale checks hasLiveConfirmation again on
// every pass over every entry, before ever consulting the aged candidate's
// due time, so a claim that shows up between the aging pass and the pass
// that would otherwise delete it must still block the removal.
func TestClientStoreGCRechecksConfirmationBeforeDeleting(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	writer, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	confirmer, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	writer.now = func() time.Time { return base }
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}
	oldID, _, err := writer.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _, err := writer.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	// Age oldID into a GC candidate with no confirmation claim yet.
	collector.now = func() time.Time { return base }
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(InodeID(oldID), gcCandidateName); err != nil {
		t.Fatalf("incarnation was not aged into a GC candidate: %v", err)
	}

	// A confirmation claims oldID only after it was already aged.
	confirmer.now = func() time.Time { return base.Add(30 * time.Second) }
	err = confirmer.beginConfirmation(InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}

	// Once the candidate is due, the delete-time pass must recheck the live
	// claim, not just the roots snapshot from before the claim existed.
	collector.now = func() time.Time { return base.Add(nfsLeaseTime + time.Second) }
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(InodeID(oldID)); err != nil {
		t.Fatalf("GC removed an incarnation with a live confirmation claim: %v", err)
	}
}

func TestClientStoreRebootCleanupIsRetryable(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	first, err := NewClientStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	failingFS := &removeFailureVFS{TernVFS: baseFS, remaining: 1}
	second, err := NewClientStore(failingFS)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}
	oldID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmClientID(
		oldID, confirm, owner.principal,
	); err != nil {
		t.Fatal(err)
	}
	var stateID StateID
	stateID[0] = 1
	if err := first.MarkOpen(oldID, stateID); err != nil {
		t.Fatal(err)
	}
	failingFS.name = activeOpenName(stateID)

	newID, confirm, err := first.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		newID, confirm, owner.principal,
	); !errors.Is(err, errInjectedRemove) {
		t.Fatalf("reboot cleanup error = %v, want injected failure", err)
	}
	if confirmed, err := first.IsConfirmed(oldID); err != nil {
		t.Fatal(err)
	} else if confirmed {
		t.Fatal("old client remained confirmed after pointer switch")
	}
	if confirmed, err := first.IsConfirmed(newID); err != nil {
		t.Fatal(err)
	} else if !confirmed {
		t.Fatal("new client was not confirmed before cleanup retry")
	}
	if err := first.collectStaleForClient(InodeID(newID)); err != nil {
		t.Fatal(err)
	}
	if _, err := baseFS.Lookup(InodeID(oldID), gcCandidateName); !os.IsNotExist(err) {
		t.Fatalf("in-progress reboot target was marked for collection: %v", err)
	}

	replacedID, err := second.ConfirmClientID(
		newID, confirm, owner.principal)
	if err != nil {
		t.Fatal(err)
	}
	if replacedID != oldID {
		t.Fatalf("cleanup retry replaced clientid %d, want %d", replacedID, oldID)
	}
	if active, err := first.HasOpen(oldID, stateID); err != nil {
		t.Fatal(err)
	} else if active {
		t.Fatal("cleanup retry left old open active")
	}

	failingFS.name = rebootName
	failingFS.remaining = 1
	thirdID, confirm, err := first.SetClientID(
		[8]byte{3}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		thirdID, confirm, owner.principal,
	); !errors.Is(err, errInjectedRemove) {
		t.Fatalf("reboot marker cleanup error = %v, want injected failure",
			err)
	}
	replacedID, err = second.ConfirmClientID(
		thirdID, confirm, owner.principal)
	if err != nil {
		t.Fatal(err)
	}
	if replacedID != newID {
		t.Fatalf("marker retry replaced clientid %d, want %d", replacedID, newID)
	}
}

func TestClientStoreGCIsBoundedAndRetryable(t *testing.T) {
	baseFS := NewLocalTernVFS(t.TempDir())
	writer, err := NewClientStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	failingFS := &removeFailureVFS{
		TernVFS: baseFS,
		name:    clientRecordName,
	}
	collector, err := NewClientStore(failingFS)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	collector.now = func() time.Time { return base }
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "owner"}}

	var clientID uint64
	for i := 0; i < maxClientGCRemovals+3; i++ {
		clientID, _, err = writer.SetClientID(
			[8]byte{byte(i + 1)}, []byte("client"), owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	identityID, err := baseFS.LookupParent(InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := baseFS.Lookup(InodeID(clientID), gcCandidateName); !os.IsNotExist(err) {
		t.Fatalf("current incarnation has a GC candidate marker: %v", err)
	}

	collector.now = func() time.Time {
		return base.Add(clientGCGrace + time.Second)
	}
	failingFS.remaining = 1
	if err := collector.collectStaleForClient(
		InodeID(clientID),
	); !errors.Is(err, errInjectedRemove) {
		t.Fatalf("GC error = %v, want injected failure", err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != maxClientGCRemovals+3 {
		t.Fatalf("incarnations after failed GC = %d, want %d",
			got, maxClientGCRemovals+3)
	}

	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != 3 {
		t.Fatalf("incarnations after bounded GC = %d, want 3", got)
	}
	if err := collector.collectStaleForClient(InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != 1 {
		t.Fatalf("incarnations after GC retry = %d, want 1", got)
	}
}
