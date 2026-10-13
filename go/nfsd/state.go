// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

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
