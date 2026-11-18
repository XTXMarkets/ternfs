// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestS3FileTypeRoundTrip(t *testing.T) {
	registry := os.Getenv("TERNFS_TEST_REGISTRY")
	if registry == "" {
		t.Skip("needs a running TernFS cluster; from go/ run " +
			"TERNFS_TEST_REGISTRY=127.0.0.1:45551 GOTOOLCHAIN=local " +
			"go test -mod=vendor ./s3 -run TestS3FileTypeRoundTrip -count=1 -v")
	}
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	c, err := client.NewClient(logger, nil, registry, msgs.AddrsInfo{})
	if err != nil {
		t.Fatalf("connect to registry %q: %v", registry, err)
	}
	t.Cleanup(c.Close)
	deadline := time.Now().Add(20 * time.Second)
	for !c.IsReady() {
		if time.Now().After(deadline) {
			t.Fatalf("registry %q: client not ready after 20s", registry)
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.SetFetchBlockServices()
	bufPool := bufpool.NewBufPool()
	server := NewS3Server(logger, c, bufPool, client.NewDirInfoCache(),
		map[string]string{"bucket": "/"}, "")
	dirName := "s3-file-types-" + rand.Text()
	dir := msgs.MakeDirectoryResp{}
	if err := c.CDCRequest(logger, &msgs.MakeDirectoryReq{
		OwnerId: msgs.ROOT_DIR_INODE_ID, Name: dirName,
	}, &dir); err != nil {
		t.Fatalf("create test directory %q: %v", dirName, err)
	}
	t.Cleanup(func() {
		for _, name := range []string{"link", "directory"} {
			entry := msgs.LookupResp{}
			err := c.ShardRequest(logger, dir.Id.Shard(),
				&msgs.LookupReq{DirId: dir.Id, Name: name}, &entry)
			if errors.Is(err, msgs.NAME_NOT_FOUND) {
				continue
			}
			if err != nil {
				t.Errorf("cleanup lookup %s/%s: %v", dirName, name, err)
				continue
			}
			if entry.TargetId.Type() == msgs.DIRECTORY {
				err = c.CDCRequest(logger, &msgs.SoftUnlinkDirectoryReq{
					OwnerId: dir.Id, TargetId: entry.TargetId,
					Name: name, CreationTime: entry.CreationTime,
				}, &msgs.SoftUnlinkDirectoryResp{})
			} else {
				err = c.ShardRequest(logger, dir.Id.Shard(), &msgs.SoftUnlinkFileReq{
					OwnerId: dir.Id, FileId: entry.TargetId,
					Name: name, CreationTime: entry.CreationTime,
				}, &msgs.SoftUnlinkFileResp{})
			}
			if err != nil {
				t.Errorf("cleanup %s/%s (inode %v): %v",
					dirName, name, entry.TargetId, err)
			}
		}
		if err := c.CDCRequest(logger, &msgs.SoftUnlinkDirectoryReq{
			OwnerId: msgs.ROOT_DIR_INODE_ID, TargetId: dir.Id,
			Name: dirName, CreationTime: dir.CreationTime,
		}, &msgs.SoftUnlinkDirectoryResp{}); err != nil {
			t.Errorf("cleanup test directory %q: %v", dirName, err)
		}
	})

	for _, tc := range []struct {
		name string
		mode string
		body string
		typ  msgs.InodeType
	}{
		{"link", "0120777", " ../dir//target/\n", msgs.SYMLINK},
		{"directory", "0040755", "", msgs.DIRECTORY},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "/bucket/" + dirName + "/" + tc.name
			req := httptest.NewRequest(http.MethodPut, key, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Amz-Meta-File-Permissions", tc.mode)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("PUT %s: want status 200, got %d: %s",
					key, rec.Code, rec.Body.String())
			}
			entry := msgs.LookupResp{}
			if err := c.ShardRequest(logger, dir.Id.Shard(),
				&msgs.LookupReq{DirId: dir.Id, Name: tc.name}, &entry); err != nil {
				t.Fatalf("native lookup after PUT %s: %v", key, err)
			}
			if got := entry.TargetId.Type(); got != tc.typ {
				t.Fatalf("PUT %s: want inode type %v, got %v (inode %v)",
					key, tc.typ, got, entry.TargetId)
			}
			if tc.typ == msgs.SYMLINK {
				reader, err := c.ReadFile(logger, bufPool, entry.TargetId)
				if err != nil {
					t.Fatalf("native read %s (inode %v): %v", key, entry.TargetId, err)
				}
				body, readErr := io.ReadAll(reader)
				closeErr := reader.Close()
				if readErr != nil || closeErr != nil {
					t.Fatalf("native read %s: read error %v, close error %v",
						key, readErr, closeErr)
				}
				if string(body) != tc.body {
					t.Errorf("native read %s: want bytes %q, got %q", key, tc.body, body)
				}
			}
			reads := []struct {
				method string
				suffix string
			}{{http.MethodGet, ""}, {http.MethodHead, ""}, {http.MethodGet, "?attributes"}}
			if tc.typ == msgs.DIRECTORY {
				reads = reads[:1]
				reads[0].method = http.MethodHead
				reads[0].suffix = "/"
			}
			for _, read := range reads {
				target := key + read.suffix
				req := httptest.NewRequest(read.method, target, nil)
				if read.suffix == "?attributes" {
					req.Header.Set("X-Amz-Object-Attributes", "ETag,ObjectSize")
				}
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s %s: want status 200, got %d: %s",
						read.method, target, rec.Code, rec.Body.String())
				}
				if mode := rec.Header().Get("X-Amz-Meta-File-Permissions"); mode != tc.mode {
					t.Errorf("%s %s: want mode %q, got %q",
						read.method, target, tc.mode, mode)
				}
				if values := rec.Header().Values("X-Amz-Meta-Mode"); len(values) != 0 {
					t.Errorf("%s %s: want no legacy Mode header, got %q",
						read.method, target, values)
				}
				if read.method == http.MethodGet && read.suffix == "" {
					if body := rec.Body.String(); body != tc.body {
						t.Errorf("GET %s: want bytes %q, got %q", target, tc.body, body)
					}
				}
			}
		})
	}
}
