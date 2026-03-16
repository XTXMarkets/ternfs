// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

// StateID is the 12-byte "other" field of an NFSv4 stateid4.
type StateID [12]byte

// nfsError is a simple error type that carries an NFS status code.
type nfsError uint32

func (e nfsError) Error() string {
	return "nfs error"
}

func nfsErrCode(err error) uint32 {
	if e, ok := err.(nfsError); ok {
		return uint32(e)
	}
	return NFS4ERR_SERVERFAULT
}
