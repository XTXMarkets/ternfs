// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

const (
	maxReplyWorkBytes   = 512 << 20
	cachedReplyOverhead = 128
	maxSessionRecords   = 4096
	// A group is referenced by a wire record, a session, or a request or
	// CREATE_SESSION in progress. The first two are bounded by their own
	// caps; the credit bounds groups that only running work keeps.
	maxClientGroups = 2 * maxSessionRecords
)
