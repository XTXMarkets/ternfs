// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

const (
	// cbProgramVersion is the callback RPC version: NFS_CB = 1 in the
	// official NFSv4.1 XDR (nfs.x), which Linux sends and accepts. RFC 8881
	// section 18.36.3's prose says 4; the XDR and implementations use 1.
	cbProgramVersion = 1
	cbProcNull       = 0
	cbProcCompound   = 1
	cbMaxDelays      = 3
)
