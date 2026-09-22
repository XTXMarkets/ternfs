// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stagingMetaV4Magic = "NFS4"
)

const stagingHydrationChunk = 1 << 20
const finishHydrationWorkers = 4

// stagingQuarantineDirName is the subdirectory of the staging directory which
// holds checkpoints whose metadata sidecar would not decode. Recovery moves
// them aside rather than deleting them; nfsd inspect lists them.
const stagingQuarantineDirName = "quarantine"

var errStagingRemoved = errors.New("staging file was removed")

type stagingTarget struct {
	dirID InodeID
	name  string
}

// StagingMeta contains the state needed to complete CLOSE after a restart.
type StagingMeta struct {
	DirID           InodeID  // directory to link the file into on CLOSE
	FileName        string   // name in directory
	TernCookie      Cookie   // cookie from VFS ConstructFile
	NFSStateID      StateID  // random, returned to NFS client as stateid "other"
	ClientID        uint64   // owning client; zero in sidecars written by older nfsd
	OpenOwner       string   // separates recovered writers belonging to the same client
	OwnerKnown      bool     // an empty owner is valid; legacy sidecars have no identity
	RecoveryKey     [32]byte // stable client identity, boot verifier and principal
	Retired         bool     // lease expired; retain acknowledged data for recovery
	ReadOnly        bool     // CREATE may initialize size/times with read-only share access
	BaseID          InodeID  // immutable file being replaced; zero for a new file
	BaseSize        uint64
	Size            uint64
	Dirty           byteRangeSet
	MetadataChanged bool
	Attrs           NodeInfo // Size is stored separately; zero Change denotes legacy metadata
	version         uint8
}

type stagingHydrationReader func(cancel <-chan struct{}, fileID InodeID, offset uint64, dest []byte) (int, bool, error)

type stagingBaseReader func(
	fileID InodeID,
	offset uint64,
	dest []byte,
) (n int, eof bool, err error)

// StagingFile is the interface for locally staged file data during NFS writes.
// The store owns the file lifecycle; there is no Close method.
type StagingFile interface {
	Stat() NodeInfo
	SetTime(mtime, atime *time.Time) error
	SetSize(size uint64) error
	Write(offset uint64, data []byte) error
	Read(offset uint64, dest []byte, readBase stagingBaseReader) (n int, eof bool, err error)
	Cache(offset uint64, data []byte) error
	Base() (id InodeID, size uint64)
	Dirty() bool
	DataChanged() bool
	Retire(clientID uint64, stateID StateID) error
	StartHydration(readBase stagingHydrationReader)
	FinishHydration(readBase stagingBaseReader) error
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
	// FindTargets returns the staging sessions indexed by dirID/name.
	FindTargets(dirID InodeID, name string) map[InodeID]StagingMeta
	// Entries returns a snapshot of all staging metadata keyed by staging inode.
	Entries() map[InodeID]StagingMeta
	// Rebind updates the owning client and stateid of a recovered staging file.
	Rebind(id InodeID, clientID uint64, stateID StateID) error
	// TargetBusy reports whether a staged file will publish at dirID/name.
	TargetBusy(dirID InodeID, name string) bool
	// Remove closes and removes the staging file and sidecar for the given inode.
	Remove(id InodeID)
	// StagedSize returns the staged size for the given inode.
	// Returns (0, false) if the inode has no staging file.
	StagedSize(id InodeID) (uint64, bool)
}

// LocalStagingStore manages staging files as local files in a directory.
// File names encode the InodeID so state can be recovered on restart.
type LocalStagingStore struct {
	mu      sync.Mutex
	dir     string
	files   map[InodeID]*localStagingEntry
	targets map[stagingTarget]map[InodeID]struct{}
	log     *slog.Logger
}

type localStagingEntry struct {
	file   *localStagingFile
	target stagingTarget
}

func NewLocalStagingStore(dir string, logger *slog.Logger) (*LocalStagingStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &LocalStagingStore{
		dir:     dir,
		files:   make(map[InodeID]*localStagingEntry),
		targets: make(map[stagingTarget]map[InodeID]struct{}),
		log:     logger,
	}
	// Scan the staging directory and re-register any staging files
	// left from a previous run.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") &&
			strings.Contains(name, ".meta.tmp-") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				s.log.Warn("staging recover: cannot remove temporary metadata",
					"file", name, "err", err)
			}
			continue
		}
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
		metaPath := filepath.Join(dir, fmt.Sprintf("%016x.meta", uint64(id)))
		meta, metaErr := loadStagingMeta(metaPath)
		if metaErr != nil {
			f.Close()
			// Invalid metadata cannot establish ownership or a publication
			// target. Keep the bytes for manual recovery, outside the index.
			quarantine := filepath.Join(dir, stagingQuarantineDirName)
			if err := os.MkdirAll(quarantine, 0700); err != nil {
				return nil, err
			}
			quarantine, err = os.MkdirTemp(quarantine, idStr+"-")
			if err != nil {
				return nil, err
			}
			if err := os.Rename(path, filepath.Join(quarantine, name)); err != nil {
				return nil, err
			}
			if err := os.Rename(metaPath, filepath.Join(quarantine, filepath.Base(metaPath))); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			s.log.Warn("staging recover: quarantined invalid checkpoint", "file", name, "err", metaErr)
			continue
		}
		size := uint64(info.Size())
		if meta.version == 4 && meta.BaseID != 0 {
			size = meta.Size
			if err := f.Truncate(int64(size)); err != nil {
				f.Close()
				s.log.Warn("staging recover: cannot restore size",
					"path", path, "size", size, "err", err)
				continue
			}
		}
		target := stagingTarget{dirID: meta.DirID, name: meta.FileName}
		entry := &localStagingEntry{
			file: &localStagingFile{
				f:               f,
				size:            size,
				meta:            meta,
				metaPath:        metaPath,
				dirty:           append(byteRangeSet(nil), meta.Dirty...),
				attrs:           stagingAttributes(meta.Attrs, info.ModTime()),
				metadataChanged: meta.MetadataChanged,
			},
			target: target,
		}
		if s.targets[target] == nil {
			s.targets[target] = make(map[InodeID]struct{})
		}
		s.targets[target][id] = struct{}{}
		if meta.Retired {
			_ = f.Close()
		}
		s.log.Info("staging recover", "file", name, "inode", fmt.Sprintf("%016x", uint64(id)), "size", size)
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
	target := stagingTarget{dirID: meta.DirID, name: meta.FileName}
	meta.Size = meta.BaseSize
	meta.version = 4
	meta.Attrs = stagingAttributes(meta.Attrs, time.Now())
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
	if meta.BaseID != 0 {
		if err := f.Truncate(int64(meta.BaseSize)); err != nil {
			f.Close()
			os.Remove(path)
			os.Remove(metaPath)
			return nil, err
		}
	}
	sf := &localStagingFile{
		f:        f,
		size:     meta.BaseSize,
		meta:     meta,
		metaPath: metaPath,
		dirty:    append(byteRangeSet(nil), meta.Dirty...),
		attrs:    meta.Attrs,
	}
	s.files[id] = &localStagingEntry{
		file:   sf,
		target: target,
	}
	if s.targets[target] == nil {
		s.targets[target] = make(map[InodeID]struct{})
	}
	s.targets[target][id] = struct{}{}
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
	entry := s.files[id]
	if entry == nil {
		return StagingMeta{}, false
	}
	return entry.file.Meta(), true
}

func (s *LocalStagingStore) FindTargets(dirID InodeID, name string) map[InodeID]StagingMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.targets[stagingTarget{dirID: dirID, name: name}]
	result := make(map[InodeID]StagingMeta, len(ids))
	for id := range ids {
		result[id] = s.files[id].file.Meta()
	}
	return result
}

func (s *LocalStagingStore) Entries() map[InodeID]StagingMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := make(map[InodeID]StagingMeta, len(s.files))
	for id, entry := range s.files {
		entries[id] = entry.file.Meta()
	}
	return entries
}

func (s *LocalStagingStore) Rebind(
	id InodeID,
	clientID uint64,
	stateID StateID,
) error {
	s.mu.Lock()
	entry := s.files[id]
	if entry == nil {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	file := entry.file
	s.mu.Unlock()
	return file.rebind(clientID, stateID)
}

func (s *LocalStagingStore) TargetBusy(dirID InodeID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.targets[stagingTarget{dirID: dirID, name: name}] {
		if !s.files[id].file.Meta().Retired {
			return true
		}
	}
	return false
}

func (s *LocalStagingStore) Remove(id InodeID) {
	s.mu.Lock()
	entry := s.files[id]
	delete(s.files, id)
	if entry != nil {
		if ids := s.targets[entry.target]; ids != nil {
			delete(ids, id)
			if len(ids) == 0 {
				delete(s.targets, entry.target)
			}
		}
	}
	s.mu.Unlock()
	if entry != nil {
		entry.file.prepareRemove()
		name := entry.file.f.Name()
		os.Remove(name)
		metaPath := strings.TrimSuffix(name, ".staging") + ".meta"
		os.Remove(metaPath)
	}
}

func (s *LocalStagingStore) StagedSize(id InodeID) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[id]
	if entry == nil {
		return 0, false
	}
	return entry.file.Size(), true
}

// readOnlyStagingStore is used when no staging directory is configured.
type readOnlyStagingStore struct{}

func (readOnlyStagingStore) ReadOnly() bool { return true }
func (readOnlyStagingStore) Create(InodeID, StagingMeta) (StagingFile, error) {
	return nil, os.ErrPermission
}
func (readOnlyStagingStore) Get(InodeID) StagingFile                             { return nil }
func (readOnlyStagingStore) GetMeta(InodeID) (StagingMeta, bool)                 { return StagingMeta{}, false }
func (readOnlyStagingStore) FindTargets(InodeID, string) map[InodeID]StagingMeta { return nil }
func (readOnlyStagingStore) Entries() map[InodeID]StagingMeta                    { return nil }
func (readOnlyStagingStore) Rebind(InodeID, uint64, StateID) error {
	return os.ErrPermission
}
func (readOnlyStagingStore) TargetBusy(InodeID, string) bool   { return false }
func (readOnlyStagingStore) Remove(InodeID)                    {}
func (readOnlyStagingStore) StagedSize(InodeID) (uint64, bool) { return 0, false }

// localStagingFile is a disk-backed staging file. For replacement files,
// dirty identifies authoritative local bytes and hydrated identifies clean
// base bytes which have already been cached locally.
type localStagingFile struct {
	mu              sync.Mutex
	f               *os.File
	size            uint64
	meta            StagingMeta
	metaPath        string
	dirty           byteRangeSet
	attrs           NodeInfo
	metadataChanged bool
	hydrated        byteRangeSet
	removed         bool

	hydrateMu      sync.Mutex
	hydrateDone    chan struct{}
	hydrateCancel  chan struct{}
	hydrateRemoved bool
}

func stagingAttributes(info NodeInfo, fallback time.Time) NodeInfo {
	info.Size = 0
	if info.Mtime.IsZero() {
		info.Mtime = fallback
	}
	if info.Atime.IsZero() {
		info.Atime = info.Mtime
	}
	if info.Ctime.IsZero() {
		info.Ctime = info.Mtime
	}
	if info.Change == 0 {
		info.Change = uint64(info.Mtime.UnixNano())
	}
	return info
}

func (sf *localStagingFile) Stat() NodeInfo {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	info := sf.attrs
	info.Size = sf.size
	return info
}

func (sf *localStagingFile) changedLocked() {
	now := time.Now()
	sf.attrs.Change = max(sf.attrs.Change+1, uint64(now.UnixNano()))
	sf.attrs.Ctime = now
}

func (sf *localStagingFile) SetTime(mtime, atime *time.Time) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	if mtime != nil {
		sf.attrs.Mtime = *mtime
	}
	if atime != nil {
		sf.attrs.Atime = *atime
	}
	sf.changedLocked()
	sf.metadataChanged = true
	return nil
}

func (sf *localStagingFile) Size() uint64 {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.size
}

func (sf *localStagingFile) Meta() StagingMeta {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	meta := sf.meta
	meta.Dirty = append(byteRangeSet(nil), sf.dirty...)
	return meta
}

func (sf *localStagingFile) Base() (InodeID, uint64) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.meta.BaseID, sf.meta.BaseSize
}

func (sf *localStagingFile) Dirty() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.metadataChanged || len(sf.dirty) != 0 || sf.size != sf.meta.BaseSize
}

func (sf *localStagingFile) DataChanged() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return len(sf.dirty) != 0 || sf.size != sf.meta.BaseSize
}

func (sf *localStagingFile) Retire(clientID uint64, stateID StateID) error {
	sf.hydrateMu.Lock()
	done := sf.cancelHydrationLocked()
	sf.hydrateMu.Unlock()
	if done != nil {
		<-done
	}
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	// The lease check may race a recovery OPEN which rebinds this file.
	if sf.meta.ClientID != clientID || sf.meta.NFSStateID != stateID || sf.meta.Retired {
		return nil
	}
	if err := sf.f.Sync(); err != nil {
		return err
	}
	previous := sf.meta
	sf.meta.Retired = true
	if err := sf.saveMetaLocked(); err != nil {
		sf.meta = previous
		return err
	}
	return sf.f.Close()
}

func (sf *localStagingFile) saveMetaLocked() error {
	meta := sf.meta
	meta.Dirty = append(byteRangeSet(nil), sf.dirty...)
	meta.Size = sf.size
	meta.Attrs = sf.attrs
	meta.MetadataChanged = sf.metadataChanged
	meta.version = 4
	if err := saveStagingMeta(sf.metaPath, meta); err != nil {
		return err
	}
	// Only a successfully persisted checkpoint can suppress a later retry.
	sf.meta = meta
	return nil
}

func (sf *localStagingFile) checkpointChangedLocked() bool {
	if sf.meta.MetadataChanged != sf.metadataChanged ||
		sf.meta.Attrs != sf.attrs || sf.meta.Size != sf.size ||
		len(sf.meta.Dirty) != len(sf.dirty) {
		return true
	}
	for i := range sf.dirty {
		if sf.meta.Dirty[i] != sf.dirty[i] {
			return true
		}
	}
	return false
}

func (sf *localStagingFile) rebind(
	clientID uint64,
	stateID StateID,
) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	oldMeta := sf.meta
	if oldMeta.Retired {
		f, err := os.OpenFile(sf.f.Name(), os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		sf.f = f
	}
	sf.meta.ClientID = clientID
	sf.meta.NFSStateID = stateID
	sf.meta.Retired = false
	err := saveStagingMeta(sf.metaPath, sf.meta)
	if err == nil {
		return nil
	}

	sf.meta = oldMeta
	if oldMeta.Retired {
		_ = sf.f.Close()
	}
	restoreErr := saveStagingMeta(sf.metaPath, oldMeta)
	if restoreErr != nil {
		restoreErr = fmt.Errorf(
			"restore previous staging owner: %w", restoreErr,
		)
	}
	return errors.Join(err, restoreErr)
}

func (sf *localStagingFile) SetSize(size uint64) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	oldSize := sf.size
	if size == oldSize {
		return nil
	}
	if err := sf.f.Truncate(int64(size)); err != nil {
		return err
	}
	sf.size = size
	sf.changedLocked()
	sf.attrs.Mtime = sf.attrs.Ctime
	if sf.meta.BaseID == 0 {
		return nil
	}
	if size < oldSize {
		sf.dirty.add(size, oldSize)
		sf.hydrated.remove(size, oldSize)
	} else {
		sf.dirty.add(oldSize, size)
	}
	return nil
}

func (sf *localStagingFile) Write(offset uint64, data []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	if len(data) == 0 {
		return nil
	}
	end := offset + uint64(len(data))
	if end < offset {
		return fmt.Errorf("staging write offset overflow")
	}
	if end > sf.size {
		if err := sf.f.Truncate(int64(end)); err != nil {
			return err
		}
		sf.size = end
	}
	if _, err := sf.f.WriteAt(data, int64(offset)); err != nil {
		return err
	}
	sf.changedLocked()
	sf.attrs.Mtime = sf.attrs.Ctime
	if sf.meta.BaseID == 0 {
		return nil
	}
	sf.dirty.add(offset, end)
	sf.hydrated.remove(offset, end)
	return nil
}

func (sf *localStagingFile) Read(
	offset uint64,
	dest []byte,
	readBase stagingBaseReader,
) (int, bool, error) {
	sf.mu.Lock()
	if sf.removed {
		sf.mu.Unlock()
		return 0, false, errStagingRemoved
	}
	if offset >= sf.size {
		sf.mu.Unlock()
		return 0, true, nil
	}
	avail := sf.size - offset
	if uint64(len(dest)) > avail {
		dest = dest[:avail]
	}
	end := offset + uint64(len(dest))
	if sf.meta.BaseID == 0 {
		n, err := sf.f.ReadAt(dest, int64(offset))
		eof := offset+uint64(n) >= sf.size
		sf.mu.Unlock()
		if err != nil && err != io.EOF {
			return n, false, err
		}
		return n, eof, nil
	}
	clear(dest)

	baseID := sf.meta.BaseID
	baseSize := sf.meta.BaseSize
	coverage := sf.localCoverageLocked()
	missing := coverage.gaps(offset, min(end, baseSize))
	sf.mu.Unlock()

	for _, gap := range missing {
		part := dest[gap.start-offset : gap.end-offset]
		if err := readStagingBase(readBase, baseID, gap.start, part); err != nil {
			return 0, false, err
		}
	}

	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return 0, false, errStagingRemoved
	}
	for _, gap := range missing {
		part := dest[gap.start-offset : gap.end-offset]
		if err := sf.cacheLocked(gap.start, part); err != nil {
			return 0, false, err
		}
	}
	if offset >= sf.size {
		return 0, true, nil
	}
	if uint64(len(dest)) > sf.size-offset {
		dest = dest[:sf.size-offset]
		end = sf.size
	}
	if err := sf.readLocalLocked(offset, dest); err != nil {
		return 0, false, err
	}
	return len(dest), end >= sf.size, nil
}

func readStagingBase(
	readBase stagingBaseReader,
	baseID InodeID,
	offset uint64,
	dest []byte,
) error {
	if readBase == nil {
		return fmt.Errorf("staging base reader is not configured")
	}
	for len(dest) != 0 {
		n, eof, err := readBase(baseID, offset, dest)
		if err != nil {
			return err
		}
		if n < 0 || n > len(dest) {
			return fmt.Errorf("staging base reader returned invalid count %d", n)
		}
		offset += uint64(n)
		dest = dest[n:]
		if len(dest) == 0 {
			return nil
		}
		if eof || n == 0 {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

func (sf *localStagingFile) localCoverageLocked() byteRangeSet {
	return sf.dirty.union(sf.hydrated)
}

func (sf *localStagingFile) readLocalLocked(offset uint64, dest []byte) error {
	if len(dest) == 0 {
		return nil
	}
	n, err := sf.f.ReadAt(dest, int64(offset))
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(dest) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (sf *localStagingFile) Cache(offset uint64, data []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	return sf.cacheLocked(offset, data)
}

func (sf *localStagingFile) cacheLocked(offset uint64, data []byte) error {
	if sf.meta.BaseID == 0 || offset >= sf.size || offset >= sf.meta.BaseSize {
		return nil
	}
	end := offset + uint64(len(data))
	if end < offset {
		return fmt.Errorf("staging cache offset overflow")
	}
	end = min(end, sf.size, sf.meta.BaseSize)
	for _, clean := range sf.dirty.gaps(offset, end) {
		part := data[clean.start-offset : clean.end-offset]
		if _, err := sf.f.WriteAt(part, int64(clean.start)); err != nil {
			return err
		}
		sf.hydrated.add(clean.start, clean.end)
	}
	return nil
}

func (sf *localStagingFile) StartHydration(readBase stagingHydrationReader) {
	sf.hydrateMu.Lock()
	if sf.hydrateDone != nil || sf.hydrateRemoved {
		sf.hydrateMu.Unlock()
		return
	}
	done := make(chan struct{})
	cancel := make(chan struct{})
	sf.hydrateDone = done
	sf.hydrateCancel = cancel
	sf.hydrateMu.Unlock()

	go func() {
		_ = sf.hydrate(func(id InodeID, offset uint64, dest []byte) (int, bool, error) {
			return readBase(cancel, id, offset, dest)
		}, cancel)
		close(done)
	}()
}

func (sf *localStagingFile) hydrate(
	readBase stagingBaseReader,
	cancel <-chan struct{},
) error {
	buf := make([]byte, stagingHydrationChunk)
	for {
		sf.mu.Lock()
		baseID := sf.meta.BaseID
		end := min(sf.size, sf.meta.BaseSize)
		gaps := sf.localCoverageLocked().gaps(0, end)
		sf.mu.Unlock()
		if baseID == 0 || len(gaps) == 0 {
			return nil
		}
		if cancel != nil {
			select {
			case <-cancel:
				return nil
			default:
			}
		}
		gap := gaps[0]
		count := min(uint64(len(buf)), gap.end-gap.start)
		part := buf[:count]
		if err := readStagingBase(readBase, baseID, gap.start, part); err != nil {
			return err
		}
		if err := sf.Cache(gap.start, part); err != nil {
			return err
		}
	}
}

func (sf *localStagingFile) cancelHydrationLocked() chan struct{} {
	if sf.hydrateCancel != nil {
		close(sf.hydrateCancel)
		sf.hydrateCancel = nil
	}
	return sf.hydrateDone
}

func (sf *localStagingFile) FinishHydration(
	readBase stagingBaseReader,
) error {
	sf.hydrateMu.Lock()
	done := sf.cancelHydrationLocked()
	sf.hydrateMu.Unlock()
	if done != nil {
		<-done
	}
	return sf.finishHydration(readBase)
}

func (sf *localStagingFile) finishHydration(
	readBase stagingBaseReader,
) error {
	for {
		sf.mu.Lock()
		baseID := sf.meta.BaseID
		end := min(sf.size, sf.meta.BaseSize)
		gaps := sf.localCoverageLocked().gaps(0, end)
		sf.mu.Unlock()
		if baseID == 0 || len(gaps) == 0 {
			return nil
		}

		jobs := make(chan byteRange)
		var wg sync.WaitGroup
		var errMu sync.Mutex
		var firstErr error
		for range finishHydrationWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf := make([]byte, stagingHydrationChunk)
				for job := range jobs {
					errMu.Lock()
					failed := firstErr != nil
					errMu.Unlock()
					if failed {
						continue
					}
					part := buf[:job.end-job.start]
					err := readStagingBase(
						readBase, baseID, job.start, part,
					)
					if err == nil {
						err = sf.Cache(job.start, part)
					}
					if err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					}
				}
			}()
		}
		for _, gap := range gaps {
			for start := gap.start; start < gap.end; {
				end := min(start+stagingHydrationChunk, gap.end)
				jobs <- byteRange{start: start, end: end}
				start = end
			}
		}
		close(jobs)
		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}
}

func (sf *localStagingFile) prepareRemove() {
	sf.hydrateMu.Lock()
	sf.hydrateRemoved = true
	done := sf.cancelHydrationLocked()
	sf.hydrateMu.Unlock()
	if done != nil {
		<-done
	}
	sf.mu.Lock()
	sf.removed = true
	_ = sf.f.Close()
	sf.mu.Unlock()
}

func (sf *localStagingFile) Sync() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return errStagingRemoved
	}
	if err := sf.f.Sync(); err != nil {
		return err
	}
	if !sf.checkpointChangedLocked() {
		return nil
	}
	return sf.saveMetaLocked()
}

func syncStagingDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (sf *localStagingFile) Reader() (io.ReadSeeker, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return nil, errStagingRemoved
	}
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
//   [8]  ClientID (absent in sidecars written by older nfsd)
//   [4]  "NFS4" (absent in sidecars written by older nfsd)
//   [8]  BaseID
//   [8]  BaseSize
//   [8]  LogicalSize
//   [4]  DirtyRangeCount
//   [16 * count] Dirty ranges as start/end pairs
//   [8] Change
//   [8] Mtime, [8] Atime, [8] Ctime (Unix nanoseconds; absent before NFS4)
//   [1] flags: bit 0 MetadataChanged, bit 1 OwnerKnown, bit 2 ReadOnly, bit 3 Retired
//   [4] OpenOwnerLen, [N] OpenOwner (opaque bytes)
//   [32] RecoveryKey (optional for older NFS4 checkpoints)

func saveStagingMeta(path string, meta StagingMeta) error {
	nameBytes := []byte(meta.FileName)
	buf := make([]byte, 8+8+12+2+len(nameBytes)+8+4+8+8+8+4+16*len(meta.Dirty)+32+1+4+len(meta.OpenOwner)+32)
	binary.BigEndian.PutUint64(buf[0:8], uint64(meta.DirID))
	copy(buf[8:16], meta.TernCookie[:])
	copy(buf[16:28], meta.NFSStateID[:])
	binary.BigEndian.PutUint16(buf[28:30], uint16(len(nameBytes)))
	copy(buf[30:], nameBytes)
	off := 30 + len(nameBytes)
	binary.BigEndian.PutUint64(buf[off:off+8], meta.ClientID)
	off += 8
	copy(buf[off:off+4], stagingMetaV4Magic)
	off += 4
	binary.BigEndian.PutUint64(buf[off:off+8], uint64(meta.BaseID))
	off += 8
	binary.BigEndian.PutUint64(buf[off:off+8], meta.BaseSize)
	off += 8
	binary.BigEndian.PutUint64(buf[off:off+8], meta.Size)
	off += 8
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(meta.Dirty)))
	off += 4
	for _, r := range meta.Dirty {
		binary.BigEndian.PutUint64(buf[off:off+8], r.start)
		binary.BigEndian.PutUint64(buf[off+8:off+16], r.end)
		off += 16
	}
	binary.BigEndian.PutUint64(buf[off:off+8], meta.Attrs.Change)
	for i, stamp := range []time.Time{meta.Attrs.Mtime, meta.Attrs.Atime, meta.Attrs.Ctime} {
		binary.BigEndian.PutUint64(buf[off+8+i*8:off+16+i*8], uint64(stamp.UnixNano()))
	}
	off += 32
	if meta.MetadataChanged {
		buf[off] = 1
	}
	if meta.OwnerKnown {
		buf[off] |= 2
	}
	if meta.ReadOnly {
		buf[off] |= 4
	}
	if meta.Retired {
		buf[off] |= 8
	}
	off++
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(meta.OpenOwner)))
	copy(buf[off+4:], meta.OpenOwner)
	copy(buf[off+4+len(meta.OpenOwner):], meta.RecoveryKey[:])
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if n, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return err
	} else if n != len(buf) {
		tmp.Close()
		return io.ErrShortWrite
	}
	// Persist the new file before replacing the last checkpoint's name.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncStagingDir(dir)
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
	nameLen := int(binary.BigEndian.Uint16(data[28:30]))
	nameEnd := 30 + nameLen
	if len(data) < nameEnd {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	meta.FileName = string(data[30:nameEnd])
	if len(data) == nameEnd {
		return meta, nil
	}
	if len(data) < nameEnd+8 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	meta.ClientID = binary.BigEndian.Uint64(data[nameEnd : nameEnd+8])
	off := nameEnd + 8
	if len(data) == off {
		return meta, nil
	}
	if len(data) < off+4 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	magic := string(data[off : off+4])
	if magic != stagingMetaV4Magic {
		return StagingMeta{}, fmt.Errorf("unknown meta file extension")
	}
	off += 4
	if len(data) < off+20 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	meta.BaseID = InodeID(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8
	meta.BaseSize = binary.BigEndian.Uint64(data[off : off+8])
	off += 8
	if len(data) < off+8 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	meta.Size = binary.BigEndian.Uint64(data[off : off+8])
	meta.version = 4
	off += 8
	if len(data) < off+4 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	count := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	if count > (len(data)-off)/16 {
		return StagingMeta{}, fmt.Errorf("invalid dirty range count")
	}
	for range count {
		start := binary.BigEndian.Uint64(data[off : off+8])
		end := binary.BigEndian.Uint64(data[off+8 : off+16])
		if start >= end {
			return StagingMeta{}, fmt.Errorf("invalid dirty range")
		}
		if len(meta.Dirty) != 0 && start <= meta.Dirty[len(meta.Dirty)-1].end {
			return StagingMeta{}, fmt.Errorf("dirty ranges are not normalized")
		}
		meta.Dirty = append(meta.Dirty, byteRange{start: start, end: end})
		off += 16
	}
	if magic == stagingMetaV4Magic {
		meta.version = 4
		if len(data)-off < 37 {
			return StagingMeta{}, fmt.Errorf("truncated staging attributes")
		}
		meta.Attrs.Change = binary.BigEndian.Uint64(data[off : off+8])
		if meta.Attrs.Change != 0 {
			meta.Attrs.Mtime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+8:off+16])))
			meta.Attrs.Atime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+16:off+24])))
			meta.Attrs.Ctime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+24:off+32])))
		}
		off += 32
		if data[off]&^byte(15) != 0 {
			return StagingMeta{}, fmt.Errorf("invalid metadata change flag")
		}
		meta.MetadataChanged = data[off]&1 != 0
		meta.OwnerKnown = data[off]&2 != 0
		meta.ReadOnly = data[off]&4 != 0
		meta.Retired = data[off]&8 != 0
		off++
		ownerLen := uint64(binary.BigEndian.Uint32(data[off : off+4]))
		off += 4
		if ownerLen > uint64(len(data)-off) || (uint64(len(data)-off)-ownerLen != 0 && uint64(len(data)-off)-ownerLen != 32) {
			return StagingMeta{}, fmt.Errorf("invalid staging owner length")
		}
		meta.OpenOwner = string(data[off : off+int(ownerLen)])
		off += int(ownerLen)
		copy(meta.RecoveryKey[:], data[off:])
	}
	if meta.Size > math.MaxInt64 || meta.BaseSize > math.MaxInt64 {
		return StagingMeta{}, fmt.Errorf("staging size exceeds supported offset")
	}
	return meta, nil
}
