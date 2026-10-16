// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package nfs4

// Error is a simple error type that carries an NFS status code.
type Error uint32

func (e Error) Error() string {
	return "nfs error"
}

func ErrorCode(err error) uint32 {
	if e, ok := err.(Error); ok {
		return uint32(e)
	}
	return NFS4ERR_SERVERFAULT
}
