// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/byterange"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

const stagingHydrationChunk = 1 << 20
const finishHydrationWorkers = 4

// LocalFile is a disk-backed staging file. For replacement files,
// dirty identifies authoritative local bytes and hydrated identifies clean
// base bytes which have already been cached locally.
type LocalFile struct {
	mu              sync.Mutex
	f               *os.File
	size            uint64
	meta            Meta
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

func (sf *LocalFile) Meta() Meta {
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
	meta.Version = MetaVersion
	if err := SaveMeta(sf.metaPath, meta); err != nil {
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
func (sf *LocalFile) updateMetaLocked(meta Meta) error {
	if sf.removed {
		return ErrRemoved
	}
	previous := sf.meta
	if err := SaveMeta(sf.metaPath, meta); err != nil {
		// A directory sync can fail after the replacement was installed.
		restoreErr := SaveMeta(sf.metaPath, previous)
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
