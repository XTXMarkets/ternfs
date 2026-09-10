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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	stagingMetaV2Magic = "NFS2"
	stagingMetaV3Magic = "NFS3"
)

const stagingHydrationChunk = 1 << 20
const finishHydrationWorkers = 4

var errStagingBaseBusy = errors.New("mutable staging base is already open")
var errStagingTargetBusy = errors.New("mutable staging target is already open")
var errStagingRemoved = errors.New("staging file was removed")

type stagingTarget struct {
	dirID InodeID
	name  string
}

// StagingMeta contains the state needed to complete CLOSE after a restart.
type StagingMeta struct {
	DirID      InodeID // directory to link the file into on CLOSE
	FileName   string  // name in directory
	TernCookie Cookie  // cookie from VFS ConstructFile
	NFSStateID StateID // random, returned to NFS client as stateid "other"
	ClientID   uint64  // owning client; zero in sidecars written by older nfsd
	BaseID     InodeID // immutable file being replaced; zero for a new file
	BaseSize   uint64
	Size       uint64
	Dirty      byteRangeSet
	version    uint8
}

type stagingBaseReader func(
	fileID InodeID,
	offset uint64,
	dest []byte,
) (n int, eof bool, err error)

// StagingFile is the interface for locally staged file data during NFS writes.
// The store owns the file lifecycle; there is no Close method.
type StagingFile interface {
	SetSize(size uint64) error
	Write(offset uint64, data []byte) error
	Read(offset uint64, dest []byte, readBase stagingBaseReader) (n int, eof bool, err error)
	Cache(offset uint64, data []byte) error
	Base() (id InodeID, size uint64)
	Dirty() bool
	StartHydration(readBase stagingBaseReader)
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
	// ResolveID maps a staging alias to its transient staging inode.
	ResolveID(id InodeID) (InodeID, bool)
	// GetMeta returns the metadata sidecar for the given inode.
	GetMeta(id InodeID) (StagingMeta, bool)
	// FindTarget returns the staging inode and metadata for dirID/name.
	FindTarget(dirID InodeID, name string) (InodeID, StagingMeta, bool)
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
	// StagedSizes returns a snapshot of all staged InodeID → size.
	StagedSizes() map[InodeID]uint64
}

// LocalStagingStore manages staging files as local files in a directory.
// File names encode the InodeID so state can be recovered on restart.
type LocalStagingStore struct {
	mu      sync.Mutex
	dir     string
	files   map[InodeID]*localStagingEntry
	bases   map[InodeID]uint
	targets map[stagingTarget]map[InodeID]struct{}
	aliases map[InodeID]InodeID
	log     *slog.Logger
}

type localStagingEntry struct {
	file   *localStagingFile
	baseID InodeID
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
		bases:   make(map[InodeID]uint),
		targets: make(map[stagingTarget]map[InodeID]struct{}),
		aliases: make(map[InodeID]InodeID),
		log:     logger,
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
		size := uint64(info.Size())
		if meta.version >= 3 && meta.BaseID != 0 {
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
				f:        f,
				size:     size,
				meta:     meta,
				metaPath: metaPath,
				dirty:    append(byteRangeSet(nil), meta.Dirty...),
			},
			baseID: meta.BaseID,
			target: target,
		}
		if metaErr == nil {
			if meta.BaseID != 0 {
				s.bases[meta.BaseID]++
				if _, exists := s.aliases[meta.BaseID]; !exists {
					s.aliases[meta.BaseID] = id
				}
			}
			if s.targets[target] == nil {
				s.targets[target] = make(map[InodeID]struct{})
			}
			s.targets[target][id] = struct{}{}
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
	if meta.BaseID != 0 && s.bases[meta.BaseID] != 0 {
		return nil, errStagingBaseBusy
	}
	target := stagingTarget{dirID: meta.DirID, name: meta.FileName}
	if len(s.targets[target]) != 0 {
		return nil, errStagingTargetBusy
	}
	meta.Size = meta.BaseSize
	meta.version = 3
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
	}
	s.files[id] = &localStagingEntry{
		file:   sf,
		baseID: meta.BaseID,
		target: target,
	}
	if meta.BaseID != 0 {
		s.bases[meta.BaseID]++
		s.aliases[meta.BaseID] = id
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
	resolved, ok := s.resolveIDLocked(id)
	if !ok {
		return nil
	}
	entry := s.files[resolved]
	if entry == nil {
		return nil
	}
	return entry.file
}

func (s *LocalStagingStore) resolveIDLocked(id InodeID) (InodeID, bool) {
	if _, ok := s.files[id]; ok {
		return id, true
	}
	resolved, ok := s.aliases[id]
	if !ok {
		return 0, false
	}
	_, ok = s.files[resolved]
	return resolved, ok
}

func (s *LocalStagingStore) ResolveID(id InodeID) (InodeID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolveIDLocked(id)
}

func (s *LocalStagingStore) GetMeta(id InodeID) (StagingMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolved, ok := s.resolveIDLocked(id)
	if !ok {
		return StagingMeta{}, false
	}
	entry := s.files[resolved]
	return entry.file.Meta(), true
}

func (s *LocalStagingStore) FindTarget(
	dirID InodeID,
	name string,
) (InodeID, StagingMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.targets[stagingTarget{dirID: dirID, name: name}]
	if len(ids) == 0 {
		return 0, StagingMeta{}, false
	}
	var id InodeID
	for id = range ids {
		break
	}
	entry := s.files[id]
	if entry == nil {
		return 0, StagingMeta{}, false
	}
	return id, entry.file.Meta(), true
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
	resolved, ok := s.resolveIDLocked(id)
	if !ok {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	file := s.files[resolved].file
	s.mu.Unlock()
	return file.rebind(clientID, stateID)
}

func (s *LocalStagingStore) TargetBusy(dirID InodeID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.targets[stagingTarget{dirID: dirID, name: name}]) != 0
}

func (s *LocalStagingStore) Remove(id InodeID) {
	s.mu.Lock()
	if resolved, ok := s.resolveIDLocked(id); ok {
		id = resolved
	}
	entry := s.files[id]
	delete(s.files, id)
	if entry != nil {
		baseID := entry.baseID
		if baseID != 0 {
			if s.bases[baseID] <= 1 {
				delete(s.bases, baseID)
			} else {
				s.bases[baseID]--
			}
			if s.aliases[baseID] == id {
				delete(s.aliases, baseID)
				for otherID, other := range s.files {
					if otherID != id && other.baseID == baseID {
						s.aliases[baseID] = otherID
						break
					}
				}
			}
		}
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
	resolved, ok := s.resolveIDLocked(id)
	if !ok {
		return 0, false
	}
	entry := s.files[resolved]
	return entry.file.Size(), true
}

func (s *LocalStagingStore) StagedSizes() map[InodeID]uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.files) == 0 {
		return nil
	}
	m := make(map[InodeID]uint64, len(s.files))
	for id, entry := range s.files {
		m[id] = entry.file.Size()
		if entry.baseID != 0 {
			m[entry.baseID] = entry.file.Size()
		}
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
func (readOnlyStagingStore) ResolveID(InodeID) (InodeID, bool)   { return 0, false }
func (readOnlyStagingStore) GetMeta(InodeID) (StagingMeta, bool) { return StagingMeta{}, false }
func (readOnlyStagingStore) FindTarget(InodeID, string) (InodeID, StagingMeta, bool) {
	return 0, StagingMeta{}, false
}
func (readOnlyStagingStore) Entries() map[InodeID]StagingMeta { return nil }
func (readOnlyStagingStore) Rebind(InodeID, uint64, StateID) error {
	return os.ErrPermission
}
func (readOnlyStagingStore) TargetBusy(InodeID, string) bool   { return false }
func (readOnlyStagingStore) Remove(InodeID)                    {}
func (readOnlyStagingStore) StagedSize(InodeID) (uint64, bool) { return 0, false }
func (readOnlyStagingStore) StagedSizes() map[InodeID]uint64   { return nil }

// localStagingFile is a disk-backed staging file. For replacement files,
// dirty identifies authoritative local bytes and hydrated identifies clean
// base bytes which have already been cached locally.
type localStagingFile struct {
	mu       sync.Mutex
	f        *os.File
	size     uint64
	meta     StagingMeta
	metaPath string
	dirty    byteRangeSet
	hydrated byteRangeSet
	removed  bool

	hydrateMu      sync.Mutex
	hydrateDone    chan struct{}
	hydrateCancel  chan struct{}
	hydrateRemoved bool
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
	return len(sf.dirty) != 0 || sf.size != sf.meta.BaseSize
}

func (sf *localStagingFile) saveMetaLocked() error {
	sf.meta.Dirty = append(sf.meta.Dirty[:0], sf.dirty...)
	sf.meta.Size = sf.size
	sf.meta.version = 3
	return saveStagingMeta(sf.metaPath, sf.meta)
}

func (sf *localStagingFile) checkpointChangedLocked() bool {
	if sf.meta.Size != sf.size || len(sf.meta.Dirty) != len(sf.dirty) {
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
	sf.meta.ClientID = clientID
	sf.meta.NFSStateID = stateID
	err := saveStagingMeta(sf.metaPath, sf.meta)
	if err == nil {
		err = syncStagingMeta(sf.metaPath)
	}
	if err == nil {
		return nil
	}

	sf.meta = oldMeta
	restoreErr := saveStagingMeta(sf.metaPath, oldMeta)
	if restoreErr == nil {
		restoreErr = syncStagingMeta(sf.metaPath)
	}
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
	if err := sf.f.Truncate(int64(size)); err != nil {
		return err
	}
	sf.size = size
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

func (sf *localStagingFile) StartHydration(readBase stagingBaseReader) {
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
		_ = sf.hydrate(readBase, cancel)
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
	if err := sf.saveMetaLocked(); err != nil {
		return err
	}
	return syncStagingMeta(sf.metaPath)
}

func syncStagingMeta(path string) error {
	metaFile, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := metaFile.Sync(); err != nil {
		metaFile.Close()
		return err
	}
	if err := metaFile.Close(); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
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
//   [4]  "NFS3" (absent in sidecars written by older nfsd)
//   [8]  BaseID
//   [8]  BaseSize
//   [8]  LogicalSize
//   [4]  DirtyRangeCount
//   [16 * count] Dirty ranges as start/end pairs

func saveStagingMeta(path string, meta StagingMeta) error {
	nameBytes := []byte(meta.FileName)
	buf := make([]byte, 8+8+12+2+len(nameBytes)+8+4+8+8+8+4+16*len(meta.Dirty))
	binary.BigEndian.PutUint64(buf[0:8], uint64(meta.DirID))
	copy(buf[8:16], meta.TernCookie[:])
	copy(buf[16:28], meta.NFSStateID[:])
	binary.BigEndian.PutUint16(buf[28:30], uint16(len(nameBytes)))
	copy(buf[30:], nameBytes)
	off := 30 + len(nameBytes)
	binary.BigEndian.PutUint64(buf[off:off+8], meta.ClientID)
	off += 8
	copy(buf[off:off+4], stagingMetaV3Magic)
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
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
	if magic != stagingMetaV2Magic && magic != stagingMetaV3Magic {
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
	if magic == stagingMetaV3Magic {
		if len(data) < off+8 {
			return StagingMeta{}, fmt.Errorf("meta file truncated")
		}
		meta.Size = binary.BigEndian.Uint64(data[off : off+8])
		meta.version = 3
		off += 8
	} else {
		meta.version = 2
	}
	if len(data) < off+4 {
		return StagingMeta{}, fmt.Errorf("meta file truncated")
	}
	count := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	if count > (len(data)-off)/16 || len(data) != off+count*16 {
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
	return meta, nil
}
