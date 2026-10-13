// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

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

type OwnerKey struct {
	ClientID uint64
	Owner    string
}

// State is one owner's open of one file. FileID is the filehandle the
// client holds: the published inode at the name when it opened. DataID is
// what its I/O uses: the writer's staging inode, or for a read open the
// published inode current at OPEN.
type State struct {
	ID     nfs4.StateID
	FileID vfs.InodeID
	DataID vfs.InodeID
	Owner  OwnerKey
	// Access is the READ/WRITE share access granted. modes holds bit v for
	// each distinct share_access value v the open was granted, and
	// denyModes bit d for each share_deny value d requested. Deny is kept
	// for OPEN_DOWNGRADE's validation only; it is not enforced (canonical
	// r3 §11).
	Access     uint32
	modes      uint8
	denyModes  uint8
	Generation uint32
	// seq orders opens of one file for choosing the most recent.
	seq uint64
}

// Writes reports write access: the open has a writer.
func (s State) Writes() bool { return s.Access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 }

// grant is one OPEN or OPEN_DOWNGRADE's share access and deny.
type Grant struct {
	Access, Deny uint32
}

// DowngradeStatus checks an OPEN_DOWNGRADE of state to g (RFC 8881 section
// 18.18.3). The target must be a subset of the current access and deny,
// and equal the union of the recorded values inside it; a recovered open
// has no recorded values and gets only the subset rule.
func (s State) DowngradeStatus(g Grant, recovered bool) uint32 {
	union := func(set uint8, within uint32) uint32 {
		var u uint32
		for v := uint32(0); v <= 3; v++ {
			if set&(1<<v) != 0 && v&^within == 0 {
				u |= v
			}
		}
		return u
	}
	if g.Access&^s.Access != 0 || g.Deny&^union(s.denyModes, 3) != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	if recovered {
		return nfs4.NFS4_OK
	}
	if union(s.modes, g.Access) != g.Access || union(s.denyModes, g.Deny) != g.Deny {
		return nfs4.NFS4ERR_INVAL
	}
	return nfs4.NFS4_OK
}

// narrowLocked applies an accepted downgrade to g: the recorded values
// outside it are dropped. The caller holds os.mu.
func (s *State) narrowLocked(g Grant) {
	s.Access = g.Access
	for v := uint32(0); v <= 3; v++ {
		if v&^g.Access != 0 {
			s.modes &^= 1 << v
		}
		if v&^g.Deny != 0 {
			s.denyModes &^= 1 << v
		}
	}
	s.modes |= 1 << g.Access
	s.denyModes |= 1 << g.Deny
}

type openFileKey struct {
	owner  OwnerKey
	fileID vfs.InodeID
}

type Store struct {
	mu sync.Mutex

	Epoch   uint32
	nextID  uint64
	nextSeq uint64
	states  map[nfs4.StateID]*State
	byFile  map[openFileKey]*State
	// expired maps an expired stateid's tombstone to its owner incarnation,
	// so only the owner sees EXPIRED or may free it (canonical r3 §10).
	expired map[nfs4.StateID]uint64
	// expiredIDs bounds expired.
	expiredIDs []nfs4.StateID
	fileLocks  keyedlock.Locker[vfs.InodeID]
	Now        func() time.Time
}

func NewStore() *Store {
	var epochBytes [4]byte
	for {
		if _, err := rand.Read(epochBytes[:]); err != nil {
			panic("generate NFS open-state epoch: " + err.Error())
		}
		epoch := binary.BigEndian.Uint32(epochBytes[:])
		if epoch > 1 {
			return &Store{
				Epoch:   epoch,
				states:  make(map[nfs4.StateID]*State),
				byFile:  make(map[openFileKey]*State),
				expired: make(map[nfs4.StateID]uint64),
				Now:     time.Now,
			}
		}
	}
}

func (os *Store) newStateIDLocked() nfs4.StateID {
	os.nextID++
	var id nfs4.StateID
	binary.BigEndian.PutUint32(id[0:4], os.Epoch)
	binary.BigEndian.PutUint64(id[4:12], os.nextID)
	return id
}

func (os *Store) NewStateID() nfs4.StateID {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.newStateIDLocked()
}

func NextGeneration(generation uint32) uint32 {
	generation++
	if generation == 0 {
		return 1
	}
	return generation
}

func (os *Store) markExpiredLocked(id nfs4.StateID, owner uint64) {
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
func (os *Store) LockFile(fileID vfs.InodeID) func() {
	return os.fileLocks.Lock(fileID)
}

// existingOpen returns the owner's open of fileID. The caller holds
// lockFile for the pair.
func (os *Store) ExistingOpen(owner OwnerKey, fileID vfs.InodeID) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.byFile[openFileKey{owner: owner, fileID: fileID}]
	if state == nil {
		return State{}, false
	}
	return *state, true
}

// open records a successful OPEN. A repeated OPEN by the same owner of the
// same file returns the existing stateid with an incremented generation
// (RFC 8881 section 8.2.2), its access ORed in and its share values
// recorded. An OPEN for write of an owner's read open upgrades it to
// dataID, the new writer's staging. The caller holds lockFile for the pair.
func (os *Store) Open(
	owner OwnerKey,
	fileID vfs.InodeID,
	dataID vfs.InodeID,
	g Grant,
	id nfs4.StateID,
) (State, error) {
	os.mu.Lock()
	defer os.mu.Unlock()
	key := openFileKey{owner: owner, fileID: fileID}
	os.nextSeq++
	if state := os.byFile[key]; state != nil {
		if g.Access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 && !state.Writes() {
			state.DataID = dataID
		}
		state.Access |= g.Access
		state.modes |= 1 << g.Access
		state.denyModes |= 1 << g.Deny
		state.Generation = NextGeneration(state.Generation)
		state.seq = os.nextSeq
		return *state, nil
	}
	if id == (nfs4.StateID{}) {
		id = os.newStateIDLocked()
	}
	if os.states[id] != nil {
		return State{}, errors.New("NFS stateid index slot is already occupied")
	}
	state := &State{
		ID:         id,
		FileID:     fileID,
		DataID:     dataID,
		Owner:      owner,
		Access:     g.Access,
		modes:      1 << g.Access,
		denyModes:  1 << g.Deny,
		Generation: 1,
		seq:        os.nextSeq,
	}
	os.states[id] = state
	os.byFile[key] = state
	return *state, nil
}

// ownerOf returns the incarnation owning a live or expired stateid.
func (os *Store) OwnerOf(id nfs4.StateID) (uint64, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	if state := os.states[id]; state != nil {
		return state.Owner.ClientID, true
	}
	owner, ok := os.expired[id]
	return owner, ok
}

// adoptable reports that adopt would succeed: no live open has the stateid,
// and the owner has no live open of fileID. The caller holds lockFile.
func (os *Store) Adoptable(owner OwnerKey, fileID vfs.InodeID, id nfs4.StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.states[id] == nil && os.byFile[openFileKey{owner: owner, fileID: fileID}] == nil
}

// adopt installs a recovered open as a live one under its stateid, at
// generation, after OPEN_DOWNGRADE published its writer. The caller holds
// lockFile for the pair.
func (os *Store) Adopt(owner OwnerKey, fileID, dataID vfs.InodeID, g Grant, id nfs4.StateID, generation uint32) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	key := openFileKey{owner: owner, fileID: fileID}
	if os.states[id] != nil || os.byFile[key] != nil {
		return State{}, false
	}
	os.nextSeq++
	state := &State{
		ID: id, FileID: fileID, DataID: dataID, Owner: owner,
		Access: g.Access, modes: 1 << g.Access, denyModes: 1 << g.Deny,
		Generation: generation, seq: os.nextSeq,
	}
	os.states[id] = state
	os.byFile[key] = state
	return *state, true
}

// lookup validates a stateid that clientID presented with fileID. A zero
// seqid means the current generation (RFC 8881 section 8.2.2). Another
// client's stateid, live or expired, is BAD_STATEID (RFC 8881 section
// 8.2.2), with no other effect.
func (os *Store) Lookup(
	clientID uint64,
	id nfs4.StateID,
	generation uint32,
	fileID vfs.InodeID,
) (State, uint32) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state, status := os.lookupLocked(clientID, id, generation, fileID)
	if status != nfs4.NFS4_OK {
		return State{}, status
	}
	return *state, nfs4.NFS4_OK
}

func (os *Store) unknownStateStatusLocked(clientID uint64, id nfs4.StateID) uint32 {
	if owner, ok := os.expired[id]; ok && owner == clientID {
		return nfs4.NFS4ERR_EXPIRED
	}
	return nfs4.NFS4ERR_BAD_STATEID
}

func (os *Store) lookupLocked(
	clientID uint64,
	id nfs4.StateID,
	generation uint32,
	fileID vfs.InodeID,
) (*State, uint32) {
	state := os.states[id]
	if state == nil {
		return nil, os.unknownStateStatusLocked(clientID, id)
	}
	if state.Owner.ClientID != clientID || state.FileID != fileID {
		return nil, nfs4.NFS4ERR_BAD_STATEID
	}
	if generation != 0 && generation < state.Generation {
		return nil, nfs4.NFS4ERR_OLD_STATEID
	}
	if generation > state.Generation {
		return nil, nfs4.NFS4ERR_BAD_STATEID
	}
	return state, nfs4.NFS4_OK
}

// status answers TEST_STATEID for clientID without a filehandle.
func (os *Store) Status(clientID uint64, id nfs4.StateID, generation uint32) uint32 {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return os.unknownStateStatusLocked(clientID, id)
	}
	if state.Owner.ClientID != clientID {
		return nfs4.NFS4ERR_BAD_STATEID
	}
	if generation != 0 && generation < state.Generation {
		return nfs4.NFS4ERR_OLD_STATEID
	}
	if generation > state.Generation {
		return nfs4.NFS4ERR_BAD_STATEID
	}
	return nfs4.NFS4_OK
}

// downgrade narrows an open to g with I/O on dataID: for a write open
// downgraded to READ, the version its writer published. It advances the
// generation. The caller holds lockFile for the pair and has checked g.
func (os *Store) Downgrade(id nfs4.StateID, dataID vfs.InodeID, g Grant) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return State{}, false
	}
	state.narrowLocked(g)
	state.DataID = dataID
	state.Generation = NextGeneration(state.Generation)
	return *state, true
}

// clientOpens returns clientID's opens of fileID, most recent first.
func (os *Store) ClientOpens(clientID uint64, fileID vfs.InodeID) []State {
	os.mu.Lock()
	defer os.mu.Unlock()
	var result []State
	for key, state := range os.byFile {
		if key.owner.ClientID == clientID && key.fileID == fileID {
			result = append(result, *state)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].seq > result[j].seq })
	return result
}

// writerOf returns a write open by clientID of fileID, if any. Opens by one
// client of one handle share the client's page cache, so they share one
// staging writer.
func (os *Store) WriterOf(clientID uint64, fileID vfs.InodeID) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for key, state := range os.byFile {
		if key.owner.ClientID == clientID && key.fileID == fileID && state.Writes() {
			return *state, true
		}
	}
	return State{}, false
}

// otherWriter returns an open other than id which writes to dataID.
func (os *Store) OtherWriter(id nfs4.StateID, dataID vfs.InodeID) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for other, state := range os.states {
		if other != id && state.Writes() && state.DataID == dataID {
			return *state, true
		}
	}
	return State{}, false
}

// forget drops clientID's expired stateid's tombstone (FREE_STATEID). It
// reports false, changing nothing, for any other stateid.
func (os *Store) Forget(clientID uint64, id nfs4.StateID) bool {
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
func (os *Store) Close(id nfs4.StateID) (State, bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return State{}, false
	}
	delete(os.states, id)
	key := openFileKey{owner: state.Owner, fileID: state.FileID}
	if os.byFile[key] == state {
		delete(os.byFile, key)
	}
	closed := *state
	closed.Generation = NextGeneration(closed.Generation)
	return closed, true
}

// expire removes an open whose durable lease state has gone.
func (os *Store) Expire(id nfs4.StateID) {
	os.mu.Lock()
	defer os.mu.Unlock()
	state := os.states[id]
	if state == nil {
		return
	}
	delete(os.states, id)
	key := openFileKey{owner: state.Owner, fileID: state.FileID}
	if os.byFile[key] == state {
		delete(os.byFile, key)
	}
	os.markExpiredLocked(id, state.Owner.ClientID)
}

// retarget points the opens whose I/O uses stagingID, a staging a clean or
// timestamp-only CLOSE is discarding, back to their own fileID (canonical
// r3 §11). The caller holds lockFile.
func (os *Store) Retarget(stagingID vfs.InodeID) {
	os.mu.Lock()
	defer os.mu.Unlock()
	for _, state := range os.states {
		if state.DataID == stagingID {
			state.DataID = state.FileID
		}
	}
}

// readers reports opens other than id whose I/O uses dataID.
func (os *Store) Readers(dataID vfs.InodeID, id nfs4.StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	for other, state := range os.states {
		if other != id && state.DataID == dataID {
			return true
		}
	}
	return false
}

// live reports whether id names a local open.
func (os *Store) Live(id nfs4.StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.states[id] != nil
}

// canRecover reports whether id names a write open issued by an earlier
// nfsd process. Its staging sidecar may still hold the client's data.
func (os *Store) CanRecover(id nfs4.StateID) bool {
	os.mu.Lock()
	defer os.mu.Unlock()
	if _, ok := os.expired[id]; ok || os.states[id] != nil {
		return false
	}
	return binary.BigEndian.Uint32(id[0:4]) != os.Epoch
}

// expireClient drops process-local state. Persistent client-store checks
// prevent a replaced clientid from opening new state.
func (os *Store) ExpireClient(clientID uint64) []State {
	if clientID == 0 {
		return nil
	}
	os.mu.Lock()
	defer os.mu.Unlock()
	var expired []State
	for id, state := range os.states {
		if state.Owner.ClientID != clientID {
			continue
		}
		delete(os.states, id)
		key := openFileKey{owner: state.Owner, fileID: state.FileID}
		if os.byFile[key] == state {
			delete(os.byFile, key)
		}
		os.markExpiredLocked(id, clientID)
		expired = append(expired, *state)
	}
	return expired
}

func (os *Store) ActiveClientIDs() []uint64 {
	os.mu.Lock()
	defer os.mu.Unlock()

	clients := make(map[uint64]struct{})
	for _, state := range os.states {
		clients[state.Owner.ClientID] = struct{}{}
	}
	result := make([]uint64, 0, len(clients))
	for clientID := range clients {
		result = append(result, clientID)
	}
	return result
}

func (os *Store) ClientStates(clientID uint64) []State {
	os.mu.Lock()
	defer os.mu.Unlock()
	var result []State
	for _, state := range os.states {
		if state.Owner.ClientID == clientID {
			result = append(result, *state)
		}
	}
	return result
}
