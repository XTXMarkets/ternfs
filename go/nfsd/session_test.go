// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// A confirmed client may create more sessions, continuing its sequence.
func TestCreateSessionAfterConfirmation(t *testing.T) {
	addr, cleanup := startTestServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID, confirm := requestClientID(t, conn, &xid, "sessions", [8]byte{1})
	seq := decodeConfirm(confirm)
	for i := uint32(0); i < 3; i++ {
		if _, status, err := createSession(conn, xid, clientID, seq+i); err != nil || status != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %d = %s, %v", i, nfs4.Nfsstat4Name(status), err)
		}
		xid++
	}
	if _, status, _ := createSession(conn, xid, clientID, seq+5); status != nfs4.NFS4ERR_SEQ_MISORDERED {
		t.Fatalf("out-of-sequence CREATE_SESSION = %s", nfs4.Nfsstat4Name(status))
	}
}
