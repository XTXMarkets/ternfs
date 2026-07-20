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

const spansPerDestructTurn = 32

type ScratchFile interface {
	Close()
	Lock() (*lockedScratchFile, error)
	// does not ensure valid
	FileId() msgs.InodeId
}

type pendingInode struct {
	id       msgs.InodeId
	cookie   [8]byte
	scrapped bool
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
		wake:          make(chan struct{}, 1),

		id: msgs.NULL_INODE_ID,
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				scratch.releaseScratchFile()
				scratch.drainPending(false)
			case <-scratch.wake:
				scratch.drainPending(false)
			case <-scratch.done:
				scratch.drainPending(true)
				return
			}
		}
	}()
	return scratch
}

func (f *lockedScratchFile) Unlock() {
	f.locked = false
	if f.clearOnUnlock {
		f.log.Info("recycling scratch file %v, reason: %s", f.id, f.clearReason)
		f.clearOnUnlock = false
		f.clearReason = ""
		f.enqueueForDestructLocked()
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

func (f *scratchFile) Close() {
	f.mu.Lock()
	f.enqueueForDestructLocked()
	f.mu.Unlock()
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
	wake          chan struct{}

	mu      sync.Mutex
	id      msgs.InodeId
	cookie  [8]byte
	size    uint64
	pending []pendingInode
}

func (s *scratchFile) enqueueForDestructLocked() {
	if s.id == msgs.NULL_INODE_ID {
		return
	}
	s.pending = append(s.pending, pendingInode{id: s.id, cookie: s.cookie})
	s.id = msgs.NULL_INODE_ID
	s.size = 0
	s.cookie = [8]byte{}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *scratchFile) releaseScratchFile() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.id == msgs.NULL_INODE_ID {
		return
	}
	if msgs.Now() > s.deadline {
		s.log.Info("scratch file %v with note (%s) lifetime passed, recycling", s.id, s.note)
		s.enqueueForDestructLocked()
	}
}

func (s *scratchFile) drainPending(flush bool) {
	for {
		s.mu.Lock()
		if len(s.pending) == 0 {
			s.mu.Unlock()
			return
		}
		p := s.pending[0]
		s.pending = s.pending[1:]
		s.mu.Unlock()

		if s.destructTurn(&p) {
			continue
		}

		s.mu.Lock()
		s.pending = append(s.pending, p)
		s.mu.Unlock()
		if !flush {
			select {
			case s.wake <- struct{}{}:
			default:
			}
			return
		}
	}
}

func (s *scratchFile) destructTurn(p *pendingInode) (finished bool) {

	if !p.scrapped {
		err := s.c.ShardRequest(s.log, s.shard, &msgs.ScrapTransientFileReq{Id: p.id, Cookie: p.cookie}, &msgs.ScrapTransientFileResp{})
		if err == msgs.FILE_NOT_FOUND {
			return true // GC got there first
		}
		if err != nil {
			s.log.Info("could not scrap scratch file %v: %v; leaving it to GC", p.id, err)
			return true
		}
		p.scrapped = true
	}

	initReq := msgs.RemoveSpanInitiateReq{FileId: p.id, Cookie: p.cookie}
	initResp := msgs.RemoveSpanInitiateResp{}
	for i := 0; i < spansPerDestructTurn; i++ {
		err := s.c.ShardRequest(s.log, s.shard, &initReq, &initResp)
		if err == msgs.FILE_EMPTY {
			if err := s.c.ShardRequest(s.log, s.shard, &msgs.RemoveInodeReq{Id: p.id}, &msgs.RemoveInodeResp{}); err != nil {
				s.log.Info("could not remove scratch inode %v: %v; leaving it to GC", p.id, err)
			}
			return true
		}
		if err == msgs.FILE_NOT_FOUND {
			return true // GC got there first
		}
		if err != nil {
			s.log.Info("could not initiate span removal for scratch file %v: %v; leaving it to GC", p.id, err)
			return true
		}
		if len(initResp.Blocks) == 0 {
			continue
		}
		certifyReq := msgs.RemoveSpanCertifyReq{FileId: p.id, Cookie: p.cookie, ByteOffset: initResp.ByteOffset}
		certifyReq.Proofs = make([]msgs.BlockProof, len(initResp.Blocks))
		for j := range initResp.Blocks {
			block := &initResp.Blocks[j]
			var proof [8]byte
			var err error
			if block.BlockServiceFlags.HasAny(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) {
				proof, err = s.c.EraseDecommissionedBlock(block)
			} else {
				proof, err = s.c.EraseBlock(s.log, block)
			}
			if err != nil {
				s.log.Info("could not erase block %v for scratch file %v: %v; leaving it to GC", block.BlockId, p.id, err)
				return true
			}
			certifyReq.Proofs[j].BlockId = block.BlockId
			certifyReq.Proofs[j].Proof = proof
		}

		err = s.c.ShardRequest(s.log, s.shard, &certifyReq, &msgs.RemoveSpanCertifyResp{})
		if err == msgs.FILE_NOT_FOUND {
			return true // GC removed the whole inode
		}
		if err != nil {
			s.log.Info("could not certify span removal for scratch file %v: %v; leaving it to GC", p.id, err)
			return true
		}
	}
	return false
}
