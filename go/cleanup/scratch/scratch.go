// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package scratch

import (
	"fmt"
	"sync"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type ScratchFile interface {
	Close()
	Lock() (*lockedScratchFile, error)
	// does not ensure valid
	FileId() msgs.InodeId
}

func NewScratchFile(log *log.Logger, c *client.Client, shard msgs.ShardId, note string) ScratchFile {
	scratch := &scratchFile{
		log:   log,
		c:     c,
		shard: shard,
		note:  note,

		clearOnUnlock: false,
		clearReason:   "",
		deadline:      0,
		done:          make(chan struct{}),

		id: msgs.NULL_INODE_ID,
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				scratch.releaseScratchFile()
			case _, ok := <-scratch.done:
				if !ok {
					return
				}
			}
		}
	}()
	return scratch
}

func (f *lockedScratchFile) Unlock() {
	f.locked = false
	if f.clearOnUnlock {
		f.log.Info("closing scratch file %v, reason: %s", f.id, f.clearReason)
		resp := msgs.ScrapTransientFileResp{}
		err := f.c.ShardRequest(
			f.log,
			f.shard,
			&msgs.ScrapTransientFileReq{
				Id:     f.id,
				Cookie: f.cookie,
			},
			&resp,
		)
		if err != nil {
			f.log.Info("failed to scrap transient file: %v", err)
		}

		f.clearOnUnlock = false
		f.clearReason = ""
		f.id = msgs.NULL_INODE_ID
		f.size = 0
		f.cookie = [8]byte{}
	}
	f.mu.Unlock()
}

func (f *lockedScratchFile) FileId() msgs.InodeId {
	if !f.locked {
		panic(fmt.Errorf("accessing id on unlocked scratch file with note %s", f.note))
	}
	return f.id
}

func (f *lockedScratchFile) Cookie() [8]byte {
	if !f.locked {
		panic(fmt.Errorf("accessing cookie on unlocked scratch file with note %s", f.note))
	}
	return f.cookie
}

func (f *lockedScratchFile) Note() string {
	if !f.locked {
		panic(fmt.Errorf("accessing note on unlocked scratch file with note %s", f.note))
	}
	return f.note
}

func (f *lockedScratchFile) Shard() msgs.ShardId {
	if !f.locked {
		panic(fmt.Errorf("accessing shard on unlocked scratch file with note %s", f.note))
	}
	return f.shard
}

func (f *lockedScratchFile) Size() uint64 {
	if !f.locked {
		panic(fmt.Errorf("accessing size on unlocked scratch file with note %s", f.note))
	}
	return f.size
}

func (f *lockedScratchFile) AddSize(size uint64) {
	if !f.locked {
		panic(fmt.Errorf("accessing size on unlocked scratch file with note %s", f.note))
	}
	f.size += size
}

func (f *lockedScratchFile) ClearOnUnlock(reason string) {
	if !f.locked {
		panic(fmt.Errorf("accessing size on unlocked scratch file with note %s", f.note))
	}
	f.clearOnUnlock = true
	f.clearReason = reason
}

type lockedScratchFile struct {
	*scratchFile
	locked bool
}

func (s *scratchFile) Lock() (*lockedScratchFile, error) {
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		panic("locking closed scratch file")
	default:
	}
	if s.id == msgs.NULL_INODE_ID {
		resp := msgs.ConstructFileResp{}
		err := s.c.ShardRequest(
			s.log,
			s.shard,
			&msgs.ConstructFileReq{
				Type: msgs.FILE,
				Note: s.note,
			},
			&resp,
		)
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.log.Info("created scratch file %v", resp.Id)
		s.id = resp.Id
		s.cookie = resp.Cookie
		s.size = 0
		s.deadline = msgs.MakeTernTime(time.Now().Add(3 * time.Hour))
	}

	return &lockedScratchFile{s, true}, nil
}

func (s *scratchFile) _lock() *lockedScratchFile {
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		return nil
	default:
	}
	return &lockedScratchFile{s, true}
}

func (f *scratchFile) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	close(f.done)
}

func (f *scratchFile) FileId() msgs.InodeId {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.id
}

type scratchFile struct {
	log   *log.Logger
	c     *client.Client
	shard msgs.ShardId
	note  string

	clearOnUnlock bool
	clearReason   string
	deadline      msgs.TernTime
	done          chan struct{}

	mu     sync.Mutex
	id     msgs.InodeId
	cookie [8]byte
	size   uint64
}

func (s *scratchFile) releaseScratchFile() {
	lockedScratchFile := s._lock()
	// If the scratch file is already closed, do nothing.
	if lockedScratchFile == nil {
		return
	}
	defer lockedScratchFile.Unlock()
	if s.id == msgs.NULL_INODE_ID {
		return
	}
	if msgs.Now() > s.deadline {
		lockedScratchFile.ClearOnUnlock(fmt.Sprintf("scratch file %v with note (%s) lifetime passed resetting", s.id, s.note))
		return
	}
}
