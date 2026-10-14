// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// pagedVFS serves a large directory in MTU-bounded pages, like
// RemoteTernVFS: each page decodes fresh names, and every entry has a name of
// the maximum length.
type pagedVFS struct {
	vfs.TernVFS
	entries int
	sample  func()
	stats   atomic.Int32
}

func (fs *pagedVFS) Readdir(dir vfs.InodeID, start uint64) ([]vfs.DirEntry, uint64, error) {
	if dir != fs.RootID() {
		return fs.TernVFS.Readdir(dir, start)
	}
	fs.sample()
	per := msgs.MAX_UDP_MTU / (8 + 8 + 1 + maxTernNameLength + 8)
	first := max(int(start), 3)
	var page []vfs.DirEntry
	for hash := first; hash < first+per && hash < fs.entries+3; hash++ {
		name := fmt.Sprintf("%0*d", maxTernNameLength, hash)
		page = append(page, vfs.DirEntry{Name: name, ID: vfs.MakeInodeID(vfs.InodeTypeFile, uint64(hash)), NameHash: uint64(hash)})
	}
	next := uint64(first + per)
	if int(next) >= fs.entries+3 {
		next = 0
	}
	return page, next, nil
}

func (fs *pagedVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	if id.Type() != vfs.InodeTypeFile {
		return fs.TernVFS.Stat(id)
	}
	if fs.stats.Add(1)%16 == 0 {
		fs.sample()
	}
	return vfs.NodeInfo{Size: uint64(id)}, nil
}

// READDIR of a large directory prepares its page within the work it reserved,
// returns the largest page that fits, and continues from its last cookie.
func TestReaddirPreparationWithinReservation(t *testing.T) {
	fs := &pagedVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), entries: 12000}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	sess.maxResp = 1 << 20
	readdir := func(cookie uint64) func(w *nfs4.COMPOUND4argsWriter) {
		return func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
			r := w.AppendArgarray_Readdir()
			r.SetCookie(cookie)
			r.SetMaxcount(1 << 20)
			r.SetDircount(1 << 20)
			bits := r.StartAttrRequest()
			bits.AppendData(supportedAttrs0)
			bits.AppendData(supportedAttrs1)
			bits.AppendData(supportedAttrs2)
			r.Resume(bits.Finish())
			w.Resume(r.Finish())
		}
	}
	wire := admissionSequence(buildCompoundBody(nil, readdir(0)), ts, 0, 1, false)
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, wire))
	if err != nil {
		t.Fatal(err)
	}
	if !req.acquireReplyWork(srv) {
		t.Fatal("work admission")
	}
	var stats runtime.MemStats
	live := func() int {
		runtime.GC()
		runtime.ReadMemStats(&stats)
		return int(stats.HeapAlloc)
	}
	// Peak retained heap during preparation, against the reservation the call
	// actually holds. Samples follow a full collection, so only reachable
	// allocations count.
	base, peak := live(), 0
	fs.sample = func() { peak = max(peak, live()-base) }
	reply := srv.safeHandleCompound(req, "readdir-peak")
	work := req.replyWork.charge
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	reply = nil
	req.releaseReply()
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_READDIR)
	if peak == 0 || peak > work {
		t.Fatalf("READDIR preparation retained %d bytes, reservation %d", peak, work)
	}
	// The READDIR-specific obligation is the writer plus its explicit allowance;
	// it does not rely on the multiplier's general slack.
	if allowance := int(sess.maxResp) + readdirPrepareBytes; peak > allowance+allowance/8 {
		t.Fatalf("READDIR preparation retained %d bytes, allowance %d", peak, allowance)
	}
	t.Logf("peak %d, writer+allowance %d, reservation %d", peak, int(sess.maxResp)+readdirPrepareBytes, work)

	// The streamed page is the exact largest prefix and continues correctly.
	it := res.Resarray()
	it.Next()
	it.Next()
	it.Next()
	ok := it.Resarray().Value().AsREADDIR4resEntry().Value().AsREADDIR4resok()
	entrySize := 4 + 8 + 4 + maxTernNameLength + 1 + 16 + 4 + 312 + 4
	want := (min(1<<20, int(sess.maxResp)-rpcAcceptedHeaderSize-8-60-8-8) - readdirResokOverhead) / entrySize
	got, last := 0, uint64(0)
	if ok.Reply().Eof() != nfs4.FALSE {
		t.Fatal("partial page reported EOF")
	}
	for e := ok.Reply().Entries(); e.Disc() == nfs4.TRUE; {
		entry := e.AsEntry4()
		if entry.Cookie() != uint64(got+3) || len(entry.Name().Data()) != maxTernNameLength {
			t.Fatalf("entry %d cookie %d", got, entry.Cookie())
		}
		last = entry.Cookie()
		got++
		e = entry.Nextentry()
	}
	if got < want-1 || got > want {
		t.Fatalf("page has %d entries, want about %d", got, want)
	}
	wire = admissionSequence(buildCompoundBody(nil, readdir(last)), ts, 0, 2, false)
	_, res = admissionRequest(t, srv, wire, 2, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_READDIR)
	it = res.Resarray()
	it.Next()
	it.Next()
	it.Next()
	ok = it.Resarray().Value().AsREADDIR4resEntry().Value().AsREADDIR4resok()
	if e := ok.Reply().Entries(); e.Disc() != nfs4.TRUE || e.AsEntry4().Cookie() != last+1 {
		t.Fatal("continuation did not resume after the last cookie")
	}
	if a := srv.sessions.accounting(); a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("READDIR work not released %+v", a)
	}
}
