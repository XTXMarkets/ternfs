// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
)

type openOwnerKey struct {
	clientID uint64
	owner    string
}

type openState struct {
	id                     StateID
	fileID                 InodeID
	owner                  openOwnerKey
	write                  bool
	generation             uint32
	confirmed              bool
	confirmSeq             uint32
	confirmInputGeneration uint32
	closed                 bool
	closeSeq               uint32
	closeInputGeneration   uint32
}

type openOwnerState struct {
	nextSeq     uint32
	initialized bool
	lastOpenSeq uint32
	lastOpenID  StateID
}

type openStateStore struct {
	mu      sync.Mutex
	epoch   uint32
	nextID  uint64
	states  map[StateID]*openState
	owners  map[openOwnerKey]*openOwnerState
	expired map[StateID]bool
}

func newOpenStateStore() *openStateStore {
	var epochBytes [4]byte
	for {
		if _, err := rand.Read(epochBytes[:]); err != nil {
			panic("generate NFS open-state epoch: " + err.Error())
		}
		epoch := binary.BigEndian.Uint32(epochBytes[:])
		if epoch > 1 {
			return &openStateStore{
				epoch:   epoch,
				states:  make(map[StateID]*openState),
				owners:  make(map[openOwnerKey]*openOwnerState),
				expired: make(map[StateID]bool),
			}
		}
	}
}

func (os *openStateStore) newStateIDLocked() StateID {
	os.nextID++
	var id StateID
	binary.BigEndian.PutUint32(id[0:4], os.epoch)
	binary.BigEndian.PutUint64(id[4:12], os.nextID)
	return id
}

func (os *openStateStore) newStateID() StateID {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.newStateIDLocked()
}

func (os *openStateStore) beginOpen(
	owner openOwnerKey,
	seq uint32,
) (openState, bool, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.owners[owner]
	if state == nil || !state.initialized || seq == state.nextSeq {
		return openState{}, false, NFS4_OK
	}
	if seq == state.lastOpenSeq && state.lastOpenID != (StateID{}) {
		open := os.states[state.lastOpenID]
		if open != nil {
			return *open, true, NFS4_OK
		}
	}
	return openState{}, false, NFS4ERR_BAD_SEQID
}

func (os *openStateStore) addOpen(
	owner openOwnerKey,
	seq uint32,
	fileID InodeID,
	write bool,
	id StateID,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()

	ownerState := os.owners[owner]
	if ownerState == nil {
		ownerState = &openOwnerState{}
		os.owners[owner] = ownerState
	}
	if ownerState.initialized && seq != ownerState.nextSeq {
		return openState{}, NFS4ERR_BAD_SEQID
	}

	if id == (StateID{}) {
		id = os.newStateIDLocked()
	} else {
		binary.BigEndian.PutUint32(id[0:4], os.epoch)
	}
	state := &openState{
		id:         id,
		fileID:     fileID,
		owner:      owner,
		write:      write,
		generation: 1,
	}
	os.states[id] = state
	ownerState.initialized = true
	ownerState.lastOpenSeq = seq
	ownerState.lastOpenID = id
	ownerState.nextSeq = seq + 1
	return *state, NFS4_OK
}

func (os *openStateStore) confirm(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()

	state := os.states[id]
	if state != nil && state.fileID == fileID && state.confirmed &&
		seq == state.confirmSeq &&
		generation == state.confirmInputGeneration {
		return *state, NFS4_OK
	}
	state, status := os.lookupLocked(id, generation, fileID)
	if status != NFS4_OK {
		return openState{}, status
	}
	owner := os.owners[state.owner]
	if owner == nil || seq != owner.nextSeq {
		return openState{}, NFS4ERR_BAD_SEQID
	}
	if state.confirmed {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	state.confirmed = true
	state.confirmSeq = seq
	state.confirmInputGeneration = generation
	state.generation++
	owner.lastOpenID = StateID{}
	owner.nextSeq++
	return *state, NFS4_OK
}

func (os *openStateStore) lookup(
	id StateID,
	generation uint32,
	fileID InodeID,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state, status := os.lookupLocked(id, generation, fileID)
	if status == NFS4_OK && (!state.confirmed || state.closed) {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	if status != NFS4_OK {
		return openState{}, status
	}
	return *state, NFS4_OK
}

func (os *openStateStore) lookupLocked(
	id StateID,
	generation uint32,
	fileID InodeID,
) (*openState, uint32) {
	if os.expired[id] {
		return nil, NFS4ERR_EXPIRED
	}
	state := os.states[id]
	if state == nil {
		epoch := binary.BigEndian.Uint32(id[0:4])
		if epoch == 1 {
			return nil, NFS4ERR_STALE_STATEID
		}
		return nil, NFS4ERR_BAD_STATEID
	}
	if state.fileID != fileID {
		return nil, NFS4ERR_BAD_STATEID
	}
	if generation < state.generation {
		return nil, NFS4ERR_OLD_STATEID
	}
	if generation > state.generation {
		return nil, NFS4ERR_BAD_STATEID
	}
	return state, NFS4_OK
}

func (os *openStateStore) validateClose(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (openState, bool, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()

	if os.expired[id] {
		return openState{}, false, NFS4ERR_EXPIRED
	}
	state := os.states[id]
	if state == nil {
		epoch := binary.BigEndian.Uint32(id[0:4])
		if epoch == 1 {
			return openState{}, false, NFS4ERR_STALE_STATEID
		}
		return openState{}, false, NFS4ERR_BAD_STATEID
	}
	if state.fileID != fileID {
		return openState{}, false, NFS4ERR_BAD_STATEID
	}
	if state.closed && seq == state.closeSeq &&
		generation == state.closeInputGeneration {
		return *state, true, NFS4_OK
	}
	if generation < state.generation {
		return openState{}, false, NFS4ERR_OLD_STATEID
	}
	if generation > state.generation {
		return openState{}, false, NFS4ERR_BAD_STATEID
	}
	if !state.confirmed {
		return openState{}, false, NFS4ERR_BAD_STATEID
	}
	owner := os.owners[state.owner]
	if owner == nil || seq != owner.nextSeq {
		return openState{}, false, NFS4ERR_BAD_SEQID
	}
	return *state, false, NFS4_OK
}

func (os *openStateStore) close(
	id StateID,
	seq uint32,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		if os.expired[id] {
			return openState{}, NFS4ERR_EXPIRED
		}
		return openState{}, NFS4ERR_BAD_STATEID
	}
	if state.closed {
		return *state, NFS4_OK
	}
	state.closeSeq = seq
	state.closeInputGeneration = state.generation
	state.generation++
	state.closed = true
	if owner := os.owners[state.owner]; owner != nil {
		owner.lastOpenID = StateID{}
		owner.nextSeq++
	}
	return *state, NFS4_OK
}

func (os *openStateStore) canRecover(id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	if os.expired[id] || os.states[id] != nil {
		return false
	}
	return binary.BigEndian.Uint32(id[0:4]) != os.epoch
}

func (os *openStateStore) closeRecovered(
	id StateID,
	fileID InodeID,
	generation uint32,
	seq uint32,
) openState {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := &openState{
		id:                   id,
		fileID:               fileID,
		generation:           generation + 1,
		confirmed:            true,
		closed:               true,
		closeSeq:             seq,
		closeInputGeneration: generation,
	}
	os.states[id] = state
	return *state
}

func (os *openStateStore) purgeClient(clientID uint64) []InodeID {
	if clientID == 0 {
		return nil
	}
	os.mu.Lock()
	defer os.mu.Unlock()
	fileIDs := make(map[InodeID]struct{})
	for id, state := range os.states {
		if state.owner.clientID == clientID {
			delete(os.states, id)
			os.expired[id] = true
			fileIDs[state.fileID] = struct{}{}
		}
	}
	for owner := range os.owners {
		if owner.clientID == clientID {
			delete(os.owners, owner)
		}
	}
	result := make([]InodeID, 0, len(fileIDs))
	for fileID := range fileIDs {
		result = append(result, fileID)
	}
	return result
}
