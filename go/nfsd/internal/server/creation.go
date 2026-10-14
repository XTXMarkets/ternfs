// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// creation holds one CREATE_SESSION's reservations: the incarnation's group
// hold, its session record and its creation slot for seq. They are released
// once, by finish, by the request or by the confirmation that took them over.
type creation struct {
	s          *Server
	c          *sessionClient
	seq        uint32
	id         SessionID
	fore, back channelAttrs
	endCreate  func()
	record     bool // the session-record reservation is held
	slot       bool // c.csBusy for seq belongs to this creation
	created    *session
	resume     bool // look for an unfinished replacement purge after commit
	// leaseRecord releases the refresher record reserved before the slot.
	leaseRecord func()
	// backGrant: CONN_BACK_CHAN was asked for on a transport that can carry
	// callbacks, so the result grants it.
	backGrant bool
	// The callback program and credential, for the session.
	cbProgram     uint32
	cbSec         callbackSec
	backRequested bool
}

// cacheLocked records the slot's outcome for seq and releases the slot.
func (cr *creation) cacheLocked(status uint32, reply createSessionResult) {
	c := cr.c
	c.csSeq, c.csCached, c.csStatus, c.csReply = cr.seq, true, status, reply
	c.csBusy, cr.slot = false, false
}

// commitLocked inserts the session and caches OK. It reports false, with
// nothing inserted, if the record was retired meanwhile.
func (cr *creation) commitLocked() (createSessionResult, bool) {
	ss, c := cr.s.sessions, cr.c
	// A retired record, or an incarnation this process has expired (its
	// wire record stays for cached replay), gets no new session.
	if g := ss.groups[c.incarnation]; c.retired || g != nil && g.lease == gateExpired {
		c.csBusy, cr.slot = false, false
		return createSessionResult{}, false
	}
	sess := &session{
		id:        cr.id,
		clientID:  c.incarnation,
		slots:     make([]sessionSlot, cr.fore.maxReqs),
		maxOps:    cr.fore.maxOps,
		maxReq:    cr.fore.maxReq,
		maxResp:   cr.fore.maxResp,
		maxCached: cr.fore.maxCached,

		cbProgram:     cr.cbProgram,
		cbSec:         cr.cbSec,
		backRequested: cr.backRequested,
		backMaxReq:    cr.back.maxReq,
		backMaxResp:   cr.back.maxResp,
		backMaxOps:    cr.back.maxOps,
		backMaxReqs:   cr.back.maxReqs,
	}
	sess.cb = newCBSender(cr.s, sess, cr.cbProgram, cr.cbSec)
	ss.insertSessionLocked(sess)
	cr.created = sess
	ss.confirmWireLocked(c)
	c.hasConfirm = false
	// Every successful CREATE_SESSION of a confirmed incarnation finishes a
	// replacement purge left by an earlier one, here or before a restart.
	cr.resume = true
	result := createSessionResult{id: cr.id, seq: cr.seq, foreAttrs: cr.fore, backAttr: cr.back, back: cr.backGrant}
	cr.cacheLocked(nfs4.NFS4_OK, result)
	return result, true
}

// finish runs when the request returns, with no lock held: the committed
// session's bootstrap, the resumed replacement purge, then the release of
// every reservation still held. An uncached slot is rolled back. The purge
// contains its own panics, and the release is deferred, so it runs
// regardless.
func (cr *creation) finish() {
	s := cr.s
	defer cr.release()
	if cr.created != nil {
		created := cr.created
		cr.created = nil
		// The only place the three identities a capture can show are logged
		// together: the wire clientid EXCHANGE_ID returned, the durable
		// incarnation it is bound to, and the session.
		s.log.Info("session created",
			"wire_clientid", wireText(cr.c.clientID),
			"incarnation", incarnationText(created.clientID),
			"session", sessionText(created.id))
		// The first durable publication: until it, SEQUENCE gets DELAY
		// (canonical r3 §9.1). The worker writes it; the reply waits a
		// bounded time for it, so a client's first SEQUENCE normally finds
		// it published. Clients such as libnfs treat a DELAY there as a
		// mount failure.
		s.clients.StartLeaseBootstrap(created.clientID)
		s.clients.AwaitLeasePublished(created.clientID, clientstore.BootstrapWait)
	}
	if cr.resume {
		cr.resume = false
		s.resumeReplacementPurge(cr.c.incarnation)
	}
}

// release ends the creation's reservations, once each.
func (cr *creation) release() {
	ss := cr.s.sessions
	if cr.slot {
		ss.mu.Lock()
		cr.c.csBusy = false
		ss.mu.Unlock()
		cr.slot = false
	}
	if cr.record {
		ss.releaseSessionRecord()
		cr.record = false
	}
	if cr.endCreate != nil {
		cr.endCreate()
		cr.endCreate = nil
	}
	if cr.leaseRecord != nil {
		cr.leaseRecord()
		cr.leaseRecord = nil
	}
}
