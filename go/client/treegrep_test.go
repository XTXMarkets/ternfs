// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"golang.org/x/sync/errgroup"
)

func treeGrepTestReply(packet []byte, body bincode.Packable) ([]byte, error) {
	data, err := bincode.Pack(body)
	if err != nil {
		return nil, err
	}
	reply := binary.LittleEndian.AppendUint32(nil, msgs.SHARD_RESP_PROTOCOL_VERSION)
	reply = append(reply, packet[4:13]...)
	return append(reply, data...), nil
}

func TestTreeGrepWalkErrorCancelsBudgetWait(t *testing.T) {
	for _, source := range []string{"filter", "directory"} {
		t.Run(source, func(t *testing.T) {
			busy := msgs.MakeInodeId(msgs.DIRECTORY, 1, 1)
			fail := msgs.MakeInodeId(msgs.DIRECTORY, 2, 2)
			file := msgs.MakeInodeId(msgs.FILE, 0, 3)
			fileWaiting := make(chan struct{})
			var wantErr error = msgs.NOT_AUTHORISED
			if source == "filter" {
				wantErr = errors.New("filter failed")
			}
			c, logger := metadataRetryClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte) ([]byte, error) {
				switch msgs.ShardMessageKind(packet[12]) {
				case msgs.LOOKUP:
					var req msgs.LookupReq
					if err := bincode.Unpack(packet[13:], &req); err != nil {
						return nil, err
					}
					id := busy
					if req.Name == "fail" {
						id = fail
					}
					return treeGrepTestReply(packet, &msgs.LookupResp{TargetId: id})
				case msgs.READ_DIR:
					var req msgs.ReadDirReq
					if err := bincode.Unpack(packet[13:], &req); err != nil {
						return nil, err
					}
					if req.DirId == fail {
						return metadataRetryReply(packet, msgs.NOT_AUTHORISED), nil
					}
					return treeGrepTestReply(packet, &msgs.ReadDirResp{Results: []msgs.CurrentEdge{{Name: "file", TargetId: file}}})
				default:
					return nil, fmt.Errorf("unexpected shard request %v", msgs.ShardMessageKind(packet[12]))
				}
			})
			var addrs [256]msgs.AddrsInfo
			for i := range addrs {
				addrs[i] = metadataTestAddrs
			}
			c.SetAddrs(metadataTestAddrs, &addrs)
			grep := NewTreeGrep(logger, c, 1)
			// Another search holds the shared budget throughout this call.
			grep.sem <- struct{}{}
			defer func() { <-grep.sem }()
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			finished := make(chan struct{})
			t.Cleanup(func() {
				cancel()
				receiveMetadata(t, finished)
			})
			go func() {
				defer close(finished)
				_, err := grep.Grep(ctx, &TreeGrepOptions{
					Roots:   []string{"/busy", "/fail"},
					Pattern: regexp.MustCompile("foo"),
					Parwalk: ParwalkOptions{WorkersPerShard: 1, QueueSize: 1},
					Filter: func(_ string, id msgs.InodeId) (bool, error) {
						if id == file {
							close(fileWaiting)
						}
						if id == fail {
							select {
							case <-fileWaiting:
							case <-ctx.Done():
								return false, context.Cause(ctx)
							}
							if source == "filter" {
								return false, wantErr
							}
						}
						return true, nil
					},
				}, TreeGrepCallbacks{})
				done <- err
			}()
			if err := receiveMetadata(t, done); !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestTreeGrepFileFinishesAfterWalk(t *testing.T) {
	var walkCtx context.Context
	c, logger := metadataRetryClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte) ([]byte, error) {
		// Complete the data read only after the walk has returned successfully.
		<-walkCtx.Done()
		return treeGrepTestReply(packet, &msgs.LocalFileSpansResp{Spans: []msgs.FetchedSpan{{
			Header: msgs.FetchedSpanHeader{Size: 4, StorageClass: msgs.INLINE_STORAGE},
			Body:   &msgs.FetchedInlineSpan{Body: []byte("foo\n")},
		}}})
	})
	group, ctx := errgroup.WithContext(t.Context())
	run := &treeGrepRun{
		group:   group,
		ctx:     ctx,
		options: &TreeGrepOptions{Pattern: regexp.MustCompile("foo")},
	}
	grep := NewTreeGrep(logger, c, 1)
	file := msgs.MakeInodeId(msgs.FILE, 0, 1)
	err := ParwalkFromInode(ctx, logger, c, &ParwalkOptions{WorkersPerShard: 1, QueueSize: 1}, file, "/file",
		func(ctx context.Context, _ msgs.InodeId, _ string, _ string, _ msgs.TernTime, id msgs.InodeId, _ bool, _ bool) error {
			walkCtx = ctx
			return grep.visit(run, ctx, "/file", id)
		})
	if err != nil {
		t.Fatal(err)
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	if run.stats.FilesScanned != 1 || run.stats.MatchesFound != 1 {
		t.Fatalf("file scan stopped with the walk: %+v", run.stats)
	}
}

func TestTreeGrepEmitStopsAfterCancel(t *testing.T) {
	stopErr := errors.New("stop")
	ctx, cancel := context.WithCancelCause(context.Background())
	run := treeGrepRun{ctx: ctx}

	callbackErr := errors.New("output failed")
	if err := run.emit(func() error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("error = %v, want %v", err, callbackErr)
	}
	cancel(stopErr)
	err := run.emit(func() error {
		t.Error("callback ran after cancellation")
		return nil
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("error = %v, want %v", err, stopErr)
	}
}

func TestTreeGrepProgressErrorDuringShutdown(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	group, workerCtx := errgroup.WithContext(ctx)
	progressErr := errors.New("progress output failed")
	run := treeGrepRun{ctx: workerCtx, callbacks: TreeGrepCallbacks{
		Progress: func(TreeGrepStats) error {
			// Normal group shutdown races a callback that is still running.
			if err := group.Wait(); err != nil {
				t.Fatal(err)
			}
			return progressErr
		},
	}}
	if err := run.reportProgress(cancel); !errors.Is(err, progressErr) {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), progressErr) {
		t.Fatalf("progress error lost during shutdown: %v", context.Cause(ctx))
	}
}
