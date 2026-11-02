// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The first start on an empty staging directory writes server-id, and every
// later start on the directory reads the same one. A server-id that does
// not decode stops startup rather than being replaced.
func TestServerIDWrittenOnceAndKept(t *testing.T) {
	dir := t.TempDir()
	first, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	CloseFiles(t, first)
	if (first.ServerID() == ServerID{}) {
		t.Fatal("zero server id")
	}
	again, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	CloseFiles(t, again)
	if again.ServerID() != first.ServerID() {
		t.Fatalf("restart read server id %s, want %s", again.ServerID(), first.ServerID())
	}
	other, err := NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	CloseFiles(t, other)
	if other.ServerID() == first.ServerID() {
		t.Fatal("two empty staging directories got the same server id")
	}
	path := filepath.Join(dir, ServerIDFileName)
	if err := os.WriteFile(path, []byte("not hex\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalStore(dir, nil); err == nil {
		t.Fatal("startup accepted a malformed server-id")
	}
	if data, _ := os.ReadFile(path); string(data) != "not hex\n" {
		t.Fatalf("malformed server-id was replaced with %q", data)
	}
}

// A staging directory with a server's state but no server-id is a partial
// copy. Startup refuses it and writes nothing. Empty subdirectories, an
// alias log and quarantined pairs are not state that makes it a server.
func TestPartialCopyRefused(t *testing.T) {
	for _, state := range []string{
		"4000000000000064.staging",
		"4000000000000064.meta",
		filepath.Join(SessionsDirName, "0102"),
		filepath.Join(ClientsDirName, "0102"),
	} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, state)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewLocalStore(dir, nil); !errors.Is(err, ErrPartialCopy) {
				t.Fatalf("startup on a partial copy = %v, want ErrPartialCopy", err)
			}
			if _, err := os.Stat(filepath.Join(dir, ServerIDFileName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refused startup wrote a server-id: %v", err)
			}
		})
	}
	dir := t.TempDir()
	for _, sub := range []string{SessionsDirName, ClientsDirName, AliasesDirName, filepath.Join(QuarantineDirName, "4000000000000064-x")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, AliasesDirName, "published.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatalf("startup without server state = %v", err)
	}
	CloseFiles(t, store)
}
