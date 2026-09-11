// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"testing"
)

// These adapters keep focused store tests concise. Production operations use
// start*/finish* directly so owner locks cover their filesystem side effects.

func (op *openOwnerOperation) abort() {
	op.finished = true
	op.store.releaseOwner(op.owner)
}

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
			add := func(state *openState) {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				store.mu.Lock()
				defer store.mu.Unlock()
				store.addStateLocked(owner, state)
			}
			add(first)
			if recovered := panicValue(func() {
				add(&test.second)
			}); recovered == nil {
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

func TestWriteReopenReturnsPerm(t *testing.T) {
	store := newOpenStateStore()
	owner := openOwnerKey{clientID: 1, owner: "owner"}
	fileID := MakeInodeID(InodeTypeFile, 1)
	state := addConfirmedOpen(t, store, owner, fileID)

	var status uint32
	if recovered := panicValue(func() {
		_, status = store.addOpen(
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
	t.Run("confirm", func(t *testing.T) {
		store := newOpenStateStore()
		defer assertOpenStateIndex(t, store)
		owner := openOwnerKey{clientID: 1, owner: "owner"}
		fileID := MakeInodeID(InodeTypeFile, 1)
		state, status := store.addOpen(
			owner, 1, fileID, false, StateID{})
		if status != NFS4_OK {
			t.Fatal(Nfsstat4Name(status))
		}
		store.mu.Lock()
		store.states[state.id].generation = ^uint32(0)
		store.mu.Unlock()
		confirmed, status := store.confirm(
			state.id, ^uint32(0), fileID, 2)
		if status != NFS4_OK || confirmed.generation != 1 {
			t.Fatalf("wrapped OPEN_CONFIRM = generation %d, %s",
				confirmed.generation, Nfsstat4Name(status))
		}
	})

	t.Run("reopen", func(t *testing.T) {
		store := newOpenStateStore()
		defer assertOpenStateIndex(t, store)
		owner := openOwnerKey{clientID: 1, owner: "owner"}
		fileID := MakeInodeID(InodeTypeFile, 1)
		state := addConfirmedOpen(t, store, owner, fileID)
		store.mu.Lock()
		store.states[state.id].generation = ^uint32(0)
		store.mu.Unlock()
		reopened, status := store.addOpen(
			owner, 3, fileID, false, StateID{})
		if status != NFS4_OK || reopened.generation != 1 {
			t.Fatalf("wrapped reopen = generation %d, %s",
				reopened.generation, Nfsstat4Name(status))
		}
	})

	t.Run("close", func(t *testing.T) {
		store := newOpenStateStore()
		defer assertOpenStateIndex(t, store)
		owner := openOwnerKey{clientID: 1, owner: "owner"}
		fileID := MakeInodeID(InodeTypeFile, 1)
		state := addConfirmedOpen(t, store, owner, fileID)
		store.mu.Lock()
		store.states[state.id].generation = ^uint32(0)
		store.mu.Unlock()
		closed, status := store.close(state.id, 3)
		if status != NFS4_OK || closed.generation != 1 {
			t.Fatalf("wrapped CLOSE = generation %d, %s",
				closed.generation, Nfsstat4Name(status))
		}
	})

	t.Run("recovered close", func(t *testing.T) {
		store := newOpenStateStore()
		var id StateID
		binary.BigEndian.PutUint32(id[:4], store.epoch+1)
		op, _, _, status := store.startRecoveredClose(
			id, MakeInodeID(InodeTypeFile, 1), ^uint32(0), 1)
		if status != NFS4_OK {
			t.Fatal(Nfsstat4Name(status))
		}
		response := op.finish(NFS4_OK)
		if response.state.generation != 1 {
			t.Fatalf("wrapped recovered CLOSE generation = %d, want 1",
				response.state.generation)
		}
	})
}

func ownerOperationResult(
	op *openOwnerOperation,
	state openState,
	response openOwnerResponse,
	replay bool,
	status uint32,
	finish func(*openOwnerOperation) openOwnerResponse,
) (openState, bool, uint32) {
	if op == nil {
		if replay {
			return response.state, true, response.status
		}
		return state, false, status
	}
	if finish == nil {
		op.abort()
		return state, false, status
	}
	response = finish(op)
	return response.state, false, response.status
}

func (os *openStateStore) beginOpen(
	key openOwnerKey,
	seq uint32,
) (openState, bool, uint32) {
	op, response, replay, status := os.startOpen(key, seq)
	return ownerOperationResult(
		op, openState{}, response, replay, status, nil)
}

func (os *openStateStore) addOpen(
	owner openOwnerKey,
	seq uint32,
	fileID InodeID,
	write bool,
	id StateID,
) (openState, uint32) {
	op, response, replay, status := os.startOpen(owner, seq)
	state, _, status := ownerOperationResult(
		op, openState{}, response, replay, status,
		func(op *openOwnerOperation) openOwnerResponse {
			return op.finishOpen(fileID, write, id, false)
		})
	return state, status
}

func (os *openStateStore) confirm(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, uint32) {
	op, state, response, replay, status := os.startConfirm(
		id, generation, fileID, seq)
	state, _, status = ownerOperationResult(
		op, state, response, replay, status,
		func(op *openOwnerOperation) openOwnerResponse {
			return op.finishConfirm(id, generation, fileID)
		})
	return state, status
}

func (os *openStateStore) validateClose(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, bool, uint32) {
	op, state, response, replay, status := os.startClose(
		id, generation, fileID, seq)
	return ownerOperationResult(op, state, response, replay, status, nil)
}

func (os *openStateStore) close(
	id StateID,
	seq uint32,
) (openState, uint32) {
	os.mu.Lock()
	stored := os.states[id]
	if stored == nil {
		os.mu.Unlock()
		return openState{}, NFS4ERR_BAD_STATEID
	}
	generation := stored.generation
	fileID := stored.fileID
	os.mu.Unlock()
	op, _, response, replay, status := os.startClose(
		id, generation, fileID, seq)
	state, _, status := ownerOperationResult(
		op, *stored, response, replay, status,
		func(op *openOwnerOperation) openOwnerResponse {
			return op.finishClose(id, generation, fileID)
		})
	return state, status
}

func addConfirmedOpen(
	t *testing.T,
	store *openStateStore,
	owner openOwnerKey,
	fileID InodeID,
) openState {
	t.Helper()
	state, status := store.addOpen(owner, 1, fileID, false, StateID{})
	if status != NFS4_OK {
		t.Fatalf("OPEN status = %s", Nfsstat4Name(status))
	}
	confirmed, status := store.confirm(state.id, 1, fileID, 2)
	if status != NFS4_OK {
		t.Fatalf("OPEN_CONFIRM status = %s", Nfsstat4Name(status))
	}
	return confirmed
}
