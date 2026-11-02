// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

var errInjectedReader = errors.New("injected reader-source fault")

// A READ that initially selected an immutable file can find a staging
// after another OPEN upgrades the same owner. Both anonymous I/O and a
// stateid with seqid zero must resolve that new source through staging.
func TestReadAcrossWriterUpgrade(t *testing.T) {
	for _, anonymous := range []bool{false, true} {
		t.Run(fmt.Sprintf("anonymous=%t", anonymous), func(t *testing.T) {
			gate := &getGateStore{entered: make(chan struct{}), release: make(chan struct{})}
			f := newPublishFixtureWithStore(t, func(store staging.Store) staging.Store {
				gate.Store = store
				return gate
			})
			writer, fh, _ := f.writer(t, "doc", "saved")
			closeFile(t, f.conn, f.xid, fh, writer)
			fh = lookupFH(t, f.conn, f.xid, "doc")
			status, reader, _ := openByHandle(t, f.conn, f.xid, f.clientID, "reader", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
			if status != nfs4.NFS4_OK {
				t.Fatal(nfs4.Nfsstat4Name(status))
			}
			binary.BigEndian.PutUint32(reader[:4], 0)
			if anonymous {
				reader = [16]byte{}
			}
			id, _ := vfs.FHToInodeID(fh)
			gate.mu.Lock()
			gate.armed = id
			gate.mu.Unlock()
			conn := dial(t, f.addr)
			defer conn.Close()
			xid := uint32(9000)
			setupClient(t, conn, &xid)
			defer testutil.CloseSignal(gate.release)
			type result struct {
				status uint32
				data   string
			}
			read := make(chan result, 1)
			go func() {
				status, data := readStateid(t, conn, &xid, fh, reader)
				read <- result{status, data}
			}()
			testutil.AwaitSignal(t, gate.entered, "READ after choosing immutable version")
			if status, _, _ := openByHandle(t, f.conn, f.xid, f.clientID, "reader", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE); status != nfs4.NFS4_OK {
				t.Fatal(nfs4.Nfsstat4Name(status))
			}
			testutil.CloseSignal(gate.release)
			got := testutil.AwaitValue(t, read, "READ")
			if got.status != nfs4.NFS4_OK || got.data != "saved" {
				t.Fatalf("READ across upgrade: %s %q, want saved", nfs4.Nfsstat4Name(got.status), got.data)
			}
		})
	}
}

// readerSourceVFS fails LinkFile of a name and fails reads of one inode.
type readerSourceVFS struct {
	vfs.TernVFS
	mu       sync.Mutex
	failLink string
	failRead vfs.InodeID
}

func (fs *readerSourceVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	fs.mu.Lock()
	fail := name != "" && name == fs.failLink
	fs.mu.Unlock()
	if fail {
		return errInjectedReader
	}
	return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
}

func (fs *readerSourceVFS) Read(id vfs.InodeID, offset uint64, dest []byte) (int, bool, error) {
	fs.mu.Lock()
	fail := id != 0 && id == fs.failRead
	fs.mu.Unlock()
	if fail {
		return 0, false, errInjectedReader
	}
	return fs.TernVFS.Read(id, offset, dest)
}

type readerFixture struct {
	srv      *Server
	fs       *readerSourceVFS
	conn     net.Conn
	xid      uint32
	clientID uint64
	fileID   vfs.InodeID
	fh       []byte
}

func newReaderFixture(t *testing.T, content string) *readerFixture {
	t.Helper()
	fs := &readerSourceVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.StopLeaseWorkers)
	addr, stop := serveTestServer(t, srv)
	t.Cleanup(stop)
	f := &readerFixture{srv: srv, fs: fs, conn: dial(t, addr), xid: 1}
	t.Cleanup(func() { f.conn.Close() })
	f.clientID = setupClient(t, f.conn, &f.xid)
	sid, fh := openCreateFile(t, f.conn, &f.xid, f.clientID, "f")
	writeFileAt(t, f.conn, &f.xid, fh, sid, 0, []byte(content))
	closeFile(t, f.conn, &f.xid, fh, sid)
	f.fh = lookupFH(t, f.conn, &f.xid, "f")
	f.fileID, _ = vfs.FHToInodeID(f.fh)
	return f
}

// openReader opens "f" for READ as owner "r", by handle or by name.
func (f *readerFixture) openReader(t *testing.T, claimFH bool) [16]byte {
	t.Helper()
	if claimFH {
		status, sid, _ := openByHandle(t, f.conn, &f.xid, f.clientID, "r", f.fh, nfs4.OPEN4_SHARE_ACCESS_READ)
		if status != nfs4.NFS4_OK {
			t.Fatalf("reader OPEN by handle %s", nfs4.Nfsstat4Name(status))
		}
		return sid
	}
	status, sid, _, _ := openFileForOwner(t, f.conn, &f.xid, f.clientID, "r", "f", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("reader OPEN by name %s", nfs4.Nfsstat4Name(status))
	}
	return sid
}

func (f *readerFixture) openWriter(t *testing.T) [16]byte {
	t.Helper()
	status, sid, _ := openByHandle(t, f.conn, &f.xid, f.clientID, "w", f.fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("writer OPEN %s", nfs4.Nfsstat4Name(status))
	}
	return sid
}

// read reads through sid and returns the status and data.
func (f *readerFixture) read(t *testing.T, sid [16]byte) (uint32, string) {
	t.Helper()
	return readStateid(t, f.conn, &f.xid, f.fh, sid)
}

func (f *readerFixture) dataID(t *testing.T, sid [16]byte) vfs.InodeID {
	t.Helper()
	var id nfs4.StateID
	copy(id[:], sid[4:])
	state, status := f.srv.opens.Lookup(incarnationForTest(t, f.srv, f.clientID), id, 0, f.fileID)
	if status != nfs4.NFS4_OK {
		t.Fatalf("reader state %s", nfs4.Nfsstat4Name(status))
	}
	return state.DataID
}

// Another owner's read OPEN sees the client's writer's staging; a clean
// writer CLOSE then leaves the reader on the published file, not NOENT
// (../openstate/README.md#reader-visibility; the conversion regression).
func TestAssessmentReadSurvivesCleanWriterClose(t *testing.T) {
	for _, claimFH := range []bool{true, false} {
		name := "CLAIM_NULL"
		if claimFH {
			name = "CLAIM_FH"
		}
		t.Run(name, func(t *testing.T) {
			f := newReaderFixture(t, "original")
			writer := f.openWriter(t)
			reader := f.openReader(t, claimFH)
			if status, data := f.read(t, reader); status != nfs4.NFS4_OK || data != "original" {
				t.Fatalf("reader before CLOSE %s %q", nfs4.Nfsstat4Name(status), data)
			}
			closeFile(t, f.conn, &f.xid, f.fh, writer)
			if status, data := f.read(t, reader); status != nfs4.NFS4_OK || data != "original" {
				t.Fatalf("reader after clean CLOSE %s %q", nfs4.Nfsstat4Name(status), data)
			}
			if got := f.dataID(t, reader); got != f.fileID {
				t.Fatalf("reader source %d, want the published %d", got, f.fileID)
			}
			closeFile(t, f.conn, &f.xid, f.fh, reader)
		})
	}
}

// A timestamp-only CLOSE also returns the reader to the published file.
func TestReaderSurvivesTimestampOnlyClose(t *testing.T) {
	f := newReaderFixture(t, "original")
	writer := f.openWriter(t)
	reader := f.openReader(t, true)
	res := sendCompound(t, f.conn, f.xid, func(w *nfs4.COMPOUND4argsWriter) {
		putfhOp(w, f.fh)
		sw := w.AppendArgarray_Setattr()
		setStateid(sw.Stateid(), writer)
		aw := sw.StartObjAttributes()
		value := binary.BigEndian.AppendUint32(nil, nfs4.SET_TO_CLIENT_TIME4)
		value = binary.BigEndian.AppendUint64(value, 1_000_000)
		value = binary.BigEndian.AppendUint32(value, 0)
		sw.Resume(finishTestFattr(aw, attrMask{0, 1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32)}, value))
		w.Resume(sw.Finish())
	})
	f.xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SETATTR mtime %s", nfs4.Nfsstat4Name(res.Status()))
	}
	closeFile(t, f.conn, &f.xid, f.fh, writer)
	if status, data := f.read(t, reader); status != nfs4.NFS4_OK || data != "original" {
		t.Fatalf("reader after timestamp-only CLOSE %s %q", nfs4.Nfsstat4Name(status), data)
	}
	if got := f.dataID(t, reader); got != f.fileID {
		t.Fatalf("reader source %d, want %d", got, f.fileID)
	}
	info, err := f.fs.Stat(f.fileID)
	if err != nil || info.Mtime.Unix() != 1_000_000 {
		t.Fatalf("published mtime %v %v", info.Mtime, err)
	}
}

// After a publishing CLOSE, the reader reads what was published.
func TestReaderReadsPublishedVersion(t *testing.T) {
	f := newReaderFixture(t, "original")
	writer := f.openWriter(t)
	reader := f.openReader(t, true)
	writeFileAt(t, f.conn, &f.xid, f.fh, writer, 0, []byte("CHANGED!"))
	closeFile(t, f.conn, &f.xid, f.fh, writer)
	if status, data := f.read(t, reader); status != nfs4.NFS4_OK || data != "CHANGED!" {
		t.Fatalf("reader after publishing CLOSE %s %q", nfs4.Nfsstat4Name(status), data)
	}
}

// A failed publication leaves the reader with IO, never the old bytes and
// never NOENT: a LinkFile error of unknown outcome, or a hydration failure.
func TestReaderGetsIOAfterFailedPublication(t *testing.T) {
	t.Run("link", func(t *testing.T) {
		f := newReaderFixture(t, "original")
		writer := f.openWriter(t)
		reader := f.openReader(t, true)
		writeFileAt(t, f.conn, &f.xid, f.fh, writer, 0, []byte("CHANGED!"))
		f.fs.mu.Lock()
		f.fs.failLink = "f"
		f.fs.mu.Unlock()
		if status := closeFileStatus(t, f.conn, &f.xid, f.fh, writer); status == nfs4.NFS4_OK {
			t.Fatal("CLOSE succeeded despite the LinkFile error")
		}
		if status, data := f.read(t, reader); status != nfs4.NFS4ERR_IO {
			t.Fatalf("reader after LinkFile error %s %q, want IO", nfs4.Nfsstat4Name(status), data)
		}
	})
	t.Run("hydration", func(t *testing.T) {
		f := newReaderFixture(t, "original content")
		writer := f.openWriter(t)
		reader := f.openReader(t, true)
		// The WRITE starts hydration in the background; reading the base
		// fails from before it.
		f.fs.mu.Lock()
		f.fs.failRead = f.fileID
		f.fs.mu.Unlock()
		writeFileAt(t, f.conn, &f.xid, f.fh, writer, 0, []byte("X"))
		if status := closeFileStatus(t, f.conn, &f.xid, f.fh, writer); status == nfs4.NFS4_OK {
			t.Fatal("CLOSE succeeded despite the hydration failure")
		}
		f.fs.mu.Lock()
		f.fs.failRead = 0
		f.fs.mu.Unlock()
		if status, data := f.read(t, reader); status != nfs4.NFS4ERR_IO {
			t.Fatalf("reader after hydration failure %s %q, want IO", nfs4.Nfsstat4Name(status), data)
		}
	})
}

// A read OPEN that chose the writer's staging and then waited behind a
// clean CLOSE selects again under the file lock and ends on the file.
func TestReadOpenBehindCleanCloseEndsOnFile(t *testing.T) {
	f := newReaderFixture(t, "original")
	writer := f.openWriter(t)
	// The OPEN goes on another connection of the client: two requests in
	// flight on one connection would share its replies.
	conn := dial(t, addrOfTestServer(t, f.srv))
	defer conn.Close()
	xid := uint32(6000)
	setupClient(t, conn, &xid)
	// The CLOSE, its compound's last operation, keeps its locks until its
	// reply is recorded on the slot: hold it there.
	recording, hold := holdRecord(f.srv)
	defer testutil.CloseSignal(hold)
	closed := make(chan uint32, 1)
	go func() {
		xid := uint32(5000)
		closed <- closeFileStatus(t, f.conn, &xid, f.fh, writer)
	}()
	testutil.AwaitSignal(t, recording, "clean CLOSE recording its reply")
	opened := make(chan [16]byte, 1)
	go func() {
		status, sid, _ := openByHandle(t, conn, &xid, f.clientID, "r", f.fh, nfs4.OPEN4_SHARE_ACCESS_READ)
		if status != nfs4.NFS4_OK {
			t.Errorf("reader OPEN %s", nfs4.Nfsstat4Name(status))
		}
		opened <- sid
	}()
	select {
	case <-opened:
		t.Fatal("read OPEN did not wait for the CLOSE")
	case <-time.After(50 * time.Millisecond):
	}
	testutil.CloseSignal(hold)
	if status := testutil.AwaitValue(t, closed, "clean CLOSE"); status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE %s", nfs4.Nfsstat4Name(status))
	}
	reader := testutil.AwaitValue(t, opened, "read OPEN")
	if got := f.dataID(t, reader); got != f.fileID {
		t.Fatalf("reader source %d, want %d", got, f.fileID)
	}
	if status, data := f.read(t, reader); status != nfs4.NFS4_OK || data != "original" {
		t.Fatalf("reader %s %q", nfs4.Nfsstat4Name(status), data)
	}
}
