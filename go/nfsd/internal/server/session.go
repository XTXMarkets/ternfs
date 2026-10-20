// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sync"
	"sync/atomic"
)

type sessionSlot struct {
	mu       sync.Mutex
	seq      uint32
	inUse    bool
	cached   bool
	reply    []byte // COMPOUND4res body of the last request
	occupied bool   // a request has completed on this slot
	digest   [32]byte
	user     effectiveUser
	reserved int
	buffer   *cachedReply
	// restored: the slot's state was read from the session file at startup
	// and no request has used it since. hasIntent: its record is the intent
	// of a request whose reply was never written, with payload intent
	// (replay_decide.go).
	restored, hasIntent bool
	intent              []byte
}

type session struct {
	id        SessionID
	clientID  uint64
	slots     []sessionSlot
	maxOps    uint32
	maxReq    uint32
	maxResp   uint32
	maxCached uint32
	// The callback program and credential CREATE_SESSION gave, kept for
	// the backchannel; backRequested is CONN_BACK_CHAN in its flags.
	cbProgram     uint32
	cbSec         callbackSec
	backRequested bool
	// cb is the backchannel sender; backMax* are the negotiated back
	// channel limits. recalled: the session was asked to use half its
	// slots while the reply cache is under pressure.
	cb                                               *cbSender
	backMaxReq, backMaxResp, backMaxOps, backMaxReqs uint32
	recalled                                         atomic.Bool

	destroyed atomic.Bool
	fatal     atomic.Bool

	// file is the session's persistent record and slot table, empty when
	// the server has no staging directory or the file could not be made.
	// Requests use it without the store lock; it is closed when the
	// session's last use ends (releaseSessionUse).
	file sessionFile
	// restored: this process rebuilt the session from its file at startup.
	restored bool

	// users counts store membership plus admitted slot requests, under
	// sessionStore.mu. The record stays counted until the last one ends.
	users int
}

type channelAttrs struct {
	headerPad, maxReq, maxResp, maxCached, maxOps, maxReqs uint32
}
