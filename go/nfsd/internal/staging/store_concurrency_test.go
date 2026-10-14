// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
)

func TestStagingCreateConcurrency(t *testing.T) {
	store, err := NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	gate := testutil.NewGate()
	store.createFile = func(path string) (*os.File, error) {
		gate.Wait()
		return os.Create(path)
	}
	type result struct {
		file File
		err  error
	}
	first := make(chan result, 1)
	go func() {
		f, err := store.Create(1, Meta{})
		first <- result{f, err}
	}()
	defer testutil.CloseSignal(gate.Release)
	testutil.AwaitSignal(t, gate.Entered, "first staging creation")
	other := make(chan result, 1)
	go func() {
		f, err := store.Create(2, Meta{})
		other <- result{f, err}
	}()
	if r := testutil.AwaitValue(t, other, "independent staging creation"); r.err != nil {
		t.Fatal(r.err)
	}
	duplicate := make(chan result, 1)
	go func() {
		f, err := store.Create(1, Meta{})
		duplicate <- result{f, err}
	}()
	if store.Get(1) != nil || len(store.Entries()) != 1 {
		t.Fatal("unfinished creation was published")
	}
	testutil.CloseSignal(gate.Release)
	a := testutil.AwaitValue(t, first, "first creation")
	b := testutil.AwaitValue(t, duplicate, "duplicate creation")
	if a.err != nil || b.err != nil || a.file != b.file || gate.Calls.Load() != 2 {
		t.Fatalf("duplicate creation: first=%v second=%v same=%v calls=%d",
			a.err, b.err, a.file == b.file, gate.Calls.Load())
	}
	store.Remove(1)
	store.Remove(2)
}

func TestStagingCleanupWaitsForCreation(t *testing.T) {
	for _, quarantine := range []bool{false, true} {
		t.Run(map[bool]string{false: "remove", true: "quarantine"}[quarantine], func(t *testing.T) {
			store, err := NewLocalStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			gate := testutil.NewGate()
			store.createFile = func(path string) (*os.File, error) {
				gate.Wait()
				return os.Create(path)
			}
			created := make(chan error, 1)
			go func() {
				_, err := store.Create(1, Meta{})
				created <- err
			}()
			defer testutil.CloseSignal(gate.Release)
			testutil.AwaitSignal(t, gate.Entered, "creation before cleanup")
			cleaned := make(chan error, 1)
			go func() {
				if quarantine {
					cleaned <- store.Quarantine(1)
				} else {
					store.Remove(1)
					cleaned <- nil
				}
			}()
			testutil.AssertBlocked(t, cleaned, "cleanup during creation")
			testutil.CloseSignal(gate.Release)
			if err := testutil.AwaitValue(t, created, "creation"); err != nil {
				t.Fatal(err)
			}
			if err := testutil.AwaitValue(t, cleaned, "cleanup"); err != nil {
				t.Fatal(err)
			}
			if store.Get(1) != nil || len(store.Entries()) != 0 {
				t.Fatal("cleanup left a published entry")
			}
			if quarantine {
				if _, err := store.Create(1, Meta{}); !errors.Is(err, ErrRemoved) {
					t.Fatalf("quarantined Create = %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(store.dir, "0000000000000001.staging")); !os.IsNotExist(err) {
				t.Fatalf("staging file still present: %v", err)
			}
		})
	}
}

func TestStagingCreateFailureCanRetry(t *testing.T) {
	store, err := NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// A sidecar creation failure must remove the data file and release the
	// inode lock, so a later create can complete normally.
	meta := filepath.Join(store.dir, "0000000000000001.meta")
	if err := os.Mkdir(meta, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(1, Meta{}); err == nil {
		t.Fatal("creation unexpectedly succeeded")
	}
	if store.Get(1) != nil {
		t.Fatal("failed creation was published")
	}
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(1, Meta{}); err != nil {
		t.Fatal(err)
	}
	store.Remove(1)
}
