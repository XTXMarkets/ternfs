// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// peakVFS holds user-file ConstructFile and LinkFile calls at the TernVFS
// call boundary, before they reach the wrapped VFS, until released,
// recording how many reached that boundary at once. Client-store files pass.
// This measures nfsd's own concurrency up to the VFS; it says nothing about
// concurrency inside the storage backend, native RPCs or throughput.
type peakVFS struct {
	TernVFS
	mu      sync.Mutex
	gate    string // "construct" or "link"
	now     int
	peak    int
	release chan struct{}
}

func (fs *peakVFS) hold(kind string, dir InodeID) func() {
	if clientStoreDir(fs.TernVFS, dir) {
		return func() {}
	}
	fs.mu.Lock()
	if fs.gate != kind {
		fs.mu.Unlock()
		return func() {}
	}
	fs.now++
	fs.peak = max(fs.peak, fs.now)
	release := fs.release
	fs.mu.Unlock()
	<-release
	return func() {
		fs.mu.Lock()
		fs.now--
		fs.mu.Unlock()
	}
}

func (fs *peakVFS) ConstructFile(dir InodeID) (InodeID, Cookie, error) {
	defer fs.hold("construct", dir)()
	return fs.TernVFS.ConstructFile(dir)
}

func (fs *peakVFS) LinkFile(id InodeID, cookie Cookie, dir InodeID, name string, data io.Reader) error {
	if !strings.HasPrefix(name, "independent-") {
		return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
	}
	defer fs.hold("link", dir)()
	return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
}

func (fs *peakVFS) arm(kind string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.gate, fs.now, fs.peak, fs.release = kind, 0, 0, make(chan struct{})
}

func (fs *peakVFS) awaitPeak(t *testing.T, want int) int {
	t.Helper()
	deadline := time.Now().Add(testChannelTimeout)
	for {
		fs.mu.Lock()
		peak := fs.peak
		fs.mu.Unlock()
		if peak >= want {
			return peak
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<22)
			n := runtime.Stack(buf, true)
			t.Logf("goroutines:\n%s", buf[:n])
			return peak
		}
		time.Sleep(time.Millisecond)
	}
}

func (fs *peakVFS) open() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.gate = ""
	close(fs.release)
}

// independentOpenBody is OPEN CREATE UNCHECKED of name for write, then GETFH.
func independentOpenBody(clientID uint64, name string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientID)
		ow.Resume(ownerW.SetOwner([]byte(name)).Finish())
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		faw.Resume(bmW.Finish())
		alW := faw.StartAttrVals()
		faw.Resume(alW.SetData(nil).Finish())
		chw.Resume(faw.Finish())
		ow.Resume(chw.Finish())
		cw := ow.SetClaim_Null()
		ow.Resume(cw.SetData([]byte(name)).Finish())
		w.Resume(ow.Finish())
		w.AppendArgarray_Getfh()
	})
}

// Operations on independent files reach the VFS call boundary concurrently:
// OPENs creating different files, and CLOSEs publishing them, from one
// client over one connection on different slots, are not serialized by
// admission, the lease paths, open state or the pathname and file locks.
// The peak is measured with every call held at the boundary.
func TestIndependentFilesRunConcurrently(t *testing.T) {
	const files = 8
	fs := &peakVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	staging, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	client := newCBTestClient(t, addr)
	res := client.compound(1, exchangeIDBody("independent", [8]byte{7}))
	it := res.Resarray()
	it.Next()
	ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	clientID := ex.EirClientid()
	res = client.compound(2, backCreateSessionBody(clientID, ex.EirSequenceid(), 0))
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	it = res.Resarray()
	it.Next()
	created := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	ts := newTestSession(readSessionID(created.CsrSessionid()), clientID, created.CsrForeChanAttrs().CaMaxrequests())
	var xids sync.Mutex
	nextXID := uint32(10)
	call := func(body []byte) nfs4.COMPOUND4res {
		xids.Lock()
		nextXID++
		xid := nextXID
		xids.Unlock()
		slot, seq := ts.acquire()
		defer ts.release(slot)
		return client.compound(xid, withSequence(body, ts, slot, seq))
	}

	type opened struct {
		sid [16]byte
		fh  []byte
	}
	results := make([]opened, files)
	run := func(what string, op func(i int)) int {
		var wg sync.WaitGroup
		start := time.Now()
		for i := range files {
			wg.Add(1)
			go func() {
				defer wg.Done()
				op(i)
			}()
		}
		peak := fs.awaitPeak(t, files)
		fs.open()
		wg.Wait()
		t.Logf("%s: %d of %d reached the VFS call boundary at once, %s", what, peak, files, time.Since(start))
		return peak
	}

	fs.arm("construct")
	if peak := run("OPEN creates", func(i int) {
		res := call(independentOpenBody(clientID, fmt.Sprintf("independent-%d", i)))
		if res.Status() != nfs4.NFS4_OK {
			t.Errorf("OPEN %d %s", i, nfs4.Nfsstat4Name(res.Status()))
			return
		}
		it := res.Resarray()
		it.Next()
		it.Next()
		it.Next()
		sid := it.Resarray().Value().AsOPEN4resEntry().Value().AsOPEN4resok().Stateid()
		binary.BigEndian.PutUint32(results[i].sid[:4], sid.Seqid())
		for b := range 12 {
			results[i].sid[4+b] = sid.Other(b)
		}
		it.Next()
		results[i].fh = append([]byte(nil), it.Resarray().Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	}); peak != files {
		t.Fatalf("OPEN creates peaked at %d of %d", peak, files)
	}
	for i := range files {
		res := call(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			putfhOp(w, results[i].fh)
			ww := w.AppendArgarray_Write()
			setStateid(ww.Stateid(), results[i].sid)
			ww = ww.SetOffset(0)
			ww = ww.SetStable(fileSync4)
			ww = ww.SetData([]byte("independent data"))
			w.Resume(ww.Finish())
		}))
		if res.Status() != nfs4.NFS4_OK {
			t.Fatalf("WRITE %d %s", i, nfs4.Nfsstat4Name(res.Status()))
		}
	}
	fs.arm("link")
	if peak := run("CLOSE publications", func(i int) {
		res := call(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			putfhOp(w, results[i].fh)
			cw := w.AppendArgarray_Close()
			setStateid(cw.OpenStateid(), results[i].sid)
		}))
		if res.Status() != nfs4.NFS4_OK {
			t.Errorf("CLOSE %d %s", i, nfs4.Nfsstat4Name(res.Status()))
		}
	}); peak != files {
		t.Fatalf("CLOSE publications peaked at %d of %d", peak, files)
	}
}
