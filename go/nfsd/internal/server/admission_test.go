// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func emptySetattr(w *nfs4.COMPOUND4argsWriter) {
	op := w.AppendArgarray_Setattr()
	attrs := op.StartObjAttributes()
	mask := attrs.StartAttrmask()
	attrs.Resume(mask.Finish())
	attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
	op.Resume(attrs.Finish())
	w.Resume(op.Finish())
}

// At cache limits either side of CREATE and its GETFH, CREATE runs with room for
// the handle or is refused before creating anything, and the reply replays.
func TestCreateHandleObligationAtCacheLimit(t *testing.T) {
	for _, limit := range []uint32{140, 152} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			root := t.TempDir()
			fs := vfs.NewLocalTernVFS(root)
			srv := admissionWritableServer(t, fs)
			ts := connectionSession(t, srv)
			srv.sessions.lookup(ts.id).maxCached = limit
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				admissionCreate(w, "born")
				w.AppendArgarray_Getfh()
				emptySetattr(w)
			}), ts, 0, 1, true)
			reply, res := admissionRequest(t, srv, wire, 1, nil)
			iter := res.Resarray()
			iter.Next()
			iter.Next()
			iter.Next()
			create := iter.Resarray().Value().AsCREATE4resEntry()
			if create.Disc() == nfs4.NFS4_OK {
				if !iter.Next() || iter.Resarray().Disc() != nfs4.OP_GETFH || iter.Resarray().Value().AsGETFH4resEntry().Disc() != nfs4.NFS4_OK {
					t.Fatalf("CREATE ran without promised GETFH: status=%s bytes=%d", nfs4.Nfsstat4Name(res.Status()), len(reply))
				}
			} else {
				if _, err := fs.Lookup(fs.RootID(), "born"); !os.IsNotExist(err) {
					t.Fatalf("pre-effect rejection created object: %v", err)
				}
			}
			replay, _ := admissionRequest(t, srv, wire, 2, nil)
			if !bytes.Equal(reply[24:], replay[24:]) {
				t.Fatal("truthful prefix changed on replay")
			}
		})
	}
}

// A variable-size result between CREATE and its GETFH makes the compound
// unsafe before CREATE runs.
func TestVariableResultBeforeGetfhIsUnsafe(t *testing.T) {
	root := t.TempDir()
	fs := vfs.NewLocalTernVFS(root)
	srv := admissionWritableServer(t, fs)
	ts := connectionSession(t, srv)
	srv.sessions.lookup(ts.id).maxCached = 160
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		c := w.AppendArgarray_Create()
		c.Resume(c.SetObjtype_Nf4lnk().SetData(bytes.Repeat([]byte("x"), 128)).Finish())
		c.Resume(c.StartObjname().SetData([]byte("link")).Finish())
		a := c.StartCreateattrs()
		m := a.StartAttrmask()
		a.Resume(m.Finish())
		a.Resume(a.StartAttrVals().SetData(nil).Finish())
		c.Resume(a.Finish())
		w.Resume(c.Finish())
		w.AppendArgarray_Readlink()
		w.AppendArgarray_Getfh()
	}), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	if _, err := fs.Lookup(fs.RootID(), "link"); !os.IsNotExist(err) {
		t.Fatalf("unsafe variable path created link: %v, status=%s", err, nfs4.Nfsstat4Name(res.Status()))
	}
	admissionStatus(t, res, nfs4.NFS4ERR_UNSAFE_COMPOUND, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_CREATE)
}

// Removed NFSv4.0 operations get NOTSUPP and unknown ones OP_ILLEGAL, and an
// uncached retry replays the same reply without changing the slot.
func TestMandatoryOperationErrorsReplay(t *testing.T) {
	for _, opcode := range []uint32{9999, nfs4.OP_OPEN_CONFIRM, nfs4.OP_RENEW, nfs4.OP_RELEASE_LOCKOWNER, nfs4.OP_SETCLIENTID_CONFIRM} {
		t.Run(fmt.Sprint(opcode), func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			body := buildCompoundBody(nil, nil)
			binary.BigEndian.PutUint32(body[8:], 1)
			body = binary.BigEndian.AppendUint32(body, opcode)
			switch opcode {
			case nfs4.OP_OPEN_CONFIRM:
				body = append(body, make([]byte, 20)...)
			case nfs4.OP_RENEW:
				body = append(body, make([]byte, 8)...)
			case nfs4.OP_RELEASE_LOCKOWNER:
				body = append(body, make([]byte, 12)...)
			case nfs4.OP_SETCLIENTID_CONFIRM:
				body = append(body, make([]byte, 16)...)
			}
			wire := admissionSequence(body, ts, 0, 1, false)
			reply, res := admissionRequest(t, srv, wire, 1, nil)
			expected, disc := uint32(nfs4.NFS4ERR_NOTSUPP), opcode
			if !knownOperation(opcode) {
				expected, disc = nfs4.NFS4ERR_OP_ILLEGAL, nfs4.OP_ILLEGAL
			}
			admissionStatus(t, res, expected, nfs4.OP_SEQUENCE, disc)
			before := admissionImage(&srv.sessions.lookup(ts.id).slots[0])
			retry, res := admissionRequest(t, srv, wire, 1001, nil)
			admissionStatus(t, res, expected, nfs4.OP_SEQUENCE, disc)
			if !bytes.Equal(reply[24:], retry[24:]) || !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
				t.Fatal("mandatory error replay changed identity/prefix")
			}
		})
	}
}

// At cache limits either side of OPEN and its GETFH, OPEN runs with room for
// the handle or is refused before creating anything, and the reply replays.
func TestOpenHandleObligationAtCacheLimit(t *testing.T) {
	for _, limit := range []uint32{164, 176} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			root := t.TempDir()
			srv := admissionWritableServer(t, vfs.NewLocalTernVFS(root))
			addr, stop := serveTestServer(t, srv)
			defer stop()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			client := setupClient(t, conn, &xid)
			ts := testSessionForClient(conn, client)
			sess := srv.sessions.lookup(ts.id)
			sess.maxCached = limit
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				admissionOpen(w, client, "created")
				w.AppendArgarray_Getfh()
				emptySetattr(w)
			}), ts, 0, 1, true)
			reply, res := admissionRequest(t, srv, wire, 100, nil)
			it := res.Resarray()
			it.Next()
			it.Next()
			it.Next()
			open := it.Resarray().Value().AsOPEN4resEntry()
			if open.Disc() == nfs4.NFS4_OK {
				if !it.Next() || it.Resarray().Value().AsGETFH4resEntry().Disc() != nfs4.NFS4_OK {
					t.Fatal("OPEN lost admitted handle")
				}
			} else {
				if len(srv.stagingStore.Entries()) != 0 {
					t.Fatal("rejected OPEN installed staging")
				}
				if _, err := srv.fs.Lookup(srv.fs.RootID(), "created"); !os.IsNotExist(err) {
					t.Fatalf("rejected OPEN created file %v", err)
				}
			}
			replay, _ := admissionRequest(t, srv, wire, 101, nil)
			if !bytes.Equal(reply[24:], replay[24:]) {
				t.Fatal("OPEN prefix changed")
			}
		})
	}
}
