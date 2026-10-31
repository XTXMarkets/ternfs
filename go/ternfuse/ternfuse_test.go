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

func TestCanDeleteFileImmediately(t *testing.T) {
	previousCache := dirInfoCache
	dirInfoCache = client.NewDirInfoCache()
	t.Cleanup(func() { dirInfoCache = previousCache })

	timeRetention := msgs.ActiveDeleteAfterTime(30 * 24 * time.Hour)
	timeZero := msgs.ActiveDeleteAfterTime(0)
	versionRetention := msgs.ActiveDeleteAfterVersions(1)
	versionZero := msgs.ActiveDeleteAfterVersions(0)
	tests := []struct {
		name     string
		time     msgs.DeleteAfterTime
		versions msgs.DeleteAfterVersions
		want     bool
	}{
		{"both_inactive", 0, 0, false},
		{"time_retention", timeRetention, 0, false},
		{"version_retention", 0, versionRetention, false},
		{"both_retention", timeRetention, versionRetention, false},
		{"time_zero", timeZero, 0, true},
		{"version_zero", 0, versionZero, true},
		{"time_zero_version_retention", timeZero, versionRetention, true},
		{"time_retention_version_zero", timeRetention, versionZero, true},
		{"both_zero", timeZero, versionZero, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := msgs.SnapshotPolicy{
				DeleteAfterTime:     test.time,
				DeleteAfterVersions: test.versions,
			}
			dir := msgs.ROOT_DIR_INODE_ID
			dirInfoCache.UpdatePolicy(dir, &policy)
			dirInfoCache.UpdateInheritedFrom(dir, msgs.SNAPSHOT_POLICY_TAG, dir)

			got, errno := canDeleteFileImmediately(dir)
			if errno != 0 {
				t.Fatal(errno)
			}
			if got != test.want {
				t.Errorf("canDeleteFileImmediately() = %v, want %v", got, test.want)
			}
		})
	}
}
