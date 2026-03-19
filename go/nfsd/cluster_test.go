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
