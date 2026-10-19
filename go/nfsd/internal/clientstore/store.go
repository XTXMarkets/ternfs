// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

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

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Store persists client identity and lease decisions for the nfsd fleet.
// A stable identity directory points at distinct incarnation directories. The
// inode ID of each incarnation directory is returned to the client as a
// clientid.
type Store struct {
	fs             vfs.TernVFS
	NFSDirID       vfs.InodeID
	DirID          vfs.InodeID
	NfsdID         string
	LeaseName      string
	ConfirmingName string
	Now            func() time.Time
	// clock is read by Now; SetClock swaps it while workers run.
	clock        atomic.Pointer[func() time.Time]
	LocalClients sync.Map

	identityLocksMu sync.Mutex
	identityLocks   map[vfs.InodeID]*clientIdentityLock
	LeaseLocks      keyedlock.Locker[vfs.InodeID]
	markerLocks     keyedlock.Locker[clientOpenMarker]

	// Set by the server (canonical r3 §5.4): publishing an expired marker
	// for an incarnation and collecting an identity each hold a destruction
	// hold for the whole action, refused while a confirmation protects it.
	// A resumed replacement purge holds them on n and its selected target.
	// Nil in store-only use.
	BeginMarker  func(vfs.InodeID) (func(), bool)
	BeginCollect func(vfs.InodeID) (func(), bool)
	BeginPurge   func(n, p vfs.InodeID) (func(), bool)

	// The durable lease refresher's records (lease_refresher.go), under
	// RefreshMu, with their record and orphan credits.
	RefreshMu      sync.Mutex
	Refreshers     map[vfs.InodeID]*LeaseRefresher
	RefresherLimit int
	orphans        int
	orphanLimit    int
	// leaseStop ends the workers; leaseWorkers counts them.
	leaseStop    chan struct{}
	leaseStopped bool
	leaseWorkers sync.WaitGroup
	// Set by the server (canonical r3 §§8 and 9). LeaseNeed reports the
	// logical lease's refresh need; LeaseTerminal retires an incarnation's
	// sessions once its record is stale or fleet-expired, reporting false to
	// be retried. With them set, records have workers and only the server's
	// decideExpired publishes expired markers. Without them (store-only use)
	// lease work runs inline in the caller and lease observations mark, as
	// before.
	LeaseNeed     func(uint64) bool
	LeaseTerminal func(uint64, func()) bool
	// LeaseSessions reports that the incarnation has sessions here, which
	// keeps its record (and its proven lease) between requests.
	LeaseSessions func(uint64) bool
}

// SetClock replaces the store's clock; it is safe while workers and
// connections read it.
func (cs *Store) SetClock(now func() time.Time) { cs.clock.Store(&now) }

// serverDecides reports that a server owns lease work and marker decisions.
func (cs *Store) serverDecides() bool { return cs.LeaseNeed != nil }

type clientIdentityLock struct {
	mu   sync.RWMutex
	refs int
}

type clientOpenMarker struct {
	clientID uint64
	stateID  nfs4.StateID
}

type localClientState struct {
	identityID       vfs.InodeID
	confirmedPointer vfs.InodeID

	mu    sync.Mutex
	opens map[nfs4.StateID]struct{}
}

type confirmedClientLocation struct {
	identityID vfs.InodeID
	symlinkID  vfs.InodeID
}

// ClientInUseError reports that a client identity with live state belongs to
// a different principal.
type ClientInUseError struct{}

func (e ClientInUseError) Error() string {
	return "NFS client identity is in use by another principal"
}

type slotScan struct {
	live         bool
	found        bool
	expiredNames []string
}

const (
	ConfirmedName       = "confirmed"
	pendingName         = "pending"
	ClientRecordName    = "client"
	updateName          = "update"
	RebootName          = "reboot"
	GCCandidateName     = "gc"
	ExpiredName         = "expired"
	IncarnationPrefix   = "i."
	activeOpenPrefix    = "o."
	LeasePrefix         = "lease."
	confirmingPrefix    = "confirming."
	TempPrefix          = "t."
	maxClientRecordSize = 64 << 10
	LeaseTime           = 90 * time.Second
	LeaseRenewAfter     = LeaseTime / 2
	ClientGCGrace       = LeaseTime
	MaxClientGCRemovals = 8
)

func NewStore(fs vfs.TernVFS) (*Store, error) {
	nfsID, err := EnsureDir(fs, fs.RootID(), vfs.NFSDirName)
	if err != nil {
		return nil, fmt.Errorf("client store: create %s dir: %w", vfs.NFSDirName, err)
	}

	clientsID, err := EnsureDir(fs, nfsID, "clients")
	if err != nil {
		return nil, fmt.Errorf("client store: create clients dir: %w", err)
	}
	return NewStoreAt(fs, nfsID, clientsID)
}

// NewStoreAt builds a Store over existing directories with a fresh
// process identity.
func NewStoreAt(
	fs vfs.TernVFS,
	nfsID vfs.InodeID,
	clientsID vfs.InodeID,
) (*Store, error) {
	nfsdID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("client store: generate nfsd id: %w", err)
	}
	cs := &Store{
		fs:             fs,
		NFSDirID:       nfsID,
		DirID:          clientsID,
		NfsdID:         nfsdID,
		LeaseName:      LeasePrefix + nfsdID,
		ConfirmingName: confirmingPrefix + nfsdID,
		identityLocks:  make(map[vfs.InodeID]*clientIdentityLock),
		Refreshers:     make(map[vfs.InodeID]*LeaseRefresher),
		RefresherLimit: maxRefreshers,
		orphanLimit:    maxRecordOrphans * maxRefreshers,
		leaseStop:      make(chan struct{}),
	}
	cs.Now = func() time.Time { return (*cs.clock.Load())() }
	cs.SetClock(time.Now)
	return cs, nil
}

// MaxSessionRecords bounds the session store's wire clientids and sessions.
const MaxSessionRecords = 4096

// maxRefreshers bounds lease refresher records (canonical r3 §9.4).
const maxRefreshers = MaxSessionRecords

func EnsureDir(fs vfs.TernVFS, parentID vfs.InodeID, name string) (vfs.InodeID, error) {
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

func (cs *Store) LockIdentity(identityID vfs.InodeID) func() {
	return cs.lockIdentityMode(identityID, false)
}

func (cs *Store) readLockIdentity(identityID vfs.InodeID) func() {
	return cs.lockIdentityMode(identityID, true)
}

func (cs *Store) lockIdentityMode(identityID vfs.InodeID, shared bool) func() {
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
func (cs *Store) SetClientID(
	verifier [8]byte,
	id []byte,
	owner ClientOwner,
) (uint64, [8]byte, error) {
	identityID, err := cs.identityDir(id)
	if err != nil {
		return 0, [8]byte{}, err
	}
	unlock := cs.LockIdentity(identityID)
	defer unlock()

	confirmedID, confirmed, err := cs.ReadIdentitySymlink(
		identityID, ConfirmedName)
	if err != nil {
		return 0, [8]byte{}, err
	}
	if confirmed {
		record, found, err := cs.readRecord(confirmedID, ClientRecordName)
		if err != nil {
			return 0, [8]byte{}, err
		}
		if !found {
			return 0, [8]byte{}, fmt.Errorf(
				"client store: confirmed client %d has no record", confirmedID)
		}
		samePrincipal := record.principal() == owner.Principal
		if !samePrincipal {
			active, err := cs.hasActiveOpen(confirmedID)
			if err != nil {
				return 0, [8]byte{}, err
			}
			if active {
				return 0, [8]byte{}, ClientInUseError{}
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
func (cs *Store) ExchangeID(
	verifier [8]byte,
	id []byte,
	owner ClientOwner,
) (incarnation uint64, confirm [8]byte, adopted bool, err error) {
	identityID, err := cs.identityDir(id)
	if err != nil {
		return 0, [8]byte{}, false, err
	}
	unlock := cs.LockIdentity(identityID)
	defer unlock()

	confirmedID, confirmed, err := cs.ReadIdentitySymlink(
		identityID, ConfirmedName)
	if err != nil {
		return 0, [8]byte{}, false, err
	}
	if confirmed {
		record, found, err := cs.readRecord(confirmedID, ClientRecordName)
		if err != nil {
			return 0, [8]byte{}, false, err
		}
		if !found {
			return 0, [8]byte{}, false, fmt.Errorf(
				"client store: confirmed client %d has no record", confirmedID)
		}
		samePrincipal := record.principal() == owner.Principal
		if !samePrincipal {
			active, err := cs.hasActiveOpen(confirmedID)
			if err != nil {
				return 0, [8]byte{}, false, err
			}
			if active {
				return 0, [8]byte{}, false, ClientInUseError{}
			}
		}
		marked, err := cs.hasExpiredMarker(confirmedID)
		if err != nil {
			return 0, [8]byte{}, false, err
		}
		leases, err := cs.scanSlots(confirmedID, LeasePrefix)
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
func (cs *Store) createPending(
	identityID vfs.InodeID,
	verifier [8]byte,
	id []byte,
	owner ClientOwner,
) (uint64, [8]byte, error) {
	confirm, err := newClientConfirmVerifier()
	if err != nil {
		return 0, [8]byte{}, err
	}
	incarnationID, err := cs.NewIncarnation(identityID)
	if err != nil {
		return 0, [8]byte{}, err
	}
	record := newDurableClientRecord(id, verifier, confirm, owner)
	if _, err := cs.CreateJSON(
		incarnationID, ClientRecordName, record,
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
func (cs *Store) ConfirmClientID(
	clientID uint64,
	confirm [8]byte,
	principal RPCPrincipal,
) (uint64, error) {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return 0, err
	}
	if !valid {
		return 0, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.LockIdentity(identityID)
	defer unlock()
	record, found, err := cs.readRecord(incarnationID, ClientRecordName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}

	pendingID, pending, err := cs.ReadIdentitySymlink(
		identityID, pendingName)
	if err != nil {
		return 0, err
	}
	confirmedID, confirmed, err := cs.ReadIdentitySymlink(
		identityID, ConfirmedName)
	if err != nil {
		return 0, err
	}

	if pending && pendingID == incarnationID &&
		bytes.Equal(record.Confirm, confirm[:]) {
		if record.principal() != principal {
			return 0, nfs4.Error(nfs4.NFS4ERR_CLID_INUSE)
		}
		// The claim keeps this incarnation alive while another nfsd may
		// replace pending.
		if err := cs.beginConfirmation(incarnationID); err != nil {
			return 0, err
		}
		pendingID, pending, err = cs.ReadIdentitySymlink(
			identityID, pendingName)
		if err != nil {
			return 0, err
		}
		if !pending || pendingID != incarnationID {
			_ = cs.removeIfExists(incarnationID, cs.ConfirmingName)
			return 0, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
		}
		if !confirmed || confirmedID != incarnationID {
			if confirmed {
				if _, err := cs.replaceRebootSymlink(
					incarnationID, confirmedID); err != nil {
					return 0, err
				}
			}
			if _, err := cs.replaceIdentitySymlink(
				identityID, ConfirmedName, incarnationID,
			); err != nil {
				return 0, err
			}
		}
		replacedID, err := cs.finishConfirmed(incarnationID)
		_ = cs.removeIfExists(incarnationID, cs.ConfirmingName)
		return replacedID, err
	}

	if !confirmed || confirmedID != incarnationID {
		return 0, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	if bytes.Equal(record.Confirm, confirm[:]) {
		if record.principal() != principal {
			return 0, nfs4.Error(nfs4.NFS4ERR_CLID_INUSE)
		}
		return cs.finishConfirmed(incarnationID)
	}

	update, updateFound, err := cs.readRecord(incarnationID, updateName)
	if err != nil {
		return 0, err
	}
	if !updateFound || !bytes.Equal(update.Confirm, confirm[:]) {
		return 0, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	if update.principal() != principal {
		return 0, nfs4.Error(nfs4.NFS4ERR_CLID_INUSE)
	}
	if _, err := cs.replaceJSON(
		incarnationID, ClientRecordName, update,
	); err != nil {
		return 0, err
	}
	if err := cs.removeIfExists(incarnationID, updateName); err != nil {
		return 0, err
	}
	return 0, nil
}

func (cs *Store) finishConfirmed(
	incarnationID vfs.InodeID,
) (uint64, error) {
	replacedID, err := cs.finishReboot(incarnationID)
	if replacedID != 0 {
		cs.LocalClients.Delete(vfs.InodeID(replacedID))
	}
	return replacedID, err
}

func (cs *Store) beginConfirmation(incarnationID vfs.InodeID) error {
	_, err := cs.replaceJSON(
		incarnationID,
		cs.ConfirmingName,
		DurableLease{
			ExpiresUnixNano: cs.Now().Add(ClientGCGrace).UnixNano(),
		},
	)
	return err
}

func (cs *Store) finishReboot(incarnationID vfs.InodeID) (uint64, error) {
	oldID, found, err := cs.ReadRebootSymlink(incarnationID)
	if err != nil || !found {
		return 0, err
	}
	if err := cs.purgeState(oldID); err != nil {
		return 0, err
	}
	if err := cs.removeIfExists(incarnationID, RebootName); err != nil {
		return 0, err
	}
	return uint64(oldID), nil
}

func (cs *Store) purgeState(incarnationID vfs.InodeID) error {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !IsActiveOpenName(entry.Name) && !isLeaseName(entry.Name) {
			continue
		}
		if err := cs.removeIfExists(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func (cs *Store) RemoveIncarnation(
	identityID vfs.InodeID,
	incarnationID vfs.InodeID,
	name string,
) error {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name == GCCandidateName {
			continue
		}
		if err := cs.removeIfExists(incarnationID, entry.Name); err != nil {
			return err
		}
	}
	if err := cs.removeIfExists(incarnationID, GCCandidateName); err != nil {
		return err
	}
	return cs.removeIfExists(identityID, name)
}

func (cs *Store) IncarnationName(
	identityID vfs.InodeID,
	incarnationID vfs.InodeID,
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

func (cs *Store) requireIncarnationName(
	identityID vfs.InodeID,
	incarnationID vfs.InodeID,
) (string, error) {
	name, found, err := cs.IncarnationName(identityID, incarnationID)
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

func (cs *Store) CollectStaleForClientResult(
	clientID vfs.InodeID,
) (bool, error) {
	identityID, err := cs.fs.LookupParent(clientID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if cs.BeginCollect != nil {
		// Held through the identity lock wait and the whole collection.
		end, ok := cs.BeginCollect(identityID)
		if !ok {
			// A confirmation may still leave N or P unrooted; check again later.
			return true, nil
		}
		defer end()
	}
	unlock := cs.LockIdentity(identityID)
	defer unlock()
	return cs.collectStaleResult(identityID)
}

func (cs *Store) collectStaleResult(
	identityID vfs.InodeID,
) (bool, error) {
	roots, err := cs.clientRoots(identityID)
	if err != nil {
		return false, err
	}
	entries, err := cs.entries(identityID)
	if err != nil {
		return false, err
	}

	now := cs.Now()
	removed := 0
	recheck := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, TempPrefix) {
			if err := cs.collectOldTemp(
				identityID, entry, now,
			); err != nil {
				return false, err
			}
			continue
		}
		if !strings.HasPrefix(entry.Name, IncarnationPrefix) {
			continue
		}
		if err := cs.collectOldTemps(entry.ID, now); err != nil {
			return false, err
		}
		if _, rooted := roots[entry.ID]; rooted {
			if err := cs.removeIfExists(entry.ID, GCCandidateName); err != nil {
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
			if err := cs.removeIfExists(entry.ID, GCCandidateName); err != nil {
				return false, err
			}
			continue
		}

		var candidate durableGCCandidate
		found, err := cs.readJSON(entry.ID, GCCandidateName, &candidate)
		if err != nil {
			return false, err
		}
		if !found {
			// Age the decision that this incarnation is unreachable, not the
			// incarnation itself.
			candidate.CollectAfterUnixNano = now.Add(ClientGCGrace).UnixNano()
			if _, err := cs.CreateJSON(
				entry.ID, GCCandidateName, candidate,
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
			if err := cs.removeIfExists(entry.ID, GCCandidateName); err != nil {
				return false, err
			}
			continue
		}
		if err := cs.RemoveIncarnation(
			identityID, entry.ID, entry.Name,
		); err != nil {
			return false, err
		}
		removed++
		if removed == MaxClientGCRemovals {
			return true, nil
		}
	}
	return recheck, nil
}

func (cs *Store) collectOldTemps(
	dirID vfs.InodeID,
	now time.Time,
) error {
	entries, err := cs.entries(dirID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, TempPrefix) {
			continue
		}
		if err := cs.collectOldTemp(dirID, entry, now); err != nil {
			return err
		}
	}
	return nil
}

func (cs *Store) collectOldTemp(
	dirID vfs.InodeID,
	entry vfs.DirEntry,
	now time.Time,
) error {
	info, err := cs.fs.Stat(entry.ID)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mtime.After(now.Add(-LeaseTime)) {
		return nil
	}
	return cs.removeIfExists(dirID, entry.Name)
}

func (cs *Store) hasLiveConfirmation(
	incarnationID vfs.InodeID,
) (bool, error) {
	live, _, err := cs.liveSlots(incarnationID, confirmingPrefix)
	return live, err
}

func (cs *Store) clientRoots(
	identityID vfs.InodeID,
) (map[vfs.InodeID]struct{}, error) {
	roots := make(map[vfs.InodeID]struct{})
	var primary []vfs.InodeID
	confirmedID, confirmed, err := cs.ReadIdentitySymlink(
		identityID, ConfirmedName)
	if err != nil {
		return nil, err
	}
	if confirmed {
		primary = append(primary, confirmedID)
		roots[confirmedID] = struct{}{}
	}
	pendingID, pending, err := cs.ReadIdentitySymlink(
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
		rebootID, found, err := cs.ReadRebootSymlink(id)
		if err != nil {
			return nil, err
		}
		if found {
			roots[rebootID] = struct{}{}
		}
	}
	return roots, nil
}

func (cs *Store) IsConfirmed(clientID uint64) (bool, error) {
	incarnationID := vfs.InodeID(clientID)
	if value, ok := cs.LocalClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		identityID := state.identityID
		pointerID := state.confirmedPointer
		state.mu.Unlock()

		currentPointer, err := cs.fs.Lookup(identityID, ConfirmedName)
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

	unlock := cs.LockIdentity(identityID)
	defer unlock()

	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	return confirmed, err
}

func (cs *Store) IsLeaseExpired(clientID uint64) (bool, error) {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return false, err
	}
	if !valid {
		return true, nil
	}
	unlock := cs.LockIdentity(identityID)
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

func (cs *Store) ExpireIfLeaseDead(
	clientID uint64,
) (bool, error) {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return false, err
	}
	if !valid {
		// The incarnation directory has been collected, so no lease or
		// marker can be live. Expire the local state, as IsLeaseExpired
		// already reports this case.
		cs.LocalClients.Delete(incarnationID)
		return true, nil
	}
	unlock := cs.LockIdentity(identityID)
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
		cs.LocalClients.Delete(incarnationID)
		return true, nil
	}
	if err := cs.purgeState(incarnationID); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	cs.LocalClients.Delete(incarnationID)
	return true, nil
}

// identityForIncarnation validates that a clientid is an incarnation
// directory below /.nfs/clients, then returns its identity directory. This
// prevents wire-supplied directory inode IDs from escaping the client store.
func (cs *Store) identityForIncarnation(
	incarnationID vfs.InodeID,
) (vfs.InodeID, bool, error) {
	if !IsDirectoryInodeID(incarnationID) {
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
	if parentID != cs.DirID {
		return 0, false, nil
	}
	return identityID, true, nil
}

// confirmedLocationForIdentity checks whether identityID's confirmed symlink
// names incarnationID. It returns the symlink inode so callers can detect a
// pointer replacement.
func (cs *Store) confirmedLocationForIdentity(
	incarnationID vfs.InodeID,
	identityID vfs.InodeID,
) (confirmedClientLocation, bool, error) {
	confirmedID, pointerID, found, err := cs.readIdentitySymlinkWithFile(
		identityID, ConfirmedName)
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
func (cs *Store) Renew(clientID uint64) error {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return err
	}
	if !valid {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	marked, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return err
	}
	if marked {
		return nfs4.Error(nfs4.NFS4ERR_EXPIRED)
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

// MarkOpen records an open with a durable marker in the incarnation
// directory, so that other nfsd processes, and this one after a restart,
// can see that the open is active. Opens that write or hold staging are
// marked.
func (cs *Store) MarkOpen(clientID uint64, stateID nfs4.StateID) error {
	return cs.registerOpen(clientID, stateID, true)
}

// NoteOpen admits a read-only open without a durable marker: the client
// must be confirmed with a live lease, as for MarkOpen. A read-only open
// has no staging to recover and nothing another process needs to see, so
// it skips the marker's shard mutations. The caller's open state is its
// only record; the client store keeps nothing for it.
func (cs *Store) NoteOpen(clientID uint64, stateID nfs4.StateID) error {
	return cs.registerOpen(clientID, stateID, false)
}

func (cs *Store) registerOpen(clientID uint64, stateID nfs4.StateID, durable bool) error {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return err
	}
	if !valid {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	unlockMarker := cs.markerLocks.Lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()

	location, confirmed, err := cs.confirmedLocationForIdentity(incarnationID, identityID)
	if err != nil {
		return err
	}
	if !confirmed {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	state, err := cs.renewLease(incarnationID, location, false)
	if err != nil {
		return err
	}
	// Independent markers share identity ownership, but neither the lease
	// lock nor the cache map lock is held across marker I/O.
	if !durable {
		return nil
	}
	if err := cs.EnsureMarker(incarnationID, ActiveOpenName(stateID)); err != nil {
		return err
	}
	state.mu.Lock()
	state.opens[stateID] = struct{}{}
	state.mu.Unlock()
	return nil
}

// renewLease checks a confirmed client's lease for registerOpen and
// hasOpen, for marked and unmarked opens alike. It writes nothing: every
// lease write belongs to the refresher (canonical r3 §9.1). registerOpen
// (requireLiveLease false) needs a live record whose proven lease is in the
// future, else DELAY while a verify runs. hasOpen with requireLiveLease
// reads the durable lease, which after a restart may be another process's,
// and gets EXPIRED when it is dead. The caller holds shared identity
// ownership.
func (cs *Store) renewLease(
	incarnationID vfs.InodeID, location confirmedClientLocation,
	requireLiveLease bool,
) (*localClientState, error) {
	state := cs.localClientState(incarnationID, location.identityID, location.symlinkID)
	if requireLiveLease {
		live, _, err := cs.observeLeaseStatus(incarnationID)
		if err != nil {
			return nil, err
		}
		if !live {
			return nil, nfs4.Error(nfs4.NFS4ERR_EXPIRED)
		}
	} else {
		var view leaseView
		if cs.serverDecides() {
			var ok bool
			if view, ok = cs.LeaseView(uint64(incarnationID)); !ok {
				return nil, nfs4.Error(nfs4.NFS4ERR_DELAY)
			}
		} else {
			marked, err := cs.hasExpiredMarker(incarnationID)
			if err != nil {
				return nil, err
			}
			if marked {
				return nil, nfs4.Error(nfs4.NFS4ERR_EXPIRED)
			}
			if view, err = cs.refreshInline(uint64(incarnationID)); err != nil {
				return nil, err
			}
		}
		if err := cs.leaseStatusError(view); err != nil {
			switch nfs4.ErrorCode(err) {
			case nfs4.NFS4ERR_DELAY:
				cs.RequestLeaseVerify(uint64(incarnationID))
			case nfs4.NFS4ERR_STALE_CLIENTID:
				cs.LocalClients.Delete(incarnationID)
			}
			return nil, err
		}
	}
	state.mu.Lock()
	state.confirmedPointer = location.symlinkID
	state.mu.Unlock()
	return state, nil
}

// LeaseProven reports whether this process's refresher has published a
// lease for the client that is still in the future.
func (cs *Store) LeaseProven(incarnationID vfs.InodeID) bool {
	cs.RefreshMu.Lock()
	r := cs.Refreshers[incarnationID]
	cs.RefreshMu.Unlock()
	if r == nil {
		return false
	}
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.State == LeaseRecordLive && r.Proven > cs.Now().UnixNano()
}

// MarkerGone reports that the open's durable marker is definitely absent:
// the open no longer exists, whatever the client's lease.
func (cs *Store) MarkerGone(clientID uint64, stateID nfs4.StateID) bool {
	_, err := cs.fs.Lookup(vfs.InodeID(clientID), ActiveOpenName(stateID))
	return errors.Is(err, os.ErrNotExist)
}

func (cs *Store) RemoveOpen(clientID uint64, stateID nfs4.StateID) error {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, lockErr := cs.identityForIncarnation(incarnationID)
	if lockErr != nil {
		return lockErr
	}
	if !valid {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	unlockMarker := cs.markerLocks.Lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()
	err := cs.removeIfExists(incarnationID, ActiveOpenName(stateID))
	if value, ok := cs.LocalClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		delete(state.opens, stateID)
		state.mu.Unlock()
	}
	return err
}

// CachedClientIDs includes clients which only renew and have no open state.
func (cs *Store) CachedClientIDs() []uint64 {
	var result []uint64
	cs.LocalClients.Range(func(key, _ any) bool {
		result = append(result, uint64(key.(vfs.InodeID)))
		return true
	})
	return result
}

// HasOpen reports that a marked open is active: its client is confirmed
// with a live lease and its marker exists.
func (cs *Store) HasOpen(clientID uint64, stateID nfs4.StateID) (bool, error) {
	return cs.hasOpen(clientID, stateID, true)
}

// HasLocalOpen reports that the client of an open admitted with NoteOpen is
// confirmed with a live lease. The caller has already found the open in its
// own state. With no marker to check, the cached client answers from its
// lease proof without an entry for the open, so a check in flight when the
// open closes can still succeed.
func (cs *Store) HasLocalOpen(clientID uint64, stateID nfs4.StateID) (bool, error) {
	return cs.hasOpen(clientID, stateID, false)
}

func (cs *Store) hasOpen(clientID uint64, stateID nfs4.StateID, marked bool) (bool, error) {
	incarnationID := vfs.InodeID(clientID)
	if value, ok := cs.LocalClients.Load(incarnationID); ok {
		state := value.(*localClientState)

		state.mu.Lock()
		_, open := state.opens[stateID]
		identityID := state.identityID
		pointerID := state.confirmedPointer
		state.mu.Unlock()

		// A marked open's entry stands in for its marker; an unmarked open
		// needs only the client.
		if open || !marked {
			currentPointer, err := cs.fs.Lookup(identityID, ConfirmedName)
			if errors.Is(err, os.ErrNotExist) {
				cs.LocalClients.Delete(incarnationID)
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
					cs.LocalClients.Delete(incarnationID)
					return false, nil
				}

				state.mu.Lock()
				state.confirmedPointer = currentPointer
				state.mu.Unlock()

			}

			// Our own published lease answers without reading the lease,
			// unless the client has been expired; otherwise the durable
			// path below reads it.
			if cs.LeaseProven(incarnationID) {
				marked, err := cs.hasExpiredMarker(incarnationID)
				if err != nil {
					return false, err
				}
				if marked {
					cs.LocalClients.Delete(incarnationID)
					return false, nfs4.Error(nfs4.NFS4ERR_EXPIRED)
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
	unlockMarker := cs.markerLocks.Lock(clientOpenMarker{clientID, stateID})
	defer unlockMarker()
	location, confirmed, err := cs.confirmedLocationForIdentity(
		incarnationID, identityID)
	if err != nil || !confirmed {
		return false, err
	}
	if marked {
		name := ActiveOpenName(stateID)
		if _, err := cs.fs.Lookup(incarnationID, name); errors.Is(err, os.ErrNotExist) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	state, err := cs.renewLease(incarnationID, location, true)
	if nfs4.ErrorCode(err) == nfs4.NFS4ERR_EXPIRED {
		cs.LocalClients.Delete(incarnationID)
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if marked {
		state.mu.Lock()
		state.opens[stateID] = struct{}{}
		state.mu.Unlock()
	}

	return true, nil
}

func (cs *Store) localClientState(
	incarnationID vfs.InodeID,
	identityID vfs.InodeID,
	confirmedPointer vfs.InodeID,
) *localClientState {
	state := &localClientState{
		identityID:       identityID,
		confirmedPointer: confirmedPointer,
		opens:            make(map[nfs4.StateID]struct{}),
	}
	value, _ := cs.LocalClients.LoadOrStore(incarnationID, state)
	return value.(*localClientState)
}

func (cs *Store) hasActiveOpen(incarnationID vfs.InodeID) (bool, error) {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return false, err
	}
	now := cs.Now().UnixNano()
	active := false
	live := false
	for _, entry := range entries {
		switch {
		case IsActiveOpenName(entry.Name):
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
func (cs *Store) hasLiveLease(incarnationID vfs.InodeID) (bool, error) {
	live, _, err := cs.leaseStatus(incarnationID)
	return live, err
}

// observeLeaseStatus reports whether the client's durable lease is live
// (any process's slot) and whether any slot was found, without writing:
// leaseStatus(mark=false). An expired marker reads as dead.
func (cs *Store) observeLeaseStatus(incarnationID vfs.InodeID) (bool, bool, error) {
	expired, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return false, false, err
	}
	scan, err := cs.scanSlots(incarnationID, LeasePrefix)
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
	Collected   bool // the incarnation directory is gone
	Unconfirmed bool // the identity's confirmed pointer names another
	Marked      bool // an expired marker exists
	Live        bool // some process's lease is live
	Found       bool // some lease slot exists
}

// ObserveLease reads an incarnation's lease state without writing.
func (cs *Store) ObserveLease(clientID uint64) (leaseObservation, error) {
	incarnationID := vfs.InodeID(clientID)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return leaseObservation{}, err
	}
	if !valid {
		return leaseObservation{Collected: true}, nil
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	_, confirmed, err := cs.confirmedLocationForIdentity(incarnationID, identityID)
	if err != nil {
		return leaseObservation{}, err
	}
	if !confirmed {
		return leaseObservation{Unconfirmed: true}, nil
	}
	marked, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return leaseObservation{}, err
	}
	scan, err := cs.scanSlots(incarnationID, LeasePrefix)
	if err != nil {
		return leaseObservation{}, err
	}
	return leaseObservation{Marked: marked, Live: scan.live && !marked, Found: scan.found}, nil
}

// PublishExpiredMarker is decideExpired's only write: the marker, after
// arbitration (canonical r3 §8 step 3).
func (cs *Store) PublishExpiredMarker(clientID uint64) error {
	return cs.EnsureMarker(vfs.InodeID(clientID), ExpiredName)
}

// ForgetUnconfirmed ends an incarnation that is no longer confirmed: its
// markers and leases are purged, as ExpireIfLeaseDead does.
func (cs *Store) ForgetUnconfirmed(clientID uint64) error {
	incarnationID := vfs.InodeID(clientID)
	if err := cs.purgeState(incarnationID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cs.LocalClients.Delete(incarnationID)
	return nil
}

// leaseStatus observes the lease and, in store-only use, publishes the
// expired marker for a dead one and removes long-expired slots. With a
// server, only decideExpired publishes markers and the refresher removes
// slots, so this only observes.
func (cs *Store) leaseStatus(
	incarnationID vfs.InodeID,
) (bool, bool, error) {
	if cs.serverDecides() {
		return cs.observeLeaseStatus(incarnationID)
	}
	expired, err := cs.hasExpiredMarker(incarnationID)
	if err != nil {
		return false, false, err
	}
	scan, err := cs.scanSlots(incarnationID, LeasePrefix)
	if err != nil {
		return false, false, err
	}
	if !expired && !scan.live && scan.found {
		if cs.BeginMarker != nil {
			end, ok := cs.BeginMarker(incarnationID)
			if !ok {
				// Deferred until the confirmation settles; the caller retries.
				return false, true, nfs4.Error(nfs4.NFS4ERR_DELAY)
			}
			defer end()
		}
		if err := cs.EnsureMarker(incarnationID, ExpiredName); err != nil {
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

func (cs *Store) hasExpiredMarker(
	incarnationID vfs.InodeID,
) (bool, error) {
	_, err := cs.fs.Lookup(incarnationID, ExpiredName)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (cs *Store) liveSlots(
	incarnationID vfs.InodeID,
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

func (cs *Store) scanSlots(
	incarnationID vfs.InodeID,
	prefix string,
) (slotScan, error) {
	entries, err := cs.entries(incarnationID)
	if err != nil {
		return slotScan{}, err
	}
	now := cs.Now().UnixNano()
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
		if lease.ExpiresUnixNano <= now-LeaseTime.Nanoseconds() {
			scan.expiredNames = append(scan.expiredNames, entry.Name)
		}
	}
	return scan, nil
}

func (cs *Store) removeExpiredSlots(
	incarnationID vfs.InodeID,
	names []string,
) error {
	for _, name := range names {
		if err := cs.removeIfExists(incarnationID, name); err != nil {
			return err
		}
	}
	return nil
}

func (cs *Store) IsInternalFilehandle(id vfs.InodeID) (bool, error) {
	for id != cs.fs.RootID() {
		if id == cs.NFSDirID {
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

func (cs *Store) identityDir(id []byte) (vfs.InodeID, error) {
	return EnsureDir(cs.fs, cs.DirID, clientIdentityKey(id))
}

func (cs *Store) NewIncarnation(identityID vfs.InodeID) (vfs.InodeID, error) {
	for {
		suffix, err := randomHex(16)
		if err != nil {
			return 0, err
		}
		id, err := cs.fs.Mkdir(identityID, IncarnationPrefix+suffix)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return id, err
	}
}

func (cs *Store) entries(dirID vfs.InodeID) ([]vfs.DirEntry, error) {
	var result []vfs.DirEntry
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

func (cs *Store) readRecord(
	incarnationID vfs.InodeID,
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

func (cs *Store) ReadIdentitySymlink(
	identityID vfs.InodeID,
	name string,
) (vfs.InodeID, bool, error) {
	id, _, found, err := cs.readIdentitySymlinkWithFile(
		identityID, name)
	return id, found, err
}

func (cs *Store) readIdentitySymlinkWithFile(
	identityID vfs.InodeID,
	name string,
) (vfs.InodeID, vfs.InodeID, bool, error) {
	return cs.readPointer(identityID, name, false)
}

func (cs *Store) readPointer(
	dirID vfs.InodeID,
	name string,
	parentRelative bool,
) (vfs.InodeID, vfs.InodeID, bool, error) {
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

func (cs *Store) ReadRebootSymlink(
	incarnationID vfs.InodeID,
) (vfs.InodeID, bool, error) {
	id, _, found, err := cs.readPointer(
		incarnationID, RebootName, true)
	return id, found, err
}

func (cs *Store) resolvePointer(
	dirID vfs.InodeID,
	fileID vfs.InodeID,
	parentRelative bool,
) (vfs.InodeID, error) {
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

func (cs *Store) resolveIdentitySymlink(
	identityID vfs.InodeID,
	fileID vfs.InodeID,
) (vfs.InodeID, error) {
	return cs.resolvePointer(identityID, fileID, false)
}

func (cs *Store) resolveIncarnationName(
	identityID vfs.InodeID,
	name string,
) (vfs.InodeID, error) {
	if !isIncarnationName(name) {
		return 0, fmt.Errorf("client store: invalid incarnation pointer")
	}
	id, err := cs.fs.Lookup(identityID, name)
	if err != nil {
		return 0, err
	}
	if !IsDirectoryInodeID(id) {
		return 0, fmt.Errorf("client store: invalid client pointer")
	}
	return id, nil
}

func (cs *Store) readLease(id vfs.InodeID) (DurableLease, error) {
	var lease DurableLease
	if err := cs.readJSONFile(id, "lease", &lease); err != nil {
		return DurableLease{}, err
	}
	return lease, nil
}

func (cs *Store) readJSON(
	dirID vfs.InodeID,
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

func (cs *Store) readJSONFile(
	id vfs.InodeID,
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

func (cs *Store) replaceIdentitySymlink(
	identityID vfs.InodeID,
	name string,
	clientID vfs.InodeID,
) (vfs.InodeID, error) {
	targetName, err := cs.requireIncarnationName(identityID, clientID)
	if err != nil {
		return 0, err
	}
	return cs.replaceSymlink(identityID, name, targetName)
}

func (cs *Store) replaceRebootSymlink(
	incarnationID vfs.InodeID,
	clientID vfs.InodeID,
) (vfs.InodeID, error) {
	identityID, err := cs.fs.LookupParent(incarnationID)
	if err != nil {
		return 0, err
	}
	targetName, err := cs.requireIncarnationName(identityID, clientID)
	if err != nil {
		return 0, err
	}
	return cs.replaceSymlink(
		incarnationID, RebootName, "../"+targetName)
}

func (cs *Store) CreateJSON(
	dirID vfs.InodeID,
	name string,
	value any,
) (vfs.InodeID, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return cs.fs.CreateFile(dirID, name, bytes.NewReader(data))
}

func (cs *Store) replaceJSON(
	dirID vfs.InodeID,
	name string,
	value any,
) (vfs.InodeID, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return cs.replaceBytes(dirID, name, data)
}

func (cs *Store) replaceBytes(
	dirID vfs.InodeID,
	name string,
	data []byte,
) (vfs.InodeID, error) {
	suffix, err := randomHex(8)
	if err != nil {
		return 0, err
	}
	tempName := TempPrefix + suffix
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

func (cs *Store) replaceSymlink(
	dirID vfs.InodeID,
	name string,
	target string,
) (vfs.InodeID, error) {
	suffix, err := randomHex(8)
	if err != nil {
		return 0, err
	}
	tempName := TempPrefix + suffix
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

func (cs *Store) EnsureMarker(dirID vfs.InodeID, name string) error {
	if _, err := cs.fs.Lookup(dirID, name); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := cs.fs.CreateFile(dirID, name, nil)
	return err
}

func (cs *Store) removeIfExists(dirID vfs.InodeID, name string) error {
	err := cs.fs.Remove(dirID, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ErrToNFS(err error) uint32 {
	var nfs nfs4.Error
	if errors.As(err, &nfs) {
		return uint32(nfs)
	}
	return nfs4.NFS4ERR_DELAY
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

func IsDirectoryInodeID(id vfs.InodeID) bool {
	return id != 0 && id.Type() == vfs.InodeTypeDir
}

func isIncarnationName(name string) bool {
	if !strings.HasPrefix(name, IncarnationPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(name, IncarnationPrefix)
	if len(encoded) != 32 {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func ActiveOpenName(stateID nfs4.StateID) string {
	return activeOpenPrefix + hex.EncodeToString(stateID[:])
}

func IsActiveOpenName(name string) bool {
	return strings.HasPrefix(name, activeOpenPrefix) &&
		len(name) > len(activeOpenPrefix)
}

func isLeaseName(name string) bool {
	return strings.HasPrefix(name, LeasePrefix) &&
		len(name) > len(LeasePrefix)
}

func isConfirmingName(name string) bool {
	return strings.HasPrefix(name, confirmingPrefix) &&
		len(name) > len(confirmingPrefix)
}

// StagingRecoveryKey identifies one client boot independently of the lease
// incarnation. Hash the principal too so an identity takeover cannot recover
// another principal's private bytes. The key survives incarnation GC.
func (cs *Store) StagingRecoveryKey(clientID uint64) ([32]byte, error) {
	id := vfs.InodeID(clientID)
	identity, valid, err := cs.identityForIncarnation(id)
	if err != nil {
		return [32]byte{}, err
	}
	if !valid {
		return [32]byte{}, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	record, found, err := cs.readRecord(id, ClientRecordName)
	if err != nil {
		return [32]byte{}, err
	}
	if !found {
		return [32]byte{}, fmt.Errorf("missing client record for %d", clientID)
	}
	data, err := json.Marshal(struct {
		Identity  vfs.InodeID
		Verifier  []byte
		Flavor    uint32
		Principal []byte
	}{identity, record.Verifier, record.PrincipalFlavor, record.PrincipalBody})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// ConfirmedRecord answers an EXCHANGE_ID update (EXCHGID4_FLAG_UPD_CONFIRMED_REC_A)
// without changing the store. RFC 8881 section 18.35.5 cases 6-8 require a
// confirmed record with the same verifier and principal.
func (cs *Store) ConfirmedRecord(
	id []byte,
	verifier [8]byte,
	principal RPCPrincipal,
) (uint64, uint32) {
	identityID, err := cs.fs.Lookup(cs.DirID, clientIdentityKey(id))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nfs4.NFS4ERR_NOENT
	}
	if err != nil {
		return 0, ErrToNFS(err)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	confirmedID, confirmed, err := cs.ReadIdentitySymlink(identityID, ConfirmedName)
	if err != nil {
		return 0, ErrToNFS(err)
	}
	if !confirmed {
		return 0, nfs4.NFS4ERR_NOENT
	}
	record, found, err := cs.readRecord(confirmedID, ClientRecordName)
	if err != nil {
		return 0, ErrToNFS(err)
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
