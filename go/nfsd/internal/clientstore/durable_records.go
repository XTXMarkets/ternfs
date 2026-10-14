// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

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

// DurableLease is the JSON body of a lease or confirming file. It records
// when the lease or claim written by one nfsd expires.
type DurableLease struct {
	ExpiresUnixNano int64 `json:"expires_unix_nano"`
}

// durableGCCandidate is the JSON body of the gc file. It records the earliest
// time at which an unreachable incarnation may be removed.
type durableGCCandidate struct {
	CollectAfterUnixNano int64 `json:"collect_after_unix_nano"`
}

func newDurableClientRecord(
	id []byte,
	verifier [8]byte,
	confirm [8]byte,
	owner ClientOwner,
) durableClientRecord {
	return durableClientRecord{
		ID:              append([]byte(nil), id...),
		Verifier:        append([]byte(nil), verifier[:]...),
		Confirm:         append([]byte(nil), confirm[:]...),
		PrincipalFlavor: owner.Principal.Flavor,
		PrincipalBody:   []byte(owner.Principal.Body),
	}
}

func (r durableClientRecord) principal() RPCPrincipal {
	return RPCPrincipal{Flavor: r.PrincipalFlavor, Body: string(r.PrincipalBody)}
}
