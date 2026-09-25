// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestStagingCreateConcurrency(t *testing.T) {
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	store.createFile = func(path string) (*os.File, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return os.Create(path)
	}
	type result struct {
		file StagingFile
		err  error
	}
	first := make(chan result, 1)
	go func() {
		f, err := store.Create(1, StagingMeta{})
		first <- result{f, err}
	}()
	defer closeSignal(release)
	awaitSignal(t, entered, "first staging creation")
	other := make(chan result, 1)
	go func() {
		f, err := store.Create(2, StagingMeta{})
		other <- result{f, err}
	}()
	if r := awaitValue(t, other, "independent staging creation"); r.err != nil {
		t.Fatal(r.err)
	}
	duplicate := make(chan result, 1)
	go func() {
		f, err := store.Create(1, StagingMeta{})
		duplicate <- result{f, err}
	}()
	if store.Get(1) != nil || len(store.Entries()) != 1 {
		t.Fatal("unfinished creation was published")
	}
	closeSignal(release)
	a := awaitValue(t, first, "first creation")
	b := awaitValue(t, duplicate, "duplicate creation")
	if a.err != nil || b.err != nil || a.file != b.file || calls.Load() != 2 {
		t.Fatalf("duplicate creation: first=%v second=%v same=%v calls=%d",
			a.err, b.err, a.file == b.file, calls.Load())
	}
	store.Remove(1)
	store.Remove(2)
}

func TestStagingCleanupWaitsForCreation(t *testing.T) {
	for _, quarantine := range []bool{false, true} {
		t.Run(map[bool]string{false: "remove", true: "quarantine"}[quarantine], func(t *testing.T) {
			store, err := NewLocalStagingStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			store.createFile = func(path string) (*os.File, error) {
				close(entered)
				<-release
				return os.Create(path)
			}
			created := make(chan error, 1)
			go func() {
				_, err := store.Create(1, StagingMeta{})
				created <- err
			}()
			defer closeSignal(release)
			awaitSignal(t, entered, "creation before cleanup")
			cleaned := make(chan error, 1)
			go func() {
				if quarantine {
					cleaned <- store.Quarantine(1)
				} else {
					store.Remove(1)
					cleaned <- nil
				}
			}()
			select {
			case err := <-cleaned:
				t.Fatalf("cleanup finished before creation: %v", err)
			case <-time.After(25 * time.Millisecond):
			}
			closeSignal(release)
			if err := awaitValue(t, created, "creation"); err != nil {
				t.Fatal(err)
			}
			if err := awaitValue(t, cleaned, "cleanup"); err != nil {
				t.Fatal(err)
			}
			if store.Get(1) != nil || len(store.Entries()) != 0 {
				t.Fatal("cleanup left a published entry")
			}
			if quarantine {
				if _, err := store.Create(1, StagingMeta{}); !errors.Is(err, errStagingRemoved) {
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
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// A sidecar creation failure must remove the data file and release the
	// inode lock, so a later create can complete normally.
	meta := filepath.Join(store.dir, "0000000000000001.meta")
	if err := os.Mkdir(meta, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(1, StagingMeta{}); err == nil {
		t.Fatal("creation unexpectedly succeeded")
	}
	if store.Get(1) != nil {
		t.Fatal("failed creation was published")
	}
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(1, StagingMeta{}); err != nil {
		t.Fatal(err)
	}
	store.Remove(1)
}
