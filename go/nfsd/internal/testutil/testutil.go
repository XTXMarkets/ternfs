// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package testutil

import (
	"sync/atomic"
	"testing"
	"time"
)

// Gate counts calls and blocks the first one, or every call when All is
// set. Tests must release the gate before waiting for worker cleanup.
type Gate struct {
	Entered chan struct{}
	Release chan struct{}
	Calls   atomic.Int32
	All     bool
}

func NewGate() *Gate {
	return &Gate{
		Entered: make(chan struct{}, 32),
		Release: make(chan struct{}),
	}
}

func (g *Gate) Wait() {
	if call := g.Calls.Add(1); g.All || call == 1 {
		g.Entered <- struct{}{}
		<-g.Release
	}
}

func AssertBlocked[T any](t *testing.T, ch <-chan T, description string) {
	t.Helper()
	select {
	case value := <-ch:
		t.Fatalf("%s completed before release: %v", description, value)
	case <-time.After(25 * time.Millisecond):
	}
}

const ChannelTimeout = 5 * time.Second

func AwaitSignal(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(ChannelTimeout):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func AwaitValue[T any](
	t *testing.T,
	ch <-chan T,
	description string,
) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(ChannelTimeout):
		t.Fatalf("timed out waiting for %s", description)
		var zero T
		return zero
	}
}

func CloseSignal(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
