// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Wire clientids are the eir_clientid values EXCHANGE_ID returns. They are
// process-local and never stored: a wire clientid is the process epoch in the
// high 32 bits and a counter in the low 32 bits. The epoch is a whole second
// chosen after this process binds its endpoint exclusively, and later than
// the second of the bind, so it exceeds the epoch of every earlier owner of
// the endpoint. A clientid from another epoch is therefore never mistaken
// for one of ours. The counter never wraps.
//
// Wire clientids are distinct from durable client incarnations and from the
// session ID's own epoch and counter.

const maxWireCounter = ^uint32(0)

var (
	errWireEpochUnset   = errors.New("wire clientid epoch is not initialized")
	errWireIDsExhausted = errors.New("wire clientids are exhausted")
)

type wireClientIDs struct {
	mu    sync.Mutex
	epoch uint32 // 0 until start
	last  uint32 // the counter of the last clientid issued
}

// start sets the epoch once. An epoch outside 1..0xffffffff stops startup.
func (w *wireClientIDs) start(epoch int64) error {
	if epoch < 1 || epoch > int64(^uint32(0)) {
		return fmt.Errorf("wire clientid epoch %d is out of range", epoch)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.epoch != 0 {
		return fmt.Errorf("wire clientid epoch is already %d", w.epoch)
	}
	w.epoch = uint32(epoch)
	return nil
}

// next issues a new wire clientid. After the counter reaches 0xffffffff,
// every later call fails.
func (w *wireClientIDs) next() (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.availableLocked(); err != nil {
		return 0, err
	}
	w.last++
	return uint64(w.epoch)<<32 | uint64(w.last), nil
}

// available reports whether next can succeed, without issuing a clientid.
func (w *wireClientIDs) available() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.availableLocked()
}

func (w *wireClientIDs) availableLocked() error {
	if w.epoch == 0 {
		return errWireEpochUnset
	}
	if w.last == maxWireCounter {
		return errWireIDsExhausted
	}
	return nil
}

// wireOwner holds an EXCHANGE_ID owner's wire records in this process: the
// confirmed one, and at most one awaiting its first CREATE_SESSION.
type wireOwner struct {
	confirmed, unconfirmed *sessionClient
}

// wireRecordLocked finds a live wire record. Never-issued, other-epoch and
// retired clientids are all absent.
func (ss *sessionStore) wireRecordLocked(clientID uint64) *sessionClient {
	return ss.clients[clientID]
}

// addWireLocked records a new unconfirmed wire record for its incarnation.
// The caller has retired any previous record of the owner or incarnation,
// and holds the incarnation's group.
func (ss *sessionStore) addWireLocked(c *sessionClient) {
	ss.clients[c.clientID] = c
	o := ss.owners[c.owner]
	if o == nil {
		o = &wireOwner{}
		ss.owners[c.owner] = o
	}
	o.unconfirmed = c
	ss.groups[c.incarnation].wire = c
}

// confirmWireLocked marks a record confirmed by CREATE_SESSION.
func (ss *sessionStore) confirmWireLocked(c *sessionClient) {
	if c.confirmed || c.retired {
		return
	}
	c.confirmed = true
	o := ss.owners[c.owner]
	if o.unconfirmed == c {
		o.unconfirmed = nil
	}
	// A confirmed predecessor stays until its incarnation is dropped; only
	// the newest confirmed record answers for the owner.
	o.confirmed = c
}

// retireWireLocked forgets a wire record. Its clientid is never valid again.
// The incarnation's sessions are not affected.
func (ss *sessionStore) retireWireLocked(c *sessionClient) {
	if c.retired {
		return
	}
	c.retired = true
	delete(ss.clients, c.clientID)
	if o := ss.owners[c.owner]; o != nil {
		if o.confirmed == c {
			o.confirmed = nil
		}
		if o.unconfirmed == c {
			o.unconfirmed = nil
		}
		if o.confirmed == nil && o.unconfirmed == nil {
			delete(ss.owners, c.owner)
		}
	}
	if g := ss.groups[c.incarnation]; g != nil && g.wire == c {
		g.wire = nil
		ss.releaseGroupLocked(g)
	}
}

// retireWireOf retires an incarnation's wire record once the client has
// expired or is no longer confirmed. Its sessions are left to fail on their
// own; a new EXCHANGE_ID then starts from the durable store.
func (ss *sessionStore) retireWireOf(incarnation uint64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	// A record being confirmed is protected; its confirmation decides.
	if g := ss.groups[incarnation]; g != nil && g.wire != nil && g.wire.confirming == nil {
		ss.retireWireLocked(g.wire)
	}
}

// reclaimUnconfirmedLocked retires records that have waited a lease interval
// for their first CREATE_SESSION, except one whose CREATE_SESSION is in
// progress.
func (ss *sessionStore) reclaimUnconfirmedLocked(now time.Time) {
	for _, c := range ss.clients {
		if !c.confirmed && !c.csBusy && !now.Before(c.createdAt.Add(nfsLeaseTime)) {
			ss.retireWireLocked(c)
		}
	}
}

// wireEpochAfter waits until the wall clock is in a later whole second than
// bound, then returns that second.
func wireEpochAfter(bound time.Time, now func() time.Time, sleep func(time.Duration)) int64 {
	for {
		t := now()
		if t.Unix() > bound.Unix() {
			return t.Unix()
		}
		sleep(time.Unix(bound.Unix()+1, 0).Sub(t))
	}
}
