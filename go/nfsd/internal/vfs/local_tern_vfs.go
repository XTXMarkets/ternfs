// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// LocalTernVFS implements TernVFS backed by a local directory for testing.
type LocalTernVFS struct {
	root   string
	rootID InodeID

	mu        sync.RWMutex
	byID      map[InodeID]string // id -> path relative to root
	byPath    map[string]InodeID // path -> id
	parent    map[InodeID]InodeID
	transient map[InodeID]transientFile // transient files not yet linked
}

type transientFile struct {
	cookie Cookie
	path   string // temp file path on disk
}

func NewLocalTernVFS(root string) *LocalTernVFS {
	lfs := &LocalTernVFS{
		root:      root,
		byID:      make(map[InodeID]string),
		byPath:    make(map[string]InodeID),
		parent:    make(map[InodeID]InodeID),
		transient: make(map[InodeID]transientFile),
	}
	rootID := lfs.statInodeID(root)
	lfs.rootID = rootID
	lfs.byID[rootID] = ""
	lfs.byPath[""] = rootID
	lfs.parent[rootID] = rootID // root is its own parent
	// Walk the directory tree to populate maps so that file handles from a
	// previous server instance remain valid after restart.
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		id := lfs.statInodeID(path)
		if id == 0 {
			return nil
		}
		parentRel := filepath.Dir(rel)
		if parentRel == "." {
			parentRel = ""
		}
		lfs.byID[id] = rel
		lfs.byPath[rel] = id
		lfs.parent[id] = lfs.byPath[parentRel]
		// Retained versions and private transients are user inodes.
		if strings.HasPrefix(rel, filepath.Join(NFSDirName, "snapshots")+string(os.PathSeparator)) ||
			strings.HasPrefix(rel, filepath.Join(NFSDirName, "transients")+string(os.PathSeparator)) {
			lfs.parent[id] = rootID
		}
		return nil
	})
	return lfs
}

func (lfs *LocalTernVFS) RootID() InodeID { return lfs.rootID }

func (lfs *LocalTernVFS) statInodeID(path string) InodeID {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return 0
	}
	var typ uint64
	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		typ = InodeTypeDir
	case syscall.S_IFLNK:
		typ = InodeTypeSymlink
	default:
		typ = InodeTypeFile
	}
	return MakeInodeID(typ, st.Ino)
}

func (lfs *LocalTernVFS) resolve(id InodeID) (string, bool) {
	lfs.mu.RLock()
	rel, ok := lfs.byID[id]
	lfs.mu.RUnlock()
	if !ok {
		return "", false
	}
	return filepath.Join(lfs.root, rel), true
}

func (lfs *LocalTernVFS) register(absPath string, parentID InodeID) InodeID {
	lfs.mu.Lock()
	defer lfs.mu.Unlock()
	id := lfs.statInodeID(absPath)
	if id == 0 {
		return 0
	}
	rel, _ := filepath.Rel(lfs.root, absPath)
	if rel == "." {
		rel = ""
	}
	lfs.byID[id] = rel
	lfs.byPath[rel] = id
	lfs.parent[id] = parentID
	return id
}

func (lfs *LocalTernVFS) Stat(id InodeID) (NodeInfo, error) {
	lfs.mu.RLock()
	defer lfs.mu.RUnlock()
	rel, ok := lfs.byID[id]
	if !ok {
		return NodeInfo{}, os.ErrNotExist
	}
	var st syscall.Stat_t
	if err := syscall.Lstat(filepath.Join(lfs.root, rel), &st); err != nil {
		return NodeInfo{}, err
	}
	return NodeInfo{
		Size:  uint64(st.Size),
		Mtime: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		Atime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
	}, nil
}

func (lfs *LocalTernVFS) Lookup(dirID InodeID, name string) (InodeID, error) {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return 0, os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)
	abs, err := filepath.Abs(childPath)
	if err != nil {
		return 0, err
	}
	rel, err := filepath.Rel(lfs.root, abs)
	if err != nil || len(rel) >= 2 && rel[:2] == ".." {
		return 0, os.ErrPermission
	}
	if _, err := os.Lstat(childPath); err != nil {
		return 0, err
	}
	return lfs.register(childPath, dirID), nil
}

func (lfs *LocalTernVFS) LookupParent(id InodeID) (InodeID, error) {
	lfs.mu.RLock()
	pid, ok := lfs.parent[id]
	lfs.mu.RUnlock()
	if !ok {
		return 0, os.ErrNotExist
	}
	return pid, nil
}

func (lfs *LocalTernVFS) Readdir(dirID InodeID, startHash uint64) ([]DirEntry, uint64, error) {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return nil, 0, os.ErrNotExist
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, 0, err
	}

	// Simulate TernFS hash-based ordering: use simple hash of name.
	// In real TernFS, NameHash is computed by the storage layer.
	type hashedEntry struct {
		name string
		hash uint64
		path string
	}
	var hashed []hashedEntry
	for _, e := range entries {
		h := hashName(e.Name())
		if h >= startHash {
			hashed = append(hashed, hashedEntry{
				name: e.Name(),
				hash: h,
				path: filepath.Join(dirPath, e.Name()),
			})
		}
	}

	// Sort by hash to match TernFS ordering.
	sort.Slice(hashed, func(i, j int) bool {
		if hashed[i].hash != hashed[j].hash {
			return hashed[i].hash < hashed[j].hash
		}
		return hashed[i].name < hashed[j].name
	})

	// Return a batch (simulate TernFS MTU limit).
	const batchSize = 16
	var result []DirEntry
	for i, he := range hashed {
		childID := lfs.register(he.path, dirID)
		if childID == 0 {
			continue
		}
		result = append(result, DirEntry{
			Name:     he.name,
			ID:       childID,
			NameHash: he.hash,
		})
		if len(result) >= batchSize {
			// Return NextHash from the next unprocessed entry.
			if i+1 < len(hashed) {
				return result, hashed[i+1].hash, nil
			}
			return result, 0, nil
		}
	}
	return result, 0, nil // EOF
}

// hashName produces a simple hash for local testing.
// Real TernFS uses its own hash function.
func hashName(name string) uint64 {
	// FNV-1a hash, offset to avoid reserved values 0-2.
	var h uint64 = 14695981039346656037
	for _, c := range []byte(name) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	// Ensure hash is >= 3 to avoid NFS reserved cookie values.
	if h < 3 {
		h += 3
	}
	return h
}

func (lfs *LocalTernVFS) Read(fileID InodeID, offset uint64, dest []byte) (int, bool, error) {
	lfs.mu.RLock()
	rel, ok := lfs.byID[fileID]
	if !ok {
		lfs.mu.RUnlock()
		return 0, false, os.ErrNotExist
	}
	f, err := os.Open(filepath.Join(lfs.root, rel))
	lfs.mu.RUnlock()
	if err != nil {
		return 0, false, err
	}
	defer f.Close()

	n, err := f.ReadAt(dest, int64(offset))
	eof := false
	if err != nil {
		if err.Error() == "EOF" || n < len(dest) {
			eof = true
		} else {
			return 0, false, err
		}
	}
	return n, eof, nil
}

func (lfs *LocalTernVFS) ReadAll(fileID InodeID) ([]byte, error) {
	path, ok := lfs.resolve(fileID)
	if !ok {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(path)
}

func (lfs *LocalTernVFS) Readlink(fileID InodeID) (string, error) {
	path, ok := lfs.resolve(fileID)
	if !ok {
		return "", os.ErrNotExist
	}
	return os.Readlink(path)
}

func (lfs *LocalTernVFS) Mkdir(dirID InodeID, name string) (InodeID, error) {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return 0, os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)
	if err := os.Mkdir(childPath, 0755); err != nil {
		return 0, err
	}
	return lfs.register(childPath, dirID), nil
}

func (lfs *LocalTernVFS) Symlink(dirID InodeID, name string, target string) (InodeID, error) {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return 0, os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)
	if err := os.Symlink(target, childPath); err != nil {
		return 0, err
	}
	return lfs.register(childPath, dirID), nil
}

func (lfs *LocalTernVFS) ConstructFile(dirID InodeID) (InodeID, Cookie, error) {
	// Transient inodes have no public pathname in TernFS.
	transients := filepath.Join(lfs.root, NFSDirName, "transients")
	if err := os.MkdirAll(transients, 0700); err != nil {
		return 0, Cookie{}, err
	}
	f, err := os.CreateTemp(transients, "file-*")
	if err != nil {
		return 0, Cookie{}, err
	}
	path := f.Name()
	f.Close()

	id := lfs.statInodeID(path)
	if id == 0 {
		os.Remove(path)
		return 0, Cookie{}, fmt.Errorf("failed to stat transient file")
	}

	var cookie Cookie
	rand.Read(cookie[:])

	rel, _ := filepath.Rel(lfs.root, path)
	lfs.mu.Lock()
	lfs.transient[id] = transientFile{cookie: cookie, path: path}
	lfs.byID[id] = rel
	lfs.mu.Unlock()

	return id, cookie, nil
}

func (lfs *LocalTernVFS) ScrapFile(fileID InodeID, cookie Cookie) error {
	lfs.mu.Lock()
	tf, ok := lfs.transient[fileID]
	if !ok {
		lfs.mu.Unlock()
		return os.ErrNotExist
	}
	if tf.cookie != cookie {
		lfs.mu.Unlock()
		return os.ErrPermission
	}
	delete(lfs.transient, fileID)
	delete(lfs.byID, fileID)
	lfs.mu.Unlock()
	return os.Remove(tf.path)
}

func (lfs *LocalTernVFS) LinkFile(fileID InodeID, cookie Cookie, dirID InodeID, name string, data io.Reader) error {
	lfs.mu.RLock()
	tf, ok := lfs.transient[fileID]
	lfs.mu.RUnlock()
	if !ok {
		return os.ErrNotExist
	}
	if tf.cookie != cookie {
		return os.ErrPermission
	}

	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)

	// Build the new inode off-path and preserve the old inode as a snapshot,
	// matching TernFS rather than mutating the published base in place.
	f, err := os.OpenFile(tf.path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if data != nil {
		if _, err := io.Copy(f, data); err != nil {
			f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	lfs.mu.Lock()
	defer lfs.mu.Unlock()
	if oldID := lfs.statInodeID(childPath); oldID != 0 {
		if err := lfs.snapshotLocked(childPath, oldID); err != nil {
			return err
		}
	}
	if err := os.Rename(tf.path, childPath); err != nil {
		return err
	}
	rel, _ := filepath.Rel(lfs.root, childPath)
	delete(lfs.transient, fileID)
	lfs.byID[fileID] = rel
	lfs.byPath[rel] = fileID
	lfs.parent[fileID] = dirID
	return nil
}

func (lfs *LocalTernVFS) CreateFile(dirID InodeID, name string, data io.Reader) (InodeID, error) {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return 0, os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)
	oldID := lfs.statInodeID(childPath)
	f, err := os.CreateTemp(dirPath, ".nfsd-create-")
	if err != nil {
		return 0, err
	}
	tempPath := f.Name()
	if data != nil {
		if _, err := io.Copy(f, data); err != nil {
			f.Close()
			os.Remove(tempPath)
			return 0, err
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return 0, err
	}
	if err := os.Rename(tempPath, childPath); err != nil {
		os.Remove(tempPath)
		return 0, err
	}
	if oldID != 0 {
		lfs.mu.Lock()
		delete(lfs.byID, oldID)
		delete(lfs.parent, oldID)
		lfs.mu.Unlock()
	}
	return lfs.register(childPath, dirID), nil
}

// snapshotLocked keeps a published inode readable by ID after its directory
// entry is removed or replaced, as a TernFS snapshot edge does until it is
// collected. The caller holds lfs.mu.
func (lfs *LocalTernVFS) snapshotLocked(path string, id InodeID) error {
	if id.Type() == InodeTypeDir {
		return nil
	}
	snapshotRel := filepath.Join(NFSDirName, "snapshots", fmt.Sprintf("%016x", uint64(id)))
	snapshotPath := filepath.Join(lfs.root, snapshotRel)
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0700); err != nil {
		return err
	}
	if err := os.Link(path, snapshotPath); err != nil && !os.IsExist(err) {
		return err
	}
	lfs.byID[id] = snapshotRel
	return nil
}

func (lfs *LocalTernVFS) LookupEdge(dirID InodeID, name string) (Edge, error) {
	id, err := lfs.Lookup(dirID, name)
	if err != nil {
		return Edge{}, err
	}
	return Edge{ID: id}, nil
}

func (lfs *LocalTernVFS) Remove(dirID InodeID, name string) error {
	return lfs.remove(dirID, name, nil)
}

func (lfs *LocalTernVFS) RemoveEdge(dirID InodeID, name string, edge Edge) error {
	return lfs.remove(dirID, name, &edge)
}

func (lfs *LocalTernVFS) remove(dirID InodeID, name string, pin *Edge) error {
	dirPath, ok := lfs.resolve(dirID)
	if !ok {
		return os.ErrNotExist
	}
	childPath := filepath.Join(dirPath, name)
	lfs.mu.Lock()
	defer lfs.mu.Unlock()
	// Check what it is to update our maps.
	childID := lfs.statInodeID(childPath)
	if pin != nil && childID != pin.ID {
		return os.ErrNotExist
	}
	if childID == 0 {
		return os.ErrNotExist
	}
	if err := lfs.snapshotLocked(childPath, childID); err != nil {
		return err
	}
	if err := os.Remove(childPath); err != nil {
		return err
	}
	rel, _ := filepath.Rel(lfs.root, childPath)
	if childID.Type() == InodeTypeDir {
		delete(lfs.byID, childID)
	}
	delete(lfs.byPath, rel)
	delete(lfs.parent, childID)
	return nil
}

func (lfs *LocalTernVFS) Rename(srcDirID InodeID, srcName string, dstDirID InodeID, dstName string) error {
	return lfs.rename(srcDirID, srcName, nil, dstDirID, dstName)
}

func (lfs *LocalTernVFS) RenameEdge(srcDirID InodeID, srcName string, edge Edge, dstDirID InodeID, dstName string) error {
	return lfs.rename(srcDirID, srcName, &edge, dstDirID, dstName)
}

func (lfs *LocalTernVFS) rename(srcDirID InodeID, srcName string, pin *Edge, dstDirID InodeID, dstName string) error {
	srcDir, ok := lfs.resolve(srcDirID)
	if !ok {
		return os.ErrNotExist
	}
	dstDir, ok := lfs.resolve(dstDirID)
	if !ok {
		return os.ErrNotExist
	}
	srcPath := filepath.Join(srcDir, srcName)
	dstPath := filepath.Join(dstDir, dstName)

	lfs.mu.Lock()
	srcID := lfs.statInodeID(srcPath)
	if pin != nil && srcID != pin.ID {
		lfs.mu.Unlock()
		return os.ErrNotExist
	}
	if srcID == 0 {
		lfs.mu.Unlock()
		return os.ErrNotExist
	}
	if dstID := lfs.statInodeID(dstPath); dstID != 0 && dstID != srcID {
		if err := lfs.snapshotLocked(dstPath, dstID); err != nil {
			lfs.mu.Unlock()
			return err
		}
	}
	if err := os.Rename(srcPath, dstPath); err != nil {
		lfs.mu.Unlock()
		return err
	}

	// Update maps: remove old path, register new path.
	srcRel, _ := filepath.Rel(lfs.root, srcPath)
	delete(lfs.byID, srcID)
	delete(lfs.byPath, srcRel)
	delete(lfs.parent, srcID)
	lfs.mu.Unlock()

	lfs.register(dstPath, dstDirID)
	return nil
}

func (lfs *LocalTernVFS) SetTime(id InodeID, mtime *time.Time, atime *time.Time) error {
	lfs.mu.RLock()
	defer lfs.mu.RUnlock()
	rel, ok := lfs.byID[id]
	if !ok {
		return os.ErrNotExist
	}
	path := filepath.Join(lfs.root, rel)
	// Read current times to preserve unchanged fields.
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return err
	}
	at := time.Unix(st.Atim.Sec, st.Atim.Nsec)
	mt := time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
	if atime != nil {
		at = *atime
	}
	if mtime != nil {
		mt = *mtime
	}
	return os.Chtimes(path, at, mt)
}

func (lfs *LocalTernVFS) ClassifyMutation(op uint32, err error) MutationOutcome {
	if err == nil {
		return MutationApplied
	}
	if op == nfs4.OP_REMOVE && errors.Is(err, os.ErrNotExist) {
		return MutationDetachOnly
	}
	// Local syscalls are not retransmitted. Only their errors establish a
	// definite rejection; injected transport or decoding errors do not.
	var pathErr *os.PathError
	var linkErr *os.LinkError
	var errno syscall.Errno
	if errors.As(err, &pathErr) || errors.As(err, &linkErr) || errors.As(err, &errno) ||
		errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrExist) || errors.Is(err, os.ErrPermission) {
		return MutationNotApplied
	}
	return MutationUnknown
}
