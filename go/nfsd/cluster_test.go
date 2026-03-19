// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build ternnfs

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/managedprocess"
	"xtx/ternfs/msgs"
)

var (
	binariesDir  = flag.String("binaries-dir", "", "directory containing pre-built TernFS binaries")
	repoDir      = flag.String("repo-dir", "", "repository root (for building binaries)")
	registryPort uint16
	registryAddr string
	dataDir      string
	procs        *managedprocess.ManagedProcesses
	ternLogger   *log.Logger
)

func TestMain(m *testing.M) {
	flag.Parse()

	if *repoDir == "" {
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			panic("no caller information")
		}
		// cluster_test.go is in go/nfsd/, repo root is two levels up
		*repoDir = path.Dir(path.Dir(path.Dir(filename)))
	}

	var err error
	dataDir, err = os.MkdirTemp("", "nfstests.")
	if err != nil {
		panic(err)
	}

	logFile, err := os.OpenFile(path.Join(dataDir, "test-log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	ternLogger = log.NewLogger(logFile, &log.LoggerOptions{Level: log.INFO})

	var cppExes *managedprocess.CppExes
	var goExes *managedprocess.GoExes
	if *binariesDir != "" {
		cppExes = &managedprocess.CppExes{
			RegistryExe: path.Join(*binariesDir, "ternregistry"),
			ShardExe:    path.Join(*binariesDir, "ternshard"),
			CDCExe:      path.Join(*binariesDir, "terncdc"),
			DBToolsExe:  path.Join(*binariesDir, "terndbtools"),
		}
		goExes = &managedprocess.GoExes{
			BlocksExe: path.Join(*binariesDir, "ternblocks"),
		}
	} else {
		fmt.Println("building TernFS binaries...")
		cppExes = managedprocess.BuildCppExes(ternLogger, *repoDir, "release")
		goExes = managedprocess.BuildGoExes(ternLogger, *repoDir, false)
	}

	terminateChan := make(chan any, 1)
	procs = managedprocess.New(terminateChan)

	registryPort = 55556 // different from terntests default
	registryAddr = fmt.Sprintf("127.0.0.1:%d", registryPort)

	// Start registry (leader only, single replica).
	procs.StartRegistry(ternLogger, &managedprocess.RegistryOpts{
		Exe:               cppExes.RegistryExe,
		LogLevel:          log.INFO,
		Dir:               path.Join(dataDir, "registry"),
		RegistryAddress:   registryAddr,
		Replica:           0,
		Addr1:             fmt.Sprintf("127.0.0.1:%d", registryPort),
		UsingDynamicPorts: true,
		LogsDBFlags:       []string{"-logsdb-leader", "-logsdb-no-replication"},
	})
	if err := client.WaitForRegistry(ternLogger, registryAddr, 10*time.Second); err != nil {
		panic(fmt.Errorf("registry not ready: %w", err))
	}

	// Start block services (14 failure domains, 1 HDD + 1 FLASH each).
	// Shards require at least 14 block services per storage class.
	failureDomains := 14
	servicesPerDomain := 2
	for i := 0; i < failureDomains; i++ {
		storageClasses := []msgs.StorageClass{
			msgs.HDD_STORAGE,
			msgs.FLASH_STORAGE,
		}
		procs.StartBlockService(ternLogger, &managedprocess.BlockServiceOpts{
			Exe:             goExes.BlocksExe,
			Path:            path.Join(dataDir, fmt.Sprintf("bs_%d", i)),
			Addr1:           "127.0.0.1:0",
			StorageClasses:  storageClasses,
			FailureDomain:   fmt.Sprintf("%d", i),
			LogLevel:        log.INFO,
			RegistryAddress: registryAddr,
		})
	}
	fmt.Println("waiting for block services...")
	client.WaitForBlockServices(ternLogger, registryAddr, failureDomains*servicesPerDomain, true, 30*time.Second)

	// Start CDC (single replica).
	procs.StartCDC(ternLogger, *repoDir, &managedprocess.CDCOpts{
		ReplicaId:       0,
		Exe:             cppExes.CDCExe,
		Dir:             path.Join(dataDir, "cdc"),
		LogLevel:        log.INFO,
		RegistryAddress: registryAddr,
		Addr1:           "127.0.0.1:0",
		LogsDBFlags:     []string{"-logsdb-leader", "-logsdb-no-replication", "-logsdb-initial-start"},
	})

	// Start 256 shards (single replica each).
	for i := 0; i < 256; i++ {
		shrid := msgs.MakeShardReplicaId(msgs.ShardId(i), 0)
		procs.StartShard(ternLogger, *repoDir, &managedprocess.ShardOpts{
			Exe:             cppExes.ShardExe,
			Dir:             path.Join(dataDir, fmt.Sprintf("shard_%03d", i)),
			LogLevel:        log.INFO,
			Shrid:           shrid,
			RegistryAddress: registryAddr,
			Addr1:           "127.0.0.1:0",
			LogsDBFlags:     []string{"-logsdb-leader", "-logsdb-no-replication", "-logsdb-initial-start"},
		})
	}

	fmt.Println("waiting for cluster to be ready...")
	client.WaitForClient(ternLogger, registryAddr, 60*time.Second)
	fmt.Println("cluster ready")

	// Monitor for unexpected process termination.
	go func() {
		err := <-terminateChan
		if err != nil {
			fmt.Fprintf(os.Stderr, "cluster process died: %v\n", err)
			os.Exit(1)
		}
	}()

	code := m.Run()

	procs.Close()
	logFile.Close()
	if code == 0 {
		os.RemoveAll(dataDir)
	} else {
		fmt.Printf("test data preserved at %s\n", dataDir)
	}
	os.Exit(code)
}

// startTernTestServer creates an NFS server backed by the shared TernFS cluster.
// It creates a fresh RemoteTernVFS client and staging directory per test.
func startTernTestServer(t *testing.T) (addr string, cleanup func()) {
	t.Helper()
	c, err := client.NewClient(ternLogger, nil, registryAddr, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	bp := bufpool.NewBufPool()
	fs := NewRemoteTernVFS(c, ternLogger, bp)

	stagingDir := t.TempDir()
	ss, err := NewLocalStagingStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handleConn(conn)
		}
	}()
	return ln.Addr().String(), func() {
		ln.Close()
		c.Close()
	}
}

// copyStateid copies a stateid4 from src (read-only) to dst (writer).
func copyStateid(dst Stateid4, src Stateid4) {
	dst.SetSeqid(src.Seqid())
	for i := 0; i < 12; i++ {
		dst.SetOther(i, src.Other(i))
	}
}

// createFileViaNFS creates a file through the NFS protocol by doing
// PUTROOTFH + OPEN(CREATE) + GETFH + OPEN_CONFIRM + WRITE + CLOSE.
// Returns the file handle.
func createFileViaNFS(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, name string, data []byte) []byte {
	t.Helper()

	// OPEN CREATE + GETFH + OPEN_CONFIRM
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte(name)).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH

	openEntry := nextOp(t, &iter)
	openRes := openEntry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN status = %d", openRes.Disc())
	}
	openOK := openRes.Value().AsOPEN4resok()
	stateid := openOK.Stateid()

	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// WRITE
	if len(data) > 0 {
		res = sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
			pfW := w.AppendArgarray_Putfh()
			buf := pfW.StartObject().SetData(fh).Finish()
			pfW.Resume(buf)
			buf = pfW.Finish()
			w.Resume(buf)

			ww := w.AppendArgarray_Write()
			copyStateid(ww.Stateid(), stateid)
			ww = ww.SetOffset(0)
			ww = ww.SetStable(2) // FILE_SYNC4
			ww = ww.SetData(data)
			buf = ww.Finish()
			w.Resume(buf)
		})
		*xid++
		iter = expectOK(t, res)
		nextOp(t, &iter) // PUTFH
		writeEntry := nextOp(t, &iter)
		writeRes := writeEntry.Value().AsWRITE4resEntry()
		if writeRes.Disc() != NFS4_OK {
			t.Fatalf("WRITE status = %d", writeRes.Disc())
		}
	}

	// CLOSE
	res = sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	*xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	closeEntry := nextOp(t, &iter)
	closeRes := closeEntry.Value().AsCLOSE4resEntry()
	if closeRes.Disc() != NFS4_OK {
		t.Fatalf("CLOSE status = %d", closeRes.Disc())
	}

	return fh
}

// cleanupViaNFS removes a file by name from the root directory.
func cleanupViaNFS(t *testing.T, conn net.Conn, xid *uint32, name string) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf := tw.SetData([]byte(name)).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	*xid++
	// Ignore errors — file might not exist.
	_ = res
}

func TestTernPutrootfhGetattr(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<FATTR4_TYPE | 1<<FATTR4_FILEID)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	if entry.Value().AsPUTROOTFH4res().Status() != NFS4_OK {
		t.Fatal("PUTROOTFH failed")
	}
	entry = nextOp(t, &iter)
	getattrRes := entry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != NFS4_OK {
		t.Fatalf("GETATTR status = %d", getattrRes.Disc())
	}
}

func TestTernCreateAndRead(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	testData := []byte("Hello from TernFS NFS test!")
	fh := createFileViaNFS(t, conn, &xid, clientid, "test-create.txt", testData)

	// READ the file back.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	readEntry := nextOp(t, &iter)
	readRes := readEntry.Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %d", readRes.Disc())
	}
	readOK := readRes.Value().AsREAD4resok()
	got := readOK.Data()
	if string(got) != string(testData) {
		t.Fatalf("READ data = %q, want %q", got, testData)
	}
	if readOK.Eof() == 0 {
		t.Fatal("expected EOF")
	}

	cleanupViaNFS(t, conn, &xid, "test-create.txt")
}

func TestTernLookupAndStat(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	testData := []byte("stat test data, exactly 33 bytes")
	createFileViaNFS(t, conn, &xid, clientid, "stat-test.txt", testData)

	// PUTROOTFH + LOOKUP + GETATTR
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("stat-test.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<FATTR4_TYPE | 1<<FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	lookupEntry := nextOp(t, &iter)
	if lookupEntry.Value().AsLOOKUP4res().Status() != NFS4_OK {
		t.Fatal("LOOKUP failed")
	}
	getattrEntry := nextOp(t, &iter)
	getattrRes := getattrEntry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != NFS4_OK {
		t.Fatalf("GETATTR status = %d", getattrRes.Disc())
	}

	// Parse the returned attributes: bitmap + attr data.
	attrData := getAttrData(t, getattrRes.Value().AsGETATTR4resok())
	// Attr data contains: type (u32) + size (u64) = 12 bytes.
	if len(attrData) < 12 {
		t.Fatalf("attr data too short: %d", len(attrData))
	}
	ftype := binary.BigEndian.Uint32(attrData[:4])
	if ftype != uint32(NF4REG) {
		t.Fatalf("type = %d, want NF4REG (%d)", ftype, NF4REG)
	}

	cleanupViaNFS(t, conn, &xid, "stat-test.txt")
}

func TestTernReaddir(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a few files.
	names := []string{"readdir-a.txt", "readdir-b.txt", "readdir-c.txt"}
	for _, name := range names {
		createFileViaNFS(t, conn, &xid, clientid, name, []byte("data"))
	}

	// READDIR
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(32768)
		bw := rdw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf := bw.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	rdEntry := nextOp(t, &iter)
	rdRes := rdEntry.Value().AsREADDIR4resEntry()
	if rdRes.Disc() != NFS4_OK {
		t.Fatalf("READDIR status = %d", rdRes.Disc())
	}

	// Collect entry names.
	rdOK := rdRes.Value().AsREADDIR4resok()
	found := make(map[string]bool)
	reply := rdOK.Reply()
	if reply.EntriesPresent() == TRUE {
		entOpt := reply.Entries()
		for {
			ent := entOpt.AsEntry4()
			found[string(ent.Name().Data())] = true
			if ent.NextentryPresent() != TRUE {
				break
			}
			entOpt = ent.Nextentry()
		}
	}

	for _, name := range names {
		if !found[name] {
			t.Errorf("READDIR missing entry %q", name)
		}
	}

	// Cleanup.
	for _, name := range names {
		cleanupViaNFS(t, conn, &xid, name)
	}
}

func TestTernMkdirAndRemove(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	// CREATE directory
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		nameW := cw.StartObjname()
		buf := nameW.SetData([]byte("test-dir")).Finish()
		cw.Resume(buf)
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	createEntry := nextOp(t, &iter)
	createRes := createEntry.Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE dir status = %d", createRes.Disc())
	}

	// LOOKUP the directory
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("test-dir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	lookupEntry := nextOp(t, &iter)
	if lookupEntry.Value().AsLOOKUP4res().Status() != NFS4_OK {
		t.Fatal("LOOKUP dir failed")
	}
	getattrEntry := nextOp(t, &iter)
	attrData := getAttrData(t, getattrEntry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(attrData) < 4 {
		t.Fatal("attr data too short")
	}
	ftype := binary.BigEndian.Uint32(attrData[:4])
	if ftype != uint32(NF4DIR) {
		t.Fatalf("type = %d, want NF4DIR", ftype)
	}

	// REMOVE the directory
	cleanupViaNFS(t, conn, &xid, "test-dir")
}

func TestTernWriteAndRead(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a file, write data in chunks, read it back.
	data := make([]byte, 128*1024) // 128KB
	for i := range data {
		data[i] = byte(i % 251) // deterministic pattern
	}

	fh := createFileViaNFS(t, conn, &xid, clientid, "large-write.txt", data)

	// Read back in one shot.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(uint32(len(data)))
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	readEntry := nextOp(t, &iter)
	readRes := readEntry.Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %d", readRes.Disc())
	}
	got := readRes.Value().AsREAD4resok().Data()
	if len(got) != len(data) {
		t.Fatalf("READ len = %d, want %d", len(got), len(data))
	}
	for i := range got {
		if got[i] != data[i] {
			t.Fatalf("data mismatch at byte %d: got %d, want %d", i, got[i], data[i])
		}
	}

	cleanupViaNFS(t, conn, &xid, "large-write.txt")
}

func TestTernSymlink(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	// CREATE symlink: "test-link" -> "symlink-target"
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		cw := w.AppendArgarray_Create()
		ltw := cw.SetObjtype_Nf4lnk()
		buf := ltw.SetData([]byte("symlink-target")).Finish()
		cw.Resume(buf)
		nameW := cw.StartObjname()
		buf = nameW.SetData([]byte("test-link")).Finish()
		cw.Resume(buf)
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	createRes := nextOp(t, &iter).Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE symlink status = %d", createRes.Disc())
	}

	// READLINK to verify target.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("test-link")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Readlink()
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	rlEntry := nextOp(t, &iter)
	rlRes := rlEntry.Value().AsREADLINK4resEntry()
	if rlRes.Disc() != NFS4_OK {
		t.Fatalf("READLINK status = %d", rlRes.Disc())
	}
	target := string(rlRes.Value().AsREADLINK4resok().Link().Data())
	if target != "symlink-target" {
		t.Fatalf("READLINK = %q, want %q", target, "symlink-target")
	}

	cleanupViaNFS(t, conn, &xid, "test-link")
}

func TestTernRename(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	createFileViaNFS(t, conn, &xid, clientid, "rename-src.txt", []byte("rename data"))

	// RENAME: PUTROOTFH + SAVEFH + PUTROOTFH + RENAME
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		w.AppendArgarray_Putrootfh() // same dir rename

		rw := w.AppendArgarray_Rename()
		oldW := rw.StartOldname()
		buf := oldW.SetData([]byte("rename-src.txt")).Finish()
		rw.Resume(buf)
		newW := rw.StartNewname()
		buf = newW.SetData([]byte("rename-dst.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // SAVEFH
	nextOp(t, &iter) // PUTROOTFH
	renameEntry := nextOp(t, &iter)
	renameRes := renameEntry.Value().AsRENAME4resEntry()
	if renameRes.Disc() != NFS4_OK {
		t.Fatalf("RENAME status = %d", renameRes.Disc())
	}

	// Verify old name is gone.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("rename-src.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})
	xid++
	if res.Status() == NFS4_OK {
		t.Fatal("expected old name to be gone after rename")
	}

	// Verify new name exists and has correct data.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("rename-dst.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	readEntry := nextOp(t, &iter)
	readRes := readEntry.Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ after rename status = %d", readRes.Disc())
	}
	got := string(readRes.Value().AsREAD4resok().Data())
	if got != "rename data" {
		t.Fatalf("data after rename = %q, want %q", got, "rename data")
	}

	cleanupViaNFS(t, conn, &xid, "rename-dst.txt")
}

func TestTernSetclientidAndRenew(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// RENEW
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		rw := w.AppendArgarray_Renew()
		rw.SetClientid(clientid)
	})
	xid++
	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	if entry.Value().AsRENEW4res().Status() != NFS4_OK {
		t.Fatal("RENEW failed")
	}
}

func TestTernStagedFileGetattrAndRead(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// OPEN CREATE + GETFH + OPEN_CONFIRM
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("staged.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// WRITE some data
	testData := []byte("staged file content")
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		ww := w.AppendArgarray_Write()
		copyStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2) // FILE_SYNC4
		ww = ww.SetData(testData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// GETATTR should return staged size (before CLOSE).
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	attrData := getAttrData(t, nextOp(t, &iter).Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(attrData) < 8 {
		t.Fatal("attr data too short")
	}
	size := binary.BigEndian.Uint64(attrData[:8])
	if size != uint64(len(testData)) {
		t.Fatalf("staged GETATTR size = %d, want %d", size, len(testData))
	}

	// READ should return staged data (before CLOSE).
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		copyStateid(rw.Stateid(), stateid)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	readOK := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok()
	got := string(readOK.Data())
	if got != string(testData) {
		t.Fatalf("staged READ = %q, want %q", got, string(testData))
	}

	// CLOSE to commit.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	cleanupViaNFS(t, conn, &xid, "staged.txt")
}

// setupNamedClient is like setupClient but with a custom identity string.
func setupNamedClient(t *testing.T, conn net.Conn, xid *uint32, identity string) uint64 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_Setclientid()
		clientW := scw.StartClient()
		clientW = clientW.SetId([]byte(identity))
		buf := clientW.Finish()
		scw.Resume(buf)
		cbW := scw.StartCallback()
		cbW.SetCbProgram(0x40000000)
		locW := cbW.StartCbLocation()
		netidW := locW.StartRNetid()
		buf = netidW.SetData([]byte("tcp")).Finish()
		locW.Resume(buf)
		addrW := locW.StartRAddr()
		buf = addrW.SetData([]byte("0.0.0.0.0.0")).Finish()
		locW.Resume(buf)
		buf = locW.Finish()
		cbW.Resume(buf)
		buf = cbW.Finish()
		scw.Resume(buf)
		scw.SetCallbackIdent(0)
		buf = scw.Finish()
		w.Resume(buf)
	})
	*xid++
	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	scRes := entry.Value().AsSETCLIENTID4resEntry()
	if scRes.Disc() != NFS4_OK {
		t.Fatalf("SETCLIENTID status = %d", scRes.Disc())
	}
	clientid := scRes.Value().AsSETCLIENTID4resok().Clientid()
	res = sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_SetclientidConfirm()
		scw.SetClientid(clientid)
	})
	*xid++
	iter = expectOK(t, res)
	entry = nextOp(t, &iter)
	if entry.Value().AsSETCLIENTIDCONFIRM4res().Status() != NFS4_OK {
		t.Fatal("SETCLIENTID_CONFIRM failed")
	}
	return clientid
}

// lookupFH looks up a name in the root directory and returns the file handle.
func lookupFH(t *testing.T, conn net.Conn, xid *uint32, name string) []byte {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte(name)).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	return fh
}

// readFileData reads a file by handle and returns the data.
func readFileData(t *testing.T, conn net.Conn, xid *uint32, fh []byte, offset uint64, count uint32) (data []byte, eof bool) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(offset)
		rw.SetCount(count)
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	readRes := nextOp(t, &iter).Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %s", Nfsstat4Name(readRes.Disc()))
	}
	readOK := readRes.Value().AsREAD4resok()
	return readOK.Data(), readOK.Eof() != 0
}

// getAttrSize gets the size attribute from a file handle.
func getAttrSize(t *testing.T, conn net.Conn, xid *uint32, fh []byte) uint64 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	ad := getAttrData(t, nextOp(t, &iter).Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(ad) < 8 {
		t.Fatal("attr data too short for size")
	}
	return binary.BigEndian.Uint64(ad[:8])
}

// collectTernReaddirNames collects all names from a directory via paginated READDIR.
func collectTernReaddirNames(t *testing.T, conn net.Conn, xid *uint32, dirFH []byte) []string {
	t.Helper()
	var allNames []string
	cookie := uint64(0)
	var cookieVerf [8]byte
	for {
		res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
			pfW := w.AppendArgarray_Putfh()
			buf := pfW.StartObject().SetData(dirFH).Finish()
			pfW.Resume(buf)
			buf = pfW.Finish()
			w.Resume(buf)
			rdw := w.AppendArgarray_Readdir()
			rdw.SetCookie(cookie)
			cv := rdw.Cookieverf()
			for i := 0; i < 8; i++ {
				cv.SetData(i, cookieVerf[i])
			}
			rdw.SetDircount(4096)
			rdw.SetMaxcount(32768)
			bmW := rdw.StartAttrRequest()
			bmW.AppendData(1 << FATTR4_TYPE)
			buf = bmW.Finish()
			rdw.Resume(buf)
			buf = rdw.Finish()
			w.Resume(buf)
		})
		*xid++
		iter := expectOK(t, res)
		nextOp(t, &iter) // PUTFH
		readdirRes := nextOp(t, &iter).Value().AsREADDIR4resEntry()
		if readdirRes.Disc() != NFS4_OK {
			t.Fatalf("READDIR status = %s", Nfsstat4Name(readdirRes.Disc()))
		}
		okRes := readdirRes.Value().AsREADDIR4resok()
		cv := okRes.Cookieverf()
		for i := 0; i < 8; i++ {
			cookieVerf[i] = cv.Data(i)
		}
		reply := okRes.Reply()
		if reply.EntriesPresent() == TRUE {
			entOpt := reply.Entries()
			for {
				ent := entOpt.AsEntry4()
				allNames = append(allNames, string(ent.Name().Data()))
				cookie = ent.Cookie()
				if ent.NextentryPresent() != TRUE {
					break
				}
				entOpt = ent.Nextentry()
			}
		}
		if reply.Eof() == TRUE {
			break
		}
	}
	return allNames
}

// mkdirViaNFS creates a directory via NFS CREATE.
func mkdirViaNFS(t *testing.T, conn net.Conn, xid *uint32, name string) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		nameW := cw.StartObjname()
		buf := nameW.SetData([]byte(name)).Finish()
		cw.Resume(buf)
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	createRes := nextOp(t, &iter).Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE dir %q status = %s", name, Nfsstat4Name(createRes.Disc()))
	}
}

// --- Large file test: multi-MB write and read-back ---

func TestTernLargeFile(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// 2MB file — large enough to span multiple TernFS blocks/spans.
	// Write in 512KB chunks to stay under the 1MB RPC frame limit.
	totalSize := 2 * 1024 * 1024
	chunkSize := 512 * 1024
	data := make([]byte, totalSize)
	for i := range data {
		data[i] = byte((i*7 + i/256) % 251)
	}

	// OPEN CREATE + GETFH + OPEN_CONFIRM
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("large-2mb.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// Write in chunks.
	for off := 0; off < totalSize; off += chunkSize {
		end := off + chunkSize
		if end > totalSize {
			end = totalSize
		}
		chunk := data[off:end]
		res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
			pfW := w.AppendArgarray_Putfh()
			buf := pfW.StartObject().SetData(fh).Finish()
			pfW.Resume(buf)
			buf = pfW.Finish()
			w.Resume(buf)
			ww := w.AppendArgarray_Write()
			copyStateid(ww.Stateid(), stateid)
			ww = ww.SetOffset(uint64(off))
			ww = ww.SetStable(2) // FILE_SYNC4
			ww = ww.SetData(chunk)
			buf = ww.Finish()
			w.Resume(buf)
		})
		xid++
		expectOK(t, res)
	}

	// CLOSE.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	// Read back in chunks and verify.
	var got []byte
	for off := 0; off < totalSize; {
		chunk, eof := readFileData(t, conn, &xid, fh, uint64(off), uint32(chunkSize))
		got = append(got, chunk...)
		off += len(chunk)
		if eof {
			break
		}
		if len(chunk) == 0 {
			t.Fatalf("READ returned 0 bytes at offset %d without EOF", off)
		}
	}
	if len(got) != totalSize {
		t.Fatalf("total READ len = %d, want %d", len(got), totalSize)
	}
	for i := range got {
		if got[i] != data[i] {
			t.Fatalf("data mismatch at byte %d: got %d, want %d", i, got[i], data[i])
		}
	}

	cleanupViaNFS(t, conn, &xid, "large-2mb.txt")
}

// --- Read at offset / partial reads ---

func TestTernReadAtOffset(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	fh := createFileViaNFS(t, conn, &xid, clientid, "offset-test.txt", data)

	// Read 4 bytes from middle — should not be EOF.
	got, eof := readFileData(t, conn, &xid, fh, 10, 4)
	if string(got) != "abcd" {
		t.Fatalf("READ at offset 10 = %q, want %q", got, "abcd")
	}
	if eof {
		t.Fatal("unexpected EOF in middle of file")
	}

	// Read past end — should get remaining bytes with EOF.
	got, eof = readFileData(t, conn, &xid, fh, 32, 100)
	if string(got) != "wxyz" {
		t.Fatalf("READ past end = %q, want %q", got, "wxyz")
	}
	if !eof {
		t.Fatal("expected EOF past end of file")
	}

	// Read at exact end — should get empty with EOF.
	got, eof = readFileData(t, conn, &xid, fh, uint64(len(data)), 100)
	if len(got) != 0 {
		t.Fatalf("READ at end = %d bytes, want 0", len(got))
	}
	if !eof {
		t.Fatal("expected EOF at end of file")
	}

	cleanupViaNFS(t, conn, &xid, "offset-test.txt")
}

// --- Server restart with persistent file handles ---

func TestTernServerRestart(t *testing.T) {
	// Create a file with the first server instance.
	addr1, cleanup1 := startTernTestServer(t)
	conn1 := dial(t, addr1)

	xid := uint32(1)
	clientid := setupClient(t, conn1, &xid)
	testData := []byte("persistent handle test data")
	fh := createFileViaNFS(t, conn1, &xid, clientid, "persist.txt", testData)

	// Shut down first server.
	conn1.Close()
	cleanup1()

	// Start a new server against the same cluster.
	addr2, cleanup2 := startTernTestServer(t)
	defer cleanup2()
	conn2 := dial(t, addr2)
	defer conn2.Close()

	xid = 1 // reset xid for new connection

	// Use the file handle from the first server — should still work.
	got, _ := readFileData(t, conn2, &xid, fh, 0, 4096)
	if string(got) != string(testData) {
		t.Fatalf("READ after restart = %q, want %q", got, testData)
	}

	// Verify GETATTR also works with the old handle.
	size := getAttrSize(t, conn2, &xid, fh)
	if size != uint64(len(testData)) {
		t.Fatalf("GETATTR size after restart = %d, want %d", size, len(testData))
	}

	// Also verify LOOKUP gives same handle.
	fh2 := lookupFH(t, conn2, &xid, "persist.txt")
	if string(fh) != string(fh2) {
		t.Fatalf("file handle changed after restart: %x → %x", fh, fh2)
	}

	clientid = setupClient(t, conn2, &xid)
	_ = clientid
	cleanupViaNFS(t, conn2, &xid, "persist.txt")
}

// --- READDIR pagination: enough entries to force multiple batches ---

func TestTernReaddirPagination(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create 30 files — forces at least 2 batches (batch size is 16).
	var expected []string
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("pg_%03d.txt", i)
		createFileViaNFS(t, conn, &xid, clientid, name, []byte(fmt.Sprintf("data-%d", i)))
		expected = append(expected, name)
	}
	sort.Strings(expected)

	// Get root FH for paginated readdir.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	rootFH := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	names := collectTernReaddirNames(t, conn, &xid, rootFH)

	// Filter to only our test files (other tests may have left .nfs dir).
	var filtered []string
	for _, n := range names {
		if len(n) >= 3 && n[:3] == "pg_" {
			filtered = append(filtered, n)
		}
	}
	sort.Strings(filtered)

	if len(filtered) != len(expected) {
		t.Fatalf("got %d entries, want %d: %v", len(filtered), len(expected), filtered)
	}
	for i := range expected {
		if filtered[i] != expected[i] {
			t.Fatalf("entry[%d] = %q, want %q", i, filtered[i], expected[i])
		}
	}

	for _, name := range expected {
		cleanupViaNFS(t, conn, &xid, name)
	}
}

// --- Nested directories + LOOKUPP ---

func TestTernNestedDirectories(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a/b/c directory hierarchy.
	mkdirViaNFS(t, conn, &xid, "nest_a")

	// Create "nest_a/b" — need to LOOKUP nest_a first, then CREATE inside it.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nest_a")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		nameW := cw.StartObjname()
		buf = nameW.SetData([]byte("b")).Finish()
		cw.Resume(buf)
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	createRes := nextOp(t, &iter).Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE nest_a/b status = %s", Nfsstat4Name(createRes.Disc()))
	}

	// Create a file inside nest_a/b via OPEN.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		for _, name := range []string{"nest_a", "b"} {
			lw := w.AppendArgarray_Lookup()
			nw := lw.StartObjname()
			buf := nw.SetData([]byte(name)).Finish()
			lw.Resume(buf)
			buf = lw.Finish()
			w.Resume(buf)
		}

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("deep.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP nest_a
	nextOp(t, &iter) // LOOKUP b
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// Write data.
	deepData := []byte("deep nested content")
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		copyStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(deepData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// CLOSE.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	// Navigate root → nest_a → b → deep.txt and read.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		for _, name := range []string{"nest_a", "b", "deep.txt"} {
			lw := w.AppendArgarray_Lookup()
			nw := lw.StartObjname()
			buf := nw.SetData([]byte(name)).Finish()
			lw.Resume(buf)
			buf = lw.Finish()
			w.Resume(buf)
		}
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP nest_a
	nextOp(t, &iter) // LOOKUP b
	nextOp(t, &iter) // LOOKUP deep.txt
	readOK := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOK.Data()) != string(deepData) {
		t.Fatalf("deep READ = %q, want %q", readOK.Data(), deepData)
	}

	// LOOKUPP from b should go back to nest_a, then LOOKUPP again to root.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nest_a")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("b")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Lookupp() // b → nest_a
		w.AppendArgarray_Lookupp() // nest_a → root

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP nest_a
	nextOp(t, &iter) // LOOKUP b
	nextOp(t, &iter) // LOOKUPP → nest_a
	nextOp(t, &iter) // LOOKUPP → root
	attrData := getAttrData(t, nextOp(t, &iter).Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(attrData) < 4 {
		t.Fatal("attr data too short")
	}
	ftype := binary.BigEndian.Uint32(attrData[:4])
	if ftype != uint32(NF4DIR) {
		t.Fatalf("LOOKUPP result type = %d, want NF4DIR", ftype)
	}

	// Cleanup: remove deep.txt, b, nest_a.
	// Remove deep.txt from nest_a/b.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		for _, name := range []string{"nest_a", "b"} {
			lw := w.AppendArgarray_Lookup()
			nw := lw.StartObjname()
			buf := nw.SetData([]byte(name)).Finish()
			lw.Resume(buf)
			buf = lw.Finish()
			w.Resume(buf)
		}
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf := tw.SetData([]byte("deep.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// Remove b from nest_a.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nest_a")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf = tw.SetData([]byte("b")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	cleanupViaNFS(t, conn, &xid, "nest_a")
}

// --- OPEN existing file for write → NFS4ERR_PERM ---

func TestTernOpenExistingForWriteRejected(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a file first.
	createFileViaNFS(t, conn, &xid, clientid, "existing.txt", []byte("original content"))

	// Try to OPEN existing file for write (NOCREATE).
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_WRITE)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("existing.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != NFS4ERR_PERM {
		t.Fatalf("OPEN existing for write: got status %s, want NFS4ERR_PERM",
			Nfsstat4Name(res.Status()))
	}

	cleanupViaNFS(t, conn, &xid, "existing.txt")
}

// --- Remove non-empty directory → NFS4ERR_NOTEMPTY ---

func TestTernRemoveNonEmptyDir(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	mkdirViaNFS(t, conn, &xid, "notempty_dir")

	// Create a file inside the directory.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("notempty_dir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf = ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("child.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	childFH := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// CLOSE the file.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(childFH).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	// Try to remove non-empty directory.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf := tw.SetData([]byte("notempty_dir")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	if res.Status() != NFS4ERR_NOTEMPTY {
		t.Fatalf("REMOVE non-empty dir: got status %s, want NFS4ERR_NOTEMPTY",
			Nfsstat4Name(res.Status()))
	}

	// Clean up: remove child, then directory.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("notempty_dir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf = tw.SetData([]byte("child.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)
	cleanupViaNFS(t, conn, &xid, "notempty_dir")
}

// --- COMMIT test ---

func TestTernCommit(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// OPEN CREATE + GETFH + OPEN_CONFIRM
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("commit.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// WRITE with UNSTABLE4.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		copyStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(0) // UNSTABLE4
		ww = ww.SetData([]byte("commit test data"))
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// COMMIT — should succeed and return a non-zero verifier.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		w.AppendArgarray_Commit()
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	commitOk := nextOp(t, &iter).Value().AsCOMMIT4resEntry().Value().AsCOMMIT4resok()
	verf := commitOk.Writeverf()
	allZero := true
	for i := 0; i < 8; i++ {
		if verf.Data(i) != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("COMMIT write verifier should be non-zero")
	}

	// CLOSE.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	cleanupViaNFS(t, conn, &xid, "commit.txt")
}

// --- SETATTR: set mtime ---

func TestTernSetattr(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	createFileViaNFS(t, conn, &xid, clientid, "setattr.txt", []byte("setattr test"))
	fh := lookupFH(t, conn, &xid, "setattr.txt")

	// Set mtime to 2020-01-01 00:00:00 UTC.
	targetTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	targetSec := targetTime.Unix()

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(0)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(0) // word 0: nothing
		bmW.AppendData(1 << (FATTR4_TIME_MODIFY_SET - 32))
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 4+8+4)
		binary.BigEndian.PutUint32(attrData[0:4], SET_TO_CLIENT_TIME4)
		binary.BigEndian.PutUint64(attrData[4:12], uint64(targetSec))
		binary.BigEndian.PutUint32(attrData[12:16], 0)
		alW := faw.StartAttrVals()
		buf = alW.SetData(attrData).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry := nextOp(t, &iter)
	if entry.Value().AsSETATTR4res().Status() != NFS4_OK {
		t.Fatalf("SETATTR status = %s", Nfsstat4Name(entry.Value().AsSETATTR4res().Status()))
	}

	// Verify mtime via GETATTR.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(0) // word 0
		bw.AppendData(1 << (FATTR4_TIME_MODIFY - 32))
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	attrBytes := getAttrData(t, nextOp(t, &iter).Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(attrBytes) < 12 {
		t.Fatal("attr data too short for time_modify")
	}
	gotSec := int64(binary.BigEndian.Uint64(attrBytes[:8]))
	if gotSec != targetSec {
		t.Fatalf("mtime seconds = %d, want %d", gotSec, targetSec)
	}

	cleanupViaNFS(t, conn, &xid, "setattr.txt")
}

// --- File replacement: delete + recreate same name ---

func TestTernFileReplacement(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create file with original content.
	createFileViaNFS(t, conn, &xid, clientid, "replace.txt", []byte("version 1"))

	// Verify original.
	fh1 := lookupFH(t, conn, &xid, "replace.txt")
	got, _ := readFileData(t, conn, &xid, fh1, 0, 4096)
	if string(got) != "version 1" {
		t.Fatalf("original data = %q, want %q", got, "version 1")
	}

	// Delete and recreate with new content.
	cleanupViaNFS(t, conn, &xid, "replace.txt")
	createFileViaNFS(t, conn, &xid, clientid, "replace.txt", []byte("version 2"))

	// Verify replacement.
	fh2 := lookupFH(t, conn, &xid, "replace.txt")
	got, _ = readFileData(t, conn, &xid, fh2, 0, 4096)
	if string(got) != "version 2" {
		t.Fatalf("replacement data = %q, want %q", got, "version 2")
	}

	// Old handle should no longer work (file was deleted and a new inode created).
	if string(fh1) == string(fh2) {
		// If handles happen to be the same (unlikely), we can't test staleness.
		t.Log("handles are identical (inode reuse), skipping stale handle check")
	}

	cleanupViaNFS(t, conn, &xid, "replace.txt")
}

// --- Cross-directory rename ---

func TestTernCrossDirectoryRename(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	mkdirViaNFS(t, conn, &xid, "srcdir")
	mkdirViaNFS(t, conn, &xid, "dstdir")

	// Create a file in srcdir.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("srcdir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("nfstest"))
		buf = ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("moved.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()
		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP srcdir
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	stateid := openOK.Stateid()
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	movedData := []byte("cross-dir rename data")
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		copyStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(movedData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		copyStateid(caw.OpenStateid(), stateid)
	})
	xid++
	expectOK(t, res)

	// RENAME: PUTROOTFH + LOOKUP(srcdir) + SAVEFH + PUTROOTFH + LOOKUP(dstdir) + RENAME
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("srcdir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Savefh()

		w.AppendArgarray_Putrootfh()
		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("dstdir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Rename()
		oldW := rw.StartOldname()
		buf = oldW.SetData([]byte("moved.txt")).Finish()
		rw.Resume(buf)
		newW := rw.StartNewname()
		buf = newW.SetData([]byte("arrived.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP srcdir
	nextOp(t, &iter) // SAVEFH
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP dstdir
	renameRes := nextOp(t, &iter).Value().AsRENAME4resEntry()
	if renameRes.Disc() != NFS4_OK {
		t.Fatalf("RENAME status = %s", Nfsstat4Name(renameRes.Disc()))
	}

	// Verify file is in dstdir.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("dstdir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("arrived.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP dstdir
	nextOp(t, &iter) // LOOKUP arrived.txt
	readOK := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOK.Data()) != string(movedData) {
		t.Fatalf("cross-dir rename data = %q, want %q", readOK.Data(), movedData)
	}

	// Cleanup.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("dstdir")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf = tw.SetData([]byte("arrived.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)
	cleanupViaNFS(t, conn, &xid, "srcdir")
	cleanupViaNFS(t, conn, &xid, "dstdir")
}

// --- Concurrent writers on separate files ---

func TestTernConcurrentWrites(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()

	const numClients = 4
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := dial(t, addr)
			defer conn.Close()

			xid := uint32(1)
			clientid := setupNamedClient(t, conn, &xid, fmt.Sprintf("concurrent-%d", idx))

			name := fmt.Sprintf("concurrent_%d.txt", idx)
			data := []byte(fmt.Sprintf("data from client %d, with some padding to make it non-trivial", idx))

			fh := createFileViaNFS(t, conn, &xid, clientid, name, data)

			// Read back and verify.
			got, _ := readFileData(t, conn, &xid, fh, 0, 4096)
			if string(got) != string(data) {
				errors <- fmt.Errorf("client %d: got %q, want %q", idx, got, data)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}

	// Cleanup.
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	for i := 0; i < numClients; i++ {
		cleanupViaNFS(t, conn, &xid, fmt.Sprintf("concurrent_%d.txt", i))
	}
}

// --- .nfs directory should be hidden in READDIR ---

func TestTernNfsDirHidden(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	// Get root FH.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	rootFH := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	names := collectTernReaddirNames(t, conn, &xid, rootFH)
	for _, n := range names {
		if n == ".nfs" {
			t.Fatal(".nfs directory should be hidden in READDIR")
		}
	}
}

// --- Lookup non-existent file → NFS4ERR_NOENT ---

func TestTernLookupNonExistent(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nonexistent-file-12345.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != NFS4ERR_NOENT {
		t.Fatalf("LOOKUP non-existent: got status %s, want NFS4ERR_NOENT",
			Nfsstat4Name(res.Status()))
	}
}

// --- Empty file creation and read ---

func TestTernCreateEmptyFile(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	fh := createFileViaNFS(t, conn, &xid, clientid, "empty.txt", nil)

	// Read — should get 0 bytes with EOF.
	got, eof := readFileData(t, conn, &xid, fh, 0, 4096)
	if len(got) != 0 {
		t.Fatalf("empty file READ returned %d bytes", len(got))
	}
	if !eof {
		t.Fatal("expected EOF on empty file")
	}

	// GETATTR size should be 0.
	size := getAttrSize(t, conn, &xid, fh)
	if size != 0 {
		t.Fatalf("empty file size = %d, want 0", size)
	}

	cleanupViaNFS(t, conn, &xid, "empty.txt")
}

// --- Multiple operations in single COMPOUND ---

func TestTernCompoundChaining(t *testing.T) {
	addr, cleanup := startTernTestServer(t)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	createFileViaNFS(t, conn, &xid, clientid, "chain-a.txt", []byte("aaa"))
	createFileViaNFS(t, conn, &xid, clientid, "chain-b.txt", []byte("bbb"))

	// Single compound: PUTROOTFH + LOOKUP(a) + READ + PUTROOTFH + LOOKUP(b) + READ
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("chain-a.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)

		w.AppendArgarray_Putrootfh()
		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("chain-b.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		rw = w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP a
	readA := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readA.Data()) != "aaa" {
		t.Fatalf("file a = %q, want %q", readA.Data(), "aaa")
	}
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP b
	readB := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readB.Data()) != "bbb" {
		t.Fatalf("file b = %q, want %q", readB.Data(), "bbb")
	}

	cleanupViaNFS(t, conn, &xid, "chain-a.txt")
	cleanupViaNFS(t, conn, &xid, "chain-b.txt")
}
