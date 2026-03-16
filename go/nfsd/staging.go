// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// StagingMeta holds metadata for an in-progress write, persisted in a
// sidecar file alongside the staging data. This is the only persistent
// record of an NFS write-open; the NFS server derives open stateids from
// the NFSStateID stored here rather than keeping separate open state.
type StagingMeta struct {
	DirID      InodeID // directory to link the file into on CLOSE
	FileName   string  // name in directory
	TernCookie Cookie  // cookie from VFS ConstructFile
	NFSStateID StateID // random, returned to NFS client as stateid "other"
}

// StagingFile is the interface for staged file data during NFS writes.
// Files are immutable after creation, so staging only applies to new files.
// The store owns the file lifecycle — there is no Close method.
type StagingFile interface {
	SetSize(size uint64) error
	Write(offset uint64, data []byte) error
	Read(offset uint64, dest []byte) (n int, eof bool, err error)
	Sync() error                    // persist to stable storage
	Reader() (io.ReadSeeker, error) // returns a reader positioned at offset 0
}

// StagingStore manages staging files keyed by InodeID.
// All staging queries go through this interface — the NFS server
// does not maintain its own staging caches.
type StagingStore interface {
	// ReadOnly returns true if no staging directory is configured.
	ReadOnly() bool
	// Create creates a new staging file with associated metadata.
	Create(id InodeID, meta StagingMeta) (StagingFile, error)
	// Get returns the staging file for the given inode, or nil.
	Get(id InodeID) StagingFile
	// GetMeta returns the metadata sidecar for the given inode.
	GetMeta(id InodeID) (StagingMeta, bool)
	// Remove closes and removes the staging file and sidecar for the given inode.
	Remove(id InodeID)
	// StagedSize returns the staged size for the given inode.
	// Returns (0, false) if the inode has no staging file.
	StagedSize(id InodeID) (uint64, bool)
	// StagedSizes returns a snapshot of all staged InodeID → size.
	StagedSizes() map[InodeID]uint64
}

// LocalStagingStore manages staging files as local files in a directory.
// File names encode the InodeID so state can be recovered on restart.
type LocalStagingStore struct {
	mu    sync.Mutex
	dir   string
	files map[InodeID]*localStagingEntry
	log   *slog.Logger
}

type localStagingEntry struct {
	file *localStagingFile
	meta StagingMeta
}

func NewLocalStagingStore(dir string, logger *slog.Logger) (*LocalStagingStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &LocalStagingStore{
		dir:   dir,
		files: make(map[InodeID]*localStagingEntry),
		log:   logger,
	}
	// Scan the staging directory and re-register any staging files
	// left from a previous run.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return s, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".staging") {
			continue
		}
		idStr := strings.TrimSuffix(name, ".staging")
		idVal, err := strconv.ParseUint(idStr, 16, 64)
		if err != nil {
			continue
		}
		id := InodeID(idVal)
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR, 0644)
		if err != nil {
			s.log.Warn("staging recover: cannot open file", "path", path, "err", err)
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		entry := &localStagingEntry{
			file: &localStagingFile{f: f, size: uint64(info.Size())},
		}
		// Try to load sidecar metadata.
		metaPath := filepath.Join(dir, fmt.Sprintf("%016x.meta", uint64(id)))
		if meta, err := loadStagingMeta(metaPath); err == nil {
			entry.meta = meta
			s.log.Info("staging recover", "file", name, "inode", fmt.Sprintf("%016x", uint64(id)), "size", info.Size())
		} else {
			s.log.Info("staging recover (no meta)", "file", name, "inode", fmt.Sprintf("%016x", uint64(id)), "size", info.Size())
		}
		s.files[id] = entry
	}
	return s, nil
}

func (s *LocalStagingStore) ReadOnly() bool { return false }

func (s *LocalStagingStore) Create(id InodeID, meta StagingMeta) (StagingFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.files[id]; ok {
		return entry.file, nil // already exists (recovered or duplicate create)
	}
	path := filepath.Join(s.dir, fmt.Sprintf("%016x.staging", uint64(id)))
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	// Write sidecar metadata.
	metaPath := filepath.Join(s.dir, fmt.Sprintf("%016x.meta", uint64(id)))
	if err := saveStagingMeta(metaPath, meta); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	sf := &localStagingFile{f: f}
	s.files[id] = &localStagingEntry{file: sf, meta: meta}
	return sf, nil
}

func (s *LocalStagingStore) Get(id InodeID) StagingFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[id]
	if entry == nil {
		return nil
	}
	return entry.file
}

func (s *LocalStagingStore) GetMeta(id InodeID) (StagingMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.files[id]
	if !ok {
		return StagingMeta{}, false
	}
	return entry.meta, true
}

func (s *LocalStagingStore) Remove(id InodeID) {
	s.mu.Lock()
	entry := s.files[id]
	delete(s.files, id)
	s.mu.Unlock()
	if entry != nil {
		name := entry.file.f.Name()
		entry.file.f.Close()
		os.Remove(name)
		metaPath := strings.TrimSuffix(name, ".staging") + ".meta"
		os.Remove(metaPath)
	}
}

func (s *LocalStagingStore) StagedSize(id InodeID) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.files[id]
	if !ok {
		return 0, false
	}
	return entry.file.size, true
}

func (s *LocalStagingStore) StagedSizes() map[InodeID]uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.files) == 0 {
		return nil
	}
	m := make(map[InodeID]uint64, len(s.files))
	for id, entry := range s.files {
		m[id] = entry.file.size
	}
	return m
}

// readOnlyStagingStore is used when no staging directory is configured.
type readOnlyStagingStore struct{}

func (readOnlyStagingStore) ReadOnly() bool { return true }
func (readOnlyStagingStore) Create(InodeID, StagingMeta) (StagingFile, error) {
	return nil, os.ErrPermission
}
func (readOnlyStagingStore) Get(InodeID) StagingFile             { return nil }
func (readOnlyStagingStore) GetMeta(InodeID) (StagingMeta, bool) { return StagingMeta{}, false }
func (readOnlyStagingStore) Remove(InodeID)                      {}
func (readOnlyStagingStore) StagedSize(InodeID) (uint64, bool)   { return 0, false }
func (readOnlyStagingStore) StagedSizes() map[InodeID]uint64     { return nil }

// localStagingFile is a disk-backed staging file for new file creation.
type localStagingFile struct {
	f    *os.File
	size uint64
}

func (sf *localStagingFile) SetSize(size uint64) error {
	if err := sf.f.Truncate(int64(size)); err != nil {
		return err
	}
	sf.size = size
	return nil
}

func (sf *localStagingFile) Write(offset uint64, data []byte) error {
	end := offset + uint64(len(data))
	if end > sf.size {
		if err := sf.f.Truncate(int64(end)); err != nil {
			return err
		}
		sf.size = end
	}
	if _, err := sf.f.WriteAt(data, int64(offset)); err != nil {
		return err
	}
	return nil
}

func (sf *localStagingFile) Read(offset uint64, dest []byte) (int, bool, error) {
	if offset >= sf.size {
		return 0, true, nil
	}
	avail := sf.size - offset
	if uint64(len(dest)) > avail {
		dest = dest[:avail]
	}
	n, err := sf.f.ReadAt(dest, int64(offset))
	if err != nil && n == 0 {
		return 0, false, err
	}
	eof := offset+uint64(n) >= sf.size
	return n, eof, nil
}

func (sf *localStagingFile) Sync() error {
	return sf.f.Sync()
}

func (sf *localStagingFile) Reader() (io.ReadSeeker, error) {
	if _, err := sf.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return sf.f, nil
}

// Sidecar file format (binary, big-endian):
//   [8]  DirID
//   [8]  TernCookie
//   [12] NFSStateID
//   [2]  FileNameLen
//   [N]  FileName (UTF-8)

func saveStagingMeta(path string, meta StagingMeta) error {
	nameBytes := []byte(meta.FileName)
	buf := make([]byte, 8+8+12+2+len(nameBytes))
	binary.BigEndian.PutUint64(buf[0:8], uint64(meta.DirID))
	copy(buf[8:16], meta.TernCookie[:])
	copy(buf[16:28], meta.NFSStateID[:])
	binary.BigEndian.PutUint16(buf[28:30], uint16(len(nameBytes)))
	copy(buf[30:], nameBytes)
	return os.WriteFile(path, buf, 0600)
}

func loadStagingMeta(path string) (StagingMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StagingMeta{}, err
	}
	if len(data) < 30 {
		return StagingMeta{}, fmt.Errorf("meta file too short")
	}
	var meta StagingMeta
	meta.DirID = InodeID(binary.BigEndian.Uint64(data[0:8]))
	copy(meta.TernCookie[:], data[8:16])
	copy(meta.NFSStateID[:], data[16:28])
	nameLen := binary.BigEndian.Uint16(data[28:30])
	if len(data) < 30+int(nameLen) {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	meta.FileName = string(data[30 : 30+nameLen])
	return meta, nil
}

// newNFSStateID generates a random 12-byte NFS state ID for write opens.
func newNFSStateID() StateID {
	var sid StateID
	rand.Read(sid[:])
	return sid
}
