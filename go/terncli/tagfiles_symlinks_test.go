// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"
	"xtx/ternfs/msgs"
)

type fakeSymlinkFilesystem struct {
	targets map[msgs.InodeId]string
	entries map[string]msgs.InodeId
}

func (fs *fakeSymlinkFilesystem) readlink(id msgs.InodeId) (string, error) {
	target, ok := fs.targets[id]
	if !ok {
		return "", msgs.FILE_NOT_FOUND
	}
	return target, nil
}

func (fs *fakeSymlinkFilesystem) lookup(dir msgs.InodeId, name string) (msgs.InodeId, error) {
	id, ok := fs.entries[fmt.Sprintf("%s/%s", dir.String(), name)]
	if !ok {
		return msgs.NULL_INODE_ID, msgs.NAME_NOT_FOUND
	}
	return id, nil
}

type fakeSymlinkTree struct {
	*fakeSymlinkFilesystem
	t      *testing.T
	paths  map[string]msgs.InodeId
	nextID uint64
}

func newFakeSymlinkTree(t *testing.T) *fakeSymlinkTree {
	t.Helper()
	return &fakeSymlinkTree{
		fakeSymlinkFilesystem: &fakeSymlinkFilesystem{
			targets: make(map[msgs.InodeId]string),
			entries: make(map[string]msgs.InodeId),
		},
		t:      t,
		paths:  map[string]msgs.InodeId{"/": msgs.ROOT_DIR_INODE_ID},
		nextID: 1,
	}
}

func (tree *fakeSymlinkTree) add(pathname string, inodeType msgs.InodeType, target string) msgs.InodeId {
	tree.t.Helper()
	if !path.IsAbs(pathname) {
		tree.t.Fatalf("fake path %q is not absolute", pathname)
	}
	pathname = path.Clean(pathname)
	if _, exists := tree.paths[pathname]; exists {
		tree.t.Fatalf("fake path %q already exists", pathname)
	}
	parentPath := path.Dir(pathname)
	parent, exists := tree.paths[parentPath]
	if !exists {
		tree.t.Fatalf("add parent %q before child %q", parentPath, pathname)
	}
	if parent.Type() != msgs.DIRECTORY {
		tree.t.Fatalf("parent %q of %q is not a directory", parentPath, pathname)
	}

	id := msgs.MakeInodeId(inodeType, 1, tree.nextID)
	tree.nextID++
	tree.paths[pathname] = id
	tree.entries[fmt.Sprintf("%s/%s", parent.String(), path.Base(pathname))] = id
	if inodeType == msgs.SYMLINK {
		tree.targets[id] = target
	}
	return id
}

func (tree *fakeSymlinkTree) addDirectory(pathname string) {
	tree.t.Helper()
	tree.add(pathname, msgs.DIRECTORY, "")
}

func (tree *fakeSymlinkTree) addFile(pathname string) {
	tree.t.Helper()
	tree.add(pathname, msgs.FILE, "")
}

func (tree *fakeSymlinkTree) addSymlink(pathname, target string) msgs.InodeId {
	tree.t.Helper()
	return tree.add(pathname, msgs.SYMLINK, target)
}

func TestSymlinkIsDanglingAbsoluteFileTarget(t *testing.T) {
	// /data/link -> /data/file
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	tree.addFile("/data/file")
	link := tree.addSymlink("/data/link", "/data/file")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if dangling {
		t.Fatal("absolute file target was classified as dangling")
	}
}

func TestSymlinkIsDanglingRelativeDirectoryTarget(t *testing.T) {
	// /data/link -> dir
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	tree.addDirectory("/data/dir")
	link := tree.addSymlink("/data/link", "dir")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if dangling {
		t.Fatal("relative directory target was classified as dangling")
	}
}

func TestSymlinkIsDanglingMissingRelativeTarget(t *testing.T) {
	// /data/link -> missing
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	link := tree.addSymlink("/data/link", "missing")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if !dangling {
		t.Fatal("missing relative target was not classified as dangling")
	}
}

func TestSymlinkIsDanglingSymlinkChain(t *testing.T) {
	// /data/link -> file-link -> /data/file
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	tree.addFile("/data/file")
	tree.addSymlink("/data/file-link", "/data/file")
	link := tree.addSymlink("/data/link", "file-link")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if dangling {
		t.Fatal("symlink chain to a file was classified as dangling")
	}
}

func TestSymlinkIsDanglingDotDotAfterSymlink(t *testing.T) {
	// /data/link -> archive-dir/../file
	// /data/archive-dir -> /archive/dir
	// The target therefore resolves to /archive/file, not /data/file.
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	tree.addDirectory("/archive")
	tree.addDirectory("/archive/dir")
	tree.addFile("/archive/file")
	tree.addSymlink("/data/archive-dir", "/archive/dir")
	link := tree.addSymlink("/data/link", "archive-dir/../file")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if dangling {
		t.Fatal("target with dot-dot after a symlink was classified as dangling")
	}
}

func TestSymlinkIsDanglingTrailingSlashOnFile(t *testing.T) {
	// /data/link -> file/
	tree := newFakeSymlinkTree(t)
	tree.addDirectory("/data")
	tree.addFile("/data/file")
	link := tree.addSymlink("/data/link", "file/")

	dangling, err := symlinkIsDangling(tree, "/data/link", link)
	if err != nil {
		t.Fatalf("symlinkIsDangling: %v", err)
	}
	if !dangling {
		t.Fatal("regular-file target with a trailing slash was not classified as dangling")
	}
}

func TestSymlinkResolutionDoesNotTreatLookupErrorsAsDangling(t *testing.T) {
	link := msgs.MakeInodeId(msgs.SYMLINK, 1, 1)
	fs := &fakeSymlinkFilesystem{
		targets: map[msgs.InodeId]string{link: "/target"},
		entries: map[string]msgs.InodeId{},
	}

	errorFS := symlinkFilesystemFuncs{
		readlinkFunc: fs.readlink,
		lookupFunc: func(msgs.InodeId, string) (msgs.InodeId, error) {
			return msgs.NULL_INODE_ID, msgs.TIMEOUT
		},
	}
	dangling, err := symlinkIsDangling(errorFS, "/link", link)
	if err == nil {
		t.Fatal("expected lookup error")
	}
	if dangling {
		t.Fatal("lookup error was classified as dangling")
	}
}

func TestFirstSymlinkMatchTargetPredicate(t *testing.T) {
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	rules := []*Rule{
		{
			Name:            "dangling",
			SymlinkTarget:   "dangling",
			IncludePatterns: []*regexp.Regexp{regexp.MustCompile(".*")},
			SuffixPatterns:  []*regexp.Regexp{regexp.MustCompile(".*")},
		},
		{
			Name:            "not-dangling",
			SymlinkTarget:   "not-dangling",
			IncludePatterns: []*regexp.Regexp{regexp.MustCompile(".*")},
			SuffixPatterns:  []*regexp.Regexp{regexp.MustCompile(".*")},
		},
	}
	for _, tc := range []struct {
		name       string
		isDangling bool
		want       string
	}{
		{"dangling", true, "dangling"},
		{"not dangling", false, "not-dangling"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FirstSymlinkMatch(rules, "/link", 0, now, now, now, func() (bool, error) {
				return tc.isDangling, nil
			})
			if err != nil {
				t.Fatalf("FirstSymlinkMatch: %v", err)
			}
			if got == nil || got.Name != tc.want {
				t.Fatalf("got rule %+v, want %q", got, tc.want)
			}
		})
	}
}

func TestFirstSymlinkMatchWithoutTargetPredicateDoesNotResolve(t *testing.T) {
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	rule := &Rule{
		Name:            "any",
		IncludePatterns: []*regexp.Regexp{regexp.MustCompile(".*")},
		SuffixPatterns:  []*regexp.Regexp{regexp.MustCompile(".*")},
	}
	got, err := FirstSymlinkMatch([]*Rule{rule}, "/link", 0, now, now, now, func() (bool, error) {
		t.Fatal("target resolver called")
		return false, nil
	})
	if err != nil {
		t.Fatalf("FirstSymlinkMatch: %v", err)
	}
	if got != rule {
		t.Fatalf("got rule %+v, want %+v", got, rule)
	}
}

type symlinkFilesystemFuncs struct {
	readlinkFunc func(msgs.InodeId) (string, error)
	lookupFunc   func(msgs.InodeId, string) (msgs.InodeId, error)
}

func (fs symlinkFilesystemFuncs) readlink(id msgs.InodeId) (string, error) {
	return fs.readlinkFunc(id)
}

func (fs symlinkFilesystemFuncs) lookup(dir msgs.InodeId, name string) (msgs.InodeId, error) {
	return fs.lookupFunc(dir, name)
}

func TestSymlinkRowContract(t *testing.T) {
	job := symlinkJob{
		id:       msgs.MakeInodeId(msgs.SYMLINK, 7, 42),
		fullPath: "/data/dangling",
	}
	resp := msgs.StatFileResp{Size: 12, Atime: 34, Mtime: 56}
	rule := &Rule{Name: "delete_dangling"}
	row := symlinkRow(job, resp, rule)
	fields := strings.Split(row, "\t")
	if len(fields) != 6 {
		t.Fatalf("row has %d fields, want 6: %q", len(fields), row)
	}
	if fields[0] != job.id.String() || fields[1] != "12" || fields[2] != "34" ||
		fields[3] != "56" || fields[4] != rule.Name || fields[5] != job.fullPath {
		t.Fatalf("unexpected row fields: %q", fields)
	}
}
