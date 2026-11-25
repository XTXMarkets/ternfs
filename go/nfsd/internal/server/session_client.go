// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
)

// sessionClient is the wire record of one EXCHANGE_ID clientid. clientID is
// the volatile wire clientid; incarnation is the client it is bound to,
// which owns sessions and state. A pending record's incarnation is chosen
// at EXCHANGE_ID and recorded when CREATE_SESSION confirms it. A process
// has at most one live wire record per incarnation.
type sessionClient struct {
	clientID    uint64
	incarnation uint64
	owner       string
	verifier    [8]byte
	principal   clientrecords.Principal
	createdAt   time.Time
	// confirmed is set by the first successful CREATE_SESSION. An adopted
	// record names an incarnation that a client record already confirms.
	confirmed, adopted bool
	retired            bool

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
}

// pending reports a registration that CREATE_SESSION has yet to confirm.
func (c *sessionClient) pending() bool { return !c.confirmed && !c.adopted }
