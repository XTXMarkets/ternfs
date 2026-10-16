// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/certificate"
	"github.com/XTXMarkets/ternfs/go/core/crc32c"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"golang.org/x/sys/unix"
)

type ioRequestResult struct {
	keepConnection bool
	err            error
}

func ioRequestEnv(t *testing.T, opts diskIOLimitOptions) (*env, map[msgs.BlockServiceId]*blockService) {
	t.Helper()
	key, err := aes.NewCipher(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	bs := &blockService{path: t.TempDir(), cipher: key}
	if err := os.Mkdir(filepath.Join(bs.path, "with_crc"), 0700); err != nil {
		t.Fatal(err)
	}
	bs.storage.Store(&blockServiceStorage{reported: storageSpace{1 << 40, 1 << 40}})
	e := &env{
		bufPool:   bufpool.NewBufPool(),
		stats:     map[msgs.BlockServiceId]*blockServiceStats{1: {}},
		counters:  make(map[msgs.BlocksMessageKind]*timing.Timings),
		ioLimiter: newDiskIOLimiter(opts, []msgs.BlockServiceId{1, 2}),
	}
	t.Cleanup(e.ioLimiter.close)
	for _, kind := range msgs.AllBlocksMessageKind {
		e.counters[kind] = timing.NewTimings(40, time.Microsecond, 1.5)
	}
	return e, map[msgs.BlockServiceId]*blockService{1: bs}
}

func startIORequest(t *testing.T, e *env, services map[msgs.BlockServiceId]*blockService, req msgs.BlocksRequest, timeout time.Duration) (*net.TCPConn, <-chan ioRequestResult) {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	client.SetDeadline(time.Now().Add(5 * time.Second))
	server, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	logger := testLogger(t)
	done := make(chan ioRequestResult, 1)
	go func() {
		var lastError error
		keep := handleSingleRequest(logger, e, nil, &lastError, services, nil, server, 0, timeout)
		server.Close()
		done <- ioRequestResult{keep, lastError}
	}()
	var header bytes.Buffer
	binary.Write(&header, binary.LittleEndian, msgs.BLOCKS_REQ_PROTOCOL_VERSION)
	binary.Write(&header, binary.LittleEndian, uint64(1))
	header.WriteByte(byte(req.BlocksRequestKind()))
	if err := req.Pack(&header); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Write(header.Bytes()); err != nil {
		t.Fatal(err)
	}
	return client, done
}

func readIOResponse(t *testing.T, conn *net.TCPConn, resp msgs.BlocksResponse) msgs.TernError {
	t.Helper()
	var version uint32
	if err := binary.Read(conn, binary.LittleEndian, &version); err != nil {
		t.Fatal(err)
	}
	if version != msgs.BLOCKS_RESP_PROTOCOL_VERSION {
		t.Fatalf("bad response version: %x", version)
	}
	var kind [1]byte
	if _, err := io.ReadFull(conn, kind[:]); err != nil {
		t.Fatal(err)
	}
	if kind[0] == msgs.ERROR {
		var code uint16
		if err := binary.Read(conn, binary.LittleEndian, &code); err != nil {
			t.Fatal(err)
		}
		return msgs.TernError(code)
	}
	if resp == nil || msgs.BlocksMessageKind(kind[0]) != resp.BlocksResponseKind() {
		t.Fatalf("unexpected response kind: %d", kind[0])
	}
	if err := resp.Unpack(conn); err != nil {
		t.Fatal(err)
	}
	return 0
}

func finishIORequest(t *testing.T, done <-chan ioRequestResult) ioRequestResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("request did not finish")
		return ioRequestResult{}
	}
}

func TestOverloadedIORequestsRejectBeforeDiskOrPayload(t *testing.T) {
	for _, req := range []msgs.BlocksRequest{
		&msgs.FetchBlockReq{Count: msgs.TERN_PAGE_SIZE},
		&msgs.FetchBlockWithCrcReq{Count: msgs.TERN_PAGE_SIZE},
		&msgs.CheckBlockReq{},
		&msgs.WriteBlockReq{Size: msgs.TERN_PAGE_SIZE},
		&msgs.TestWriteReq{Size: uint64(msgs.TERN_PAGE_SIZE)},
		&msgs.EraseBlockReq{},
	} {
		t.Run(req.BlocksRequestKind().String(), func(t *testing.T) {
			e, services := ioRequestEnv(t, smallDiskIOLimits())
			// A filesystem access would return ENOENT; trying to consume a
			// write body would hang because we only send the header.
			services[1].path = filepath.Join(services[1].path, "missing")
			release := mustAcquireIO(t, e.ioLimiter, 1, req.BlocksRequestKind())
			defer release()
			conn, done := startIORequest(t, e, services, req, 0)
			conn.SetReadDeadline(time.Now().Add(time.Second))
			if code := readIOResponse(t, conn, nil); code != msgs.TIMEOUT {
				t.Fatalf("expected overload TIMEOUT, got %v", code)
			}
			result := finishIORequest(t, done)
			if result.keepConnection || !errors.Is(result.err, errDiskIOBusy) {
				t.Fatalf("unsafe overload result: %+v", result)
			}
			if services[1].ioErrors != 0 || services[1].noSpaceErrors.Load() != 0 {
				t.Fatal("overload classified as hardware failure or space exhaustion")
			}
			global, _ := e.ioLimiter.snapshot()
			if global.active[diskIORequestClass(req.BlocksRequestKind())] != 1 {
				t.Fatal("rejected request released another operation's slot")
			}
		})
	}
}

func TestBlockedFilesystemOpenRetainsAdmission(t *testing.T) {
	for _, req := range []msgs.BlocksRequest{
		&msgs.FetchBlockReq{BlockId: 1, Count: msgs.TERN_PAGE_SIZE},
		&msgs.FetchBlockWithCrcReq{BlockId: 1, Count: msgs.TERN_PAGE_SIZE},
		&msgs.CheckBlockReq{BlockId: 1, Size: msgs.TERN_PAGE_SIZE},
	} {
		t.Run(req.BlocksRequestKind().String(), func(t *testing.T) {
			e, services := ioRequestEnv(t, smallDiskIOLimits())
			fifo := filepath.Join(services[1].path, msgs.BlockId(1).Path())
			if err := os.MkdirAll(filepath.Dir(fifo), 0700); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			// Opening a FIFO without a writer blocks in an actual syscall.
			// Unblock it on all test exit paths so no OS thread is leaked.
			unblock := func() func() {
				fd, err := unix.Open(fifo, unix.O_RDWR|unix.O_NONBLOCK, 0)
				if err != nil {
					t.Error(err)
					return func() {}
				}
				return func() { unix.Close(fd) }
			}
			conn, done := startIORequest(t, e, services, req, 20*time.Millisecond)
			finished := false
			t.Cleanup(func() {
				if finished {
					return
				}
				closeWriter := unblock()
				defer closeWriter()
				select {
				case <-done:
				case <-time.After(time.Second):
				}
			})
			deadline := time.After(5 * time.Second)
			for {
				global, _ := e.ioLimiter.snapshot()
				if global.active[diskRead] == 1 {
					break
				}
				select {
				case <-deadline:
					t.Fatal("read was not admitted")
				case <-time.After(time.Millisecond):
				}
			}
			// Let the socket deadline pass and close the client. Neither
			// action can cancel the filesystem open or return its slot.
			time.Sleep(40 * time.Millisecond)
			conn.Close()
			select {
			case result := <-done:
				t.Fatalf("blocked filesystem call completed early: %+v", result)
			default:
			}
			expectIOBusy(t, e.ioLimiter, 1, msgs.CHECK_BLOCK)
			mustAcquireIO(t, e.ioLimiter, 2, msgs.FETCH_BLOCK)()
			mustAcquireIO(t, e.ioLimiter, 1, msgs.WRITE_BLOCK)()
			mustAcquireIO(t, e.ioLimiter, 1, msgs.ERASE_BLOCK)()
			closeWriter := unblock()
			defer closeWriter()
			finishIORequest(t, done)
			finished = true
			mustAcquireIO(t, e.ioLimiter, 1, msgs.FETCH_BLOCK)()
		})
	}
}

func TestAdmittedBlockRequests(t *testing.T) {
	e, services := ioRequestEnv(t, smallDiskIOLimits())
	data := bytes.Repeat([]byte{0x42}, int(msgs.TERN_PAGE_SIZE))
	id := msgs.BlockId(msgs.Now())
	crc := msgs.Crc(crc32c.Sum(0, data))
	write := &msgs.WriteBlockReq{BlockId: id, Size: uint32(len(data)), Crc: crc}
	write.Certificate = certificate.BlockWriteCertificate(services[1].cipher, 1, write)
	conn, done := startIORequest(t, e, services, write, time.Second)
	if _, err := conn.Write(data); err != nil {
		t.Fatal(err)
	}
	if code := readIOResponse(t, conn, &msgs.WriteBlockResp{}); code != 0 {
		t.Fatalf("write failed: %v", code)
	}
	finishIORequest(t, done)
	for _, withCrc := range []bool{false, true} {
		var fetch msgs.BlocksRequest = &msgs.FetchBlockReq{BlockId: id, Count: uint32(len(data))}
		var resp msgs.BlocksResponse = &msgs.FetchBlockResp{}
		size := len(data)
		if withCrc {
			fetch = &msgs.FetchBlockWithCrcReq{BlockId: id, Count: uint32(len(data))}
			resp = &msgs.FetchBlockWithCrcResp{}
			size += 4
		}
		conn, done := startIORequest(t, e, services, fetch, time.Second)
		if code := readIOResponse(t, conn, resp); code != 0 {
			t.Fatalf("fetch failed: %v", code)
		}
		got := make([]byte, size)
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[:len(data)], data) {
			t.Fatal("fetched data differs")
		}
		finishIORequest(t, done)
	}
	conn, done = startIORequest(t, e, services, &msgs.CheckBlockReq{BlockId: id, Size: uint32(len(data)), Crc: crc}, time.Second)
	if code := readIOResponse(t, conn, &msgs.CheckBlockResp{}); code != 0 {
		t.Fatalf("check failed: %v", code)
	}
	finishIORequest(t, done)
	erase := &msgs.EraseBlockReq{BlockId: id}
	erase.Certificate = certificate.BlockEraseCertificate(1, id, services[1].cipher)
	conn, done = startIORequest(t, e, services, erase, time.Second)
	if code := readIOResponse(t, conn, &msgs.EraseBlockResp{}); code != 0 {
		t.Fatalf("erase failed: %v", code)
	}
	finishIORequest(t, done)
	if _, err := os.Stat(filepath.Join(services[1].path, id.Path())); !os.IsNotExist(err) {
		t.Fatalf("block was not erased: %v", err)
	}
	global, _ := e.ioLimiter.snapshot()
	if global.active != ([diskIOClassCount]int{}) || global.totalWaiting() != 0 {
		t.Fatalf("completed requests leaked admissions: %+v", global)
	}
}

func TestTestWriteSizeBound(t *testing.T) {
	e, services := ioRequestEnv(t, smallDiskIOLimits())
	conn, done := startIORequest(t, e, services, &msgs.TestWriteReq{Size: uint64(MAX_OBJECT_SIZE) + 1}, time.Second)
	if code := readIOResponse(t, conn, nil); code != msgs.BLOCK_TOO_BIG {
		t.Fatalf("expected size rejection before reading payload, got %v", code)
	}
	finishIORequest(t, done)
	global, _ := e.ioLimiter.snapshot()
	if global.active[diskWrite] != 0 {
		t.Fatal("size rejection leaked write admission")
	}
}
