// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

const (
	maxRecoveredCloseResponses = 1024
	maxTombstones              = 1024
)

type openOwnerKey struct {
	clientID uint64
	owner    string
}

type openState struct {
	id         StateID
	fileID     InodeID
	owner      openOwnerKey
	write      bool
	generation uint32
	confirmed  bool
}

type openOwnerOperationKind uint8

const (
	openOwnerOperationOpen openOwnerOperationKind = iota + 1
	openOwnerOperationConfirm
	openOwnerOperationClose
)

type openOwnerResponse struct {
	kind            openOwnerOperationKind
	seq             uint32
	status          uint32
	state           openState
	inputStateID    StateID
	inputGeneration uint32
	inputFileID     InodeID
	changeBefore    uint64
	changeAfter     uint64
	requireConfirm  bool
}

func (r openOwnerResponse) matches(
	kind openOwnerOperationKind,
	seq uint32,
	stateID StateID,
	generation uint32,
	fileID InodeID,
) bool {
	if r.kind != kind || r.seq != seq {
		return false
	}
	if kind == openOwnerOperationOpen {
		return true
	}
	return r.inputStateID == stateID &&
		r.inputGeneration == generation &&
		r.inputFileID == fileID
}

type openOwnerState struct {
	mu sync.Mutex

	nextSeq      uint32
	initialized  bool
	confirmed    bool
	lastResponse openOwnerResponse
	lastUsed     time.Time

	// states indexes this owner's open states by file. Protected by
	// openStateStore.mu, like openStateStore.states. Every entry here is
	// also present in openStateStore.states, and every state in
	// openStateStore.states whose owner is this owner is present here.
	states map[InodeID]*openState
}

type recoveredCloseLock struct {
	mu   sync.Mutex
	refs int
}

type openStateStore struct {
	mu sync.Mutex

	epoch         uint32
	nextID        uint64
	states        map[StateID]*openState
	owners        map[openOwnerKey]*openOwnerState
	replayOwners  map[StateID]*openOwnerState
	expired       map[StateID]struct{}
	expiredIDs    []StateID
	recovered     map[StateID]openOwnerResponse
	recoveredIDs  []StateID
	recoveryLocks map[StateID]*recoveredCloseLock
	now           func() time.Time
}

type openOwnerOperation struct {
	store       *openStateStore
	key         openOwnerKey
	owner       *openOwnerState
	kind        openOwnerOperationKind
	seq         uint32
	stateID     StateID
	generation  uint32
	fileID      InodeID
	needConfirm bool
	abandoned   []openState
	finished    bool
}

type recoveredCloseOperation struct {
	store           *openStateStore
	stateID         StateID
	inputGeneration uint32
	fileID          InodeID
	seq             uint32
	lock            *recoveredCloseLock
	finished        bool
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
				epoch:         epoch,
				states:        make(map[StateID]*openState),
				owners:        make(map[openOwnerKey]*openOwnerState),
				replayOwners:  make(map[StateID]*openOwnerState),
				expired:       make(map[StateID]struct{}),
				recovered:     make(map[StateID]openOwnerResponse),
				recoveryLocks: make(map[StateID]*recoveredCloseLock),
				now:           time.Now,
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

// addStateLocked requires owner.mu and os.mu.
func (os *openStateStore) addStateLocked(
	owner *openOwnerState,
	state *openState,
) error {
	if existing := os.states[state.id]; existing != nil {
		return errors.New("NFS stateid index slot is already occupied")
	}
	if owner.states != nil {
		if existing := owner.states[state.fileID]; existing != nil {
			return errors.New(
				"NFS open-owner file index slot is already occupied")
		}
	} else {
		owner.states = make(map[InodeID]*openState)
	}
	os.states[state.id] = state
	owner.states[state.fileID] = state
	return nil
}

// removeStateLocked requires owner.mu and os.mu.
func (os *openStateStore) removeStateLocked(
	owner *openOwnerState,
	state *openState,
) {
	delete(os.states, state.id)
	if owner.states[state.fileID] == state {
		delete(owner.states, state.fileID)
	}
}

func openOwnerSeqidIsExempt(status uint32) bool {
	switch status {
	case NFS4ERR_BAD_SEQID,
		NFS4ERR_STALE_CLIENTID,
		NFS4ERR_STALE_STATEID,
		NFS4ERR_BAD_STATEID,
		NFS4ERR_BADXDR,
		NFS4ERR_RESOURCE,
		NFS4ERR_NOFILEHANDLE,
		NFS4ERR_MOVED:
		return true
	default:
		return false
	}
}

func nextStateidGeneration(generation uint32) uint32 {
	generation++
	if generation == 0 {
		return 1
	}
	return generation
}

func (os *openStateStore) markExpiredLocked(id StateID) {
	if _, exists := os.expired[id]; exists {
		return
	}
	os.expired[id] = struct{}{}
	os.expiredIDs = append(os.expiredIDs, id)
	if len(os.expiredIDs) > maxTombstones {
		oldest := os.expiredIDs[0]
		os.expiredIDs = os.expiredIDs[1:]
		delete(os.expired, oldest)
	}
}

func (os *openStateStore) acquireOwnerForOpen(
	key openOwnerKey,
) (*openOwnerState, uint32) {
	os.mu.Lock()
	owner := os.owners[key]
	if owner == nil {
		owner = &openOwnerState{}
		os.owners[key] = owner
	}
	os.mu.Unlock()

	owner.mu.Lock()
	os.mu.Lock()
	if os.owners[key] != owner {
		os.mu.Unlock()
		owner.mu.Unlock()
		return nil, NFS4ERR_STALE_CLIENTID
	}
	os.mu.Unlock()
	return owner, NFS4_OK
}

func (os *openStateStore) acquireOwnerForState(
	id StateID,
) (*openOwnerState, openOwnerKey, *openState, uint32) {
	os.mu.Lock()
	state := os.states[id]
	if state == nil {
		status := os.unknownStateStatusLocked(id)
		os.mu.Unlock()
		return nil, openOwnerKey{}, nil, status
	}
	key := state.owner
	owner := os.owners[key]
	if owner == nil {
		os.mu.Unlock()
		return nil, openOwnerKey{}, nil, NFS4ERR_EXPIRED
	}
	os.mu.Unlock()

	owner.mu.Lock()
	os.mu.Lock()
	state = os.states[id]
	if os.owners[key] != owner {
		os.mu.Unlock()
		owner.mu.Unlock()
		return nil, openOwnerKey{}, nil, NFS4ERR_EXPIRED
	}
	os.mu.Unlock()
	return owner, key, state, NFS4_OK
}

func (os *openStateStore) releaseOwner(owner *openOwnerState) {
	owner.mu.Unlock()
}

// abandonOwnerStates requires owner.mu.
func (os *openStateStore) abandonOwnerStates(
	owner *openOwnerState,
) []openState {
	os.mu.Lock()
	states := make([]*openState, 0, len(owner.states))
	for _, state := range owner.states {
		states = append(states, state)
	}
	abandoned := make([]openState, 0, len(states))
	for _, state := range states {
		abandoned = append(abandoned, *state)
		os.removeStateLocked(owner, state)
	}
	os.mu.Unlock()
	return abandoned
}

func (os *openStateStore) startOwnerOperation(
	key openOwnerKey,
	owner *openOwnerState,
	kind openOwnerOperationKind,
	seq uint32,
	stateID StateID,
	generation uint32,
	fileID InodeID,
) (*openOwnerOperation, openOwnerResponse, bool, uint32) {
	if owner.initialized && seq != owner.nextSeq {
		response := owner.lastResponse
		if response.matches(kind, seq, stateID, generation, fileID) {
			owner.lastUsed = os.now()
			os.releaseOwner(owner)
			return nil, response, true, response.status
		}
		if kind != openOwnerOperationOpen || owner.confirmed {
			os.releaseOwner(owner)
			return nil, openOwnerResponse{}, false, NFS4ERR_BAD_SEQID
		}
		owner.initialized = false
		owner.nextSeq = 0
		owner.lastResponse = openOwnerResponse{}
	}
	op := &openOwnerOperation{
		store:       os,
		key:         key,
		owner:       owner,
		kind:        kind,
		seq:         seq,
		stateID:     stateID,
		generation:  generation,
		fileID:      fileID,
		needConfirm: !owner.confirmed,
	}
	if kind == openOwnerOperationOpen && !owner.confirmed {
		op.abandoned = os.abandonOwnerStates(owner)
	}
	return op, openOwnerResponse{}, false, NFS4_OK
}

func (os *openStateStore) startOpen(
	key openOwnerKey,
	seq uint32,
) (*openOwnerOperation, openOwnerResponse, bool, uint32) {
	owner, status := os.acquireOwnerForOpen(key)
	if status != NFS4_OK {
		return nil, openOwnerResponse{}, false, status
	}
	return os.startOwnerOperation(
		key, owner, openOwnerOperationOpen, seq,
		StateID{}, 0, 0,
	)
}

func (os *openStateStore) startConfirm(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (*openOwnerOperation, openState, openOwnerResponse, bool, uint32) {
	owner, key, state, status := os.acquireOwnerForState(id)
	if status != NFS4_OK {
		return nil, openState{}, openOwnerResponse{}, false, status
	}
	op, response, replay, status := os.startOwnerOperation(
		key, owner, openOwnerOperationConfirm, seq,
		id, generation, fileID,
	)
	if op == nil {
		return nil, response.state, response, replay, status
	}
	if state == nil {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if state.fileID != fileID {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if generation < state.generation {
		op.finishError(NFS4ERR_OLD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_OLD_STATEID
	}
	if generation > state.generation || state.confirmed {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	return op, *state, openOwnerResponse{}, false, NFS4_OK
}

func (os *openStateStore) startClose(
	id StateID,
	generation uint32,
	fileID InodeID,
	seq uint32,
) (*openOwnerOperation, openState, openOwnerResponse, bool, uint32) {
	os.mu.Lock()
	if response, ok := os.recovered[id]; ok {
		os.mu.Unlock()
		if response.matches(
			openOwnerOperationClose, seq, id, generation, fileID,
		) {
			return nil, response.state, response, true, response.status
		}
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if os.states[id] == nil {
		if owner := os.replayOwners[id]; owner != nil {
			key := owner.lastResponse.state.owner
			os.mu.Unlock()
			owner.mu.Lock()
			os.mu.Lock()
			if os.owners[key] != owner || os.replayOwners[id] != owner {
				os.mu.Unlock()
				owner.mu.Unlock()
				return nil, openState{}, openOwnerResponse{}, false,
					NFS4ERR_BAD_STATEID
			}
			os.mu.Unlock()
			op, response, replay, status := os.startOwnerOperation(
				key, owner, openOwnerOperationClose, seq,
				id, generation, fileID,
			)
			if op != nil {
				op.finishError(NFS4ERR_BAD_STATEID)
				return nil, openState{}, openOwnerResponse{}, false,
					NFS4ERR_BAD_STATEID
			}
			return nil, response.state, response, replay, status
		}
	}
	os.mu.Unlock()

	owner, key, state, status := os.acquireOwnerForState(id)
	if status != NFS4_OK {
		return nil, openState{}, openOwnerResponse{}, false, status
	}
	op, response, replay, status := os.startOwnerOperation(
		key, owner, openOwnerOperationClose, seq,
		id, generation, fileID,
	)
	if op == nil {
		return nil, response.state, response, replay, status
	}
	if state == nil {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if state.fileID != fileID {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if generation < state.generation {
		op.finishError(NFS4ERR_OLD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_OLD_STATEID
	}
	if generation > state.generation || !state.confirmed {
		op.finishError(NFS4ERR_BAD_STATEID)
		return nil, openState{}, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	return op, *state, openOwnerResponse{}, false, NFS4_OK
}

func (op *openOwnerOperation) finishLocked(response openOwnerResponse) {
	owner := op.owner
	owner.lastUsed = op.store.now()
	if !openOwnerSeqidIsExempt(response.status) {
		if owner.lastResponse.kind == openOwnerOperationClose {
			delete(op.store.replayOwners, owner.lastResponse.inputStateID)
		}
		owner.initialized = true
		owner.nextSeq = op.seq + 1
		if owner.nextSeq == 0 {
			owner.nextSeq = 1
		}
		owner.lastResponse = response
		if response.kind == openOwnerOperationClose &&
			response.status == NFS4_OK {
			op.store.replayOwners[response.inputStateID] = owner
		}
	}
	op.finished = true
}

func (op *openOwnerOperation) finishError(status uint32) openOwnerResponse {
	response := openOwnerResponse{
		kind:            op.kind,
		seq:             op.seq,
		status:          status,
		inputStateID:    op.stateID,
		inputGeneration: op.generation,
		inputFileID:     op.fileID,
	}
	op.store.mu.Lock()
	op.finishLocked(response)
	op.store.mu.Unlock()
	op.store.releaseOwner(op.owner)
	return response
}

func (op *openOwnerOperation) finishOpen(
	fileID InodeID,
	write bool,
	id StateID,
	created bool,
) openOwnerResponse {
	os := op.store
	os.mu.Lock()
	if state := op.owner.states[fileID]; state != nil && state.confirmed {
		if write {
			os.mu.Unlock()
			return op.finishError(NFS4ERR_PERM)
		}
		// Reopen. RFC 7530 §9.11 requires the same "other" with an
		// incremented generation.
		state.generation = nextStateidGeneration(state.generation)
		now := uint64(time.Now().UnixNano())
		response := openOwnerResponse{
			kind:         openOwnerOperationOpen,
			seq:          op.seq,
			status:       NFS4_OK,
			state:        *state,
			changeBefore: now,
			changeAfter:  now,
		}
		op.finishLocked(response)
		os.mu.Unlock()
		os.releaseOwner(op.owner)
		return response
	}
	if id == (StateID{}) {
		id = os.newStateIDLocked()
	}
	state := &openState{
		id:         id,
		fileID:     fileID,
		owner:      op.key,
		write:      write,
		generation: 1,
		confirmed:  !op.needConfirm,
	}
	if err := os.addStateLocked(op.owner, state); err != nil {
		os.mu.Unlock()
		return op.finishError(NFS4ERR_SERVERFAULT)
	}
	now := uint64(time.Now().UnixNano())
	response := openOwnerResponse{
		kind:           openOwnerOperationOpen,
		seq:            op.seq,
		status:         NFS4_OK,
		state:          *state,
		changeBefore:   now,
		changeAfter:    now,
		requireConfirm: op.needConfirm,
	}
	if created && response.changeBefore != 0 {
		response.changeBefore--
	}
	op.finishLocked(response)
	os.mu.Unlock()
	os.releaseOwner(op.owner)
	return response
}

func (op *openOwnerOperation) finishConfirm(
	id StateID,
	generation uint32,
	fileID InodeID,
) openOwnerResponse {
	os := op.store
	os.mu.Lock()
	state := os.states[id]
	state.confirmed = true
	state.generation = nextStateidGeneration(state.generation)
	op.owner.confirmed = true
	response := openOwnerResponse{
		kind:            openOwnerOperationConfirm,
		seq:             op.seq,
		status:          NFS4_OK,
		state:           *state,
		inputStateID:    id,
		inputGeneration: generation,
		inputFileID:     fileID,
	}
	op.finishLocked(response)
	os.mu.Unlock()
	os.releaseOwner(op.owner)
	return response
}

func (op *openOwnerOperation) finishClose(
	id StateID,
	generation uint32,
	fileID InodeID,
) openOwnerResponse {
	os := op.store
	os.mu.Lock()
	state := os.states[id]
	state.generation = nextStateidGeneration(state.generation)
	closed := *state
	os.removeStateLocked(op.owner, state)
	response := openOwnerResponse{
		kind:            openOwnerOperationClose,
		seq:             op.seq,
		status:          NFS4_OK,
		state:           closed,
		inputStateID:    id,
		inputGeneration: generation,
		inputFileID:     fileID,
	}
	op.finishLocked(response)
	os.mu.Unlock()
	os.releaseOwner(op.owner)
	return response
}

func (op *openOwnerOperation) finishExpiredClose() openOwnerResponse {
	os := op.store
	os.mu.Lock()
	state := os.states[op.stateID]
	var expired openState
	if state != nil {
		expired = *state
		os.removeStateLocked(op.owner, state)
		os.markExpiredLocked(op.stateID)
	}
	response := openOwnerResponse{
		kind:            openOwnerOperationClose,
		seq:             op.seq,
		status:          NFS4ERR_EXPIRED,
		state:           expired,
		inputStateID:    op.stateID,
		inputGeneration: op.generation,
		inputFileID:     op.fileID,
	}
	op.finishLocked(response)
	os.mu.Unlock()
	os.releaseOwner(op.owner)
	return response
}

// existingOpen reports the confirmed open this owner already holds for
// fileID. The caller holds the owner operation, so the answer cannot change
// before finishOpen.
func (op *openOwnerOperation) existingOpen(fileID InodeID) (openState, bool) {
	op.store.mu.Lock()
	defer op.store.mu.Unlock()
	state := op.owner.states[fileID]
	if state == nil || !state.confirmed {
		return openState{}, false
	}
	return *state, true
}

func (op *openOwnerOperation) finishServerFaultIfNeeded() {
	if !op.finished {
		op.finishError(NFS4ERR_SERVERFAULT)
	}
}

func (os *openStateStore) lookup(
	id StateID,
	generation uint32,
	fileID InodeID,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state, status := os.lookupLocked(id, generation, fileID)
	if status == NFS4_OK && !state.confirmed {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	if status != NFS4_OK {
		return openState{}, status
	}
	return *state, NFS4_OK
}

func (os *openStateStore) unknownStateStatusLocked(id StateID) uint32 {
	if _, ok := os.expired[id]; ok {
		return NFS4ERR_EXPIRED
	}
	// Pynfs's makeStaleId helper writes epoch 1. Real epochs deliberately
	// exclude 0 and 1, so other unknown epochs remain BAD_STATEID.
	if binary.BigEndian.Uint32(id[0:4]) == 1 {
		return NFS4ERR_STALE_STATEID
	}
	return NFS4ERR_BAD_STATEID
}

func (os *openStateStore) lookupLocked(
	id StateID,
	generation uint32,
	fileID InodeID,
) (*openState, uint32) {
	state := os.states[id]
	if state == nil {
		return nil, os.unknownStateStatusLocked(id)
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

func (os *openStateStore) canRecover(id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	if _, ok := os.expired[id]; ok || os.states[id] != nil {
		return false
	}
	if _, ok := os.recovered[id]; ok {
		return false
	}
	return binary.BigEndian.Uint32(id[0:4]) != os.epoch
}

func (os *openStateStore) acquireRecoveryLock(
	id StateID,
) *recoveredCloseLock {
	os.mu.Lock()
	lock := os.recoveryLocks[id]
	if lock == nil {
		lock = &recoveredCloseLock{}
		os.recoveryLocks[id] = lock
	}
	lock.refs++
	os.mu.Unlock()
	lock.mu.Lock()
	return lock
}

func (os *openStateStore) releaseRecoveryLock(
	id StateID,
	lock *recoveredCloseLock,
) {
	lock.mu.Unlock()
	os.mu.Lock()
	lock.refs--
	if lock.refs == 0 && os.recoveryLocks[id] == lock {
		delete(os.recoveryLocks, id)
	}
	os.mu.Unlock()
}

func (os *openStateStore) startRecoveredClose(
	id StateID,
	fileID InodeID,
	generation uint32,
	seq uint32,
) (*recoveredCloseOperation, openOwnerResponse, bool, uint32) {
	lock := os.acquireRecoveryLock(id)
	os.mu.Lock()
	if response, ok := os.recovered[id]; ok {
		os.mu.Unlock()
		os.releaseRecoveryLock(id, lock)
		if response.matches(
			openOwnerOperationClose, seq, id, generation, fileID,
		) {
			return nil, response, true, response.status
		}
		return nil, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	if _, expired := os.expired[id]; expired || os.states[id] != nil ||
		binary.BigEndian.Uint32(id[0:4]) == os.epoch {
		os.mu.Unlock()
		os.releaseRecoveryLock(id, lock)
		return nil, openOwnerResponse{}, false, NFS4ERR_BAD_STATEID
	}
	os.mu.Unlock()
	return &recoveredCloseOperation{
		store:           os,
		stateID:         id,
		inputGeneration: generation,
		fileID:          fileID,
		seq:             seq,
		lock:            lock,
	}, openOwnerResponse{}, false, NFS4_OK
}

func (op *recoveredCloseOperation) finish(status uint32) openOwnerResponse {
	state := openState{
		id:         op.stateID,
		fileID:     op.fileID,
		generation: nextStateidGeneration(op.inputGeneration),
		confirmed:  true,
	}
	response := openOwnerResponse{
		kind:            openOwnerOperationClose,
		seq:             op.seq,
		status:          status,
		state:           state,
		inputStateID:    op.stateID,
		inputGeneration: op.inputGeneration,
		inputFileID:     op.fileID,
	}
	if !openOwnerSeqidIsExempt(status) {
		op.store.mu.Lock()
		if _, exists := op.store.recovered[op.stateID]; !exists {
			op.store.recoveredIDs = append(
				op.store.recoveredIDs, op.stateID)
		}
		op.store.recovered[op.stateID] = response
		if len(op.store.recoveredIDs) > maxRecoveredCloseResponses {
			oldest := op.store.recoveredIDs[0]
			op.store.recoveredIDs = op.store.recoveredIDs[1:]
			delete(op.store.recovered, oldest)
		}
		op.store.mu.Unlock()
	}
	op.finished = true
	op.store.releaseRecoveryLock(op.stateID, op.lock)
	return response
}

func (op *recoveredCloseOperation) finishServerFaultIfNeeded() {
	if !op.finished {
		op.finish(NFS4ERR_SERVERFAULT)
	}
}

// expireClient drops process-local state. Persistent client-store checks
// prevent a replaced clientid from opening new state.
func (os *openStateStore) expireClient(clientID uint64) []openState {
	if clientID == 0 {
		return nil
	}
	var expired []openState
	for {
		os.mu.Lock()
		var key openOwnerKey
		var owner *openOwnerState
		for candidateKey, candidate := range os.owners {
			if candidateKey.clientID == clientID {
				key = candidateKey
				owner = candidate
				break
			}
		}
		os.mu.Unlock()
		if owner == nil {
			break
		}

		owner.mu.Lock()
		os.mu.Lock()
		if os.owners[key] == owner {
			delete(os.owners, key)
			if owner.lastResponse.kind == openOwnerOperationClose {
				os.markExpiredLocked(owner.lastResponse.inputStateID)
				delete(os.replayOwners, owner.lastResponse.inputStateID)
			}
			states := make([]*openState, 0, len(owner.states))
			for _, state := range owner.states {
				states = append(states, state)
			}
			for _, state := range states {
				os.removeStateLocked(owner, state)
				os.markExpiredLocked(state.id)
				expired = append(expired, *state)
			}
		}
		os.mu.Unlock()
		owner.mu.Unlock()
	}
	return expired
}

// activeClientIDs includes owners retained for seqid replay after CLOSE.
func (os *openStateStore) activeClientIDs() []uint64 {
	os.mu.Lock()
	defer os.mu.Unlock()

	clients := make(map[uint64]struct{})
	for _, state := range os.states {
		clients[state.owner.clientID] = struct{}{}
	}
	for key := range os.owners {
		clients[key.clientID] = struct{}{}
	}
	result := make([]uint64, 0, len(clients))
	for clientID := range clients {
		result = append(result, clientID)
	}
	return result
}

func (os *openStateStore) clientStates(clientID uint64) []openState {
	os.mu.Lock()
	defer os.mu.Unlock()
	var result []openState
	for _, state := range os.states {
		if state.owner.clientID == clientID {
			result = append(result, *state)
		}
	}
	return result
}

// evictIdleOwners drops replay-only owners after one lease of inactivity.
// A retransmitted CLOSE after this retention period gets BAD_STATEID, as
// permitted by RFC 7530 section 16.18.5.
func (os *openStateStore) evictIdleOwners(cutoff time.Time) int {
	os.mu.Lock()
	keys := make([]openOwnerKey, 0, len(os.owners))
	for key := range os.owners {
		keys = append(keys, key)
	}
	os.mu.Unlock()

	evicted := 0
	for _, key := range keys {
		os.mu.Lock()
		owner := os.owners[key]
		os.mu.Unlock()
		if owner == nil {
			continue
		}

		owner.mu.Lock()
		os.mu.Lock()
		if os.owners[key] == owner &&
			len(owner.states) == 0 &&
			!owner.lastUsed.IsZero() &&
			!owner.lastUsed.After(cutoff) {
			delete(os.owners, key)
			if owner.lastResponse.kind == openOwnerOperationClose {
				delete(os.replayOwners, owner.lastResponse.inputStateID)
			}
			evicted++
		}
		os.mu.Unlock()
		owner.mu.Unlock()
	}
	return evicted
}
