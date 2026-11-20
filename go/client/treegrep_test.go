// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/sync/errgroup"
)

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
