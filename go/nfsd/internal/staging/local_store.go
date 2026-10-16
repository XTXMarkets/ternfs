// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"fmt"
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
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

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
		meta, metaErr := LoadMeta(metaPath)
		var formatErr FormatError
		if errors.As(metaErr, &formatErr) {
			f.Close()
			s.CloseAll()
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
func (s *LocalStore) CloseAll() {
	for _, entry := range s.files {
		entry.file.prepareRemove()
	}
	s.files = nil
}

func (s *LocalStore) ReadOnly() bool { return false }

func (s *LocalStore) Create(id vfs.InodeID, meta Meta) (File, error) {
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
	meta.Version = MetaVersion
	meta.Attrs = stagingAttributes(meta.Attrs, time.Now())
	path := filepath.Join(s.dir, fmt.Sprintf("%016x.staging", uint64(id)))
	f, err := s.createFile(path)
	if err != nil {
		return nil, err
	}
	// Write sidecar metadata.
	metaPath := filepath.Join(s.dir, fmt.Sprintf("%016x.meta", uint64(id)))
	if err := SaveMeta(metaPath, meta); err != nil {
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

func (s *LocalStore) GetMeta(id vfs.InodeID) (Meta, bool) {
	file := s.Get(id)
	if file == nil {
		return Meta{}, false
	}
	return file.(*LocalFile).Meta(), true
}

func (s *LocalStore) FindTargets(dirID vfs.InodeID, name string) map[vfs.InodeID]Meta {
	s.mu.Lock()
	files := make(map[vfs.InodeID]*LocalFile)
	for id := range s.targets[stagingTarget{dirID: dirID, name: name}] {
		files[id] = s.files[id].file
	}
	s.mu.Unlock()
	result := make(map[vfs.InodeID]Meta, len(files))
	for id, file := range files {
		result[id] = file.Meta()
	}
	return result
}

func (s *LocalStore) Entries() map[vfs.InodeID]Meta {
	s.mu.Lock()
	files := make(map[vfs.InodeID]*LocalFile)
	for id, entry := range s.files {
		if !entry.failed {
			files[id] = entry.file
		}
	}
	s.mu.Unlock()
	entries := make(map[vfs.InodeID]Meta, len(files))
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
func (s *LocalStore) updateNamespace(id vfs.InodeID, update func(*Meta)) error {
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
	err := SaveMeta(sf.metaPath, sf.meta)
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
	return s.updateNamespace(id, func(meta *Meta) { meta.Guarded = guarded })
}

func (s *LocalStore) Detach(id vfs.InodeID) error {
	return s.updateNamespace(id, func(meta *Meta) {
		meta.Unlinked, meta.Guarded = true, false
	})
}

func (s *LocalStore) Retarget(id vfs.InodeID, dirID vfs.InodeID, name string) error {
	return s.updateNamespace(id, func(meta *Meta) {
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
		if err := syncDir(path); err != nil {
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
		if err := syncDir(path); err != nil {
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
