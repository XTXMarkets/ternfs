// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const nfsDirName = ".nfs"

// ClientStore keeps client identity and lease decisions consistent across the
// nfsd fleet. A stable identity directory points at distinct incarnation
// directories whose inode IDs are the clientids returned to clients.
type ClientStore struct {
	mu             sync.Mutex
	fs             TernVFS
	dirID          InodeID
	leaseName      string
	confirmingName string
	now            func() time.Time
}

type rpcPrincipal struct {
	flavor uint32
	body   string
}

type clientOwner struct {
	principal rpcPrincipal
	netid     string
	addr      string
}

type clientInUseError struct {
	owner clientOwner
}

func (e clientInUseError) Error() string {
	return "NFS client identity is in use by another principal"
}

type durableClientRecord struct {
	Verifier        []byte `json:"verifier"`
	Confirm         []byte `json:"confirm"`
	PrincipalFlavor uint32 `json:"principal_flavor"`
	PrincipalBody   []byte `json:"principal_body"`
	NetID           string `json:"netid"`
	Addr            string `json:"addr"`
}

type durableClientPointer struct {
	ClientID uint64 `json:"client_id"`
}

type durableLease struct {
	ExpiresUnixNano int64 `json:"expires_unix_nano"`
}

type durableGCCandidate struct {
	CollectAfterUnixNano int64 `json:"collect_after_unix_nano"`
}

const (
	confirmedName       = "confirmed"
	pendingName         = "pending"
	clientRecordName    = "client"
	updateName          = "update"
	rebootName          = "reboot"
	gcCandidateName     = "gc"
	incarnationPrefix   = "i."
	activeOpenPrefix    = "o."
	leasePrefix         = "lease."
	confirmingPrefix    = "confirming."
	tempPrefix          = "t."
	maxClientRecordSize = 64 << 10
	nfsLeaseTime        = 90 * time.Second
	clientGCGrace       = nfsLeaseTime
	maxClientGCRemovals = 8
)

func NewClientStore(fs TernVFS) (*ClientStore, error) {
	nfsID, err := ensureDir(fs, fs.RootID(), nfsDirName)
	if err != nil {
		return nil, fmt.Errorf("client store: create %s dir: %w", nfsDirName, err)
	}

	clientsID, err := ensureDir(fs, nfsID, "clients")
	if err != nil {
		return nil, fmt.Errorf("client store: create clients dir: %w", err)
	}

	nfsdID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("client store: generate nfsd id: %w", err)
	}
	return &ClientStore{
		fs:             fs,
		dirID:          clientsID,
		leaseName:      leasePrefix + nfsdID,
		confirmingName: confirmingPrefix + nfsdID,
		now:            time.Now,
	}, nil
}

func ensureDir(fs TernVFS, parentID InodeID, name string) (InodeID, error) {
	id, err := fs.Lookup(parentID, name)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	id, err = fs.Mkdir(parentID, name)
	if errors.Is(err, os.ErrExist) {
		return fs.Lookup(parentID, name)
	}
	return id, err
}

// SetClientID creates an unconfirmed incarnation, except when a matching
// verifier denotes a callback update to the confirmed incarnation.
func (cs *ClientStore) SetClientID(
	verifier [8]byte,
	id []byte,
	owner clientOwner,
) (uint64, [8]byte, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	identityID, err := cs.identityDirLocked(id)
	if err != nil {
		return 0, [8]byte{}, err
	}

	confirmedID, confirmed, err := cs.readPointerLocked(identityID, confirmedName)
	if err != nil {
		return 0, [8]byte{}, err
	}
	if confirmed {
		record, found, err := cs.readRecordLocked(confirmedID, clientRecordName)
		if err != nil {
			return 0, [8]byte{}, err
		}
		if !found {
			return 0, [8]byte{}, fmt.Errorf(
				"client store: confirmed client %d has no record", confirmedID)
		}
		if bytes.Equal(record.Verifier, verifier[:]) {
			if record.principal() != owner.principal {
				active, err := cs.hasActiveOpenLocked(confirmedID)
				if err != nil {
					return 0, [8]byte{}, err
				}
				if active {
					return 0, [8]byte{}, clientInUseError{owner: record.owner()}
				}
			}
			confirm, err := newClientConfirmVerifier()
			if err != nil {
				return 0, [8]byte{}, err
			}
			update := newDurableClientRecord(verifier, confirm, owner)
			if _, err := cs.replaceJSONLocked(confirmedID, updateName, update); err != nil {
				return 0, [8]byte{}, err
			}
			// A callback update supersedes any earlier unconfirmed incarnation.
			if _, err := cs.replacePointerLocked(
				identityID, pendingName, confirmedID,
			); err != nil {
				return 0, [8]byte{}, err
			}
			return uint64(confirmedID), confirm, nil
		}
	}

	confirm, err := newClientConfirmVerifier()
	if err != nil {
		return 0, [8]byte{}, err
	}
	incarnationID, err := cs.newIncarnationLocked(identityID)
	if err != nil {
		return 0, [8]byte{}, err
	}
	record := newDurableClientRecord(verifier, confirm, owner)
	if _, err := cs.createJSONLocked(
		incarnationID, clientRecordName, record,
	); err != nil {
		return 0, [8]byte{}, err
	}
	if _, err := cs.replacePointerLocked(
		identityID, pendingName, incarnationID,
	); err != nil {
		return 0, [8]byte{}, err
	}
	return uint64(incarnationID), confirm, nil
}

// ConfirmClientID returns the clientid replaced by a newly confirmed
// incarnation. Replays return zero once reboot cleanup has completed.
func (cs *ClientStore) ConfirmClientID(
	clientID uint64,
	confirm [8]byte,
) (uint64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	incarnationID := InodeID(clientID)
	if !isDirectoryInodeID(incarnationID) {
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}
	identityID, err := cs.fs.LookupParent(incarnationID)
	if err != nil {
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}
	record, found, err := cs.readRecordLocked(incarnationID, clientRecordName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}

	pendingID, pending, err := cs.readPointerLocked(identityID, pendingName)
	if err != nil {
		return 0, err
	}
	confirmedID, confirmed, err := cs.readPointerLocked(identityID, confirmedName)
	if err != nil {
		return 0, err
	}

	if pending && pendingID == incarnationID &&
		bytes.Equal(record.Confirm, confirm[:]) {
		// The claim keeps this incarnation alive while another nfsd may
		// replace pending.
		if err := cs.beginConfirmationLocked(incarnationID); err != nil {
			return 0, err
		}
		pendingID, pending, err = cs.readPointerLocked(identityID, pendingName)
		if err != nil {
			return 0, err
		}
		if !pending || pendingID != incarnationID {
			_ = cs.removeIfExistsLocked(incarnationID, cs.confirmingName)
			return 0, nfsError(NFS4ERR_STALE_CLIENTID)
		}
		if !confirmed || confirmedID != incarnationID {
			if confirmed {
				if _, err := cs.replacePointerLocked(
					incarnationID, rebootName, confirmedID,
				); err != nil {
					return 0, err
				}
			}
			if _, err := cs.replacePointerLocked(
				identityID, confirmedName, incarnationID,
			); err != nil {
				return 0, err
			}
		}
		replacedID, err := cs.finishRebootLocked(incarnationID)
		_ = cs.removeIfExistsLocked(incarnationID, cs.confirmingName)
		return replacedID, err
	}

	if !confirmed || confirmedID != incarnationID {
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}
	if bytes.Equal(record.Confirm, confirm[:]) {
		return cs.finishRebootLocked(incarnationID)
	}

	update, updateFound, err := cs.readRecordLocked(incarnationID, updateName)
	if err != nil {
		return 0, err
	}
	if !updateFound || !bytes.Equal(update.Confirm, confirm[:]) {
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}
	if _, err := cs.replaceJSONLocked(
		incarnationID, clientRecordName, update,
	); err != nil {
		return 0, err
	}
	if err := cs.removeIfExistsLocked(incarnationID, updateName); err != nil {
		return 0, err
	}
	return 0, nil
}

func (cs *ClientStore) beginConfirmationLocked(incarnationID InodeID) error {
	_, err := cs.replaceJSONLocked(
		incarnationID,
		cs.confirmingName,
		durableLease{
			ExpiresUnixNano: cs.now().Add(clientGCGrace).UnixNano(),
		},
	)
	return err
}

func (cs *ClientStore) finishRebootLocked(incarnationID InodeID) (uint64, error) {
	oldID, found, err := cs.readPointerLocked(incarnationID, rebootName)
	if err != nil || !found {
		return 0, err
	}
	if err := cs.purgeStateLocked(oldID); err != nil {
		return 0, err
	}
	if err := cs.removeIfExistsLocked(incarnationID, rebootName); err != nil {
		return 0, err
	}
	return uint64(oldID), nil
}

func (cs *ClientStore) purgeStateLocked(incarnationID InodeID) error {
	entries, err := cs.entriesLocked(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !isActiveOpenName(entry.Name) && !isLeaseName(entry.Name) {
			continue
		}
		if err := cs.removeIfExistsLocked(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ClientStore) removeIncarnationLocked(
	identityID InodeID,
	incarnationID InodeID,
) error {
	name, found, err := cs.incarnationNameLocked(identityID, incarnationID)
	if err != nil || !found {
		return err
	}
	entries, err := cs.entriesLocked(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name == gcCandidateName {
			continue
		}
		if err := cs.removeIfExistsLocked(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	if err := cs.removeIfExistsLocked(incarnationID, gcCandidateName); err != nil {
		return err
	}
	return cs.removeIfExistsLocked(identityID, name)
}

func (cs *ClientStore) incarnationNameLocked(
	identityID InodeID,
	incarnationID InodeID,
) (string, bool, error) {
	entries, err := cs.entriesLocked(identityID)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.ID == incarnationID && strings.HasPrefix(entry.Name, incarnationPrefix) {
			return entry.Name, true, nil
		}
	}
	return "", false, nil
}

func (cs *ClientStore) collectStaleForClient(clientID InodeID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	identityID, err := cs.fs.LookupParent(clientID)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return cs.collectStaleLocked(identityID)
}

func (cs *ClientStore) collectStaleLocked(identityID InodeID) error {
	roots, err := cs.clientRootsLocked(identityID)
	if err != nil {
		return err
	}
	// One page bounds the work added to each client registration.
	entries, _, err := cs.fs.Readdir(identityID, 0)
	if err != nil {
		return err
	}

	now := cs.now()
	removed := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, incarnationPrefix) {
			continue
		}
		if _, rooted := roots[entry.ID]; rooted {
			if err := cs.removeIfExistsLocked(entry.ID, gcCandidateName); err != nil {
				return err
			}
			continue
		}
		confirming, err := cs.hasLiveConfirmationLocked(entry.ID)
		if err != nil {
			return err
		}
		if confirming {
			if err := cs.removeIfExistsLocked(entry.ID, gcCandidateName); err != nil {
				return err
			}
			continue
		}

		var candidate durableGCCandidate
		found, err := cs.readJSONLocked(entry.ID, gcCandidateName, &candidate)
		if err != nil {
			return err
		}
		if !found {
			// Age the decision that this incarnation is unreachable, not the
			// incarnation itself.
			candidate.CollectAfterUnixNano = now.Add(clientGCGrace).UnixNano()
			if _, err := cs.createJSONLocked(
				entry.ID, gcCandidateName, candidate,
			); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			continue
		}
		if candidate.CollectAfterUnixNano <= 0 {
			return fmt.Errorf("client store: invalid GC candidate")
		}
		if candidate.CollectAfterUnixNano > now.UnixNano() {
			continue
		}

		roots, err = cs.clientRootsLocked(identityID)
		if err != nil {
			return err
		}
		if _, rooted := roots[entry.ID]; rooted {
			if err := cs.removeIfExistsLocked(entry.ID, gcCandidateName); err != nil {
				return err
			}
			continue
		}
		if err := cs.removeIncarnationLocked(identityID, entry.ID); err != nil {
			return err
		}
		removed++
		if removed == maxClientGCRemovals {
			return nil
		}
	}
	return nil
}

func (cs *ClientStore) hasLiveConfirmationLocked(
	incarnationID InodeID,
) (bool, error) {
	entries, err := cs.entriesLocked(incarnationID)
	if err != nil {
		return false, err
	}
	now := cs.now().UnixNano()
	for _, entry := range entries {
		if !isConfirmingName(entry.Name) {
			continue
		}
		claim, err := cs.readLeaseLocked(entry.ID)
		if err != nil {
			return false, err
		}
		if claim.ExpiresUnixNano > now {
			return true, nil
		}
		if err := cs.removeIfExistsLocked(incarnationID, entry.Name); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (cs *ClientStore) clientRootsLocked(
	identityID InodeID,
) (map[InodeID]struct{}, error) {
	roots := make(map[InodeID]struct{})
	var primary []InodeID
	for _, name := range []string{confirmedName, pendingName} {
		id, found, err := cs.readPointerLocked(identityID, name)
		if err != nil {
			return nil, err
		}
		if found {
			roots[id] = struct{}{}
			primary = append(primary, id)
		}
	}
	// A reboot target remains live until confirmation has purged its state.
	for _, id := range primary {
		rebootID, found, err := cs.readPointerLocked(id, rebootName)
		if err != nil {
			return nil, err
		}
		if found {
			roots[rebootID] = struct{}{}
		}
	}
	return roots, nil
}

func (cs *ClientStore) IsConfirmed(clientID uint64) (bool, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.isConfirmedLocked(InodeID(clientID))
}

func (cs *ClientStore) isConfirmedLocked(incarnationID InodeID) (bool, error) {
	if !isDirectoryInodeID(incarnationID) {
		return false, nil
	}
	identityID, err := cs.fs.LookupParent(incarnationID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	confirmedID, found, err := cs.readPointerLocked(identityID, confirmedName)
	return found && confirmedID == incarnationID, err
}

func (cs *ClientStore) Renew(clientID uint64) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	incarnationID := InodeID(clientID)
	confirmed, err := cs.isConfirmedLocked(incarnationID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfsError(NFS4ERR_STALE_CLIENTID)
	}
	return cs.renewLocked(incarnationID)
}

func (cs *ClientStore) renewLocked(incarnationID InodeID) error {
	lease := durableLease{
		ExpiresUnixNano: cs.now().Add(nfsLeaseTime).UnixNano(),
	}
	_, err := cs.replaceJSONLocked(incarnationID, cs.leaseName, lease)
	return err
}

func (cs *ClientStore) MarkOpen(clientID uint64, stateID StateID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	incarnationID := InodeID(clientID)
	confirmed, err := cs.isConfirmedLocked(incarnationID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfsError(NFS4ERR_STALE_CLIENTID)
	}
	if err := cs.ensureMarkerLocked(
		incarnationID, activeOpenName(stateID),
	); err != nil {
		return err
	}
	return cs.renewLocked(incarnationID)
}

func (cs *ClientStore) RemoveOpen(clientID uint64, stateID StateID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.removeIfExistsLocked(
		InodeID(clientID), activeOpenName(stateID))
}

func (cs *ClientStore) HasOpen(clientID uint64, stateID StateID) (bool, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	incarnationID := InodeID(clientID)
	confirmed, err := cs.isConfirmedLocked(incarnationID)
	if err != nil || !confirmed {
		return false, err
	}
	name := activeOpenName(stateID)
	if _, err := cs.fs.Lookup(incarnationID, name); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	live, err := cs.hasLiveLeaseLocked(incarnationID)
	if err != nil {
		return false, err
	}
	if !live {
		_ = cs.removeIfExistsLocked(incarnationID, name)
	}
	return live, nil
}

func (cs *ClientStore) hasActiveOpenLocked(incarnationID InodeID) (bool, error) {
	live, err := cs.hasLiveLeaseLocked(incarnationID)
	if err != nil {
		return false, err
	}
	entries, err := cs.entriesLocked(incarnationID)
	if err != nil {
		return false, err
	}
	active := false
	for _, entry := range entries {
		if !isActiveOpenName(entry.Name) {
			continue
		}
		if live {
			active = true
			continue
		}
		_ = cs.removeIfExistsLocked(incarnationID, entry.Name)
	}
	return active, nil
}

// A lease is live while any nfsd process has renewed its own slot. Taking the
// maximum prevents a slow writer from shortening another process's lease.
func (cs *ClientStore) hasLiveLeaseLocked(incarnationID InodeID) (bool, error) {
	entries, err := cs.entriesLocked(incarnationID)
	if err != nil {
		return false, err
	}
	now := cs.now().UnixNano()
	live := false
	for _, entry := range entries {
		if !isLeaseName(entry.Name) {
			continue
		}
		lease, err := cs.readLeaseLocked(entry.ID)
		if err != nil {
			return false, err
		}
		if lease.ExpiresUnixNano > now {
			live = true
			continue
		}
		_ = cs.removeIfExistsLocked(incarnationID, entry.Name)
	}
	return live, nil
}

func (cs *ClientStore) identityDirLocked(id []byte) (InodeID, error) {
	return ensureDir(cs.fs, cs.dirID, clientIdentityKey(id))
}

func (cs *ClientStore) newIncarnationLocked(identityID InodeID) (InodeID, error) {
	for {
		suffix, err := randomHex(16)
		if err != nil {
			return 0, err
		}
		id, err := cs.fs.Mkdir(identityID, incarnationPrefix+suffix)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return id, err
	}
}

func (cs *ClientStore) entriesLocked(dirID InodeID) ([]DirEntry, error) {
	var result []DirEntry
	var cursor uint64
	for {
		entries, next, err := cs.fs.Readdir(dirID, cursor)
		if err != nil {
			return nil, err
		}
		result = append(result, entries...)
		if next == 0 {
			return result, nil
		}
		if next == cursor {
			return nil, fmt.Errorf("client store: readdir cursor did not advance")
		}
		cursor = next
	}
}

func (cs *ClientStore) readRecordLocked(
	incarnationID InodeID,
	name string,
) (durableClientRecord, bool, error) {
	var record durableClientRecord
	found, err := cs.readJSONLocked(incarnationID, name, &record)
	if err != nil || !found {
		return durableClientRecord{}, found, err
	}
	if len(record.Verifier) != 8 || len(record.Confirm) != 8 {
		return durableClientRecord{}, false,
			fmt.Errorf("client store: invalid client record")
	}
	return record, true, nil
}

func (cs *ClientStore) readPointerLocked(
	dirID InodeID,
	name string,
) (InodeID, bool, error) {
	var pointer durableClientPointer
	found, err := cs.readJSONLocked(dirID, name, &pointer)
	if err != nil || !found {
		return 0, found, err
	}
	id := InodeID(pointer.ClientID)
	if !isDirectoryInodeID(id) {
		return 0, false, fmt.Errorf("client store: invalid client pointer")
	}
	return id, true, nil
}

func (cs *ClientStore) readLeaseLocked(id InodeID) (durableLease, error) {
	data, err := cs.readFileLocked(id)
	if err != nil {
		return durableLease{}, err
	}
	var lease durableLease
	if err := json.Unmarshal(data, &lease); err != nil {
		return durableLease{}, fmt.Errorf("client store: decode lease: %w", err)
	}
	return lease, nil
}

func (cs *ClientStore) readJSONLocked(
	dirID InodeID,
	name string,
	value any,
) (bool, error) {
	id, err := cs.fs.Lookup(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	data, err := cs.readFileLocked(id)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, fmt.Errorf("client store: decode %s: %w", name, err)
	}
	return true, nil
}

func (cs *ClientStore) readFileLocked(id InodeID) ([]byte, error) {
	info, err := cs.fs.Stat(id)
	if err != nil {
		return nil, err
	}
	if info.Size > maxClientRecordSize {
		return nil, fmt.Errorf("client store: record is too large: %d", info.Size)
	}
	data := make([]byte, info.Size)
	n, _, err := cs.fs.Read(id, 0, data)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, fmt.Errorf(
			"client store: short record read: got %d, want %d", n, len(data))
	}
	return data, nil
}

func (cs *ClientStore) replacePointerLocked(
	dirID InodeID,
	name string,
	clientID InodeID,
) (InodeID, error) {
	return cs.replaceJSONLocked(dirID, name, durableClientPointer{
		ClientID: uint64(clientID),
	})
}

func (cs *ClientStore) createJSONLocked(
	dirID InodeID,
	name string,
	value any,
) (InodeID, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return cs.fs.CreateFile(dirID, name, bytes.NewReader(data))
}

func (cs *ClientStore) replaceJSONLocked(
	dirID InodeID,
	name string,
	value any,
) (InodeID, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return cs.replaceBytesLocked(dirID, name, data)
}

func (cs *ClientStore) replaceBytesLocked(
	dirID InodeID,
	name string,
	data []byte,
) (InodeID, error) {
	suffix, err := randomHex(8)
	if err != nil {
		return 0, err
	}
	tempName := tempPrefix + suffix
	id, err := cs.fs.CreateFile(dirID, tempName, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	if err := cs.fs.Rename(dirID, tempName, dirID, name); err != nil {
		_ = cs.fs.Remove(dirID, tempName)
		return 0, err
	}
	return id, nil
}

func (cs *ClientStore) ensureMarkerLocked(dirID InodeID, name string) error {
	if _, err := cs.fs.Lookup(dirID, name); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := cs.fs.CreateFile(dirID, name, nil)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	return err
}

func (cs *ClientStore) removeIfExistsLocked(dirID InodeID, name string) error {
	err := cs.fs.Remove(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func newDurableClientRecord(
	verifier [8]byte,
	confirm [8]byte,
	owner clientOwner,
) durableClientRecord {
	return durableClientRecord{
		Verifier:        append([]byte(nil), verifier[:]...),
		Confirm:         append([]byte(nil), confirm[:]...),
		PrincipalFlavor: owner.principal.flavor,
		PrincipalBody:   []byte(owner.principal.body),
		NetID:           owner.netid,
		Addr:            owner.addr,
	}
}

func (r durableClientRecord) owner() clientOwner {
	return clientOwner{
		principal: r.principal(),
		netid:     r.NetID,
		addr:      r.Addr,
	}
}

func (r durableClientRecord) principal() rpcPrincipal {
	return rpcPrincipal{flavor: r.PrincipalFlavor, body: string(r.PrincipalBody)}
}

func newClientConfirmVerifier() ([8]byte, error) {
	for {
		var confirm [8]byte
		if _, err := rand.Read(confirm[:]); err != nil {
			return [8]byte{}, err
		}
		if confirm != [8]byte{} {
			return confirm, nil
		}
	}
}

func randomHex(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func clientIdentityKey(id []byte) string {
	sum := sha256.Sum256(id)
	return hex.EncodeToString(sum[:])
}

func isDirectoryInodeID(id InodeID) bool {
	return id != 0 && id.Type() == InodeTypeDir
}

func activeOpenName(stateID StateID) string {
	return activeOpenPrefix + hex.EncodeToString(stateID[:])
}

func isActiveOpenName(name string) bool {
	return strings.HasPrefix(name, activeOpenPrefix) &&
		len(name) > len(activeOpenPrefix)
}

func isLeaseName(name string) bool {
	return strings.HasPrefix(name, leasePrefix) &&
		len(name) > len(leasePrefix)
}

func isConfirmingName(name string) bool {
	return strings.HasPrefix(name, confirmingPrefix) &&
		len(name) > len(confirmingPrefix)
}
