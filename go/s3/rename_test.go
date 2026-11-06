// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

const renameFile = msgs.InodeId(msgs.FILE)<<61 | 2

type renameTestClient struct {
	*client.Client
	t         *testing.T
	edges     map[msgs.LookupReq]msgs.LookupResp
	mutations []any
	lookups   int
	err       error
	cancel    context.CancelFunc
}

func (c *renameTestClient) ShardRequest(_ *log.Logger, shard msgs.ShardId, req msgs.ShardRequest, resp msgs.ShardResponse) error {
	switch req := req.(type) {
	case *msgs.LookupReq:
		if c.edges == nil || shard != req.DirId.Shard() {
			c.t.Fatalf("unexpected lookup: %+v shard=%d", req, shard)
		}
		c.lookups++
		if c.cancel != nil {
			c.cancel()
		}
		edge, found := c.edges[*req]
		if !found {
			return msgs.NAME_NOT_FOUND
		}
		*resp.(*msgs.LookupResp) = edge
		return nil
	case *msgs.SameDirectoryRenameReq:
		if shard != req.DirId.Shard() {
			c.t.Fatalf("wrong rename shard: %d", shard)
		}
	default:
		c.t.Fatalf("unexpected shard RPC: %T", req)
	}
	c.mutations = append(c.mutations, req)
	return c.err
}

func (c *renameTestClient) CDCRequest(_ *log.Logger, req msgs.CDCRequest, _ msgs.CDCResponse) error {
	c.mutations = append(c.mutations, req)
	return c.err
}

func TestServeHTTPRename(t *testing.T) {
	root := msgs.ROOT_DIR_INODE_ID
	for _, tc := range []struct {
		name, source, destination string
		backendErr                error
		status                    int
		code                      string
		mutation                  any
		cancelAt                  string
	}{
		{"same directory", "old", "new", nil, 200, "", &msgs.SameDirectoryRenameReq{
			TargetId: renameFile, DirId: root, OldName: "old", OldCreationTime: 99, NewName: "new",
		}, ""},
		{"cross directory", "old", "sub/new", nil, 200, "", &msgs.RenameFileReq{
			TargetId: renameFile, OldOwnerId: root, OldName: "old", OldCreationTime: 99, NewOwnerId: root | 1, NewName: "new",
		}, ""},
		{"cross directory same shard", "old", "other/new", nil, 200, "", &msgs.RenameFileReq{
			TargetId: renameFile, OldOwnerId: root, OldName: "old", OldCreationTime: 99, NewOwnerId: root | 0x100, NewName: "new",
		}, ""},
		{"same name", "old", "old", nil, 200, "", nil, ""},
		{"missing source", "missing", "new", nil, 404, "NoSuchKey", nil, ""},
		{"missing parent", "old", "missing/new", nil, 404, "NoSuchKey", nil, ""},
		{"file as parent", "old", "old/new", nil, 400, "InvalidRequest", nil, ""},
		{"null parent", "old", "null/new", nil, 400, "InvalidRequest", nil, ""},
		{"directory source", "sub", "new", nil, 400, "InvalidRequest", nil, ""},
		{"canceled", "old", "new", nil, 500, "InternalError", nil, "before"},
		{"canceled after lookup", "old", "new", nil, 500, "InternalError", nil, "lookup"},
		{"canceled after parent lookup", "old", "sub/new", nil, 500, "InternalError", nil, "parent"},
		{"conflict", "old", "new", msgs.CANNOT_OVERRIDE_NAME, 409, "OperationAborted", nil, ""},
		{"edge changed", "old", "new", msgs.MISMATCHING_CREATION_TIME, 409, "OperationAborted", nil, ""},
		{"target changed", "old", "new", msgs.MISMATCHING_TARGET, 409, "OperationAborted", nil, ""},
		{"owner changed", "old", "new", msgs.MISMATCHING_OWNER, 409, "OperationAborted", nil, ""},
		{"edge removed", "old", "new", msgs.EDGE_NOT_FOUND, 409, "OperationAborted", nil, ""},
		{"file removed", "old", "new", msgs.FILE_NOT_FOUND, 404, "NoSuchKey", nil, ""},
		{"directory removed", "old", "new", msgs.DIRECTORY_NOT_FOUND, 404, "NoSuchKey", nil, ""},
		{"source parent removed", "old", "sub/new", msgs.OLD_DIRECTORY_NOT_FOUND, 404, "NoSuchKey", nil, ""},
		{"destination parent removed", "old", "sub/new", msgs.NEW_DIRECTORY_NOT_FOUND, 404, "NoSuchKey", nil, ""},
		{"backend cause", "old", "new", fmt.Errorf("rename: %w", context.DeadlineExceeded), 500, "InternalError", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpc := &renameTestClient{t: t, err: tc.backendErr, edges: map[msgs.LookupReq]msgs.LookupResp{
				{DirId: root, Name: "old"}:   {TargetId: renameFile, CreationTime: 99},
				{DirId: root, Name: "sub"}:   {TargetId: root | 1},
				{DirId: root, Name: "other"}: {TargetId: root | 0x100},
				{DirId: root, Name: "null"}:  {TargetId: msgs.NULL_INODE_ID},
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelAt == "before" {
				cancel()
			} else if tc.cancelAt != "" {
				rpc.cancel = func() {
					if tc.cancelAt == "lookup" || rpc.lookups == 2 {
						cancel()
					}
				}
			}
			r := renameRequest("bucket/"+tc.source, tc.destination).WithContext(ctx)
			if tc.name == "same directory" {
				r.Header.Set("If-None-Match", "*")
				r.Header.Set("X-Amz-Rename-Source-If-Match", `"unmatched"`)
				r.Header.Set("X-Amz-Client-Token", "ignored-token")
			}
			result := checkRenameHTTP(t, renameServer(rpc), r, tc.status, tc.code)
			cause := tc.backendErr
			if ctx.Err() != nil {
				cause = ctx.Err()
			}
			if tc.status == 500 && result.Message != cause.Error() {
				t.Fatalf("message=%q want=%q", result.Message, cause)
			}
			var want []any
			if tc.mutation != nil {
				want = []any{tc.mutation}
			} else if tc.backendErr != nil {
				if len(rpc.mutations) != 1 {
					t.Fatalf("mutation count=%d", len(rpc.mutations))
				}
				return
			}
			if !reflect.DeepEqual(rpc.mutations, want) {
				t.Fatalf("mutations=%#v want=%#v", rpc.mutations, want)
			}
			if tc.cancelAt == "before" && rpc.lookups != 0 || tc.source == tc.destination && rpc.lookups != 1 {
				t.Fatalf("lookups=%d", rpc.lookups)
			}
		})
	}
}

func TestServeHTTPRenameDecoding(t *testing.T) {
	root, bucket := msgs.ROOT_DIR_INODE_ID, msgs.ROOT_DIR_INODE_ID|0x100
	for source, name := range map[string]string{
		"bucket/a%20b": "a b", "/bucket/a+b": "a+b", "bucket/100%25": "100%",
		"bucket/a%252Fb": "a%2Fb", "bucket/a%2Fb": "a/b",
		"bucket/%C3%A9%E2%82%AC": "\u00e9\u20ac", "bucket/a!$&'()*,;=:@~b": "a!$&'()*,;=:@~b",
		"bucket/" + strings.Repeat("a", 255):                          strings.Repeat("a", 255),
		"bucket/" + url.PathEscape(strings.Repeat("\u00e9", 127)+"a"): strings.Repeat("\u00e9", 127) + "a",
	} {
		t.Run(source, func(t *testing.T) {
			parts := parseObjectPath(name)
			parent := bucket
			if len(parts.segments) > 1 {
				parent = bucket | 1
			}
			rpc := &renameTestClient{t: t, edges: map[msgs.LookupReq]msgs.LookupResp{
				{DirId: root, Name: "export"}:           {TargetId: bucket},
				{DirId: bucket, Name: "a"}:              {TargetId: bucket | 1},
				{DirId: parent, Name: parts.basename()}: {TargetId: renameFile, CreationTime: 99},
			}}
			s := renameServer(rpc)
			s.bucketPaths["bucket"], s.virtualHost = "/export", "s3.test"
			r := renameRequest(source, name)
			r.Host, r.URL.Path = "bucket.s3.test", "/"+name
			r.URL.RawQuery = "renameObject=&x-id=RenameObject&X-Amz-Signature=ab"
			r.Header = map[string][]string{"x-amz-rename-source": {source}}
			checkRenameHTTP(t, s, r, 200, "")
			if rpc.lookups != 1+len(parts.segments) || len(rpc.mutations) != 0 {
				t.Fatalf("lookups=%d mutations=%v", rpc.lookups, rpc.mutations)
			}
		})
	}
}
