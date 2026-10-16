// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestClientStoreErrToNFS(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want uint32
	}{
		{"NFS status", nfs4.Error(nfs4.NFS4ERR_EXPIRED), nfs4.NFS4ERR_EXPIRED},
		{"missing object", fmt.Errorf("lookup: %w", os.ErrNotExist),
			nfs4.NFS4ERR_DELAY},
		{"storage failure", errors.New("storage failed"), nfs4.NFS4ERR_DELAY},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ErrToNFS(test.err); got != test.want {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(test.want))
			}
		})
	}
}

func TestClientStorePrunesIdentityLocks(t *testing.T) {
	store, err := NewStore(vfs.NewLocalTernVFS(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	unlock := store.LockIdentity(42)
	unlock()
	store.identityLocksMu.Lock()
	defer store.identityLocksMu.Unlock()
	if len(store.identityLocks) != 0 {
		t.Fatalf("idle identity locks = %d, want 0",
			len(store.identityLocks))
	}
}

func TestClientStoreDurableState(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	first, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}

	owner1 := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner-1"},
	}
	owner2 := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner-2"},
	}
	verifier := [8]byte{1}
	base := time.Unix(1000, 0)
	first.Now = func() time.Time { return base }
	second.Now = func() time.Time { return base }

	var clientID uint64
	var confirm [8]byte
	t.Run("registration and callback update", func(t *testing.T) {
		if _, err := first.ConfirmClientID(
			0, [8]byte{}, RPCPrincipal{},
		); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
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
		if _, err := fs.Stat(vfs.InodeID(staleID)); err != nil {
			t.Fatalf("superseded pending incarnation %d was removed synchronously: %v",
				staleID, err)
		}
		if _, err := second.ConfirmClientID(
			clientID, staleConfirm, owner1.Principal,
		); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("replaced confirmation error = %v, want NFS4ERR_STALE_CLIENTID", err)
		}
		if _, err := second.ConfirmClientID(
			clientID, confirm, owner1.Principal,
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
			clientID, confirm, owner1.Principal,
		); err != nil {
			t.Fatalf("previous confirmed token should remain replayable: %v", err)
		}
		if _, err := first.ConfirmClientID(
			clientID, updateConfirm, owner1.Principal,
		); err != nil {
			t.Fatal(err)
		}
	})

	var stateID nfs4.StateID
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
			var inUse ClientInUseError
			if !errors.As(err, &inUse) {
				t.Fatalf("different-verifier owner conflict = %v", err)
			}
		}
		if _, _, err := second.SetClientID(
			verifier, []byte("client"), owner2,
		); err == nil {
			t.Fatal("conflicting owner with active state was accepted")
		} else {
			var inUse ClientInUseError
			if !errors.As(err, &inUse) {
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
			takeoverID, takeoverConfirm, owner2.Principal,
		); err != nil {
			t.Fatal(err)
		}
		clientID = takeoverID

		second.Now = func() time.Time { return base.Add(LeaseWriteTerm + time.Second) }
		stateID[0] = 2
		if err := first.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		reregisterID, reregisterConfirm, err := second.SetClientID(
			verifier, []byte("client"), owner1,
		)
		if err != nil {
			t.Fatalf("expired active-open marker blocked takeover: %v", err)
		}
		if reregisterID == clientID {
			t.Fatal("registration after lease expiry reused clientid")
		}
		if _, err := second.ConfirmClientID(
			reregisterID, reregisterConfirm, owner1.Principal,
		); err != nil {
			t.Fatal(err)
		}
		active, err := second.HasOpen(clientID, stateID)
		if err != nil {
			t.Fatal(err)
		}
		if active {
			t.Fatal("expired active-open marker still reported as active")
		}
		clientID = reregisterID
	})

	t.Run("reboot and collection", func(t *testing.T) {
		second.Now = func() time.Time { return base }
		stateID[0] = 3
		if err := second.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		rebootID, rebootConfirm, err := first.SetClientID(
			[8]byte{2}, []byte("client"), owner1)
		if err != nil {
			t.Fatal(err)
		}
		if rebootID == clientID {
			t.Fatal("client reboot reused clientid")
		}
		replacedID, err := second.ConfirmClientID(
			rebootID, rebootConfirm, owner1.Principal)
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
		if err := first.Renew(clientID); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("old client RENEW error = %v, want NFS4ERR_STALE_CLIENTID", err)
		}
		if err := first.Renew(rebootID); err != nil {
			t.Fatalf("new client RENEW failed: %v", err)
		}

		if err := first.CollectStaleForClient(vfs.InodeID(rebootID)); err != nil {
			t.Fatal(err)
		}
		first.Now = func() time.Time { return base.Add(ClientGCGrace + time.Second) }
		if err := first.CollectStaleForClient(vfs.InodeID(rebootID)); err != nil {
			t.Fatal(err)
		}

		identityID, err := fs.LookupParent(vfs.InodeID(rebootID))
		if err != nil {
			t.Fatal(err)
		}
		entries, _, err := fs.Readdir(identityID, 0)
		if err != nil {
			t.Fatal(err)
		}
		incarnations := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, IncarnationPrefix) {
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
	fs := vfs.NewLocalTernVFS(t.TempDir())
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	otherPrincipal := RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "other"}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.SetClientID(
		[8]byte{2}, []byte("client"), owner,
	); err != nil {
		t.Fatal(err)
	}
	if replaced, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
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
		clientID, wrongConfirm, owner.Principal,
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("wrong callback-update token error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if _, err := store.ConfirmClientID(
		clientID, updateConfirm, otherPrincipal,
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_CLID_INUSE {
		t.Fatalf("callback-update principal error = %v, want NFS4ERR_CLID_INUSE",
			err)
	}
	if _, err := store.ConfirmClientID(
		clientID, updateConfirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
}

func TestClientStoreLeaseSlotsDoNotShortenEachOther(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	first, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.Now = func() time.Time { return base }
	second.Now = func() time.Time { return base }
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	clientID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	var stateID nfs4.StateID
	stateID[0] = 1
	if err := first.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	secondRenewAt := base.Add(LeaseRenewAfter)
	second.Now = func() time.Time { return secondRenewAt }
	if err := second.Renew(clientID); err != nil {
		t.Fatal(err)
	}
	secondExpiry := secondRenewAt.Add(LeaseWriteTerm)
	firstRenewAt := secondExpiry.Add(-time.Second)
	first.Now = func() time.Time { return firstRenewAt }
	if err := first.Renew(clientID); err != nil {
		t.Fatal(err)
	}
	firstExpiry := firstRenewAt.Add(LeaseWriteTerm)

	entries, _, err := fs.Readdir(vfs.InodeID(clientID), 0)
	if err != nil {
		t.Fatal(err)
	}
	leases := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, LeasePrefix) {
			leases++
		}
	}
	if leases != 2 {
		t.Fatalf("lease slots = %d, want 2", leases)
	}

	second.Now = func() time.Time { return secondExpiry.Add(time.Second) }
	live, _, err := second.leaseStatus(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Fatal("shorter lease slot hid a longer lease")
	}
	second.Now = func() time.Time { return firstExpiry.Add(time.Second) }
	live, _, err = second.leaseStatus(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Fatal("expired lease slots kept an open marker active")
	}
}

func TestClientStoreRemovesExpiredProcessSlots(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	first, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	first.Now = func() time.Time { return base }
	if err := first.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second.Now = func() time.Time { return base }
	if err := second.beginConfirmation(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	observer, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	observer.Now = func() time.Time {
		return base.Add(LeaseWriteTerm + time.Second)
	}
	if _, _, err := observer.leaseStatus(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := observer.hasLiveConfirmation(
		vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{first.LeaseName, second.ConfirmingName} {
		if _, err := fs.Lookup(vfs.InodeID(clientID), name); err != nil {
			t.Fatalf("process slot %q removed without cleanup grace: %v",
				name, err)
		}
	}
	collector, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector.Now = func() time.Time {
		return base.Add(LeaseWriteTerm + LeaseTime + time.Second)
	}
	if _, _, err := collector.leaseStatus(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.hasLiveConfirmation(
		vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{first.LeaseName, second.ConfirmingName} {
		if _, err := fs.Lookup(
			vfs.InodeID(clientID), name,
		); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired process slot %q remains: %v", name, err)
		}
	}
}

func TestClientStoreExpiredLeaseDoesNotReviveAfterSlotCleanup(t *testing.T) {
	for _, test := range []struct {
		name string
		use  func(*Store, uint64) error
	}{
		{"RENEW", func(store *Store, clientID uint64) error {
			return store.Renew(clientID)
		}},
		{"OPEN", func(store *Store, clientID uint64) error {
			return store.MarkOpen(clientID, nfs4.StateID{2})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fs := vfs.NewLocalTernVFS(t.TempDir())
			base := time.Unix(1000, 0)
			owner := ClientOwner{
				Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
			}
			first, clientID := NewConfirmedStoreClient(
				t, fs, []byte("client"), [8]byte{1}, owner)
			first.Now = func() time.Time { return base }
			if err := first.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
				t.Fatal(err)
			}

			collector, err := NewStore(fs)
			if err != nil {
				t.Fatal(err)
			}
			collector.Now = func() time.Time {
				return base.Add(LeaseWriteTerm + LeaseTime + time.Second)
			}
			if _, _, err := collector.leaseStatus(
				vfs.InodeID(clientID),
			); err != nil {
				t.Fatal(err)
			}

			restarted, err := NewStore(fs)
			if err != nil {
				t.Fatal(err)
			}
			restarted.Now = collector.Now
			if err := test.use(
				restarted, clientID,
			); nfs4.ErrorCode(err) != nfs4.NFS4ERR_EXPIRED {
				t.Fatalf("%s after slot cleanup = %v, want NFS4ERR_EXPIRED",
					test.name, err)
			}
		})
	}
}

func TestClientStoreCachedRenewSeesExpiredMarker(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	first, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	first.Now = func() time.Time { return base }
	stateID := nfs4.StateID{1}
	if err := first.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	second, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second.Now = func() time.Time {
		return base.Add(LeaseWriteTerm + time.Second)
	}
	if live, found, err := second.leaseStatus(
		vfs.InodeID(clientID),
	); err != nil {
		t.Fatal(err)
	} else if live || !found {
		t.Fatalf("expired lease status = live %t, found %t", live, found)
	}

	first.Now = func() time.Time {
		return base.Add(LeaseRenewAfter + time.Second)
	}
	if active, err := first.HasOpen(
		clientID, stateID,
	); active || nfs4.ErrorCode(err) != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("cached renewal after fleet expiry = active %t, err %v",
			active, err)
	}
}

func TestClientStorePersistsExpiryBeforeSlotCleanup(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &RemoveFailureVFS{TernVFS: baseFS}
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.Now = func() time.Time { return base }
	if err := store.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}

	store.Now = func() time.Time {
		return base.Add(LeaseWriteTerm + LeaseTime + time.Second)
	}
	fs.Name = store.LeaseName
	fs.Remaining = 1
	if _, _, err := store.leaseStatus(
		vfs.InodeID(clientID),
	); !errors.Is(err, ErrInjectedRemove) {
		t.Fatalf("lease cleanup error = %v, want injected remove failure", err)
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(clientID), ExpiredName,
	); err != nil {
		t.Fatalf("expiry was not durable before slot cleanup: %v", err)
	}
}

func TestClientStoreIsConfirmedUsesCachedLocation(t *testing.T) {
	fs := &CountingVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	store, clientID := NewConfirmedStoreClient(
		t,
		fs,
		[]byte("client"),
		[8]byte{1},
		ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}},
	)
	if err := store.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}
	fs.Reset()
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
	fs := &CountingVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.Now = func() time.Time { return base }
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	// The fast path, while this process's published lease lasts: the
	// confirmed pointer and the expired marker, and no lease write. Lease
	// writes are the refresher's alone (canonical r3 §9.1).
	for _, elapsed := range []time.Duration{
		time.Second,
		LeaseRenewAfter + time.Second,
		2 * (LeaseRenewAfter + time.Second),
	} {
		store.Now = func() time.Time { return base.Add(elapsed) }
		fs.Reset()
		active, err := store.HasOpen(clientID, stateID)
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			t.Fatalf("fresh open was not active at %s", elapsed)
		}
		if fs.lookup != 2 || fs.lookupParent != 0 || fs.readdir != 0 ||
			fs.stat != 0 || fs.read != 0 || fs.readlink != 0 ||
			fs.CreateFiles != 0 || fs.Renames != 0 {
			t.Fatalf("fast HasOpen calls at %s: lookup=%d parent=%d readdir=%d stat=%d read=%d readlink=%d create=%d rename=%d",
				elapsed, fs.lookup, fs.lookupParent, fs.readdir, fs.stat, fs.read,
				fs.readlink, fs.CreateFiles, fs.Renames)
		}
	}
	// Past the published lease, HasOpen reads the durable lease, finds it
	// dead and writes nothing.
	store.Now = func() time.Time { return base.Add(LeaseWriteTerm + time.Second) }
	fs.Reset()
	if active, err := store.HasOpen(clientID, stateID); err != nil || active {
		t.Fatalf("open past the lease %t %v", active, err)
	}
	if fs.CreateFiles != 0 || fs.Renames != 0 {
		t.Fatalf("HasOpen wrote: create=%d rename=%d", fs.CreateFiles, fs.Renames)
	}
}

func TestClientStoreMarkOpenKeepsExistingMarkerInode(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	store, clientID := NewConfirmedStoreClient(
		t,
		fs,
		[]byte("client"),
		[8]byte{1},
		ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}},
	)
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}
	before, err := fs.Lookup(vfs.InodeID(clientID), ActiveOpenName(stateID))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}
	after, err := fs.Lookup(vfs.InodeID(clientID), ActiveOpenName(stateID))
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("replayed OPEN replaced marker inode %d with %d",
			before, after)
	}
}

func TestClientStoreImplicitRenewalRechecksReboot(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	hookFS := &renameHookVFS{TernVFS: baseFS}
	first, err := NewStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.Now = func() time.Time { return base }
	second.Now = func() time.Time { return base }
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	oldID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmClientID(
		oldID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	stateID := nfs4.StateID{1}
	if err := first.MarkOpen(oldID, stateID); err != nil {
		t.Fatal(err)
	}
	newID, confirm, err := second.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	// HasOpen writes no lease (canonical r3 §9.1), so a reboot confirmed by
	// another process cannot be raced by a renewal: the cached open sees the
	// confirmed pointer move and reports nothing.
	if _, err := second.ConfirmClientID(newID, confirm, owner.Principal); err != nil {
		t.Fatalf("reboot confirmation failed: %v", err)
	}
	first.Now = func() time.Time {
		return base.Add(LeaseRenewAfter + time.Second)
	}
	active, err := first.HasOpen(oldID, stateID)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("cached open survived reboot confirmation")
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(oldID), first.LeaseName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reboot confirmation left a stale lease: %v", err)
	}
	_ = hookFS
	if _, err := baseFS.Lookup(
		vfs.InodeID(oldID), ActiveOpenName(stateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reboot confirmation left a stale open marker: %v", err)
	}
}

func TestClientStoreRejectsClientIDOutsideStateDirectory(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := fs.Mkdir(fs.RootID(), "user")
	if err != nil {
		t.Fatal(err)
	}
	incarnationID, err := fs.Mkdir(
		parentID, IncarnationPrefix+strings.Repeat("0", 32))
	if err != nil {
		t.Fatal(err)
	}
	confirm := [8]byte{1}
	record := newDurableClientRecord(
		[]byte("client"), [8]byte{1}, confirm, ClientOwner{})
	if _, err := store.CreateJSON(
		incarnationID, ClientRecordName, record,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.replaceIdentitySymlink(
		parentID, pendingName, incarnationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		uint64(incarnationID), confirm, RPCPrincipal{},
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid confirmation error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(
		uint64(incarnationID), stateID,
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid MarkOpen error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if _, err := fs.Lookup(
		incarnationID, ActiveOpenName(stateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("forged clientid created an open marker: %v", err)
	}
	if err := store.RemoveOpen(
		uint64(incarnationID), stateID,
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("forged clientid RemoveOpen error = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	if err := store.Renew(
		uint64(incarnationID),
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
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

type readAllHookVFS struct {
	vfs.TernVFS
	id   vfs.InodeID
	once sync.Once
	hook func()
}

func (fs *readAllHookVFS) ReadAll(id vfs.InodeID) ([]byte, error) {
	if id == fs.id {
		fs.once.Do(fs.hook)
	}
	return fs.TernVFS.ReadAll(id)
}

type renameHookVFS struct {
	vfs.TernVFS
	dstName string
	once    sync.Once
	hook    func()
}

type beforeRenameHookVFS struct {
	vfs.TernVFS
	dstName string
	once    sync.Once
	hook    func()
}

type renameFailureVFS struct {
	vfs.TernVFS
	dstName   string
	remaining int
}

var errInjectedRename = errors.New("injected rename failure")

func (fs *renameFailureVFS) Rename(
	srcDirID vfs.InodeID,
	srcName string,
	dstDirID vfs.InodeID,
	dstName string,
) error {
	if dstName == fs.dstName && fs.remaining > 0 {
		fs.remaining--
		return errInjectedRename
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func (fs *renameFailureVFS) RenameEdge(srcDirID vfs.InodeID, srcName string, edge vfs.Edge, dstDirID vfs.InodeID, dstName string) error {
	if dstName == fs.dstName && fs.remaining > 0 {
		fs.remaining--
		return errInjectedRename
	}
	return fs.TernVFS.RenameEdge(srcDirID, srcName, edge, dstDirID, dstName)
}

func (fs *renameHookVFS) Rename(
	srcDirID vfs.InodeID,
	srcName string,
	dstDirID vfs.InodeID,
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
	srcDirID vfs.InodeID,
	srcName string,
	dstDirID vfs.InodeID,
	dstName string,
) error {
	if dstName == fs.dstName {
		fs.once.Do(fs.hook)
	}
	return fs.TernVFS.Rename(srcDirID, srcName, dstDirID, dstName)
}

func TestClientStoreConfirmationRechecksPending(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	hookFS := &renameHookVFS{TernVFS: baseFS}
	first, err := NewStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	oldID, oldConfirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	var newID uint64
	var newConfirm [8]byte
	hookFS.dstName = first.ConfirmingName
	hookFS.hook = func() {
		newID, newConfirm, err = second.SetClientID(
			[8]byte{2}, []byte("client"), owner)
	}
	_, confirmErr := first.ConfirmClientID(
		oldID, oldConfirm, owner.Principal)
	if err != nil {
		t.Fatal(err)
	}
	if nfs4.ErrorCode(confirmErr) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("superseded confirmation error = %v, want NFS4ERR_STALE_CLIENTID",
			confirmErr)
	}
	if _, err := second.ConfirmClientID(
		newID, newConfirm, owner.Principal,
	); err != nil {
		t.Fatalf("replacement confirmation failed: %v", err)
	}
	if confirmed, err := first.IsConfirmed(oldID); err != nil {
		t.Fatal(err)
	} else if confirmed {
		t.Fatal("superseded confirmation replaced the confirmed pointer")
	}
}

func TestClientStoreHasOpenRequiresLiveDurableLease(t *testing.T) {
	for _, cached := range []bool{false, true} {
		for _, expired := range []bool{false, true} {
			t.Run(fmt.Sprintf("cached=%t/expired=%t", cached, expired), func(t *testing.T) {
				fs := vfs.NewLocalTernVFS(t.TempDir())
				store, id := NewConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, ClientOwner{})
				now := time.Unix(1000, 0)
				store.Now = func() time.Time { return now }
				sid := nfs4.StateID{1}
				if err := store.MarkOpen(id, sid); err != nil {
					t.Fatal(err)
				}
				leaseID, err := fs.Lookup(vfs.InodeID(id), store.LeaseName)
				if err != nil {
					t.Fatal(err)
				}
				if cached {
					state, _ := store.LocalClients.Load(vfs.InodeID(id))
					delete(state.(*localClientState).opens, sid)
				} else {
					store.LocalClients.Delete(vfs.InodeID(id))
				}
				if expired {
					now = now.Add(LeaseWriteTerm + time.Second)
				} else if err := fs.Remove(vfs.InodeID(id), store.LeaseName); err != nil {
					t.Fatal(err)
				}
				if active, err := store.HasOpen(id, sid); err != nil || active {
					t.Fatalf("marker without live lease = %v, %v", active, err)
				}
				currentID, err := fs.Lookup(vfs.InodeID(id), store.LeaseName)
				if expired {
					// The expired slot is still inside its cleanup grace period.
					if err != nil || currentID != leaseID {
						t.Fatalf("HasOpen replaced the expired lease: %v, %v", currentID, err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("HasOpen recreated the lease: %v", err)
				}
			})
		}
	}
}

func TestClientStoreMarkOpenKeepsExistingMarkerOnRenewFailure(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &renameFailureVFS{TernVFS: baseFS}
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.Now = func() time.Time { return base }
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	// A refresh that fails while the published lease still lasts does not
	// fail MarkOpen: the lease it relies on is proven (canonical r3 §9.1).
	store.Now = func() time.Time {
		return base.Add(LeaseRenewAfter + time.Second)
	}
	fs.dstName = store.LeaseName
	fs.remaining = 1
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatalf("MarkOpen within the proven lease = %v", err)
	}
	if fs.remaining != 0 {
		t.Fatal("the due refresh was not attempted")
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(clientID), ActiveOpenName(stateID),
	); err != nil {
		t.Fatalf("refresh failure removed existing marker: %v", err)
	}

	// Past the proven lease, with publication still failing, MarkOpen gets
	// DELAY and creates no marker.
	store.Now = func() time.Time { return base.Add(LeaseWriteTerm + time.Second) }
	fs.remaining = 100
	newStateID := nfs4.StateID{2}
	if err := store.MarkOpen(
		clientID, newStateID,
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_DELAY {
		t.Fatalf("new MarkOpen error = %v, want DELAY", err)
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(clientID), ActiveOpenName(newStateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed refresh created a new marker: %v", err)
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(clientID), ActiveOpenName(stateID),
	); err != nil {
		t.Fatalf("refresh failure removed existing marker: %v", err)
	}
}

func TestClientStoreLiveSlotsSkipsRemovedEntry(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &readAllHookVFS{TernVFS: baseFS}
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.Now = func() time.Time { return base }
	if err := store.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}
	leaseID, err := baseFS.Lookup(vfs.InodeID(clientID), store.LeaseName)
	if err != nil {
		t.Fatal(err)
	}
	var hookErr error
	fs.id = leaseID
	fs.hook = func() {
		hookErr = baseFS.Remove(vfs.InodeID(clientID), store.LeaseName)
	}

	live, found, err := store.liveSlots(vfs.InodeID(clientID), LeasePrefix)
	if err != nil {
		t.Fatalf("raced slot scan failed: %v", err)
	}
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	// TernFS keeps a removed file readable by inode until it is collected, so
	// a scan racing the removal reads the stale slot rather than failing.
	if !live || !found {
		t.Fatalf("raced slot scan = live %t, found %t; want true, true",
			live, found)
	}
	if _, err := store.replaceJSON(
		vfs.InodeID(clientID),
		store.LeaseName,
		DurableLease{
			ExpiresUnixNano: base.Add(LeaseTime).UnixNano(),
		},
	); err != nil {
		t.Fatal(err)
	}
	if live, err := store.hasLiveLease(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	} else if !live {
		t.Fatal("replacement lease slot was not live on the next scan")
	}
}

func TestClientStoreRenewalDoesNotBlockActiveState(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &BlockingRenameVFS{
		TernVFS: baseFS,
		Entered: make(chan struct{}),
		Release: make(chan struct{}),
	}
	base := time.Unix(1000, 0)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	store.Now = func() time.Time { return base }
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}

	store.Now = func() time.Time {
		return base.Add(LeaseRenewAfter + time.Second)
	}
	fs.DstName = store.LeaseName
	renewed := make(chan error, 1)
	go func() {
		renewed <- store.Renew(clientID)
	}()
	<-fs.Entered

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
		close(fs.Release)
		t.Fatal("active state blocked behind lease renewal")
	}
	close(fs.Release)
	if err := <-renewed; err != nil {
		t.Fatal(err)
	}
}

func TestClientStoreConfirmLeavesPendingUntilReplacement(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	store, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	identityID, err := fs.LookupParent(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	if pendingID, found, err := store.ReadIdentitySymlink(
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
	if pendingID, found, err := store.ReadIdentitySymlink(
		identityID, pendingName,
	); err != nil {
		t.Fatal(err)
	} else if !found || uint64(pendingID) != replacementID {
		t.Fatalf("replacement pending pointer = %d, %t; want %d, true",
			pendingID, found, replacementID)
	}
}

func TestClientStoreGCDoesNotRemoveInFlightTemporaryEntry(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	hookFS := &beforeRenameHookVFS{
		TernVFS: baseFS,
		dstName: pendingName,
	}
	first, err := NewStore(hookFS)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	identityID, err := first.identityDir([]byte("client"))
	if err != nil {
		t.Fatal(err)
	}
	incarnationID, err := first.NewIncarnation(identityID)
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
	if got, found, err := first.ReadIdentitySymlink(
		identityID, pendingName,
	); err != nil {
		t.Fatal(err)
	} else if !found || got != incarnationID {
		t.Fatalf("pending pointer = %d, %t; want %d, true",
			got, found, incarnationID)
	}
}

func TestClientStoreGCCollectsTemporaryAndHalfCreatedEntries(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := NewConfirmedStoreClient(
		t, fs, []byte("client"), [8]byte{1}, owner)
	identityID, err := fs.LookupParent(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	tempName := TempPrefix + "orphan"
	tempID, err := fs.CreateFile(identityID, tempName, nil)
	if err != nil {
		t.Fatal(err)
	}
	incarnationTempName := TempPrefix + "incarnation-orphan"
	incarnationTempID, err := fs.CreateFile(
		vfs.InodeID(clientID), incarnationTempName, nil)
	if err != nil {
		t.Fatal(err)
	}
	halfCreated, err := fs.Mkdir(
		identityID, IncarnationPrefix+strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.Now = func() time.Time { return base }
	oldTemp := base.Add(-LeaseTime - time.Second)
	if err := fs.SetTime(tempID, &oldTemp, nil); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetTime(incarnationTempID, &oldTemp, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(
		identityID, tempName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary entry remains after GC: %v", err)
	}
	if _, err := fs.Lookup(
		vfs.InodeID(clientID), incarnationTempName,
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incarnation temporary entry remains after GC: %v", err)
	}
	store.Now = func() time.Time {
		return base.Add(ClientGCGrace + time.Second)
	}
	if err := store.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(halfCreated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("half-created incarnation remains after GC: %v", err)
	}
}

func TestClientStoreGCHonorsConfirmationClaims(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	first, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	first.Now = func() time.Time { return base }
	second.Now = func() time.Time { return base.Add(30 * time.Second) }
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	oldID, _, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	err = first.beginConfirmation(vfs.InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}
	err = second.beginConfirmation(vfs.InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}
	clientID, _, err := first.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}

	collector.Now = func() time.Time { return base.Add(100 * time.Second) }
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(vfs.InodeID(oldID), GCCandidateName); !os.IsNotExist(err) {
		t.Fatalf("live confirmation claim did not protect incarnation: %v", err)
	}

	collector.Now = func() time.Time { return base.Add(121 * time.Second) }
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(vfs.InodeID(oldID), GCCandidateName); err != nil {
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
	fs := vfs.NewLocalTernVFS(t.TempDir())
	writer, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	confirmer, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	writer.Now = func() time.Time { return base }
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
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
	collector.Now = func() time.Time { return base }
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lookup(vfs.InodeID(oldID), GCCandidateName); err != nil {
		t.Fatalf("incarnation was not aged into a GC candidate: %v", err)
	}

	// A confirmation claims oldID only after it was already aged.
	confirmer.Now = func() time.Time { return base.Add(30 * time.Second) }
	err = confirmer.beginConfirmation(vfs.InodeID(oldID))
	if err != nil {
		t.Fatal(err)
	}

	// Once the candidate is due, the delete-time pass must recheck the live
	// claim, not just the roots snapshot from before the claim existed.
	collector.Now = func() time.Time { return base.Add(LeaseTime + time.Second) }
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(vfs.InodeID(oldID)); err != nil {
		t.Fatalf("GC removed an incarnation with a live confirmation claim: %v", err)
	}
}

func TestClientStoreRebootCleanupIsRetryable(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	first, err := NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	failingFS := &RemoveFailureVFS{TernVFS: baseFS, Remaining: 1}
	second, err := NewStore(failingFS)
	if err != nil {
		t.Fatal(err)
	}
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	oldID, confirm, err := first.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmClientID(
		oldID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	var stateID nfs4.StateID
	stateID[0] = 1
	if err := first.MarkOpen(oldID, stateID); err != nil {
		t.Fatal(err)
	}
	failingFS.Name = ActiveOpenName(stateID)

	newID, confirm, err := first.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		newID, confirm, owner.Principal,
	); !errors.Is(err, ErrInjectedRemove) {
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
	if err := first.CollectStaleForClient(vfs.InodeID(newID)); err != nil {
		t.Fatal(err)
	}
	if _, err := baseFS.Lookup(vfs.InodeID(oldID), GCCandidateName); !os.IsNotExist(err) {
		t.Fatalf("in-progress reboot target was marked for collection: %v", err)
	}

	replacedID, err := second.ConfirmClientID(
		newID, confirm, owner.Principal)
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

	failingFS.Name = RebootName
	failingFS.Remaining = 1
	thirdID, confirm, err := first.SetClientID(
		[8]byte{3}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ConfirmClientID(
		thirdID, confirm, owner.Principal,
	); !errors.Is(err, ErrInjectedRemove) {
		t.Fatalf("reboot marker cleanup error = %v, want injected failure",
			err)
	}
	replacedID, err = second.ConfirmClientID(
		thirdID, confirm, owner.Principal)
	if err != nil {
		t.Fatal(err)
	}
	if replacedID != newID {
		t.Fatalf("marker retry replaced clientid %d, want %d", replacedID, newID)
	}
}

// The HasOpen slow path rewrites the lease in TernFS. That rewrite must not
// hold state.mu, or every fast-path stateid check for the client waits on it.
func TestClientStoreHasOpenSlowPathDoesNotBlockFastPath(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	hookFS := &markerLookupGateVFS{TernVFS: baseFS, Gate: testutil.NewGate()}
	defer testutil.CloseSignal(hookFS.Release)
	owner := ClientOwner{
		Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	first, clientID := NewConfirmedStoreClient(
		t, hookFS, []byte("client"), [8]byte{1}, owner)
	second, err := NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}

	cachedState := nfs4.StateID{1}
	if err := first.MarkOpen(clientID, cachedState); err != nil {
		t.Fatal(err)
	}
	// An open recorded by another nfsd is only visible to first through the
	// durable marker, so HasOpen takes the slow path for it.
	remoteState := nfs4.StateID{2}
	if err := second.MarkOpen(clientID, remoteState); err != nil {
		t.Fatal(err)
	}
	hookFS.name = ActiveOpenName(remoteState)

	type result struct {
		open bool
		err  error
	}
	slow := make(chan result, 1)
	go func() {
		open, err := first.HasOpen(clientID, remoteState)
		slow <- result{open: open, err: err}
	}()
	testutil.AwaitSignal(t, hookFS.Entered, "slow-path marker lookup")

	fast := make(chan result, 1)
	go func() {
		open, err := first.HasOpen(clientID, cachedState)
		fast <- result{open: open, err: err}
	}()
	got := testutil.AwaitValue(t, fast, "fast-path HasOpen during the slow path")
	if got.err != nil {
		t.Fatalf("fast-path HasOpen failed: %v", got.err)
	}
	if !got.open {
		t.Fatal("fast-path HasOpen lost a cached open during the slow path")
	}

	testutil.CloseSignal(hookFS.Release)
	got = testutil.AwaitValue(t, slow, "slow-path HasOpen")
	if got.err != nil {
		t.Fatalf("slow-path HasOpen failed: %v", got.err)
	}
	if !got.open {
		t.Fatal("slow-path HasOpen did not find the durable marker")
	}
}

func TestStagingRecoveryKeySeparatesBootsAndPrincipals(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	cs, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := ClientOwner{Principal: RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "one"}}
	key := func(identity string, verifier byte, principal string) [32]byte {
		owner.Principal.Body = principal
		id, _, err := cs.SetClientID([8]byte{verifier}, []byte(identity), owner)
		if err != nil {
			t.Fatal(err)
		}
		k, err := cs.StagingRecoveryKey(id)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	original := key("client", 1, "one")
	if original != key("client", 1, "one") {
		t.Fatal("incarnation changed recovery identity")
	}
	for _, other := range [][32]byte{
		key("client", 2, "one"), key("client", 1, "two"), key("other", 1, "one"),
	} {
		if original == other {
			t.Fatal("recovery identity crossed a client, boot or principal boundary")
		}
	}
}
