// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
)

// creation holds one CREATE_SESSION's reservations: the incarnation's group
// hold, its session record and its creation slot for seq. They are released
// once, by finish, when the request returns.
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
	// backGrant: CONN_BACK_CHAN was asked for on a transport that can carry
	// callbacks, so the result grants it.
	backGrant bool
	// The callback program and credential, for the session.
	cbProgram     uint32
	cbSec         callbackSec
	backRequested bool
	// file is the session's file, the creation's until commitLocked gives
	// it to the session. fileAttempted: a file may be at the session's path,
	// made by an attempt that did not return it.
	file          *sessionstore.File
	fileAttempted bool
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
	if cr.file != nil {
		sess.file.attach(cr.file, cr.s)
		cr.file, cr.fileAttempted = nil, false
	}
	cr.created = sess
	ss.confirmWireLocked(c)
	// A new session starts the client's lease: it expires a lease after
	// this if the client sends nothing.
	if g := ss.groups[c.incarnation]; g != nil {
		if quiet := ss.now().Add(leaseTime); quiet.After(g.quietUntil) {
			g.quietUntil = quiet
		}
	}
	result := createSessionResult{id: cr.id, seq: cr.seq, foreAttrs: cr.fore, backAttr: cr.back, back: cr.backGrant}
	cr.cacheLocked(nfs4.NFS4_OK, result)
	return result, true
}

// finish runs when the request returns, with no lock held: it logs the
// committed session, then releases every reservation still held. An
// uncached slot is rolled back.
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
	}
}

// release ends the creation's reservations, once each, and removes the
// session file of a creation that inserted no session.
func (cr *creation) release() {
	cr.s.removeCreationFile(cr)
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
}
