// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import (
	"errors"
	"os"
	"path/filepath"
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
