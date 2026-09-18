// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"testing"
)

func TestRecursiveGrepEmitCancelsOnCallbackError(t *testing.T) {
	callbackErr := errors.New("output failed")
	ctx, cancel := context.WithCancelCause(context.Background())
	run := recursiveGrepRun{
		ctx:    ctx,
		cancel: cancel,
	}

	err := run.emit(func() error {
		return callbackErr
	})
	if !errors.Is(err, callbackErr) {
		t.Fatalf("error = %v, want %v", err, callbackErr)
	}
	if !errors.Is(context.Cause(ctx), callbackErr) {
		t.Fatalf("context cause = %v, want %v", context.Cause(ctx), callbackErr)
	}
}
