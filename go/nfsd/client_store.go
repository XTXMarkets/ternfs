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
	"sync/atomic"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// ClientStore persists client identity and lease decisions for the nfsd fleet.
// A stable identity directory points at distinct incarnation directories. The
// inode ID of each incarnation directory is returned to the client as a
// clientid.
type ClientStore struct {
	fs             TernVFS
	nfsDirID       InodeID
	dirID          InodeID
	nfsdID         string
	leaseName      string
	confirmingName string
	now            func() time.Time
	// clock is what now reads; setClock swaps it while workers run.
	clock        atomic.Pointer[func() time.Time]
	localClients sync.Map

	identityLocksMu sync.Mutex
	identityLocks   map[InodeID]*clientIdentityLock
	leaseLocks      keyedLocker[InodeID]
	markerLocks     keyedLocker[clientOpenMarker]

	// Set by the server (canonical r3 §5.4): publishing an expired marker
	// for an incarnation and collecting an identity each hold a destruction
	// hold for the whole action, refused while a confirmation protects it.
	// A resumed replacement purge holds them on n and its selected target.
	// Nil in store-only use.
	beginMarker  func(InodeID) (func(), bool)
	beginCollect func(InodeID) (func(), bool)
	beginPurge   func(n, p InodeID) (func(), bool)

	// The durable lease refresher's records (lease_refresher.go), under
	// refreshMu, with their record and orphan credits.
	refreshMu      sync.Mutex
	refreshers     map[InodeID]*leaseRefresher
	refresherLimit int
	orphans        int
	orphanLimit    int
	// leaseStop ends the workers; leaseWorkers counts them.
	leaseStop    chan struct{}
	leaseStopped bool
	leaseWorkers sync.WaitGroup
	// Set by the server (canonical r3 §§8 and 9). leaseNeed reports the
	// logical lease's refresh need; leaseTerminal retires an incarnation's
	// sessions once its record is stale or fleet-expired, reporting false to
	// be retried. With them set, records have workers and only the server's
	// decideExpired publishes expired markers. Without them (store-only use)
	// lease work runs inline in the caller and lease observations mark, as
	// before.
	leaseNeed     func(uint64) bool
	leaseTerminal func(uint64, func()) bool
	// leaseSessions reports that the incarnation has sessions here, which
	// keeps its record (and its proven lease) between requests.
	leaseSessions func(uint64) bool
}

// setClock replaces the store's clock; it is safe while workers and
// connections read it.
func (cs *ClientStore) setClock(now func() time.Time) { cs.clock.Store(&now) }

// serverDecides reports that a server owns lease work and marker decisions.
func (cs *ClientStore) serverDecides() bool { return cs.leaseNeed != nil }

type clientIdentityLock struct {
	mu   sync.RWMutex
	refs int
}

type clientOpenMarker struct {
	clientID uint64
	stateID  StateID
}

type localClientState struct {
	identityID       InodeID
	confirmedPointer InodeID

	mu    sync.Mutex
	opens map[StateID]struct{}
}

type confirmedClientLocation struct {
	identityID InodeID
	symlinkID  InodeID
}

// rpcPrincipal identifies the caller of an RPC by its credential flavor and
// body. It is compared, not interpreted, so nfsd can tell whether the same
// caller is reusing a client identity.
type rpcPrincipal struct {
	flavor uint32
	body   string
}

// clientOwner is the principal recorded for a client registration.
type clientOwner struct {
	principal rpcPrincipal
}

// clientInUseError reports that a client identity with live state belongs to
// a different principal.
type clientInUseError struct{}

func (e clientInUseError) Error() string {
	return "NFS client identity is in use by another principal"
}

// durableClientRecord is the JSON body of the client and update files in an
// incarnation directory. It holds the opaque EXCHANGE_ID client owner, the
// client verifier, the confirmation verifier issued by nfsd and the RPC
// principal. The owner is stored for inspection only: the directory name is
// its hash and nfsd never reads it back.
type durableClientRecord struct {
	ID              []byte `json:"id,omitempty"`
	Verifier        []byte `json:"verifier"`
	Confirm         []byte `json:"confirm"`
	PrincipalFlavor uint32 `json:"principal_flavor"`
	PrincipalBody   []byte `json:"principal_body"`
}

// durableLease is the JSON body of a lease or confirming file. It records
// when the lease or claim written by one nfsd expires.
type durableLease struct {
	ExpiresUnixNano int64 `json:"expires_unix_nano"`
}

// durableGCCandidate is the JSON body of the gc file. It records the earliest
// time at which an unreachable incarnation may be removed.
type durableGCCandidate struct {
	CollectAfterUnixNano int64 `json:"collect_after_unix_nano"`
}

type slotScan struct {
	live         bool
	found        bool
	expiredNames []string
}

const (
	nfsDirName          = ".nfs"
	confirmedName       = "confirmed"
	pendingName         = "pending"
	clientRecordName    = "client"
	updateName          = "update"
	rebootName          = "reboot"
	gcCandidateName     = "gc"
	expiredName         = "expired"
	incarnationPrefix   = "i."
	activeOpenPrefix    = "o."
	leasePrefix         = "lease."
	confirmingPrefix    = "confirming."
	tempPrefix          = "t."
	maxClientRecordSize = 64 << 10
	nfsLeaseTime        = 90 * time.Second
	nfsLeaseRenewAfter  = nfsLeaseTime / 2
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
	return newClientStoreAt(fs, nfsID, clientsID)
}

// newClientStoreAt builds a ClientStore over existing directories with a fresh
// process identity.
func newClientStoreAt(
	fs TernVFS,
	nfsID InodeID,
	clientsID InodeID,
) (*ClientStore, error) {
	nfsdID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("client store: generate nfsd id: %w", err)
	}
	cs := &ClientStore{
		fs:             fs,
		nfsDirID:       nfsID,
		dirID:          clientsID,
		nfsdID:         nfsdID,
		leaseName:      leasePrefix + nfsdID,
		confirmingName: confirmingPrefix + nfsdID,
		identityLocks:  make(map[InodeID]*clientIdentityLock),
		refreshers:     make(map[InodeID]*leaseRefresher),
		refresherLimit: maxRefreshers,
		orphanLimit:    maxRecordOrphans * maxRefreshers,
		leaseStop:      make(chan struct{}),
	}
	cs.now = func() time.Time { return (*cs.clock.Load())() }
	cs.setClock(time.Now)
	return cs, nil
}

// maxRefreshers bounds lease refresher records (canonical r3 §9.4).
const maxRefreshers = maxSessionRecords

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

func (cs *ClientStore) lockIdentity(identityID InodeID) func() {
	return cs.lockIdentityMode(identityID, false)
}

func (cs *ClientStore) readLockIdentity(identityID InodeID) func() {
	return cs.lockIdentityMode(identityID, true)
}

func (cs *ClientStore) lockIdentityMode(identityID InodeID, shared bool) func() {
	cs.identityLocksMu.Lock()
	lock := cs.identityLocks[identityID]
	if lock == nil {
		lock = &clientIdentityLock{}
		cs.identityLocks[identityID] = lock
	}
	lock.refs++
	cs.identityLocksMu.Unlock()

	if shared {
		lock.mu.RLock()
	} else {
		lock.mu.Lock()
	}
	return func() {
		if shared {
			lock.mu.RUnlock()
		} else {
			lock.mu.Unlock()
		}

		cs.identityLocksMu.Lock()
		lock.refs--
		if lock.refs == 0 && cs.identityLocks[identityID] == lock {
			delete(cs.identityLocks, identityID)
		}
		cs.identityLocksMu.Unlock()
	}
}

// SetClientID creates an unconfirmed incarnation, except when a matching
// verifier denotes a callback update to the confirmed incarnation.
func (cs *ClientStore) SetClientID(
	verifier [8]byte,
	id []byte,
	owner clientOwner,
) (uint64, [8]byte, error) {
	identityID, err := cs.identityDir(id)
	if err != nil {
		return 0, [8]byte{}, err
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()

	confirmedID, confirmed, err := cs.readIdentitySymlink(
		identityID, confirmedName)
	if err != nil {
		return 0, [8]byte{}, err
	}
	if confirmed {
		record, found, err := cs.readRecord(confirmedID, clientRecordName)
		if err != nil {
			return 0, [8]byte{}, err
		}
		if !found {
			return 0, [8]byte{}, fmt.Errorf(
				"client store: confirmed client %d has no record", confirmedID)
		}
		samePrincipal := record.principal() == owner.principal
		if !samePrincipal {
			active, err := cs.hasActiveOpen(confirmedID)
			if err != nil {
				return 0, [8]byte{}, err
			}
			if active {
				return 0, [8]byte{}, clientInUseError{}
			}
		}
		leaseLive, leaseFound, err := cs.leaseStatus(confirmedID)
		if err != nil {
			return 0, [8]byte{}, err
		}
		expired := leaseFound && !leaseLive
		if samePrincipal && !expired &&
			bytes.Equal(record.Verifier, verifier[:]) {
			confirm, err := newClientConfirmVerifier()
			if err != nil {
				return 0, [8]byte{}, err
			}
			update := newDurableClientRecord(id, verifier, confirm, owner)
			if _, err := cs.replaceJSON(confirmedID, updateName, update); err != nil {
				return 0, [8]byte{}, err
			}
			// A callback update supersedes any earlier unconfirmed incarnation.
			if _, err := cs.replaceIdentitySymlink(
				identityID, pendingName, confirmedID,
			); err != nil {
				return 0, [8]byte{}, err
			}
			return uint64(confirmedID), confirm, nil
		}
	}

	return cs.createPending(identityID, verifier, id, owner)
}

// ExchangeID binds an NFSv4.1 EXCHANGE_ID to a durable incarnation. It
// follows SetClientID, except that a matching verifier and principal never
// update the confirmed record: they adopt it without any write, as the
// incarnation a restarted client already holds. Otherwise it creates a
// pending incarnation for CREATE_SESSION to confirm.
//
// Adoption needs no expired marker and either no lease or a live one, as
// SetClientID decides. Unlike SetClientID, the observation only reads: it
// publishes no marker and removes no lease slot, even when every lease is
// dead. A read error is an error, never evidence of absence.
func (cs *ClientStore) ExchangeID(
	verifier [8]byte,
	id []byte,
	owner clientOwner,
) (incarnation uint64, confirm [8]byte, adopted bool, err error) {
	identityID, err := cs.identityDir(id)
	if err != nil {
		return 0, [8]byte{}, false, err
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()

	confirmedID, confirmed, err := cs.readIdentitySymlink(
		identityID, confirmedName)
	if err != nil {
		return 0, [8]byte{}, false, err
	}
	if confirmed {
		record, found, err := cs.readRecord(confirmedID, clientRecordName)
		if err != nil {
			return 0, [8]byte{}, false, err
		}
		if !found {
			return 0, [8]byte{}, false, fmt.Errorf(
				"client store: confirmed client %d has no record", confirmedID)
		}
		samePrincipal := record.principal() == owner.principal
		if !samePrincipal {
			active, err := cs.hasActiveOpen(confirmedID)
			if err != nil {
				return 0, [8]byte{}, false, err
			}
			if active {
				return 0, [8]byte{}, false, clientInUseError{}
			}
		}
		marked, err := cs.hasExpiredMarker(confirmedID)
		if err != nil {
			return 0, [8]byte{}, false, err
		}
		leases, err := cs.scanSlots(confirmedID, leasePrefix)
		if err != nil {
			return 0, [8]byte{}, false, err
		}
		expired := marked || leases.found && !leases.live
		if samePrincipal && !expired &&
			bytes.Equal(record.Verifier, verifier[:]) {
			return uint64(confirmedID), [8]byte{}, true, nil
		}
	}
	incarnation, confirm, err = cs.createPending(identityID, verifier, id, owner)
	return incarnation, confirm, false, err
}

// createPending creates an unconfirmed incarnation and points the identity's
// pending symlink at it. The caller holds the identity lock.
func (cs *ClientStore) createPending(
	identityID InodeID,
	verifier [8]byte,
	id []byte,
	owner clientOwner,
) (uint64, [8]byte, error) {
	confirm, err := newClientConfirmVerifier()
	if err != nil {
		return 0, [8]byte{}, err
	}
	incarnationID, err := cs.newIncarnation(identityID)
	if err != nil {
		return 0, [8]byte{}, err
	}
	record := newDurableClientRecord(id, verifier, confirm, owner)
	if _, err := cs.createJSON(
		incarnationID, clientRecordName, record,
	); err != nil {
		return 0, [8]byte{}, err
	}
	if _, err := cs.replaceIdentitySymlink(
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
	principal rpcPrincipal,
) (uint64, error) {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return 0, err
	}
	if !valid {
		return 0, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()
	record, found, err := cs.readRecord(incarnationID, clientRecordName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}

	pendingID, pending, err := cs.readIdentitySymlink(
		identityID, pendingName)
	if err != nil {
		return 0, err
	}
	confirmedID, confirmed, err := cs.readIdentitySymlink(
		identityID, confirmedName)
	if err != nil {
		return 0, err
	}

	if pending && pendingID == incarnationID &&
		bytes.Equal(record.Confirm, confirm[:]) {
		if record.principal() != principal {
			return 0, nfsError(nfs4.NFS4ERR_CLID_INUSE)
		}
		// The claim keeps this incarnation alive while another nfsd may
		// replace pending.
		if err := cs.beginConfirmation(incarnationID); err != nil {
			return 0, err
		}
		pendingID, pending, err = cs.readIdentitySymlink(
			identityID, pendingName)
		if err != nil {
			return 0, err
		}
		if !pending || pendingID != incarnationID {
			_ = cs.removeIfExists(incarnationID, cs.confirmingName)
			return 0, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
		}
		if !confirmed || confirmedID != incarnationID {
			if confirmed {
				if _, err := cs.replaceRebootSymlink(
					incarnationID, confirmedID); err != nil {
					return 0, err
				}
			}
			if _, err := cs.replaceIdentitySymlink(
				identityID, confirmedName, incarnationID,
			); err != nil {
				return 0, err
			}
		}
		replacedID, err := cs.finishConfirmed(incarnationID)
		_ = cs.removeIfExists(incarnationID, cs.confirmingName)
		return replacedID, err
	}

	if !confirmed || confirmedID != incarnationID {
		return 0, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	if bytes.Equal(record.Confirm, confirm[:]) {
		if record.principal() != principal {
			return 0, nfsError(nfs4.NFS4ERR_CLID_INUSE)
		}
		return cs.finishConfirmed(incarnationID)
	}

	update, updateFound, err := cs.readRecord(incarnationID, updateName)
	if err != nil {
		return 0, err
	}
	if !updateFound || !bytes.Equal(update.Confirm, confirm[:]) {
		return 0, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	if update.principal() != principal {
		return 0, nfsError(nfs4.NFS4ERR_CLID_INUSE)
	}
	if _, err := cs.replaceJSON(
		incarnationID, clientRecordName, update,
	); err != nil {
		return 0, err
	}
	if err := cs.removeIfExists(incarnationID, updateName); err != nil {
		return 0, err
	}
	return 0, nil
}

func (cs *ClientStore) finishConfirmed(
	incarnationID InodeID,
) (uint64, error) {
	replacedID, err := cs.finishReboot(incarnationID)
	if replacedID != 0 {
		cs.localClients.Delete(InodeID(replacedID))
	}
	return replacedID, err
}

func (cs *ClientStore) beginConfirmation(incarnationID InodeID) error {
	_, err := cs.replaceJSON(
		incarnationID,
		cs.confirmingName,
		durableLease{
			ExpiresUnixNano: cs.now().Add(clientGCGrace).UnixNano(),
		},
	)
	return err
}

func (cs *ClientStore) finishReboot(incarnationID InodeID) (uint64, error) {
	oldID, found, err := cs.readRebootSymlink(incarnationID)
	if err != nil || !found {
		return 0, err
	}
	if err := cs.purgeState(oldID); err != nil {
		return 0, err
	}
	if err := cs.removeIfExists(incarnationID, rebootName); err != nil {
		return 0, err
	}
	return uint64(oldID), nil
}

func (cs *ClientStore) purgeState(incarnationID InodeID) error {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !isActiveOpenName(entry.Name) && !isLeaseName(entry.Name) {
			continue
		}
		if err := cs.removeIfExists(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ClientStore) removeIncarnation(
	identityID InodeID,
	incarnationID InodeID,
	name string,
) error {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name == gcCandidateName {
			continue
		}
		if err := cs.removeIfExists(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	if err := cs.removeIfExists(incarnationID, gcCandidateName); err != nil {
		return err
	}
	return cs.removeIfExists(identityID, name)
}

func (cs *ClientStore) incarnationName(
	identityID InodeID,
	incarnationID InodeID,
) (string, bool, error) {
	entries, err := cs.entries(identityID)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.ID == incarnationID && isIncarnationName(entry.Name) {
			return entry.Name, true, nil
		}
	}
	return "", false, nil
}

func (cs *ClientStore) requireIncarnationName(
	identityID InodeID,
	incarnationID InodeID,
) (string, error) {
	name, found, err := cs.incarnationName(identityID, incarnationID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf(
			"client store: incarnation %d is not below identity %d",
			incarnationID, identityID)
	}
	return name, nil
}

func (cs *ClientStore) collectStaleForClientResult(
	clientID InodeID,
) (bool, error) {
	identityID, err := cs.fs.LookupParent(clientID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if cs.beginCollect != nil {
		// Held through the identity lock wait and the whole collection.
		end, ok := cs.beginCollect(identityID)
		if !ok {
			// A confirmation may still leave N or P unrooted; check again later.
			return true, nil
		}
		defer end()
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()
	return cs.collectStaleResult(identityID)
}

func (cs *ClientStore) collectStaleResult(
	identityID InodeID,
) (bool, error) {
	roots, err := cs.clientRoots(identityID)
	if err != nil {
		return false, err
	}
	entries, err := cs.entries(identityID)
	if err != nil {
		return false, err
	}

	now := cs.now()
	removed := 0
	recheck := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, tempPrefix) {
			if err := cs.collectOldTemp(
				identityID, entry, now,
			); err != nil {
				return false, err
			}
			continue
		}
		if !strings.HasPrefix(entry.Name, incarnationPrefix) {
			continue
		}
		if err := cs.collectOldTemps(entry.ID, now); err != nil {
			return false, err
		}
		if _, rooted := roots[entry.ID]; rooted {
			if err := cs.removeIfExists(entry.ID, gcCandidateName); err != nil {
				return false, err
			}
			continue
		}
		confirming, err := cs.hasLiveConfirmation(entry.ID)
		if err != nil {
			return false, err
		}
		if confirming {
			recheck = true
			if err := cs.removeIfExists(entry.ID, gcCandidateName); err != nil {
				return false, err
			}
			continue
		}

		var candidate durableGCCandidate
		found, err := cs.readJSON(entry.ID, gcCandidateName, &candidate)
		if err != nil {
			return false, err
		}
		if !found {
			// Age the decision that this incarnation is unreachable, not the
			// incarnation itself.
			candidate.CollectAfterUnixNano = now.Add(clientGCGrace).UnixNano()
			if _, err := cs.createJSON(
				entry.ID, gcCandidateName, candidate,
			); err != nil {
				return false, err
			}
			recheck = true
			continue
		}
		if candidate.CollectAfterUnixNano <= 0 {
			return false, fmt.Errorf("client store: invalid GC candidate")
		}
		if candidate.CollectAfterUnixNano > now.UnixNano() {
			recheck = true
			continue
		}

		roots, err = cs.clientRoots(identityID)
		if err != nil {
			return false, err
		}
		if _, rooted := roots[entry.ID]; rooted {
			if err := cs.removeIfExists(entry.ID, gcCandidateName); err != nil {
				return false, err
			}
			continue
		}
		if err := cs.removeIncarnation(
			identityID, entry.ID, entry.Name,
		); err != nil {
			return false, err
		}
		removed++
		if removed == maxClientGCRemovals {
			return true, nil
		}
	}
	return recheck, nil
}

func (cs *ClientStore) collectOldTemps(
	dirID InodeID,
	now time.Time,
) error {
	entries, err := cs.entries(dirID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, tempPrefix) {
			continue
		}
		if err := cs.collectOldTemp(dirID, entry, now); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ClientStore) collectOldTemp(
	dirID InodeID,
	entry DirEntry,
	now time.Time,
) error {
	info, err := cs.fs.Stat(entry.ID)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mtime.After(now.Add(-nfsLeaseTime)) {
		return nil
	}
	return cs.removeIfExists(dirID, entry.Name)
}

func (cs *ClientStore) hasLiveConfirmation(
	incarnationID InodeID,
) (bool, error) {
	live, _, err := cs.liveSlots(incarnationID, confirmingPrefix)
	return live, err
}

func (cs *ClientStore) clientRoots(
	identityID InodeID,
) (map[InodeID]struct{}, error) {
	roots := make(map[InodeID]struct{})
	var primary []InodeID
	confirmedID, confirmed, err := cs.readIdentitySymlink(
		identityID, confirmedName)
	if err != nil {
		return nil, err
	}
	if confirmed {
		primary = append(primary, confirmedID)
		roots[confirmedID] = struct{}{}
	}
	pendingID, pending, err := cs.readIdentitySymlink(
		identityID, pendingName)
	if err != nil {
		return nil, err
	}
	if pending {
		primary = append(primary, pendingID)
		if !confirmed || pendingID != confirmedID {
			roots[pendingID] = struct{}{}
		}
	}
	// A reboot target remains live until confirmation has purged its state.
	for _, id := range primary {
		rebootID, found, err := cs.readRebootSymlink(id)
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
	incarnationID := InodeID(clientID)
	if value, ok := cs.localClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		identityID := state.identityID
		pointerID := state.confirmedPointer
		state.mu.Unlock()

		currentPointer, err := cs.fs.Lookup(identityID, confirmedName)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return currentPointer == pointerID, nil
	}
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil || !valid {
		return false, err
	}

	unlock := cs.lockIdentity(identityID)
	defer unlock()

	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	return confirmed, err
}

func (cs *ClientStore) IsLeaseExpired(clientID uint64) (bool, error) {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return false, err
	}
	if !valid {
		return true, nil
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil {
		return false, err
	}
	if !confirmed {
		return true, nil
	}
	live, found, err := cs.leaseStatus(incarnationID)
	if err != nil {
		return false, err
	}
	return found && !live, nil
}

func (cs *ClientStore) ExpireIfLeaseDead(
	clientID uint64,
) (bool, error) {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return false, err
	}
	if !valid {
		// The incarnation directory has been collected, so no lease or
		// marker can be live. Expire the local state, as IsLeaseExpired
		// already reports this case.
		cs.localClients.Delete(incarnationID)
		return true, nil
	}
	unlock := cs.lockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil {
		return false, err
	}
	if confirmed {
		live, found, err := cs.leaseStatus(incarnationID)
		if err != nil {
			return false, err
		}
		if live {
			return false, nil
		}
		// The first OPEN registers its process-local owner before it creates
		// this nfsd's lease slot. Do not let a sweep expire that operation in
		// the gap.
		if !found {
			return false, nil
		}
		cs.localClients.Delete(incarnationID)
		return true, nil
	}
	if err := cs.purgeState(incarnationID); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	cs.localClients.Delete(incarnationID)
	return true, nil
}

// identityForIncarnation validates that a clientid is an incarnation
// directory below /.nfs/clients, then returns its identity directory. This
// prevents wire-supplied directory inode IDs from escaping the client store.
func (cs *ClientStore) identityForIncarnation(
	incarnationID InodeID,
) (InodeID, bool, error) {
	if !isDirectoryInodeID(incarnationID) {
		return 0, false, nil
	}
	identityID, err := cs.fs.LookupParent(incarnationID)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	parentID, err := cs.fs.LookupParent(identityID)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if parentID != cs.dirID {
		return 0, false, nil
	}
	return identityID, true, nil
}

// confirmedLocation validates the clientid ancestry and checks that the
// identity's confirmed symlink still names this incarnation. The symlink inode
// is returned so callers can detect an atomic pointer replacement cheaply.
func (cs *ClientStore) confirmedLocationForIdentity(
	incarnationID InodeID,
	identityID InodeID,
) (confirmedClientLocation, bool, error) {
	confirmedID, pointerID, found, err := cs.readIdentitySymlinkWithFile(
		identityID, confirmedName)
	if err != nil || !found {
		return confirmedClientLocation{identityID: identityID}, false, err
	}
	return confirmedClientLocation{
		identityID: identityID,
		symlinkID:  pointerID,
	}, confirmedID == incarnationID, nil
}

// Renew runs one refresher cycle for a confirmed client in the caller and
// reports its lease: it publishes through the record like the worker. No
// serving path uses it; SEQUENCE renews from memory (canonical r3 §7.1).
func (cs *ClientStore) Renew(clientID uint64) error {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return err
	}
	if !valid {
		return nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	marked, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return err
	}
	if marked {
		return nfsError(nfs4.NFS4ERR_EXPIRED)
	}
	view, err := cs.refreshInline(clientID)
	if err != nil {
		return err
	}
	if err := cs.leaseStatusError(view); err != nil {
		return err
	}
	return nil
}

func (cs *ClientStore) MarkOpen(clientID uint64, stateID StateID) error {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return err
	}
	if !valid {
		return nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	unlockMarker := cs.markerLocks.lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()

	location, confirmed, err := cs.confirmedLocationForIdentity(incarnationID, identityID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	state, err := cs.renewLease(incarnationID, location, false)
	if err != nil {
		return err
	}
	// Independent markers share identity ownership, but neither the lease
	// lock nor the cache map lock is held across marker I/O.
	if err := cs.ensureMarker(incarnationID, activeOpenName(stateID)); err != nil {
		return err
	}
	state.mu.Lock()
	state.opens[stateID] = struct{}{}
	state.mu.Unlock()
	return nil
}

// renewLease checks a confirmed client's lease for MarkOpen and HasOpen. It
// writes nothing: every lease write belongs to the refresher (canonical r3
// §9.1). MarkOpen (requireLiveLease false) needs a live record whose proven
// lease is in the future, else DELAY while a verify runs. HasOpen with
// requireLiveLease reads the durable lease, which after a restart may be
// another process's, and gets EXPIRED when it is dead. The caller holds
// shared identity ownership.
func (cs *ClientStore) renewLease(
	incarnationID InodeID, location confirmedClientLocation,
	requireLiveLease bool,
) (*localClientState, error) {
	state := cs.localClientState(incarnationID, location.identityID, location.symlinkID)
	if requireLiveLease {
		live, _, err := cs.observeLeaseStatus(incarnationID)
		if err != nil {
			return nil, err
		}
		if !live {
			return nil, nfsError(nfs4.NFS4ERR_EXPIRED)
		}
	} else {
		var view leaseView
		if cs.serverDecides() {
			var ok bool
			if view, ok = cs.leaseView(uint64(incarnationID)); !ok {
				return nil, nfsError(nfs4.NFS4ERR_DELAY)
			}
		} else {
			marked, err := cs.hasExpiredMarker(incarnationID)
			if err != nil {
				return nil, err
			}
			if marked {
				return nil, nfsError(nfs4.NFS4ERR_EXPIRED)
			}
			if view, err = cs.refreshInline(uint64(incarnationID)); err != nil {
				return nil, err
			}
		}
		if err := cs.leaseStatusError(view); err != nil {
			switch nfsErrCode(err) {
			case nfs4.NFS4ERR_DELAY:
				cs.requestLeaseVerify(uint64(incarnationID))
			case nfs4.NFS4ERR_STALE_CLIENTID:
				cs.localClients.Delete(incarnationID)
			}
			return nil, err
		}
	}
	state.mu.Lock()
	state.confirmedPointer = location.symlinkID
	state.mu.Unlock()
	return state, nil
}

// leaseProven reports whether this process's refresher has published a
// lease for the client that is still in the future.
func (cs *ClientStore) leaseProven(incarnationID InodeID) bool {
	cs.refreshMu.Lock()
	r := cs.refreshers[incarnationID]
	cs.refreshMu.Unlock()
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == leaseRecordLive && r.proven > cs.now().UnixNano()
}

// MarkerGone reports that the open's durable marker is definitely absent:
// the open no longer exists, whatever the client's lease.
func (cs *ClientStore) MarkerGone(clientID uint64, stateID StateID) bool {
	_, err := cs.fs.Lookup(InodeID(clientID), activeOpenName(stateID))
	return errors.Is(err, os.ErrNotExist)
}

func (cs *ClientStore) RemoveOpen(clientID uint64, stateID StateID) error {
	incarnationID := InodeID(clientID)
	identityID, valid, lockErr := cs.identityForIncarnation(incarnationID)
	if lockErr != nil {
		return lockErr
	}
	if !valid {
		return nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	unlockMarker := cs.markerLocks.lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()
	err := cs.removeIfExists(incarnationID, activeOpenName(stateID))
	if value, ok := cs.localClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		delete(state.opens, stateID)
		state.mu.Unlock()
	}
	return err
}

// cachedClientIDs includes clients which only renew and have no open state.
func (cs *ClientStore) cachedClientIDs() []uint64 {
	var result []uint64
	cs.localClients.Range(func(key, _ any) bool {
		result = append(result, uint64(key.(InodeID)))
		return true
	})
	return result
}

func (cs *ClientStore) HasOpen(clientID uint64, stateID StateID) (bool, error) {
	incarnationID := InodeID(clientID)
	if value, ok := cs.localClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		_, open := state.opens[stateID]
		identityID := state.identityID
		pointerID := state.confirmedPointer
		state.mu.Unlock()

		if open {
			currentPointer, err := cs.fs.Lookup(identityID, confirmedName)
			if errors.Is(err, os.ErrNotExist) {
				cs.localClients.Delete(incarnationID)
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if currentPointer != pointerID {
				confirmedID, err := cs.resolveIdentitySymlink(
					identityID, currentPointer)
				if err != nil {
					return false, err
				}
				if confirmedID != incarnationID {
					cs.localClients.Delete(incarnationID)
					return false, nil
				}

				state.mu.Lock()
				state.confirmedPointer = currentPointer
				state.mu.Unlock()

			}

			// Our own published lease answers without reading the lease,
			// unless the client has been expired; otherwise the durable
			// path below reads it.
			if cs.leaseProven(incarnationID) {
				marked, err := cs.hasExpiredMarker(incarnationID)
				if err != nil {
					return false, err
				}
				if marked {
					cs.localClients.Delete(incarnationID)
					return false, nfsError(nfs4.NFS4ERR_EXPIRED)
				}
				return true, nil
			}
		}
	}

	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil || !valid {
		return false, err
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	// Keep the durable lookup and cache insertion ordered with operations
	// on this marker while allowing unrelated markers to proceed.
	unlockMarker := cs.markerLocks.lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()
	location, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil || !confirmed {
		return false, err
	}
	name := activeOpenName(stateID)
	if _, err := cs.fs.Lookup(incarnationID, name); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	state, err := cs.renewLease(incarnationID, location, true)
	if nfsErrCode(err) == nfs4.NFS4ERR_EXPIRED {
		cs.localClients.Delete(incarnationID)
		return false, nil
	}
	if err != nil {
		return false, err
	}

	state.mu.Lock()
	state.opens[stateID] = struct{}{}
	state.mu.Unlock()

	return true, nil
}

func (cs *ClientStore) localClientState(
	incarnationID InodeID,
	identityID InodeID,
	confirmedPointer InodeID,
) *localClientState {
	state := &localClientState{
		identityID:       identityID,
		confirmedPointer: confirmedPointer,
		opens:            make(map[StateID]struct{}),
	}
	value, _ := cs.localClients.LoadOrStore(incarnationID, state)
	return value.(*localClientState)
}

func (cs *ClientStore) hasActiveOpen(incarnationID InodeID) (bool, error) {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return false, err
	}
	now := cs.now().UnixNano()
	active := false
	live := false
	for _, entry := range entries {
		switch {
		case isActiveOpenName(entry.Name):
			active = true
		case isLeaseName(entry.Name):
			lease, err := cs.readLease(entry.ID)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, err
			}
			if lease.ExpiresUnixNano > now {
				live = true
			}
		}
	}
	return active && live, nil
}

// A lease is live while any nfsd process has renewed its own slot. Taking the
// maximum prevents a slow writer from shortening another process's lease.
func (cs *ClientStore) hasLiveLease(incarnationID InodeID) (bool, error) {
	live, _, err := cs.leaseStatus(incarnationID)
	return live, err
}

// observeLeaseStatus reports whether the client's durable lease is live
// (any process's slot) and whether any slot was found, without writing:
// leaseStatus(mark=false). An expired marker reads as dead.
func (cs *ClientStore) observeLeaseStatus(incarnationID InodeID) (bool, bool, error) {
	expired, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return false, false, err
	}
	scan, err := cs.scanSlots(incarnationID, leasePrefix)
	if err != nil {
		return false, false, err
	}
	if expired {
		return false, true, nil
	}
	return scan.live, scan.found, nil
}

// leaseObservation is decideExpired's read-only observation (canonical r3
// §8 step 2).
type leaseObservation struct {
	collected   bool // the incarnation directory is gone
	unconfirmed bool // the identity's confirmed pointer names another
	marked      bool // an expired marker exists
	live        bool // some process's lease is live
	found       bool // some lease slot exists
}

// observeLease reads an incarnation's lease state without writing.
func (cs *ClientStore) observeLease(clientID uint64) (leaseObservation, error) {
	incarnationID := InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return leaseObservation{}, err
	}
	if !valid {
		return leaseObservation{collected: true}, nil
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(incarnationID, identityID)
	if err != nil {
		return leaseObservation{}, err
	}
	if !confirmed {
		return leaseObservation{unconfirmed: true}, nil
	}
	marked, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return leaseObservation{}, err
	}
	scan, err := cs.scanSlots(incarnationID, leasePrefix)
	if err != nil {
		return leaseObservation{}, err
	}
	return leaseObservation{marked: marked, live: scan.live && !marked, found: scan.found}, nil
}

// publishExpiredMarker is decideExpired's only write: the marker, after
// arbitration (canonical r3 §8 step 3).
func (cs *ClientStore) publishExpiredMarker(clientID uint64) error {
	return cs.ensureMarker(InodeID(clientID), expiredName)
}

// forgetUnconfirmed ends an incarnation that is no longer confirmed: its
// markers and leases are purged, as ExpireIfLeaseDead does.
func (cs *ClientStore) forgetUnconfirmed(clientID uint64) error {
	incarnationID := InodeID(clientID)
	if err := cs.purgeState(incarnationID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cs.localClients.Delete(incarnationID)
	return nil
}

// leaseStatus observes the lease and, in store-only use, publishes the
// expired marker for a dead one and removes long-expired slots. With a
// server, only decideExpired publishes markers and the refresher removes
// slots, so this only observes.
func (cs *ClientStore) leaseStatus(
	incarnationID InodeID,
) (bool, bool, error) {
	if cs.serverDecides() {
		return cs.observeLeaseStatus(incarnationID)
	}
	expired, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return false, false, err
	}
	scan, err := cs.scanSlots(incarnationID, leasePrefix)
	if err != nil {
		return false, false, err
	}
	if !expired && !scan.live && scan.found {
		if cs.beginMarker != nil {
			end, ok := cs.beginMarker(incarnationID)
			if !ok {
				// Deferred until the confirmation settles; the caller retries.
				return false, true, nfsError(nfs4.NFS4ERR_DELAY)
			}
			defer end()
		}
		if err := cs.ensureMarker(incarnationID, expiredName); err != nil {
			return false, true, err
		}
		expired = true
	}
	if err := cs.removeExpiredSlots(
		incarnationID, scan.expiredNames,
	); err != nil {
		return false, expired || scan.found, err
	}
	if expired {
		return false, true, nil
	}
	return scan.live, scan.found, nil
}

func (cs *ClientStore) hasExpiredMarker(
	incarnationID InodeID,
) (bool, error) {
	_, err := cs.fs.Lookup(incarnationID, expiredName)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (cs *ClientStore) liveSlots(
	incarnationID InodeID,
	prefix string,
) (bool, bool, error) {
	scan, err := cs.scanSlots(incarnationID, prefix)
	if err != nil {
		return false, false, err
	}
	if err := cs.removeExpiredSlots(
		incarnationID, scan.expiredNames,
	); err != nil {
		return false, scan.found, err
	}
	return scan.live, scan.found, nil
}

func (cs *ClientStore) scanSlots(
	incarnationID InodeID,
	prefix string,
) (slotScan, error) {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return slotScan{}, err
	}
	now := cs.now().UnixNano()
	var scan slotScan
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, prefix) ||
			len(entry.Name) == len(prefix) {
			continue
		}
		lease, err := cs.readLease(entry.ID)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return slotScan{}, err
		}
		scan.found = true
		if lease.ExpiresUnixNano > now {
			scan.live = true
			continue
		}
		if lease.ExpiresUnixNano <= now-nfsLeaseTime.Nanoseconds() {
			scan.expiredNames = append(scan.expiredNames, entry.Name)
		}
	}
	return scan, nil
}

func (cs *ClientStore) removeExpiredSlots(
	incarnationID InodeID,
	names []string,
) error {
	for _, name := range names {
		if err := cs.removeIfExists(incarnationID, name); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ClientStore) isInternalFilehandle(id InodeID) (bool, error) {
	for id != cs.fs.RootID() {
		if id == cs.nfsDirID {
			return true, nil
		}
		parent, err := cs.fs.LookupParent(id)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if parent == id {
			return false, nil
		}
		id = parent
	}
	return false, nil
}

func (cs *ClientStore) identityDir(id []byte) (InodeID, error) {
	return ensureDir(cs.fs, cs.dirID, clientIdentityKey(id))
}

func (cs *ClientStore) newIncarnation(identityID InodeID) (InodeID, error) {
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

func (cs *ClientStore) entries(dirID InodeID) ([]DirEntry, error) {
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

func (cs *ClientStore) readRecord(
	incarnationID InodeID,
	name string,
) (durableClientRecord, bool, error) {
	var record durableClientRecord
	found, err := cs.readJSON(incarnationID, name, &record)
	if err != nil || !found {
		return durableClientRecord{}, found, err
	}
	if len(record.Verifier) != 8 || len(record.Confirm) != 8 {
		return durableClientRecord{}, false,
			fmt.Errorf("client store: invalid client record")
	}
	return record, true, nil
}

func (cs *ClientStore) readIdentitySymlink(
	identityID InodeID,
	name string,
) (InodeID, bool, error) {
	id, _, found, err := cs.readIdentitySymlinkWithFile(
		identityID, name)
	return id, found, err
}

func (cs *ClientStore) readIdentitySymlinkWithFile(
	identityID InodeID,
	name string,
) (InodeID, InodeID, bool, error) {
	return cs.readPointer(identityID, name, false)
}

func (cs *ClientStore) readPointer(
	dirID InodeID,
	name string,
	parentRelative bool,
) (InodeID, InodeID, bool, error) {
	fileID, err := cs.fs.Lookup(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	id, err := cs.resolvePointer(dirID, fileID, parentRelative)
	if errors.Is(err, os.ErrNotExist) {
		return 0, fileID, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return id, fileID, true, nil
}

func (cs *ClientStore) readRebootSymlink(
	incarnationID InodeID,
) (InodeID, bool, error) {
	id, _, found, err := cs.readPointer(
		incarnationID, rebootName, true)
	return id, found, err
}

func (cs *ClientStore) resolvePointer(
	dirID InodeID,
	fileID InodeID,
	parentRelative bool,
) (InodeID, error) {
	target, err := cs.fs.Readlink(fileID)
	if err != nil {
		return 0, err
	}
	if !parentRelative {
		return cs.resolveIncarnationName(dirID, target)
	}
	if !strings.HasPrefix(target, "../") {
		return 0, fmt.Errorf("client store: invalid reboot pointer")
	}
	identityID, err := cs.fs.LookupParent(dirID)
	if err != nil {
		return 0, err
	}
	return cs.resolveIncarnationName(
		identityID, strings.TrimPrefix(target, "../"))
}

func (cs *ClientStore) resolveIdentitySymlink(
	identityID InodeID,
	fileID InodeID,
) (InodeID, error) {
	return cs.resolvePointer(identityID, fileID, false)
}

func (cs *ClientStore) resolveIncarnationName(
	identityID InodeID,
	name string,
) (InodeID, error) {
	if !isIncarnationName(name) {
		return 0, fmt.Errorf("client store: invalid incarnation pointer")
	}
	id, err := cs.fs.Lookup(identityID, name)
	if err != nil {
		return 0, err
	}
	if !isDirectoryInodeID(id) {
		return 0, fmt.Errorf("client store: invalid client pointer")
	}
	return id, nil
}

func (cs *ClientStore) readLease(id InodeID) (durableLease, error) {
	var lease durableLease
	if err := cs.readJSONFile(id, "lease", &lease); err != nil {
		return durableLease{}, err
	}
	return lease, nil
}

func (cs *ClientStore) readJSON(
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
	if err := cs.readJSONFile(id, name, value); err != nil {
		return false, err
	}
	return true, nil
}

func (cs *ClientStore) readJSONFile(
	id InodeID,
	name string,
	value any,
) error {
	data, err := cs.fs.ReadAll(id)
	if err != nil {
		return err
	}
	if len(data) > maxClientRecordSize {
		return fmt.Errorf(
			"client store: record is too large: %d", len(data))
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("client store: decode %s: %w", name, err)
	}
	return nil
}

func (cs *ClientStore) replaceIdentitySymlink(
	identityID InodeID,
	name string,
	clientID InodeID,
) (InodeID, error) {
	targetName, err := cs.requireIncarnationName(identityID, clientID)
	if err != nil {
		return 0, err
	}
	return cs.replaceSymlink(identityID, name, targetName)
}

func (cs *ClientStore) replaceRebootSymlink(
	incarnationID InodeID,
	clientID InodeID,
) (InodeID, error) {
	identityID, err := cs.fs.LookupParent(incarnationID)
	if err != nil {
		return 0, err
	}
	targetName, err := cs.requireIncarnationName(identityID, clientID)
	if err != nil {
		return 0, err
	}
	return cs.replaceSymlink(
		incarnationID, rebootName, "../"+targetName)
}

func (cs *ClientStore) createJSON(
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

func (cs *ClientStore) replaceJSON(
	dirID InodeID,
	name string,
	value any,
) (InodeID, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return cs.replaceBytes(dirID, name, data)
}

func (cs *ClientStore) replaceBytes(
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

func (cs *ClientStore) replaceSymlink(
	dirID InodeID,
	name string,
	target string,
) (InodeID, error) {
	suffix, err := randomHex(8)
	if err != nil {
		return 0, err
	}
	tempName := tempPrefix + suffix
	id, err := cs.fs.Symlink(dirID, tempName, target)
	if err != nil {
		return 0, err
	}
	if err := cs.fs.Rename(dirID, tempName, dirID, name); err != nil {
		_ = cs.fs.Remove(dirID, tempName)
		return 0, err
	}
	return id, nil
}

func (cs *ClientStore) ensureMarker(dirID InodeID, name string) error {
	if _, err := cs.fs.Lookup(dirID, name); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := cs.fs.CreateFile(dirID, name, nil)
	return err
}

func (cs *ClientStore) removeIfExists(dirID InodeID, name string) error {
	err := cs.fs.Remove(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func clientStoreErrToNFS(err error) uint32 {
	var nfs nfsError
	if errors.As(err, &nfs) {
		return uint32(nfs)
	}
	return nfs4.NFS4ERR_DELAY
}

func newDurableClientRecord(
	id []byte,
	verifier [8]byte,
	confirm [8]byte,
	owner clientOwner,
) durableClientRecord {
	return durableClientRecord{
		ID:              append([]byte(nil), id...),
		Verifier:        append([]byte(nil), verifier[:]...),
		Confirm:         append([]byte(nil), confirm[:]...),
		PrincipalFlavor: owner.principal.flavor,
		PrincipalBody:   []byte(owner.principal.body),
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

func isIncarnationName(name string) bool {
	if !strings.HasPrefix(name, incarnationPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(name, incarnationPrefix)
	if len(encoded) != 32 {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
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

// stagingRecoveryKey identifies one client boot independently of the lease
// incarnation. Hash the principal too so an identity takeover cannot recover
// another principal's private bytes. The key survives incarnation GC.
func (cs *ClientStore) stagingRecoveryKey(clientID uint64) ([32]byte, error) {
	id := InodeID(clientID)
	identity, valid, err := cs.identityForIncarnation(id)
	if err != nil {
		return [32]byte{}, err
	}
	if !valid {
		return [32]byte{}, nfsError(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	record, found, err := cs.readRecord(id, clientRecordName)
	if err != nil {
		return [32]byte{}, err
	}
	if !found {
		return [32]byte{}, fmt.Errorf("missing client record for %d", clientID)
	}
	data, err := json.Marshal(struct {
		Identity  InodeID
		Verifier  []byte
		Flavor    uint32
		Principal []byte
	}{identity, record.Verifier, record.PrincipalFlavor, record.PrincipalBody})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// confirmedRecord answers an EXCHANGE_ID update (EXCHGID4_FLAG_UPD_CONFIRMED_REC_A)
// without changing the store. RFC 8881 section 18.35.5 cases 6-8 require a
// confirmed record with the same verifier and principal.
func (cs *ClientStore) confirmedRecord(
	id []byte,
	verifier [8]byte,
	principal rpcPrincipal,
) (uint64, uint32) {
	identityID, err := cs.fs.Lookup(cs.dirID, clientIdentityKey(id))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nfs4.NFS4ERR_NOENT
	}
	if err != nil {
		return 0, clientStoreErrToNFS(err)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	confirmedID, confirmed, err := cs.readIdentitySymlink(identityID, confirmedName)
	if err != nil {
		return 0, clientStoreErrToNFS(err)
	}
	if !confirmed {
		return 0, nfs4.NFS4ERR_NOENT
	}
	record, found, err := cs.readRecord(confirmedID, clientRecordName)
	if err != nil {
		return 0, clientStoreErrToNFS(err)
	}
	if !found {
		return 0, nfs4.NFS4ERR_NOENT
	}
	if !bytes.Equal(record.Verifier, verifier[:]) {
		return 0, nfs4.NFS4ERR_NOT_SAME
	}
	if record.principal() != principal {
		return 0, nfs4.NFS4ERR_PERM
	}
	return uint64(confirmedID), nfs4.NFS4_OK
}
