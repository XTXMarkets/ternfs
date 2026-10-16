// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
)

// sessionClient is the wire record of one EXCHANGE_ID clientid. clientID is
// the volatile wire clientid; incarnation is the durable client it is bound
// to, which owns sessions and state. A process has at most one live wire
// record per incarnation.
type sessionClient struct {
	clientID    uint64
	incarnation uint64
	owner       string
	verifier    [8]byte
	principal   clientstore.RPCPrincipal
	createdAt   time.Time
	// confirmed is set by the first successful CREATE_SESSION. An adopted
	// record names an incarnation that was already durably confirmed.
	confirmed, adopted bool
	retired            bool

	// confirm is the client-store confirmation verifier still to be
	// applied by the first CREATE_SESSION.
	confirm    [8]byte
	hasConfirm bool

	// The creation slot (RFC 8881 section 18.36.4), under sessionStore.mu.
	// csSeq is the sequence of the cached outcome. It starts at 0, so the
	// first CREATE_SESSION is 1, and wraps. When csCached, the outcome is
	// csReply if csStatus is NFS4_OK, else the error csStatus. While csBusy,
	// the request for csBusySeq owns the slot; it runs without the mutex.
	csSeq     uint32
	csCached  bool
	csReply   createSessionResult
	csStatus  uint32
	csBusy    bool
	csBusySeq uint32

	// confirming is the confirmation holding the slot for csBusySeq.
	confirming *confirmation
}
