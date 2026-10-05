// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"

	"fmt"

	"net"
	"sync"
	"testing"
	"time"
)

// cbTestClient is a client connection that also serves callbacks: its
// reader separates the server's CALLs from replies to its own requests.
type cbTestClient struct {
	t       *testing.T
	conn    net.Conn
	mu      sync.Mutex
	replies map[uint32]chan []byte
	calls   chan []byte
	writeMu sync.Mutex
	// order records frame arrivals: a reply's XID, or 0 for a callback.
	order []uint32
	// program is the callback program the client registered; every CALL
	// is checked against it (cbCallHeaderError).
	program uint32
}

// cbCallHeaderError checks a callback CALL as the client's callback
// service would: ONC RPC version 2, the session's callback program, program
// version 1 (NFS_CB in the official XDR), CB_NULL or CB_COMPOUND, and for
// CB_COMPOUND minor version 1 with callback_ident 0.
func cbCallHeaderError(call []byte, program uint32) string {
	if len(call) < 40 {
		return "short CALL"
	}
	field := func(off int) uint32 { return binary.BigEndian.Uint32(call[off:]) }
	switch {
	case field(8) != rpcVersion:
		return fmt.Sprintf("rpcvers %d", field(8))
	case field(12) != program:
		return fmt.Sprintf("program %#x, want %#x", field(12), program)
	case field(16) != 1:
		return fmt.Sprintf("program version %d, want 1", field(16))
	case field(20) != cbProcNull && field(20) != cbProcCompound:
		return fmt.Sprintf("procedure %d", field(20))
	}
	if field(20) == cbProcCompound {
		args := cbArgs(call)
		if len(args) < 12 {
			return "short CB_COMPOUND"
		}
		tag := int((binary.BigEndian.Uint32(args) + 3) &^ 3)
		if len(args) < 4+tag+8 || binary.BigEndian.Uint32(args[4+tag:]) != 1 ||
			binary.BigEndian.Uint32(args[8+tag:]) != 0 {
			return "CB_COMPOUND minor version or callback_ident"
		}
	}
	return ""
}

func newCBTestClient(t *testing.T, addr string) *cbTestClient {
	t.Helper()
	c := &cbTestClient{t: t, conn: dial(t, addr), replies: make(map[uint32]chan []byte),
		calls: make(chan []byte, 64), program: 0x40000001}
	t.Cleanup(func() { c.conn.Close() })
	go func() {
		for {
			frame, err := readFrame(c.conn)
			if err != nil {
				return
			}
			if binary.BigEndian.Uint32(frame[4:8]) == rpcCall {
				c.mu.Lock()
				c.order = append(c.order, 0)
				program := c.program
				c.mu.Unlock()
				if msg := cbCallHeaderError(frame, program); msg != "" {
					t.Errorf("invalid callback CALL: %s", msg)
				}
				c.calls <- frame
				continue
			}
			xid := binary.BigEndian.Uint32(frame[0:4])
			c.mu.Lock()
			c.order = append(c.order, xid)
			ch := c.replies[xid]
			delete(c.replies, xid)
			c.mu.Unlock()
			if ch != nil {
				ch <- frame
			}
		}
	}()
	return c
}

func (c *cbTestClient) write(frame []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := writeFrame(c.conn, frame); err != nil {
		c.t.Fatal(err)
	}
}

func (c *cbTestClient) compound(xid uint32, body []byte) COMPOUND4res {
	c.t.Helper()
	ch := make(chan []byte, 1)
	c.mu.Lock()
	c.replies[xid] = ch
	c.mu.Unlock()
	c.write(rpcCallForTest(xid, procCompound, body))
	reply := awaitValue(c.t, ch, "reply")
	res, err := parseCompoundReply(reply)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}

// cbArgs returns a CALL's arguments, after its credential and verifier.
func cbArgs(call []byte) []byte {
	off := 28
	off += 4 + int((binary.BigEndian.Uint32(call[off:])+3)&^3)
	off += 4
	off += 4 + int((binary.BigEndian.Uint32(call[off:])+3)&^3)
	return call[off:]
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(testChannelTimeout); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}
