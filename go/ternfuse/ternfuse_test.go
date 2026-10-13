// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestCDCRequestReturnsIO(t *testing.T) {
	const childEnv = "TERNFS_TEST_CDC_REQUEST_ERROR"
	if os.Getenv(childEnv) == "" {
		// Isolate the client goroutines and check that a callback error does not kill FUSE.
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCDCRequestReturnsIO$", "-test.timeout=5s")
		cmd.Env = append(os.Environ(), childEnv+"=pack")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("CDC packing failure: %v\n%s", err, output)
		}
		return
	}

	logger = log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	var err error
	c, err = client.NewClientDirectNoAddrs(logger, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	errno := cdcRequest(&msgs.MakeDirectoryReq{OwnerId: msgs.ROOT_DIR_INODE_ID, Name: strings.Repeat("x", 256)}, &msgs.MakeDirectoryResp{})
	if errno != syscall.EIO {
		t.Fatalf("cdcRequest = %v, want EIO", errno)
	}
}
