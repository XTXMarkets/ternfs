// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// NFSv4.1 open state.
//
// Session slots provide exactly-once replies, so open owners have no
// sequence numbers and no replay cache. An open owner only groups the
// owner's stateids: one stateid per (owner, file), upgraded by repeated
// OPENs. OPEN, CLOSE and OPEN_DOWNGRADE of one file are serialized across
// all clients, so an OPEN after a publishing CLOSE sees what it published.
// Different files run in parallel. Lock order: pathname, then file.

const maxTombstones = 1024

type openOwnerKey struct {
	clientID uint64
	owner    string
}

// openState is one owner's open of one file. fileID is the filehandle the
// client holds: the published inode at the name when it opened. dataID is
// what its I/O uses: the writer's staging inode, or for a read open the
// published inode current at OPEN.
type openState struct {
	id     StateID
	fileID vfs.InodeID
	dataID vfs.InodeID
	owner  openOwnerKey
	// access is the READ/WRITE share access granted. modes holds bit v for
	// each distinct share_access value v the open was granted, and
	// denyModes bit d for each share_deny value d requested. Deny is kept
	// for OPEN_DOWNGRADE's validation only; it is not enforced (canonical
	// r3 §11).
	access     uint32
	modes      uint8
	denyModes  uint8
	generation uint32
	// seq orders opens of one file for choosing the most recent.
	seq uint64
}

// writes reports write access: the open has a writer.
func (s openState) writes() bool { return s.access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 }

// grant is one OPEN or OPEN_DOWNGRADE's share access and deny.
type grant struct {
	access, deny uint32
}

// downgradeStatus checks an OPEN_DOWNGRADE of state to g (RFC 8881 section
// 18.18.3). The target must be a subset of the current access and deny,
// and equal the union of the recorded values inside it; a recovered open
// has no recorded values and gets only the subset rule.
func (s openState) downgradeStatus(g grant, recovered bool) uint32 {
	union := func(set uint8, within uint32) uint32 {
		var u uint32
		for v := uint32(0); v <= 3; v++ {
			if set&(1<<v) != 0 && v&^within == 0 {
				u |= v
			}
		}
		return u
	}
	if g.access&^s.access != 0 || g.deny&^union(s.denyModes, 3) != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	if recovered {
		return nfs4.NFS4_OK
	}
	if union(s.modes, g.access) != g.access || union(s.denyModes, g.deny) != g.deny {
		return nfs4.NFS4ERR_INVAL
	}
	return nfs4.NFS4_OK
}

// narrowLocked applies an accepted downgrade to g: the recorded values
// outside it are dropped. The caller holds os.mu.
func (s *openState) narrowLocked(g grant) {
	s.access = g.access
	for v := uint32(0); v <= 3; v++ {
		if v&^g.access != 0 {
			s.modes &^= 1 << v
		}
		if v&^g.deny != 0 {
			s.denyModes &^= 1 << v
		}
	}
	s.modes |= 1 << g.access
	s.denyModes |= 1 << g.deny
}

type openFileKey struct {
	owner  openOwnerKey
	fileID vfs.InodeID
}

type openStateStore struct {
	mu sync.Mutex

	epoch   uint32
	nextID  uint64
	nextSeq uint64
	states  map[StateID]*openState
	byFile  map[openFileKey]*openState
	// expired maps an expired stateid's tombstone to its owner incarnation,
	// so only the owner sees EXPIRED or may free it (canonical r3 §10).
	expired map[StateID]uint64
	// expiredIDs bounds expired.
	expiredIDs []StateID
	fileLocks  keyedlock.Locker[vfs.InodeID]
	now        func() time.Time
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
				byFile:  make(map[openFileKey]*openState),
				expired: make(map[StateID]uint64),
				now:     time.Now,
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

func nextStateidGeneration(generation uint32) uint32 {
	generation++
	if generation == 0 {
		return 1
	}
	return generation
}

func (os *openStateStore) markExpiredLocked(id StateID, owner uint64) {
	if _, exists := os.expired[id]; exists {
		return
	}
	os.expired[id] = owner
	os.expiredIDs = append(os.expiredIDs, id)
	if len(os.expiredIDs) > maxTombstones {
		oldest := os.expiredIDs[0]
		os.expiredIDs = os.expiredIDs[1:]
		delete(os.expired, oldest)
	}
}

// lockFile serializes OPEN, CLOSE and OPEN_DOWNGRADE of one file, named by
// the filehandle clients hold. A caller which needs the file's pathname
// lock takes it first.
func (os *openStateStore) lockFile(fileID vfs.InodeID) func() {
	return os.fileLocks.Lock(fileID)
}

// existingOpen returns the owner's open of fileID. The caller holds
// lockFile for the pair.
func (os *openStateStore) existingOpen(owner openOwnerKey, fileID vfs.InodeID) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.byFile[openFileKey{owner: owner, fileID: fileID}]
	if state == nil {
		return openState{}, false
	}
	return *state, true
}

// open records a successful OPEN. A repeated OPEN by the same owner of the
// same file returns the existing stateid with an incremented generation
// (RFC 8881 section 8.2.2), its access ORed in and its share values
// recorded. An OPEN for write of an owner's read open upgrades it to
// dataID, the new writer's staging. The caller holds lockFile for the pair.
func (os *openStateStore) open(
	owner openOwnerKey,
	fileID vfs.InodeID,
	dataID vfs.InodeID,
	g grant,
	id StateID,
) (openState, error) {
	os.mu.Lock()
	defer os.mu.Unlock()
	key := openFileKey{owner: owner, fileID: fileID}
	os.nextSeq++
	if state := os.byFile[key]; state != nil {
		if g.access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 && !state.writes() {
			state.dataID = dataID
		}
		state.access |= g.access
		state.modes |= 1 << g.access
		state.denyModes |= 1 << g.deny
		state.generation = nextStateidGeneration(state.generation)
		state.seq = os.nextSeq
		return *state, nil
	}
	if id == (StateID{}) {
		id = os.newStateIDLocked()
	}
	if os.states[id] != nil {
		return openState{}, errors.New("NFS stateid index slot is already occupied")
	}
	state := &openState{
		id:         id,
		fileID:     fileID,
		dataID:     dataID,
		owner:      owner,
		access:     g.access,
		modes:      1 << g.access,
		denyModes:  1 << g.deny,
		generation: 1,
		seq:        os.nextSeq,
	}
	os.states[id] = state
	os.byFile[key] = state
	return *state, nil
}

// ownerOf returns the incarnation owning a live or expired stateid.
func (os *openStateStore) ownerOf(id StateID) (uint64, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	if state := os.states[id]; state != nil {
		return state.owner.clientID, true
	}
	owner, ok := os.expired[id]
	return owner, ok
}

// adoptable reports that adopt would succeed: no live open has the stateid,
// and the owner has no live open of fileID. The caller holds lockFile.
func (os *openStateStore) adoptable(owner openOwnerKey, fileID vfs.InodeID, id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.states[id] == nil && os.byFile[openFileKey{owner: owner, fileID: fileID}] == nil
}

// adopt installs a recovered open as a live one under its stateid, at
// generation, after OPEN_DOWNGRADE published its writer. The caller holds
// lockFile for the pair.
func (os *openStateStore) adopt(owner openOwnerKey, fileID, dataID vfs.InodeID, g grant, id StateID, generation uint32) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	key := openFileKey{owner: owner, fileID: fileID}
	if os.states[id] != nil || os.byFile[key] != nil {
		return openState{}, false
	}
	os.nextSeq++
	state := &openState{
		id: id, fileID: fileID, dataID: dataID, owner: owner,
		access: g.access, modes: 1 << g.access, denyModes: 1 << g.deny,
		generation: generation, seq: os.nextSeq,
	}
	os.states[id] = state
	os.byFile[key] = state
	return *state, true
}

// lookup validates a stateid that clientID presented with fileID. A zero
// seqid means the current generation (RFC 8881 section 8.2.2). Another
// client's stateid, live or expired, is BAD_STATEID (RFC 8881 section
// 8.2.2), with no other effect.
func (os *openStateStore) lookup(
	clientID uint64,
	id StateID,
	generation uint32,
	fileID vfs.InodeID,
) (openState, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state, status := os.lookupLocked(clientID, id, generation, fileID)
	if status != nfs4.NFS4_OK {
		return openState{}, status
	}
	return *state, nfs4.NFS4_OK
}

func (os *openStateStore) unknownStateStatusLocked(clientID uint64, id StateID) uint32 {
	if owner, ok := os.expired[id]; ok && owner == clientID {
		return nfs4.NFS4ERR_EXPIRED
	}
	return nfs4.NFS4ERR_BAD_STATEID
}

func (os *openStateStore) lookupLocked(
	clientID uint64,
	id StateID,
	generation uint32,
	fileID vfs.InodeID,
) (*openState, uint32) {
	state := os.states[id]
	if state == nil {
		return nil, os.unknownStateStatusLocked(clientID, id)
	}
	if state.owner.clientID != clientID || state.fileID != fileID {
		return nil, nfs4.NFS4ERR_BAD_STATEID
	}
	if generation != 0 && generation < state.generation {
		return nil, nfs4.NFS4ERR_OLD_STATEID
	}
	if generation > state.generation {
		return nil, nfs4.NFS4ERR_BAD_STATEID
	}
	return state, nfs4.NFS4_OK
}

// status answers TEST_STATEID for clientID without a filehandle.
func (os *openStateStore) status(clientID uint64, id StateID, generation uint32) uint32 {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return os.unknownStateStatusLocked(clientID, id)
	}
	if state.owner.clientID != clientID {
		return nfs4.NFS4ERR_BAD_STATEID
	}
	if generation != 0 && generation < state.generation {
		return nfs4.NFS4ERR_OLD_STATEID
	}
	if generation > state.generation {
		return nfs4.NFS4ERR_BAD_STATEID
	}
	return nfs4.NFS4_OK
}

// downgrade narrows an open to g with I/O on dataID: for a write open
// downgraded to READ, the version its writer published. It advances the
// generation. The caller holds lockFile for the pair and has checked g.
func (os *openStateStore) downgrade(id StateID, dataID vfs.InodeID, g grant) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return openState{}, false
	}
	state.narrowLocked(g)
	state.dataID = dataID
	state.generation = nextStateidGeneration(state.generation)
	return *state, true
}

// clientOpens returns clientID's opens of fileID, most recent first.
func (os *openStateStore) clientOpens(clientID uint64, fileID vfs.InodeID) []openState {
	os.mu.Lock()
	defer os.mu.Unlock()
	var result []openState
	for key, state := range os.byFile {
		if key.owner.clientID == clientID && key.fileID == fileID {
			result = append(result, *state)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].seq > result[j].seq })
	return result
}

// writerOf returns a write open by clientID of fileID, if any. Opens by one
// client of one handle share the client's page cache, so they share one
// staging writer.
func (os *openStateStore) writerOf(clientID uint64, fileID vfs.InodeID) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for key, state := range os.byFile {
		if key.owner.clientID == clientID && key.fileID == fileID && state.writes() {
			return *state, true
		}
	}
	return openState{}, false
}

// otherWriter returns an open other than id which writes to dataID.
func (os *openStateStore) otherWriter(id StateID, dataID vfs.InodeID) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for other, state := range os.states {
		if other != id && state.writes() && state.dataID == dataID {
			return *state, true
		}
	}
	return openState{}, false
}

// forget drops clientID's expired stateid's tombstone (FREE_STATEID). It
// reports false, changing nothing, for any other stateid.
func (os *openStateStore) forget(clientID uint64, id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	if owner, ok := os.expired[id]; !ok || owner != clientID {
		return false
	}
	delete(os.expired, id)
	return true
}

// close removes an open and returns its final state with the generation
// incremented. The caller holds lockFile for the pair.
func (os *openStateStore) close(id StateID) (openState, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return openState{}, false
	}
	delete(os.states, id)
	key := openFileKey{owner: state.owner, fileID: state.fileID}
	if os.byFile[key] == state {
		delete(os.byFile, key)
	}
	closed := *state
	closed.generation = nextStateidGeneration(closed.generation)
	return closed, true
}

// expire removes an open whose durable lease state has gone.
func (os *openStateStore) expire(id StateID) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return
	}
	delete(os.states, id)
	key := openFileKey{owner: state.owner, fileID: state.fileID}
	if os.byFile[key] == state {
		delete(os.byFile, key)
	}
	os.markExpiredLocked(id, state.owner.clientID)
}

// retarget points the opens whose I/O uses stagingID, a staging a clean or
// timestamp-only CLOSE is discarding, back to their own fileID (canonical
// r3 §11). The caller holds lockFile.
func (os *openStateStore) retarget(stagingID vfs.InodeID) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for _, state := range os.states {
		if state.dataID == stagingID {
			state.dataID = state.fileID
		}
	}
}

// readers reports opens other than id whose I/O uses dataID.
func (os *openStateStore) readers(dataID vfs.InodeID, id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	for other, state := range os.states {
		if other != id && state.dataID == dataID {
			return true
		}
	}
	return false
}

// live reports whether id names a local open.
func (os *openStateStore) live(id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.states[id] != nil
}

// canRecover reports whether id names a write open issued by an earlier
// nfsd process. Its staging sidecar may still hold the client's data.
func (os *openStateStore) canRecover(id StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	if _, ok := os.expired[id]; ok || os.states[id] != nil {
		return false
	}
	return binary.BigEndian.Uint32(id[0:4]) != os.epoch
}

// expireClient drops process-local state. Persistent client-store checks
// prevent a replaced clientid from opening new state.
func (os *openStateStore) expireClient(clientID uint64) []openState {
	if clientID == 0 {
		return nil
	}
	os.mu.Lock()
	defer os.mu.Unlock()
	var expired []openState
	for id, state := range os.states {
		if state.owner.clientID != clientID {
			continue
		}
		delete(os.states, id)
		key := openFileKey{owner: state.owner, fileID: state.fileID}
		if os.byFile[key] == state {
			delete(os.byFile, key)
		}
		os.markExpiredLocked(id, clientID)
		expired = append(expired, *state)
	}
	return expired
}

func (os *openStateStore) activeClientIDs() []uint64 {
	os.mu.Lock()
	defer os.mu.Unlock()

	clients := make(map[uint64]struct{})
	for _, state := range os.states {
		clients[state.owner.clientID] = struct{}{}
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
