// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func sequenceArgsForTest(t *testing.T, wire []byte) nfs4.SEQUENCE4args {
	t.Helper()
	args, ok := nfs4.ReadCOMPOUND4args(wire)
	it := args.Argarray()
	if !ok || !it.Next() {
		t.Fatal("SEQUENCE arguments")
	}
	return it.Argarray().Value().AsSEQUENCE4args()
}

func readPublished(fs vfs.TernVFS, name string) ([]byte, error) {
	id, err := fs.Lookup(fs.RootID(), name)
	if err != nil {
		return nil, err
	}
	return fs.ReadAll(id)
}

// addrOfTestServer finds a served test server's address in the registry.
func addrOfTestServer(t *testing.T, srv *Server) string {
	t.Helper()
	var addr string
	testServers.Range(func(k, v any) bool {
		if v.(*Server) == srv {
			addr = k.(string)
			return false
		}
		return true
	})
	if addr == "" {
		t.Fatal("server not registered")
	}
	return addr
}

// exchangeOn registers a client over c and returns its clientid and next
// CREATE_SESSION sequence.
func exchangeOn(t *testing.T, c *cbTestClient, xid uint32, owner string) (uint64, uint32) {
	t.Helper()
	res := c.compound(xid, exchangeIDBody(owner, [8]byte{9}))
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s", nfs4.Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	return ex.EirClientid(), ex.EirSequenceid()
}

func createdSession(t *testing.T, res nfs4.COMPOUND4res) (SessionID, uint32) {
	t.Helper()
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	return readSessionID(ok.CsrSessionid()), ok.CsrFlags()
}
