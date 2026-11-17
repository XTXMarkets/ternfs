// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The private directory has to share the root's filesystem, since
// publication renames transients into the tree. A root that is a mount
// point is refused, and nothing is created beside it.
func TestCheckLocalRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "export")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := CheckLocalRoot(root); err != nil {
		t.Fatalf("root on an ordinary directory: %v", err)
	}
	if info, err := os.Stat(root + ".nfsd"); err != nil || !info.IsDir() {
		t.Fatalf("private directory not created: %v", err)
	}

	const mount = "/dev/shm"
	var mountSt, parentSt syscall.Stat_t
	if syscall.Stat(mount, &mountSt) != nil || syscall.Stat(filepath.Dir(mount), &parentSt) != nil || mountSt.Dev == parentSt.Dev {
		t.Skipf("%s is not a mount point here", mount)
	}
	if err := CheckLocalRoot(mount); err == nil {
		t.Fatalf("root on the mount point %s accepted", mount)
	}
	if _, err := os.Stat(mount + ".nfsd"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused root created %s.nfsd: %v", mount, err)
	}
}

func TestTransientIdentitiesSurviveReplacementAndRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "export")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	fs := NewLocalTernVFS(root)
	first, firstCookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.LinkFile(first, firstCookie, fs.RootID(), "file", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	second, secondCookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.LinkFile(second, secondCookie, fs.RootID(), "file", strings.NewReader("second")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   InodeID
		want string
	}{{first, "first"}, {second, "second"}} {
		data := make([]byte, 32)
		n, eof, err := fs.Read(tc.id, 0, data)
		if err != nil || !eof || string(data[:n]) != tc.want {
			t.Fatalf("read %v = %q, %t, %v; want %q", tc.id, data[:n], eof, err, tc.want)
		}
	}
	current, err := fs.Lookup(fs.RootID(), "file")
	if err != nil || current != second {
		t.Fatalf("lookup = %v, %v; want second transient %v", current, err, second)
	}

	restarted := NewLocalTernVFS(root)
	current, err = restarted.Lookup(restarted.RootID(), "file")
	if err != nil || current != second {
		t.Fatalf("lookup after restart = %v, %v; want second transient %v", current, err, second)
	}
	data := make([]byte, 32)
	n, eof, err := restarted.Read(first, 0, data)
	if err != nil || !eof || string(data[:n]) != "first" {
		t.Fatalf("read first transient after restart = %q, %t, %v", data[:n], eof, err)
	}

	live, liveCookie, err := restarted.ConstructFile(restarted.RootID())
	if err != nil {
		t.Fatal(err)
	}
	secondRestart := NewLocalTernVFS(root)
	if _, err := secondRestart.Stat(live); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("live transient stat after restart = %v", err)
	}
	if err := secondRestart.ScrapFile(live, liveCookie); err != nil {
		t.Fatalf("scrap recovered transient: %v", err)
	}
}

func TestSymlinksHaveIndependentIdentities(t *testing.T) {
	root := filepath.Join(t.TempDir(), "export")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	fs := NewLocalTernVFS(root)
	fileID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.LinkFile(fileID, cookie, fs.RootID(), "file", nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		if _, err := fs.Symlink(fs.RootID(), name, "file"); err != nil {
			t.Fatal(err)
		}
	}
	first, err := fs.Lookup(fs.RootID(), "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := fs.Lookup(fs.RootID(), "second")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first == fileID || second == fileID {
		t.Fatalf("symlink identities collide: first=%v second=%v file=%v", first, second, fileID)
	}
	for _, tc := range []struct {
		id   InodeID
		name string
	}{{first, "first"}, {second, "second"}} {
		target, err := fs.Readlink(tc.id)
		if err != nil || target != "file" {
			t.Fatalf("readlink(%s) = %q, %v; want file", tc.name, target, err)
		}
	}
}
