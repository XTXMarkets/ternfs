// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
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

// createFile publishes a file with data at name, replacing what is there,
// as a TernFS client would: through a transient.
func createFile(fs vfs.TernVFS, dir vfs.InodeID, name, data string) (vfs.InodeID, error) {
	id, cookie, err := fs.ConstructFile(dir)
	if err != nil {
		return 0, err
	}
	return id, fs.LinkFile(id, cookie, dir, name, strings.NewReader(data))
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

// stagingFiles lists the staging directory's files, leaving out its
// subdirectories: aliases and quarantine are not staged writers.
func stagingFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := entries[:0]
	for _, e := range entries {
		if !e.IsDir() && e.Name() != staging.ServerIDFileName {
			files = append(files, e)
		}
	}
	return files, nil
}

// holdRecord makes the next reply record wait, once written, until release
// is closed; recording is closed when it waits. The compound's last
// operation keeps its locks until then.
func holdRecord(srv *Server) (recording, release chan struct{}) {
	recording, release = make(chan struct{}), make(chan struct{})
	persist := srv.sessions.persistSlot
	var held atomic.Bool
	srv.sessions.persistSlot = func(sess *session, slotID uint32, rec sessionstore.Record) {
		persist(sess, slotID, rec)
		if held.CompareAndSwap(false, true) {
			close(recording)
			<-release
		}
	}
	return recording, release
}

// openBody opens name in the root for owner with access, creating nothing,
// and returns its handle.
func openBody(clientID uint64, owner, name string, access uint32) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(access)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientID)
		ow.Resume(ownerW.SetOwner([]byte(owner)).Finish())
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		ow.Resume(ow.SetClaim_Null().SetData([]byte(name)).Finish())
		w.Resume(ow.Finish())
		w.AppendArgarray_Getfh()
	})
}

// sendAsync sends wire on conn from another goroutine; the reply arrives
// on the channel, nil on a transport error.
func sendAsync(conn net.Conn, xid uint32, wire []byte) <-chan []byte {
	done := make(chan []byte, 1)
	go func() {
		reply, err := sendRawRPC(conn, xid, procCompound, wire)
		if err != nil {
			reply = nil
		}
		done <- reply
	}()
	return done
}

// newTestStaging returns a store on a fresh staging directory.
func newTestStaging(t *testing.T) *staging.LocalStore {
	t.Helper()
	store, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// startTestServerWithVFS serves fs with a fresh staging directory.
func startTestServerWithVFS(t *testing.T, fs vfs.TernVFS) (*Server, string, func()) {
	t.Helper()
	srv, err := NewServer(fs, newTestStaging(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	return srv, addr, cleanup
}
