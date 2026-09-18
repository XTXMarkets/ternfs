// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"testing"
)

func TestParwalkPoolRun(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	defer pool.Close()

	var called bool
	err := pool.run(
		context.Background(),
		func(run *parwalkRun) error {
			called = true
			if run.pool != pool {
				t.Fatal("run was not attached to pool")
			}
			if err := context.Cause(run.ctx); err != nil {
				t.Fatalf("run context: %v", err)
			}
			return nil
		},
		nil,
		ParwalkOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("run function was not called")
	}
}

func TestParwalkPoolReusedAfterRunError(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	defer pool.Close()

	runErr := errors.New("run failed")
	err := pool.run(
		context.Background(),
		func(*parwalkRun) error {
			return runErr
		},
		nil,
		ParwalkOptions{},
	)
	if !errors.Is(err, runErr) {
		t.Fatalf("first run error = %v, want %v", err, runErr)
	}

	err = pool.run(
		context.Background(),
		func(*parwalkRun) error {
			return nil
		},
		nil,
		ParwalkOptions{},
	)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
}

func TestParwalkPoolCallerCancelsActiveRun(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error)

	go func() {
		done <- pool.run(
			ctx,
			func(run *parwalkRun) error {
				close(started)
				<-run.ctx.Done()
				return context.Cause(run.ctx)
			},
			nil,
			ParwalkOptions{},
		)
	}()

	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want %v", err, context.Canceled)
	}
}

func TestParwalkPoolRejectsRunAfterClose(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	pool.Close()

	called := false
	err := pool.run(
		context.Background(),
		func(*parwalkRun) error {
			called = true
			return nil
		},
		nil,
		ParwalkOptions{},
	)
	if !errors.Is(err, ErrParwalkPoolClosed) {
		t.Fatalf("run error = %v, want %v", err, ErrParwalkPoolClosed)
	}
	if called {
		t.Fatal("run function was called after pool close")
	}
}

func TestParwalkPoolPanicsOnNilContext(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	defer pool.Close()
	defer func() {
		if recover() == nil {
			t.Fatal("run with nil context did not panic")
		}
	}()

	_ = pool.run(
		nil,
		func(*parwalkRun) error {
			return nil
		},
		nil,
		ParwalkOptions{},
	)
}
