// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"testing"
	"time"
)

func assertOpenStateIndex(t *testing.T, store *openStateStore) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()

	indexed := 0
	for key, owner := range store.owners {
		for fileID, state := range owner.states {
			indexed++
			if state.owner != key {
				t.Fatalf("owner index key = %#v, state owner = %#v",
					key, state.owner)
			}
			if state.fileID != fileID {
				t.Fatalf("owner index file = %v, state file = %v",
					fileID, state.fileID)
			}
			if store.states[state.id] != state {
				t.Fatalf("owner index state %x is absent from stateid index",
					state.id)
			}
		}
	}
	if indexed != len(store.states) {
		t.Fatalf("owner index has %d states, stateid index has %d",
			indexed, len(store.states))
	}
	for id, state := range store.states {
		owner := store.owners[state.owner]
		if owner == nil || owner.states[state.fileID] != state {
			t.Fatalf("stateid index state %x is absent from owner index", id)
		}
	}
}

func panicValue(fn func()) (value any) {
	defer func() {
		value = recover()
	}()
	fn()
	return nil
}

func TestAddStateLockedRejectsOccupiedIndexSlot(t *testing.T) {
	for _, test := range []struct {
		name   string
		second openState
	}{
		{
			name: "owner file",
			second: openState{
				id:     StateID{2},
				fileID: MakeInodeID(InodeTypeFile, 1),
			},
		},
		{
			name: "stateid",
			second: openState{
				id:     StateID{1},
				fileID: MakeInodeID(InodeTypeFile, 2),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newOpenStateStore()
			owner := &openOwnerState{}
			first := &openState{
				id:     StateID{1},
				fileID: MakeInodeID(InodeTypeFile, 1),
			}
			add := func(state *openState) error {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				store.mu.Lock()
				defer store.mu.Unlock()
				return store.addStateLocked(owner, state)
			}
			if err := add(first); err != nil {
				t.Fatal(err)
			}
			if err := add(&test.second); err == nil {
				t.Fatal("occupied index slot was overwritten")
			}
			if store.states[first.id] != first ||
				owner.states[first.fileID] != first ||
				len(store.states) != 1 || len(owner.states) != 1 {
				t.Fatal("index changed after rejected insertion")
			}
		})
	}
}

func TestFinishOpenIndexConflictReturnsServerFault(t *testing.T) {
	store := newOpenStateStore()
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	op, _, _, status := store.startOpen(owner, 1)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}

	id := StateID{1}
	store.mu.Lock()
	store.states[id] = &openState{id: id}
	store.mu.Unlock()

	finished := make(chan openOwnerResponse, 1)
	go func() {
		defer op.finishServerFaultIfNeeded()
		finished <- op.finishOpen(
			MakeInodeID(InodeTypeFile, 1), false, id, false)
	}()

	select {
	case response := <-finished:
		if response.status != NFS4ERR_SERVERFAULT {
			t.Fatalf("OPEN status = %s, want NFS4ERR_SERVERFAULT",
				Nfsstat4Name(response.status))
		}
	case <-time.After(testChannelTimeout):
		t.Fatal("OPEN deadlocked after detecting an occupied index slot")
	}
}

func TestWriteReopenReturnsPerm(t *testing.T) {
	store := newOpenStateStore()
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	fileID := MakeInodeID(InodeTypeFile, 1)
	state := addConfirmedOpen(t, store, owner, fileID)

	var status uint32
	if recovered := panicValue(func() {
		_, status = addOpenForTest(store,
			owner, 3, fileID, true, StateID{})
	}); recovered != nil {
		t.Fatalf("write reopen panicked: %v", recovered)
	}
	if status != NFS4ERR_PERM {
		t.Fatalf("write reopen = %s, want NFS4ERR_PERM",
			Nfsstat4Name(status))
	}
	if got, status := store.lookup(
		state.id, state.generation, fileID,
	); status != NFS4_OK || got.write {
		t.Fatalf("original read state after write reopen = %+v, %s",
			got, Nfsstat4Name(status))
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

func abortOwnerOperation(op *openOwnerOperation) {
	op.finished = true
	op.store.releaseOwner(op.owner)
}

func beginOpenForTest(
	os *openStateStore,
	owner openOwnerKey,
	seq uint32,
) (openState, bool, uint32) {
	op, response, replay, status := os.startOpen(owner, seq)
	if op != nil {
		abortOwnerOperation(op)
		return openState{}, false, status
	}
	return response.state, replay, status
}

func addOpenForTest(
	os *openStateStore,
	owner openOwnerKey,
	seq uint32,
	fileID InodeID,
	write bool,
	id StateID,
) (openState, uint32) {
	op, response, replay, status := os.startOpen(owner, seq)
	if op == nil {
		if replay {
			return response.state, response.status
		}
		return openState{}, status
	}
	response = op.finishOpen(fileID, write, id, false)
	return response.state, response.status
}

func confirmOpenForTest(
	os *openStateStore,
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, uint32) {
	op, state, response, replay, status := os.startConfirm(
		id, generation, fileID, seq)
	if op == nil {
		if replay {
			return response.state, response.status
		}
		return state, status
	}
	response = op.finishConfirm(id, generation, fileID)
	return response.state, response.status
}

func validateCloseForTest(
	os *openStateStore,
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, bool, uint32) {
	op, state, response, replay, status := os.startClose(
		id, generation, fileID, seq)
	if op != nil {
		abortOwnerOperation(op)
	}
	if replay {
		return response.state, true, response.status
	}
	return state, false, status
}

func closeOpenForTest(
	os *openStateStore,
	id StateID,
	seq uint32,
) (openState, uint32) {
	os.mu.Lock()
	stored := os.states[id]
	if stored == nil {
		os.mu.Unlock()
		return openState{}, NFS4ERR_BAD_STATEID
	}
	state := *stored
	os.mu.Unlock()
	op, _, response, replay, status := os.startClose(
		id, state.generation, state.fileID, seq)
	if op == nil {
		if replay {
			return response.state, response.status
		}
		return openState{}, status
	}
	response = op.finishClose(id, state.generation, state.fileID)
	return response.state, response.status
}

func addConfirmedOpen(
	t *testing.T,
	store *openStateStore,
	owner openOwnerKey,
	fileID InodeID,
) openState {
	t.Helper()
	state, status := addOpenForTest(store, owner, 1, fileID, false, StateID{})
	if status != NFS4_OK {
		t.Fatalf("OPEN status = %s", Nfsstat4Name(status))
	}
	confirmed, status := confirmOpenForTest(store, state.id, 1, fileID, 2)
	if status != NFS4_OK {
		t.Fatalf("OPEN_CONFIRM status = %s", Nfsstat4Name(status))
	}
	return confirmed
}
