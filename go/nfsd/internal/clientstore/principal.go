// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

// RPCPrincipal identifies the caller of an RPC by its credential flavor and
// body. It is compared, not interpreted, so nfsd can tell whether the same
// caller is reusing a client identity.
type RPCPrincipal struct {
	Flavor uint32
	Body   string
}

// ClientOwner is the principal recorded for a client registration.
type ClientOwner struct {
	Principal RPCPrincipal
}
