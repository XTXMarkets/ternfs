// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import "testing"

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
