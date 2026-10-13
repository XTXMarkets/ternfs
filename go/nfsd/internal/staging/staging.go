// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/byterange"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sidecar"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

const stagingHydrationChunk = 1 << 20
const finishHydrationWorkers = 4

// QuarantineDirName is the subdirectory of the staging directory which
// holds checkpoints whose metadata sidecar would not decode. Recovery moves
// them aside rather than deleting them; nfsd inspect lists them.
const QuarantineDirName = "quarantine"

var ErrRemoved = errors.New("staging file was removed")

type stagingTarget struct {
	dirID vfs.InodeID
	name  string
}

type stagingHydrationReader func(cancel <-chan struct{}, fileID vfs.InodeID, offset uint64, dest []byte) (int, bool, error)

type BaseReader func(
	fileID vfs.InodeID,
	offset uint64,
	dest []byte,
) (n int, eof bool, err error)

// File is the interface for locally staged file data during NFS writes.
// The store owns the file lifecycle; there is no Close method.
type File interface {
	Stat() vfs.NodeInfo
	SetTime(mtime, atime *time.Time) error
	SetSize(size uint64) error
	Write(offset uint64, data []byte) error
	Read(offset uint64, dest []byte, readBase BaseReader) (n int, eof bool, err error)
	Cache(offset uint64, data []byte) error
	Base() (id vfs.InodeID, size uint64)
	Dirty() bool
	DataChanged() bool
	Retire(clientID uint64, stateID nfs4.StateID) error
	StartHydration(readBase stagingHydrationReader)
	FinishHydration(readBase BaseReader) error
	Sync() error                    // persist to stable storage
	Reader() (io.ReadSeeker, error) // returns a reader positioned at offset 0
}

// Store manages staging files keyed by InodeID.
// All staging queries go through this interface — the NFS server
// does not maintain its own staging caches.
type Store interface {
	// ReadOnly returns true if no staging directory is configured.
	ReadOnly() bool
	// Create creates a new staging file with associated metadata.
	Create(id vfs.InodeID, meta sidecar.Meta) (File, error)
	// Get returns the staging file for the given inode, or nil.
	Get(id vfs.InodeID) File
	// GetMeta returns the metadata sidecar for the given inode.
	GetMeta(id vfs.InodeID) (sidecar.Meta, bool)
	// FindTargets returns the staging sessions indexed by dirID/name.
	FindTargets(dirID vfs.InodeID, name string) map[vfs.InodeID]sidecar.Meta
	// Entries returns a snapshot of all staging metadata keyed by staging inode.
	Entries() map[vfs.InodeID]sidecar.Meta
	// Rebind updates the owning client and stateid of a recovered staging file.
	Rebind(id vfs.InodeID, clientID uint64, stateID nfs4.StateID) error
	// Namespace updates preserve the current durable data checkpoint.
	SetGuard(id vfs.InodeID, guarded bool) error
	Detach(id vfs.InodeID) error
	Retarget(id vfs.InodeID, dirID vfs.InodeID, name string) error
	// Quarantine excludes an entry for the process lifetime and preserves its files.
	Quarantine(id vfs.InodeID) error
	Failed(id vfs.InodeID) bool
	// Remove closes and removes the staging file and sidecar for the given inode.
	Remove(id vfs.InodeID)
	// StagedSize returns the staged size for the given inode.
	// Returns (0, false) if the inode has no staging file.
	StagedSize(id vfs.InodeID) (uint64, bool)
}

// LocalStore manages staging files as local files in a directory.
// File names encode the InodeID so state can be recovered on restart.
type LocalStore struct {
	mu      sync.Mutex
	dir     string
	files   map[vfs.InodeID]*localStagingEntry
	targets map[stagingTarget]map[vfs.InodeID]struct{}
	log     *slog.Logger

	fileLocks  keyedlock.Locker[vfs.InodeID]
	createFile func(string) (*os.File, error)
}

type localStagingEntry struct {
	file   *LocalFile
	target stagingTarget
	failed bool
}

func NewLocalStore(dir string, logger *slog.Logger) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &LocalStore{
		dir:        dir,
		files:      make(map[vfs.InodeID]*localStagingEntry),
		targets:    make(map[stagingTarget]map[vfs.InodeID]struct{}),
		log:        logger,
		createFile: os.Create,
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
		id := vfs.InodeID(idVal)
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR, 0644)
		if err != nil {
			s.log.Warn("staging recover: cannot open file", "path", path, "err", err)
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			s.log.Warn("staging recover: cannot stat file", "path", path, "err", err)
			continue
		}
		metaPath := filepath.Join(dir, fmt.Sprintf("%016x.meta", uint64(id)))
		meta, metaErr := sidecar.Load(metaPath)
		var formatErr sidecar.FormatError
		if errors.As(metaErr, &formatErr) {
			f.Close()
			s.closeAll()
			return nil, formatErr
		}
		if metaErr != nil || meta.Guarded || meta.Unlinked && meta.Retired {
			// Quarantine a guard without querying the backend to infer its outcome.
			// Even a failed quarantine must exclude only this entry, not stop nfsd.
			f.Close()
			if err := quarantineStagingFiles(dir, id); err != nil {
				s.log.Warn("staging recover: cannot quarantine checkpoint", "inode", id, "err", err)
			}
			s.log.Warn("staging recover: excluded checkpoint", "file", name,
				"guarded", meta.Guarded, "unlinked", meta.Unlinked, "retired", meta.Retired, "err", metaErr)
			continue
		}
		size := uint64(info.Size())
		if meta.BaseID != 0 {
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
			file: &LocalFile{
				f:               f,
				size:            size,
				meta:            meta,
				metaPath:        metaPath,
				dirty:           append(byterange.Set(nil), meta.Dirty...),
				attrs:           stagingAttributes(meta.Attrs, info.ModTime()),
				metadataChanged: meta.MetadataChanged,
			},
			target: target,
		}
		if !meta.Unlinked {
			s.addTargetLocked(id, target)
		}
		if meta.Retired {
			_ = f.Close()
		}
		s.log.Info("staging recover", "file", name, "inode", fmt.Sprintf("%016x", uint64(id)), "size", size)
		s.files[id] = entry
	}
	return s, nil
}

// closeAll releases descriptors opened during a failed startup.
func (s *LocalStore) closeAll() {
	for _, entry := range s.files {
		entry.file.prepareRemove()
	}
	s.files = nil
}

func (s *LocalStore) ReadOnly() bool { return false }

func (s *LocalStore) Create(id vfs.InodeID, meta sidecar.Meta) (File, error) {
	unlock := s.fileLocks.Lock(id)
	defer unlock()
	s.mu.Lock()
	entry := s.files[id]
	s.mu.Unlock()
	if entry != nil {
		if entry.failed {
			return nil, ErrRemoved
		}
		return entry.file, nil // already exists (recovered or duplicate create)
	}
	target := stagingTarget{dirID: meta.DirID, name: meta.FileName}
	meta.Size = meta.BaseSize
	meta.Version = sidecar.Version
	meta.Attrs = stagingAttributes(meta.Attrs, time.Now())
	path := filepath.Join(s.dir, fmt.Sprintf("%016x.staging", uint64(id)))
	f, err := s.createFile(path)
	if err != nil {
		return nil, err
	}
	// Write sidecar metadata.
	metaPath := filepath.Join(s.dir, fmt.Sprintf("%016x.meta", uint64(id)))
	if err := sidecar.Save(metaPath, meta); err != nil {
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
	sf := &LocalFile{
		f:        f,
		size:     meta.BaseSize,
		meta:     meta,
		metaPath: metaPath,
		dirty:    append(byterange.Set(nil), meta.Dirty...),
		attrs:    meta.Attrs,
	}
	// Readers only see complete entries. The inode lock excludes duplicate
	// creation and cleanup while disk I/O runs without the store-wide lock.
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[id] = &localStagingEntry{
		file:   sf,
		target: target,
	}
	if !meta.Unlinked {
		s.addTargetLocked(id, target)
	}
	return sf, nil
}

func (s *LocalStore) Get(id vfs.InodeID) File {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[id]
	if entry == nil || entry.failed {
		return nil
	}
	return entry.file
}

func (s *LocalStore) Failed(id vfs.InodeID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[id]
	return entry != nil && entry.failed
}

func (s *LocalStore) GetMeta(id vfs.InodeID) (sidecar.Meta, bool) {
	file := s.Get(id)
	if file == nil {
		return sidecar.Meta{}, false
	}
	return file.(*LocalFile).Meta(), true
}

func (s *LocalStore) FindTargets(dirID vfs.InodeID, name string) map[vfs.InodeID]sidecar.Meta {
	s.mu.Lock()
	files := make(map[vfs.InodeID]*LocalFile)
	for id := range s.targets[stagingTarget{dirID: dirID, name: name}] {
		files[id] = s.files[id].file
	}
	s.mu.Unlock()
	result := make(map[vfs.InodeID]sidecar.Meta, len(files))
	for id, file := range files {
		result[id] = file.Meta()
	}
	return result
}

func (s *LocalStore) Entries() map[vfs.InodeID]sidecar.Meta {
	s.mu.Lock()
	files := make(map[vfs.InodeID]*LocalFile)
	for id, entry := range s.files {
		if !entry.failed {
			files[id] = entry.file
		}
	}
	s.mu.Unlock()
	entries := make(map[vfs.InodeID]sidecar.Meta, len(files))
	for id, file := range files {
		entries[id] = file.Meta()
	}
	return entries
}

func (s *LocalStore) Rebind(
	id vfs.InodeID,
	clientID uint64,
	stateID nfs4.StateID,
) error {
	s.mu.Lock()
	entry := s.files[id]
	if entry == nil || entry.failed {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	file := entry.file
	s.mu.Unlock()
	return file.rebind(clientID, stateID)
}

func (s *LocalStore) addTargetLocked(id vfs.InodeID, target stagingTarget) {
	if s.targets[target] == nil {
		s.targets[target] = make(map[vfs.InodeID]struct{})
	}
	s.targets[target][id] = struct{}{}
}

func (s *LocalStore) removeTargetLocked(id vfs.InodeID, entry *localStagingEntry) {
	delete(s.targets[entry.target], id)
	if len(s.targets[entry.target]) == 0 {
		delete(s.targets, entry.target)
	}
}

// The pathname lock serializes namespace callers. File checkpoint I/O takes
// only sf.mu; reindexing rechecks the entry because removal can overlap it.
func (s *LocalStore) updateNamespace(id vfs.InodeID, update func(*sidecar.Meta)) error {
	s.mu.Lock()
	entry := s.files[id]
	if entry == nil || entry.failed {
		s.mu.Unlock()
		return ErrRemoved
	}
	s.mu.Unlock()
	sf := entry.file
	sf.mu.Lock()
	defer sf.mu.Unlock()
	s.mu.Lock()
	present := s.files[id] == entry && !entry.failed
	s.mu.Unlock()
	if !present || sf.removed {
		return ErrRemoved
	}
	update(&sf.meta)
	// Meta() includes live dirty ranges. Save only sf.meta, which describes
	// the last data checkpoint and the namespace fields just assigned.
	err := sidecar.Save(sf.metaPath, sf.meta)
	sf.checkpointDirty = err != nil
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files[id] != entry || entry.failed {
		return errors.Join(err, ErrRemoved)
	}
	s.removeTargetLocked(id, entry)
	entry.target = stagingTarget{dirID: sf.meta.DirID, name: sf.meta.FileName}
	if !sf.meta.Unlinked {
		s.addTargetLocked(id, entry.target)
	}
	// Keep the known namespace state even if saving failed. The durable guard
	// or final sidecar is safe at restart; checkpointDirty makes Sync repair it.
	return err
}

func (s *LocalStore) SetGuard(id vfs.InodeID, guarded bool) error {
	return s.updateNamespace(id, func(meta *sidecar.Meta) { meta.Guarded = guarded })
}

func (s *LocalStore) Detach(id vfs.InodeID) error {
	return s.updateNamespace(id, func(meta *sidecar.Meta) {
		meta.Unlinked, meta.Guarded = true, false
	})
}

func (s *LocalStore) Retarget(id vfs.InodeID, dirID vfs.InodeID, name string) error {
	return s.updateNamespace(id, func(meta *sidecar.Meta) {
		meta.DirID, meta.FileName = dirID, name
		meta.Unlinked, meta.Guarded = false, false
	})
}

func (s *LocalStore) Quarantine(id vfs.InodeID) error {
	unlock := s.fileLocks.Lock(id)
	defer unlock()
	s.mu.Lock()
	entry := s.files[id]
	if entry == nil || entry.failed {
		s.mu.Unlock()
		return nil
	}
	// COMMIT has no stateid, so retain this tombstone even after CLOSE or
	// cleanup. Restart changes the write verifier and ends its required lifetime.
	entry.failed = true
	s.removeTargetLocked(id, entry)
	s.mu.Unlock()
	entry.file.prepareRemove()
	return quarantineStagingFiles(s.dir, id)
}

// Preserve both halves at whichever locations they reach. Sync the source,
// destination and the directory which names the quarantine subdirectory even
// on a partial move, and report every failure to the caller.
func quarantineStagingFiles(dir string, id vfs.InodeID) error {
	root := filepath.Join(dir, QuarantineDirName)
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	dst, err := os.MkdirTemp(root, fmt.Sprintf("%016x-", uint64(id)))
	if err != nil {
		return err
	}
	// Establish a durable destination path before removing either source
	// name. A crash during the moves must not strand acknowledged bytes in
	// a directory whose parent entry was never persisted.
	for _, path := range []string{dst, root, dir} {
		if err := sidecar.SyncDir(path); err != nil {
			return err
		}
	}
	var errs []error
	for _, ext := range []string{".staging", ".meta"} {
		name := fmt.Sprintf("%016x%s", uint64(id), ext)
		if err := os.Rename(filepath.Join(dir, name), filepath.Join(dst, name)); err != nil && !os.IsNotExist(err) {
			// Moving the pair is not atomic. Leave either half where it reached
			// for manual recovery; rollback or deletion could lose staged bytes.
			errs = append(errs, err)
		}
	}
	for _, path := range []string{dst, root, dir} {
		if err := sidecar.SyncDir(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *LocalStore) Remove(id vfs.InodeID) {
	unlock := s.fileLocks.Lock(id)
	defer unlock()
	s.mu.Lock()
	entry := s.files[id]
	if entry == nil || entry.failed {
		s.mu.Unlock()
		return
	}
	delete(s.files, id)
	s.removeTargetLocked(id, entry)
	s.mu.Unlock()
	entry.file.prepareRemove()
	name := entry.file.f.Name()
	os.Remove(name)
	os.Remove(strings.TrimSuffix(name, ".staging") + ".meta")
}

func (s *LocalStore) StagedSize(id vfs.InodeID) (uint64, bool) {
	file := s.Get(id)
	if file == nil {
		return 0, false
	}
	return file.(*LocalFile).Size(), true
}

// ReadOnlyStore is used when no staging directory is configured.
type ReadOnlyStore struct{}

func (ReadOnlyStore) ReadOnly() bool { return true }
func (ReadOnlyStore) Create(vfs.InodeID, sidecar.Meta) (File, error) {
	return nil, os.ErrPermission
}
func (ReadOnlyStore) Get(vfs.InodeID) File                                         { return nil }
func (ReadOnlyStore) GetMeta(vfs.InodeID) (sidecar.Meta, bool)                     { return sidecar.Meta{}, false }
func (ReadOnlyStore) FindTargets(vfs.InodeID, string) map[vfs.InodeID]sidecar.Meta { return nil }
func (ReadOnlyStore) Entries() map[vfs.InodeID]sidecar.Meta                        { return nil }
func (ReadOnlyStore) Rebind(vfs.InodeID, uint64, nfs4.StateID) error {
	return os.ErrPermission
}
func (ReadOnlyStore) SetGuard(vfs.InodeID, bool) error                { return os.ErrPermission }
func (ReadOnlyStore) Detach(vfs.InodeID) error                        { return os.ErrPermission }
func (ReadOnlyStore) Retarget(vfs.InodeID, vfs.InodeID, string) error { return os.ErrPermission }
func (ReadOnlyStore) Quarantine(vfs.InodeID) error                    { return nil }
func (ReadOnlyStore) Failed(vfs.InodeID) bool                         { return false }
func (ReadOnlyStore) Remove(vfs.InodeID)                              {}
func (ReadOnlyStore) StagedSize(vfs.InodeID) (uint64, bool)           { return 0, false }

// LocalFile is a disk-backed staging file. For replacement files,
// dirty identifies authoritative local bytes and hydrated identifies clean
// base bytes which have already been cached locally.
type LocalFile struct {
	mu              sync.Mutex
	f               *os.File
	size            uint64
	meta            sidecar.Meta
	metaPath        string
	dirty           byterange.Set
	attrs           vfs.NodeInfo
	metadataChanged bool
	hydrated        byterange.Set
	checkpointDirty bool
	removed         bool

	hydrateMu      sync.Mutex
	hydrateDone    chan struct{}
	hydrateCancel  chan struct{}
	hydrateRemoved bool
}

func stagingAttributes(info vfs.NodeInfo, fallback time.Time) vfs.NodeInfo {
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

func (sf *LocalFile) Stat() vfs.NodeInfo {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	info := sf.attrs
	info.Size = sf.size
	return info
}

func (sf *LocalFile) changedLocked() {
	now := time.Now()
	sf.attrs.Change = max(sf.attrs.Change+1, uint64(now.UnixNano()))
	sf.attrs.Ctime = now
}

func (sf *LocalFile) SetTime(mtime, atime *time.Time) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
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

func (sf *LocalFile) Size() uint64 {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.size
}

func (sf *LocalFile) Meta() sidecar.Meta {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	meta := sf.meta
	meta.Dirty = append(byterange.Set(nil), sf.dirty...)
	return meta
}

func (sf *LocalFile) Base() (vfs.InodeID, uint64) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.meta.BaseID, sf.meta.BaseSize
}

func (sf *LocalFile) Dirty() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.metadataChanged || len(sf.dirty) != 0 || sf.size != sf.meta.BaseSize
}

func (sf *LocalFile) DataChanged() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return len(sf.dirty) != 0 || sf.size != sf.meta.BaseSize
}

func (sf *LocalFile) Retire(clientID uint64, stateID nfs4.StateID) error {
	sf.hydrateMu.Lock()
	done := sf.cancelHydrationLocked()
	sf.hydrateMu.Unlock()
	if done != nil {
		<-done
	}
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
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

func (sf *LocalFile) saveMetaLocked() error {
	meta := sf.meta
	meta.Dirty = append(byterange.Set(nil), sf.dirty...)
	meta.Size = sf.size
	meta.Attrs = sf.attrs
	meta.MetadataChanged = sf.metadataChanged
	meta.Version = sidecar.Version
	if err := sidecar.Save(sf.metaPath, meta); err != nil {
		return err
	}
	// Only a successfully persisted checkpoint can suppress a later retry.
	sf.meta = meta
	sf.checkpointDirty = false
	return nil
}

func (sf *LocalFile) checkpointChangedLocked() bool {
	if sf.checkpointDirty || sf.meta.MetadataChanged != sf.metadataChanged ||
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

// updateMetaLocked changes sidecar metadata without checkpointing unstable
// data. The caller holds sf.mu.
func (sf *LocalFile) updateMetaLocked(meta sidecar.Meta) error {
	if sf.removed {
		return ErrRemoved
	}
	previous := sf.meta
	if err := sidecar.Save(sf.metaPath, meta); err != nil {
		// A directory sync can fail after the replacement was installed.
		restoreErr := sidecar.Save(sf.metaPath, previous)
		sf.checkpointDirty = restoreErr != nil
		if restoreErr != nil {
			restoreErr = fmt.Errorf("restore previous staging metadata: %w", restoreErr)
		}
		return errors.Join(err, restoreErr)
	}
	sf.meta = meta
	sf.checkpointDirty = false
	return nil
}

func (sf *LocalFile) rebind(
	clientID uint64,
	stateID nfs4.StateID,
) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
	}
	oldMeta := sf.meta
	if oldMeta.Retired {
		f, err := os.OpenFile(sf.f.Name(), os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		sf.f = f
	}
	meta := oldMeta
	meta.ClientID = clientID
	meta.NFSStateID = stateID
	meta.Retired = false
	err := sf.updateMetaLocked(meta)
	if err != nil && oldMeta.Retired {
		_ = sf.f.Close()
	}
	return err
}

func (sf *LocalFile) SetSize(size uint64) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
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
		sf.dirty.Add(size, oldSize)
		sf.hydrated.Remove(size, oldSize)
	} else {
		sf.dirty.Add(oldSize, size)
	}
	return nil
}

func (sf *LocalFile) Write(offset uint64, data []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
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
	sf.dirty.Add(offset, end)
	sf.hydrated.Remove(offset, end)
	return nil
}

func (sf *LocalFile) Read(
	offset uint64,
	dest []byte,
	readBase BaseReader,
) (int, bool, error) {
	sf.mu.Lock()
	if sf.removed {
		sf.mu.Unlock()
		return 0, false, ErrRemoved
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
	missing := coverage.Gaps(offset, min(end, baseSize))
	sf.mu.Unlock()

	for _, gap := range missing {
		part := dest[gap.Start-offset : gap.End-offset]
		if err := readStagingBase(readBase, baseID, gap.Start, part); err != nil {
			return 0, false, err
		}
	}

	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return 0, false, ErrRemoved
	}
	for _, gap := range missing {
		part := dest[gap.Start-offset : gap.End-offset]
		if err := sf.cacheLocked(gap.Start, part); err != nil {
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
	readBase BaseReader,
	baseID vfs.InodeID,
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

func (sf *LocalFile) localCoverageLocked() byterange.Set {
	return sf.dirty.Union(sf.hydrated)
}

func (sf *LocalFile) readLocalLocked(offset uint64, dest []byte) error {
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

func (sf *LocalFile) Cache(offset uint64, data []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
	}
	return sf.cacheLocked(offset, data)
}

func (sf *LocalFile) cacheLocked(offset uint64, data []byte) error {
	if sf.meta.BaseID == 0 || offset >= sf.size || offset >= sf.meta.BaseSize {
		return nil
	}
	end := offset + uint64(len(data))
	if end < offset {
		return fmt.Errorf("staging cache offset overflow")
	}
	end = min(end, sf.size, sf.meta.BaseSize)
	for _, clean := range sf.dirty.Gaps(offset, end) {
		part := data[clean.Start-offset : clean.End-offset]
		if _, err := sf.f.WriteAt(part, int64(clean.Start)); err != nil {
			return err
		}
		sf.hydrated.Add(clean.Start, clean.End)
	}
	return nil
}

func (sf *LocalFile) StartHydration(readBase stagingHydrationReader) {
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
		_ = sf.hydrate(func(id vfs.InodeID, offset uint64, dest []byte) (int, bool, error) {
			return readBase(cancel, id, offset, dest)
		}, cancel)
		close(done)
	}()
}

func (sf *LocalFile) hydrate(
	readBase BaseReader,
	cancel <-chan struct{},
) error {
	buf := make([]byte, stagingHydrationChunk)
	for {
		sf.mu.Lock()
		baseID := sf.meta.BaseID
		end := min(sf.size, sf.meta.BaseSize)
		gaps := sf.localCoverageLocked().Gaps(0, end)
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
		count := min(uint64(len(buf)), gap.End-gap.Start)
		part := buf[:count]
		if err := readStagingBase(readBase, baseID, gap.Start, part); err != nil {
			return err
		}
		if err := sf.Cache(gap.Start, part); err != nil {
			return err
		}
	}
}

func (sf *LocalFile) cancelHydrationLocked() chan struct{} {
	if sf.hydrateCancel != nil {
		close(sf.hydrateCancel)
		sf.hydrateCancel = nil
	}
	return sf.hydrateDone
}

func (sf *LocalFile) FinishHydration(
	readBase BaseReader,
) error {
	sf.hydrateMu.Lock()
	done := sf.cancelHydrationLocked()
	sf.hydrateMu.Unlock()
	if done != nil {
		<-done
	}
	return sf.finishHydration(readBase)
}

func (sf *LocalFile) finishHydration(
	readBase BaseReader,
) error {
	for {
		sf.mu.Lock()
		baseID := sf.meta.BaseID
		end := min(sf.size, sf.meta.BaseSize)
		gaps := sf.localCoverageLocked().Gaps(0, end)
		sf.mu.Unlock()
		if baseID == 0 || len(gaps) == 0 {
			return nil
		}

		jobs := make(chan byterange.Range)
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
					part := buf[:job.End-job.Start]
					err := readStagingBase(
						readBase, baseID, job.Start, part,
					)
					if err == nil {
						err = sf.Cache(job.Start, part)
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
			for start := gap.Start; start < gap.End; {
				end := min(start+stagingHydrationChunk, gap.End)
				jobs <- byterange.Range{Start: start, End: end}
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

func (sf *LocalFile) prepareRemove() {
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

func (sf *LocalFile) Sync() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return ErrRemoved
	}
	if err := sf.f.Sync(); err != nil {
		return err
	}
	if !sf.checkpointChangedLocked() {
		return nil
	}
	return sf.saveMetaLocked()
}

func (sf *LocalFile) Reader() (io.ReadSeeker, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.removed {
		return nil, ErrRemoved
	}
	if _, err := sf.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return sf.f, nil
}
