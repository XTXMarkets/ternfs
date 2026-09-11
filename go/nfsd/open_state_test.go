// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

// These adapters keep focused store tests concise. Production operations use
// start*/finish* directly so owner locks cover their filesystem side effects.

func (os *openStateStore) beginOpen(
	key openOwnerKey,
	seq uint32,
) (openState, bool, uint32) {
	op, response, replay, status := os.startOpen(key, seq)
	if op != nil {
		op.finished = true
		os.releaseOwner(op.owner)
	}
	return response.state, replay, status
}

func (os *openStateStore) addOpen(
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

func (os *openStateStore) confirm(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, uint32) {
	op, _, response, replay, status := os.startConfirm(
		id, generation, fileID, seq)
	if op == nil {
		if replay {
			return response.state, response.status
		}
		return openState{}, status
	}
	response = op.finishConfirm(id, generation, fileID)
	return response.state, response.status
}

func (os *openStateStore) validateClose(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, bool, uint32) {
	op, state, response, replay, status := os.startClose(
		id, generation, fileID, seq)
	if op != nil {
		op.finished = true
		os.releaseOwner(op.owner)
	}
	if replay {
		return response.state, true, response.status
	}
	return state, false, status
}

func (os *openStateStore) close(
	id StateID,
	seq uint32,
) (openState, uint32) {
	os.mu.Lock()
	state := os.states[id]
	if state == nil {
		os.mu.Unlock()
		return openState{}, NFS4ERR_BAD_STATEID
	}
	generation := state.generation
	fileID := state.fileID
	os.mu.Unlock()
	op, _, response, replay, status := os.startClose(
		id, generation, fileID, seq)
	if op == nil {
		if replay {
			return response.state, response.status
		}
		return openState{}, status
	}
	response = op.finishClose(id, generation, fileID)
	return response.state, response.status
}

func (os *openStateStore) closeRecovered(
	id StateID,
	fileID InodeID,
	generation uint32,
	seq uint32,
) openState {
	op, response, replay, status := os.startRecoveredClose(
		id, fileID, generation, seq)
	if op == nil {
		if replay && status == NFS4_OK {
			return response.state
		}
		return openState{}
	}
	return op.finish(NFS4_OK).state
}
