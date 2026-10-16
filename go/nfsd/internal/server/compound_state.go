// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// compoundState tracks the current/saved filehandle during COMPOUND execution.
type compoundState struct {
	currentID     vfs.InodeID
	currentIDSet  bool
	savedID       vfs.InodeID
	savedIDSet    bool
	principal     clientstore.RPCPrincipal
	connection    *rpcConnection
	replyBudget   uint32
	effectUnknown bool // Submitted mutation lacks whole-call outcome proof.

	// session and clientID come from the compound's SEQUENCE.
	session  *session
	clientID uint64

	// afterReply collects work for after the reply is written.
	afterReply *[]func()

	// currentStateID is the stateid returned by the last OPEN or CLOSE,
	// substituted for the special current stateid (RFC 8881 section
	// 16.2.3.1.2). Changing the current filehandle clears it.
	currentStateID    nfs4.StateID
	currentGeneration uint32
	currentStateIDSet bool
	savedStateID      nfs4.StateID
	savedGeneration   uint32
	savedStateIDSet   bool
}

// setCurrentFH changes the current filehandle. Operations which do so
// without producing a stateid invalidate the current stateid.
func (st *compoundState) setCurrentFH(id vfs.InodeID) {
	st.currentID = id
	st.currentIDSet = true
	st.currentStateIDSet = false
}

func (st *compoundState) setCurrentStateID(id nfs4.StateID, generation uint32) {
	st.currentStateID = id
	st.currentGeneration = generation
	st.currentStateIDSet = true
}

func (st *compoundState) observeMutation(fs vfs.TernVFS, op uint32, err error) {
	if err != nil && fs.ClassifyMutation(op, err) == vfs.MutationUnknown {
		st.effectUnknown = true
	}
}

// isCurrentStateID reports the special stateid meaning the compound's
// current stateid: seqid 1, other all zero.
func isCurrentStateID(stateid nfs4.Stateid4) bool {
	if stateid.Seqid() != 1 {
		return false
	}
	for i := 0; i < 12; i++ {
		if stateid.Other(i) != 0 {
			return false
		}
	}
	return true
}

// resolveStateID substitutes the compound's current stateid for the special
// current stateid. It returns the stateid to use and false if no current
// stateid is set.
func (st *compoundState) resolveStateID(stateid nfs4.Stateid4) (nfs4.Stateid4, bool) {
	if !isCurrentStateID(stateid) {
		return stateid, true
	}
	if !st.currentStateIDSet {
		return stateid, false
	}
	_, resolved := nfs4.StartStateid4(nil)
	writeStateID(resolved, st.currentGeneration, st.currentStateID)
	return resolved, true
}

func isSpecialStateID(stateid nfs4.Stateid4) bool {
	seqid := stateid.Seqid()
	allZero := seqid == 0
	allOne := seqid == ^uint32(0)
	for i := 0; i < 12; i++ {
		allZero = allZero && stateid.Other(i) == 0
		allOne = allOne && stateid.Other(i) == 0xff
	}
	return allZero || allOne
}

// later runs f after the compound's reply is written, or at once for a
// compound with no transport.
func (st *compoundState) later(f func()) {
	if st.afterReply == nil {
		f()
		return
	}
	*st.afterReply = append(*st.afterReply, f)
}
