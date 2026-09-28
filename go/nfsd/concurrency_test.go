// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// testGate counts calls and blocks the first one, or every call when all is
// set. Tests must release the gate before waiting for worker cleanup.
type testGate struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
	all     bool
}

func newTestGate() *testGate {
	return &testGate{
		entered: make(chan struct{}, 32),
		release: make(chan struct{}),
	}
}

func (g *testGate) wait() {
	if call := g.calls.Add(1); g.all || call == 1 {
		g.entered <- struct{}{}
		<-g.release
	}
}

func assertBlocked[T any](t *testing.T, ch <-chan T, description string) {
	t.Helper()
	select {
	case value := <-ch:
		t.Fatalf("%s completed before release: %v", description, value)
	case <-time.After(25 * time.Millisecond):
	}
}
