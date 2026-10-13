// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

func admissionWritableServer(t *testing.T, fs TernVFS) *Server {
	t.Helper()
	staging, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.waitForClientGC)
	t.Cleanup(srv.clients.stopLeaseWorkers)
	useTestWireEpoch(srv) // Tests also run compounds directly.
	return srv
}

func admissionLookup(w *nfs4.COMPOUND4argsWriter, name string) {
	op := w.AppendArgarray_Lookup()
	op.Resume(op.StartObjname().SetData([]byte(name)).Finish())
	w.Resume(op.Finish())
}
func admissionCreate(w *nfs4.COMPOUND4argsWriter, name string) {
	op := w.AppendArgarray_Create()
	op.SetObjtype_Nf4dir()
	op.Resume(op.StartObjname().SetData([]byte(name)).Finish())
	attrs := op.StartCreateattrs()
	mask := attrs.StartAttrmask()
	attrs.Resume(mask.Finish())
	attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
	op.Resume(attrs.Finish())
	w.Resume(op.Finish())
}
func admissionOpen(w *nfs4.COMPOUND4argsWriter, clientID uint64, name string) {
	op := w.AppendArgarray_Open()
	op.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
	op.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
	owner := op.StartOwner()
	owner.SetClientid(clientID)
	op.Resume(owner.SetOwner([]byte("response-owner")).Finish())
	create := op.SetOpenhow_Create()
	attrs := create.SetValue_Unchecked4()
	mask := attrs.StartAttrmask()
	attrs.Resume(mask.Finish())
	attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
	create.Resume(attrs.Finish())
	op.Resume(create.Finish())
	op.Resume(op.SetClaim_Null().SetData([]byte(name)).Finish())
	w.Resume(op.Finish())
}
func admissionRemove(w *nfs4.COMPOUND4argsWriter, name string) {
	op := w.AppendArgarray_Remove()
	op.Resume(op.StartTarget().SetData([]byte(name)).Finish())
	w.Resume(op.Finish())
}

func TestResponseAdmissionReadFullRPCBoundary(t *testing.T) {
	for _, cache := range []bool{false, true} {
		for _, limit := range []uint32{119, 120} {
			t.Run(fmt.Sprintf("cache=%t/limit=%d", cache, limit), func(t *testing.T) {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "file"), []byte("12345"), 0600); err != nil {
					t.Fatal(err)
				}
				srv, err := NewServer(NewLocalTernVFS(root), readOnlyStagingStore{}, nil)
				if err != nil {
					t.Fatal(err)
				}
				ts := connectionSession(t, srv)
				sess := srv.sessions.lookup(ts.id)
				if cache {
					sess.maxCached = limit
				} else {
					sess.maxResp = limit
				}
				wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
					w.AppendArgarray_Putrootfh()
					admissionLookup(w, "file")
					op := w.AppendArgarray_Read()
					op.SetCount(5)
				}), ts, 0, 1, cache)
				// Full RPC=24+COMPOUND(12)+SEQUENCE(44)+PUTROOTFH(8)+LOOKUP(8)+READ(16+8)=120.
				reply, res := admissionRequest(t, srv, wire, 1, nil)
				if limit == 119 {
					want := uint32(nfs4.NFS4ERR_REP_TOO_BIG)
					if cache {
						want = nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE
					}
					admissionStatus(t, res, want, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_LOOKUP, nfs4.OP_READ)
				} else {
					admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_LOOKUP, nfs4.OP_READ)
					if len(reply) != 120 {
						t.Fatalf("reply bytes=%d want120", len(reply))
					}
					it := res.Resarray()
					for range 4 {
						it.Next()
					}
					if string(it.Resarray().Value().AsREAD4resEntry().Value().AsREAD4resok().Data()) != "12345" {
						t.Fatal("READ payload")
					}
				}
				if len(reply) > int(limit) {
					t.Fatal("returned result exceeds full RPC budget")
				}
				replay, _ := admissionRequest(t, srv, wire, 2, nil)
				if cache && !bytes.Equal(reply[24:], replay[24:]) {
					t.Fatal("cached boundary result changed")
				}
			})
		}
	}
}

func TestResponseAdmissionMutationThenOversizedRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	srv := admissionWritableServer(t, NewLocalTernVFS(root))
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	sess.maxCached = 160
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		op := w.AppendArgarray_Rename()
		op.Resume(op.StartOldname().SetData([]byte("old")).Finish())
		op.Resume(op.StartNewname().SetData([]byte("new")).Finish())
		w.Resume(op.Finish())
		admissionLookup(w, "new")
		r := w.AppendArgarray_Read()
		r.SetCount(80 << 10)
		w.AppendArgarray_Getfh()
	}), ts, 0, 1, true)
	first, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_SAVEFH, nfs4.OP_RENAME, nfs4.OP_LOOKUP, nfs4.OP_READ)
	it := res.Resarray()
	for range 4 {
		it.Next()
	}
	if it.Resarray().Value().AsRENAME4resEntry().Disc() != nfs4.NFS4_OK {
		t.Fatal("executed RENAME rewritten")
	}
	if _, err := os.Stat(filepath.Join(root, "old")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source afterrename %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "old"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	replay, _ := admissionRequest(t, srv, wire, 2, nil)
	if !bytes.Equal(first[24:], replay[24:]) {
		t.Fatal("mutation/error prefix replay changed")
	}
	if data, err := os.ReadFile(filepath.Join(root, "old")); err != nil || string(data) != "replacement" {
		t.Fatal("replay repeated RENAME")
	}
}

func TestResponseAdmissionHandleBeforeEffect(t *testing.T) {
	for _, opnum := range []uint32{nfs4.OP_CREATE, nfs4.OP_OPEN} {
		for _, fits := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fits=%t", nfs4.NfsOpnum4Name(opnum), fits), func(t *testing.T) {
				root := t.TempDir()
				fs := NewLocalTernVFS(root)
				staging, err := NewLocalStagingStore(t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				srv, err := NewServer(fs, staging, nil)
				if err != nil {
					t.Fatal(err)
				}
				clientID, confirm, err := srv.clients.SetClientID([8]byte{}, []byte("handles"), clientOwner{})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = srv.clients.ConfirmClientID(clientID, confirm, rpcPrincipal{}); err != nil {
					t.Fatal(err)
				}
				ts := connectionSession(t, srv)
				sess := srv.sessions.lookup(ts.id)
				moveSessionForTest(srv.sessions, sess, clientID, sess.id)
				srv.clients.pinLeaseForTest(clientID)
				bound := uint32(140)
				if opnum == nfs4.OP_OPEN {
					bound = 164
				}
				sess.maxCached = bound
				if !fits {
					sess.maxCached--
				}
				wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
					w.AppendArgarray_Putrootfh()
					if opnum == nfs4.OP_CREATE {
						admissionCreate(w, "birth")
					} else {
						admissionOpen(w, clientID, "birth")
					}
					w.AppendArgarray_Getfh()
				}), ts, 0, 1, true)
				reply, res := admissionRequest(t, srv, wire, 1, nil)
				if fits {
					admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, opnum, nfs4.OP_GETFH)
					if len(reply) != int(bound) {
						t.Fatalf("actual bound %d want %d", len(reply), bound)
					}
					it := res.Resarray()
					for range 4 {
						it.Next()
					}
					fh := it.Resarray().Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()
					if len(fh) != 8 {
						t.Fatal("missing admitted handle")
					}
				} else {
					admissionStatus(t, res, nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, opnum)
					if _, err := fs.Lookup(fs.RootID(), "birth"); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("rejected operation created object: %v", err)
					}
					if len(staging.Entries()) != 0 {
						t.Fatal("rejected OPEN installed staging")
					}
				}
				replay, _ := admissionRequest(t, srv, wire, 2, nil)
				if !bytes.Equal(reply[24:], replay[24:]) {
					t.Fatal("handle admission replay changed")
				}
			})
		}
	}
}

type admissionPanicVFS struct {
	TernVFS
	calls int
}

func (fs *admissionPanicVFS) RemoveEdge(id InodeID, name string, edge Edge) error {
	err := fs.TernVFS.RemoveEdge(id, name, edge)
	fs.calls++
	if err == nil {
		panic("injected panic after REMOVE effect")
	}
	return err
}
func TestResponseAdmissionPanicRetainsTerminalAuthority(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "victim"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	fs := &admissionPanicVFS{TernVFS: NewLocalTernVFS(root)}
	srv := admissionWritableServer(t, fs)
	ts := connectionSession(t, srv)
	wire := admissionSequence(buildCompoundBody([]byte("effect"), func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		admissionRemove(w, "victim")
		w.AppendArgarray_Getfh()
	}), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, &rpcConnection{})
	admissionStatus(t, res, nfs4.NFS4ERR_SERVERFAULT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_REMOVE)
	if _, err := os.Stat(filepath.Join(root, "victim")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original effect missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "victim"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	_, res = admissionRequest(t, srv, wire, 2, &rpcConnection{})
	admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
	next := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh(); admissionRemove(w, "victim") }), ts, 0, 2, true)
	_, res = admissionRequest(t, srv, next, 3, &rpcConnection{})
	admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
	if data, err := os.ReadFile(filepath.Join(root, "victim")); err != nil || string(data) != "replacement" || fs.calls != 1 {
		t.Fatal("terminal slot permitted reexecution")
	}
	if srv.sessions.cachedBytes != 0 {
		t.Fatal("terminal session retained reply budget")
	}
	// Restart is TestRestartRetransmissionGetsBadSession's.
}

func TestResponseAdmissionAttributeAndDirectoryBudgets(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 2)
	closeSignal(fs.release)
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		op := w.AppendArgarray_Getattr()
		bits := op.StartAttrRequest()
		bits.AppendData(supportedAttrs0)
		bits.AppendData(supportedAttrs1)
		bits.AppendData(supportedAttrs2)
		op.Resume(bits.Finish())
		w.Resume(op.Finish())
	})
	args, ok := nfs4.ReadCOMPOUND4args(body)
	if !ok {
		t.Fatal("arguments")
	}
	it := args.Argarray()
	it.Next()
	it.Next()
	limit := uint32(88) + srv.operationReplyBound(it.Argarray().Value())
	sess.maxCached = limit
	wire := admissionSequence(body, ts, 0, 1, true)
	reply, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_GETATTR)
	if len(reply) != int(limit) {
		t.Fatalf("GETATTR bound %d actual %d", limit, len(reply))
	}
	sess.maxCached = 128
	wire = admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		r := w.AppendArgarray_Readdir()
		r.SetMaxcount(1 << 20)
		r.SetDircount(1 << 20)
		mask := r.StartAttrRequest()
		r.Resume(mask.Finish())
		w.Resume(r.Finish())
	}), ts, 0, 2, true)
	reply, res = admissionRequest(t, srv, wire, 2, nil)
	if len(reply) > 128 {
		t.Fatal("READDIR exceeded negotiated full RPC result size")
	}
	if res.Status() != nfs4.NFS4_OK && res.Status() != nfs4.NFS4ERR_TOOSMALL {
		t.Fatalf("READDIR %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestResponseAdmissionErrorDiscriminants(t *testing.T) {
	for _, minor := range []uint32{1, 2} {
		for _, op := range []uint32{nfs4.OP_DELEGPURGE, nfs4.OP_OPEN_CONFIRM, nfs4.OP_ALLOCATE, 9999} {
			t.Run(fmt.Sprintf("minor%d/op%d", minor, op), func(t *testing.T) {
				srv, _ := newConnectionTestServer(t, 2)
				srv.allowMinorVersion2 = true
				ts := connectionSession(t, srv)
				body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {})
				binary.BigEndian.PutUint32(body[4:], minor)
				binary.BigEndian.PutUint32(body[8:], 1)
				body = binary.BigEndian.AppendUint32(body, op)
				switch op {
				case nfs4.OP_DELEGPURGE:
					body = binary.BigEndian.AppendUint64(body, 1)
				case nfs4.OP_OPEN_CONFIRM:
					body = append(body, make([]byte, 20)...)
				case nfs4.OP_ALLOCATE:
					body = append(body, make([]byte, 32)...)
				}
				wire := admissionSequence(body, ts, 0, 1, true)
				_, res := admissionRequest(t, srv, wire, 1, nil)
				want, status := op, uint32(nfs4.NFS4ERR_NOTSUPP)
				if !knownOperation(op) {
					want, status = nfs4.OP_ILLEGAL, nfs4.NFS4ERR_OP_ILLEGAL
				}
				admissionStatus(t, res, status, nfs4.OP_SEQUENCE, want)
			})
		}
	}
	// Known ordinary operations outside a session retain their discriminator.
	srv, _ := newConnectionTestServer(t, 2)
	_, res := admissionRequest(t, srv, buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Getfh() }), 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_OP_NOT_IN_SESSION, nfs4.OP_GETFH)
}

type admissionPanicReadVFS struct{ TernVFS }

func (fs *admissionPanicReadVFS) Read(id InodeID, offset uint64, buf []byte) (int, bool, error) {
	panic("injected READ panic after prior REMOVE")
}
func TestResponseAdmissionPanicPreservesEarlierMutation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"victim", "reader"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	srv := admissionWritableServer(t, &admissionPanicReadVFS{TernVFS: NewLocalTernVFS(root)})
	ts := connectionSession(t, srv)
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		admissionRemove(w, "victim")
		admissionLookup(w, "reader")
		r := w.AppendArgarray_Read()
		r.SetCount(4)
		w.AppendArgarray_Getfh()
	}), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_SERVERFAULT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_REMOVE, nfs4.OP_LOOKUP, nfs4.OP_READ)
	it := res.Resarray()
	for range 3 {
		it.Next()
	}
	if it.Resarray().Value().AsREMOVE4resEntry().Disc() != nfs4.NFS4_OK {
		t.Fatal("panic invented failure for completed REMOVE")
	}
	if _, err := os.Stat(filepath.Join(root, "victim")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("REMOVE did not execute")
	}
	next := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 1, 1, true)
	_, res = admissionRequest(t, srv, next, 2, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
}

func TestResponseAdmissionUnknownNamespaceEffect(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprint(applied), func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "victim"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			fs := &namespaceFaultVFS{TernVFS: NewLocalTernVFS(root), err: errors.New("lost whole-call outcome"), apply: applied}
			srv := admissionWritableServer(t, fs)
			ts := connectionSession(t, srv)
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh(); admissionRemove(w, "victim") }), ts, 0, 1, false)
			_, res := admissionRequest(t, srv, wire, 1, nil)
			admissionStatus(t, res, nfs4.NFS4ERR_IO, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_REMOVE)
			_, err := os.Stat(filepath.Join(root, "victim"))
			if applied != errors.Is(err, os.ErrNotExist) {
				t.Fatalf("fault effect %t stat %v", applied, err)
			}
			if err := os.WriteFile(filepath.Join(root, "victim"), []byte("replacement"), 0600); err != nil {
				t.Fatal(err)
			}
			fs.err = nil
			fs.apply = true
			_, res = admissionRequest(t, srv, wire, 2, &rpcConnection{})
			admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
			next := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh(); admissionRemove(w, "victim") }), ts, 0, 2, true)
			_, res = admissionRequest(t, srv, next, 3, nil)
			admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
			if data, err := os.ReadFile(filepath.Join(root, "victim")); err != nil || string(data) != "replacement" {
				t.Fatal("unknown-effect retry executed again")
			}
		})
	}
}

func TestResponseAdmissionSetattrBudgetErrorCompleteXDR(t *testing.T) {
	for _, cache := range []bool{false, true} {
		t.Run(fmt.Sprint(cache), func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			sess := srv.sessions.lookup(ts.id)
			if cache {
				sess.maxCached = 92
			} else {
				sess.maxResp = 92
			}
			body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				op := w.AppendArgarray_Setattr()
				attrs := op.StartObjAttributes()
				mask := attrs.StartAttrmask()
				attrs.Resume(mask.Finish())
				attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
				op.Resume(attrs.Finish())
				w.Resume(op.Finish())
			})
			wire := admissionSequence(body, ts, 0, 1, cache)
			reply, res := admissionRequest(t, srv, wire, 1, nil)
			want := uint32(nfs4.NFS4ERR_REP_TOO_BIG)
			if cache {
				want = nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE
			}
			admissionStatus(t, res, want, nfs4.OP_SEQUENCE, nfs4.OP_SETATTR)
			it := res.Resarray()
			it.Next()
			it.Next()
			setattr := it.Resarray().Value().AsSETATTR4res()
			// The generated reader requires attrsset on every SETATTR result, including
			// admission errors. A status-only result is truncated XDR (8 instead of12).
			if setattr.Status() != want || setattr.Attrsset().Count() != 0 || len(reply) != 92 {
				t.Fatal("incomplete SETATTR admission error")
			}
			replay, res := admissionRequest(t, srv, wire, 2, nil)
			if cache {
				if !bytes.Equal(reply[24:], replay[24:]) {
					t.Fatal("cached SETATTR error changed")
				}
			} else {
				admissionStatus(t, res, nfs4.NFS4ERR_RETRY_UNCACHED_REP, nfs4.OP_SEQUENCE, nfs4.OP_SETATTR)
			}
		})
	}
}

type admissionLinkFaultVFS struct {
	TernVFS
	fail, apply bool
	submissions int
}

func (fs *admissionLinkFaultVFS) LinkFile(id InodeID, cookie Cookie, dir InodeID, name string, data io.Reader) error {
	if !fs.fail || data == nil || leaseTemp(name) {
		return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
	}
	fs.submissions++
	if fs.apply {
		if err := fs.TernVFS.LinkFile(id, cookie, dir, name, data); err != nil {
			return err
		}
	}
	return errors.New("unknown whole-call publication outcome")
}
func TestResponseAdmissionUnknownCloseEffect(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprint(applied), func(t *testing.T) {
			root := t.TempDir()
			fs := &admissionLinkFaultVFS{TernVFS: NewLocalTernVFS(root), apply: applied}
			srv := admissionWritableServer(t, fs)
			addr, stop := serveTestServer(t, srv)
			defer stop()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			client := setupClient(t, conn, &xid)
			sid, fh := openCreateFile(t, conn, &xid, client, "victim")
			writeFileAt(t, conn, &xid, fh, sid, 0, []byte("pending"))
			ts := testSessionForClient(conn, client)
			slot, seq := ts.acquire()
			defer ts.release(slot)
			body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				p := w.AppendArgarray_Putfh()
				p.Resume(p.StartObject().SetData(fh).Finish())
				w.Resume(p.Finish())
				c := w.AppendArgarray_Close()
				setStateid(c.OpenStateid(), sid)
			})
			wire := admissionSequence(body, ts, slot, seq, true)
			fs.fail = true
			send := func() nfs4.COMPOUND4res {
				t.Helper()
				reply, err := sendRawRPC(conn, xid, procCompound, wire)
				xid++
				if err != nil {
					t.Fatal(err)
				}
				res, err := parseCompoundReply(reply)
				if err != nil {
					t.Fatal(err)
				}
				return res
			}
			admissionStatus(t, send(), nfs4.NFS4ERR_IO, nfs4.OP_SEQUENCE, nfs4.OP_PUTFH, nfs4.OP_CLOSE)
			content, err := os.ReadFile(filepath.Join(root, "victim"))
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if applied {
				want = "pending"
			}
			if string(content) != want {
				t.Fatalf("applied=%t data=%q", applied, content)
			}
			if err := os.WriteFile(filepath.Join(root, "victim"), []byte("replacement"), 0600); err != nil {
				t.Fatal(err)
			}
			admissionStatus(t, send(), nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
			if content, err := os.ReadFile(filepath.Join(root, "victim")); err != nil || string(content) != "replacement" || fs.submissions != 1 {
				t.Fatal("unknown CLOSE reexecuted")
			}
		})
	}
}
