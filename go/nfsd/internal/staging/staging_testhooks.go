// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"os"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/byterange"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sidecar"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// testingT is the part of testing.T the hooks use, so that this package
// does not import testing.
type testingT interface {
	Fatal(args ...any)
	Helper()
}

// Test hooks: nfsd tests outside this package build, inspect and perturb
// local staging files through these.

func CreateOverlayStage(
	t testingT,
	dir string,
	baseID vfs.InodeID,
	base []byte,
) (*LocalStore, vfs.InodeID, File) {
	t.Helper()
	store, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := vfs.MakeInodeID(vfs.InodeTypeFile, 100)
	stage, err := store.Create(id, sidecar.Meta{
		DirID:    vfs.MakeInodeID(vfs.InodeTypeDir, 1),
		FileName: "file",
		BaseID:   baseID,
		BaseSize: uint64(len(base)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, id, stage
}

func SliceBaseReader(base []byte, calls *[]byterange.Range) BaseReader {
	return func(_ vfs.InodeID, offset uint64, dest []byte) (int, bool, error) {
		end := min(offset+uint64(len(dest)), uint64(len(base)))
		if calls != nil {
			*calls = append(*calls, byterange.Range{Start: offset, End: end})
		}
		if offset >= uint64(len(base)) {
			return 0, true, nil
		}
		n := copy(dest, base[offset:end])
		return n, end == uint64(len(base)), nil
	}
}

func FindTarget(store Store, dirID vfs.InodeID, name string) (vfs.InodeID, sidecar.Meta, bool) {
	for id, meta := range store.FindTargets(dirID, name) {
		return id, meta, true
	}
	return 0, sidecar.Meta{}, false
}

func TargetBusy(store Store, dirID vfs.InodeID, name string) bool {
	for _, meta := range store.FindTargets(dirID, name) {
		if !meta.Retired {
			return true
		}
	}
	return false
}

func CloseFiles(t testingT, store *LocalStore) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, entry := range store.files {
		if entry.failed || entry.file.Meta().Retired {
			continue // retirement has already synced and closed this file
		}
		if err := entry.file.f.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := entry.file.f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// PrepareRemoveAll prepares every staging file for removal.
func (s *LocalStore) PrepareRemoveAll() {
	for _, entry := range s.files {
		entry.file.prepareRemove()
	}
}

// MetaPath returns the path of the file's sidecar.
func (sf *LocalFile) MetaPath() string {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.metaPath
}

// SetMetaPath redirects the file's sidecar writes, to inject failures.
func (sf *LocalFile) SetMetaPath(path string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.metaPath = path
}

// DataFile returns the open data file.
func (sf *LocalFile) DataFile() *os.File {
	return sf.f
}

// CheckpointDirty reports whether the sidecar needs a checkpoint.
func (sf *LocalFile) CheckpointDirty() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.checkpointDirty
}
