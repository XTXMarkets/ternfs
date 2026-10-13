// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

func assertOpenStateIndex(t *testing.T, store *openStateStore) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()

	for key, state := range store.byFile {
		if state.owner != key.owner || state.fileID != key.fileID {
			t.Fatalf("file index key = %#v, state = %#v", key, state)
		}
		if store.states[state.id] != state {
			t.Fatalf("file index state %x is absent from stateid index", state.id)
		}
	}
	if len(store.byFile) != len(store.states) {
		t.Fatalf("file index has %d states, stateid index has %d",
			len(store.byFile), len(store.states))
	}
}

func TestOpenRejectsOccupiedStateID(t *testing.T) {
	store := newOpenStateStore()
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	first, err := store.open(owner, MakeInodeID(InodeTypeFile, 1), MakeInodeID(InodeTypeFile, 1), testGrant(false), StateID{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.open(
		owner, MakeInodeID(InodeTypeFile, 2), MakeInodeID(InodeTypeFile, 2), testGrant(false), first.id,
	); err == nil {
		t.Fatal("occupied stateid was reused for another file")
	}
	assertOpenStateIndex(t, store)
}

// A write OPEN by an owner that holds a read open upgrades the same
// stateid to the new writer's staging (RFC 8881 section 18.16.3).
func TestWriteReopenUpgrades(t *testing.T) {
	store := newOpenStateStore()
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	fileID := MakeInodeID(InodeTypeFile, 1)
	stagingID := MakeInodeID(InodeTypeFile, 2)
	state := addConfirmedOpen(t, store, owner, fileID)
	upgraded, err := store.open(owner, fileID, stagingID, testGrant(true), StateID{})
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.id != state.id || !upgraded.writes() || upgraded.dataID != stagingID ||
		upgraded.generation != state.generation+1 {
		t.Fatalf("upgrade = %+v from %+v", upgraded, state)
	}
	assertOpenStateIndex(t, store)
}

func TestStateidGenerationWrapsToOne(t *testing.T) {
	for _, test := range []struct {
		generation uint32
		want       uint32
	}{
		{0, 1},
		{1, 2},
		{^uint32(0) - 1, ^uint32(0)},
		{^uint32(0), 1},
	} {
		if got := nextStateidGeneration(test.generation); got != test.want {
			t.Errorf("nextStateidGeneration(%d) = %d, want %d",
				test.generation, got, test.want)
		}
	}
}

// testGrant is the grant of a test's read or write OPEN.
func testGrant(write bool) grant {
	if write {
		return grant{access: nfs4.OPEN4_SHARE_ACCESS_BOTH}
	}
	return grant{access: nfs4.OPEN4_SHARE_ACCESS_READ}
}

func addOpenForTest(
	os *openStateStore,
	owner openOwnerKey,
	fileID InodeID,
	write bool,
	id StateID,
) (openState, uint32) {
	state, err := os.open(owner, fileID, fileID, testGrant(write), id)
	if err != nil {
		return openState{}, nfs4.NFS4ERR_SERVERFAULT
	}
	return state, nfs4.NFS4_OK
}

func addConfirmedOpen(
	t *testing.T,
	store *openStateStore,
	owner openOwnerKey,
	fileID InodeID,
) openState {
	t.Helper()
	state, status := addOpenForTest(store, owner, fileID, false, StateID{})
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}
	return state
}

func TestReopenAdvancesGeneration(t *testing.T) {
	store := newOpenStateStore()
	defer assertOpenStateIndex(t, store)
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	fileID := MakeInodeID(InodeTypeFile, 1)
	first := addConfirmedOpen(t, store, owner, fileID)
	second := addConfirmedOpen(t, store, owner, fileID)
	if second.id != first.id || second.generation != first.generation+1 {
		t.Fatalf("reopen = %+v after %+v", second, first)
	}
	if _, status := store.lookup(owner.clientID, first.id, first.generation, fileID); status != nfs4.NFS4ERR_OLD_STATEID {
		t.Fatalf("old generation = %s", nfs4.Nfsstat4Name(status))
	}
	if _, status := store.lookup(owner.clientID, first.id, 0, fileID); status != nfs4.NFS4_OK {
		t.Fatalf("current-generation seqid 0 = %s", nfs4.Nfsstat4Name(status))
	}
}

func TestOpenStateStoreLookupClassifiesStateids(t *testing.T) {
	store := newOpenStateStore()
	defer assertOpenStateIndex(t, store)
	owner := openOwnerKey{clientID: 7, owner: "owner"}
	fileID := MakeInodeID(InodeTypeFile, 1)
	state := addConfirmedOpen(t, store, owner, fileID)
	for _, test := range []struct {
		name       string
		id         StateID
		generation uint32
		fileID     InodeID
		want       uint32
	}{
		{"current", state.id, 1, fileID, nfs4.NFS4_OK},
		{"future generation", state.id, 2, fileID, nfs4.NFS4ERR_BAD_STATEID},
		{"wrong file", state.id, 1, MakeInodeID(InodeTypeFile, 2), nfs4.NFS4ERR_BAD_STATEID},
		{"unknown", StateID{}, 0, fileID, nfs4.NFS4ERR_BAD_STATEID},
	} {
		if _, status := store.lookup(owner.clientID, test.id, test.generation, test.fileID); status != test.want {
			t.Errorf("%s: status = %s, want %s", test.name,
				nfs4.Nfsstat4Name(status), nfs4.Nfsstat4Name(test.want))
		}
	}
	closed, ok := store.close(state.id)
	if !ok || closed.generation != 2 {
		t.Fatalf("close = %+v, %v", closed, ok)
	}
	if _, status := store.lookup(owner.clientID, state.id, 1, fileID); status != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("closed stateid = %s", nfs4.Nfsstat4Name(status))
	}
	other := addConfirmedOpen(t, store, owner, fileID)
	store.expire(other.id)
	if _, status := store.lookup(owner.clientID, other.id, 1, fileID); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("expired stateid = %s", nfs4.Nfsstat4Name(status))
	}
}
