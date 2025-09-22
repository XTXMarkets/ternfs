// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	golog "log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"sync"
	"syscall"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var c *client.Client
var logger *log.Logger
var dirInfoCache *client.DirInfoCache
var bufPool *bufpool.BufPool
var filesGid uint32
var filesUid uint32

var closeMapCollection *ebpf.Collection
var closeMapTracepoint link.Link
var closeMap *ebpf.Map

func ternErrToErrno(err error) syscall.Errno {
	switch err {
	case msgs.INTERNAL_ERROR:
		return syscall.EIO
	case msgs.FATAL_ERROR:
		return syscall.EIO
	case msgs.TIMEOUT:
		return syscall.ETIMEDOUT
	case msgs.NOT_AUTHORISED:
		return syscall.EACCES
	case msgs.UNRECOGNIZED_REQUEST:
		return syscall.EIO
	case msgs.FILE_NOT_FOUND:
		return syscall.ENOENT
	case msgs.DIRECTORY_NOT_FOUND:
		return syscall.ENOENT
	case msgs.NAME_NOT_FOUND:
		return syscall.ENOENT
	case msgs.TYPE_IS_DIRECTORY:
		return syscall.EISDIR
	case msgs.TYPE_IS_NOT_DIRECTORY:
		return syscall.ENOTDIR
	case msgs.BAD_COOKIE:
		return syscall.EACCES
	case msgs.INCONSISTENT_STORAGE_CLASS_PARITY:
		return syscall.EINVAL
	case msgs.LAST_SPAN_STATE_NOT_CLEAN:
		return syscall.EBUSY // reasonable?
	case msgs.COULD_NOT_PICK_BLOCK_SERVICES:
		return syscall.EIO
	case msgs.BAD_SPAN_BODY:
		return syscall.EINVAL
	case msgs.SPAN_NOT_FOUND:
		return syscall.EINVAL
	case msgs.BLOCK_SERVICE_NOT_FOUND:
		return syscall.EIO
	case msgs.CANNOT_CERTIFY_BLOCKLESS_SPAN:
		return syscall.EINVAL
	case msgs.BAD_BLOCK_PROOF:
		return syscall.EINVAL
	case msgs.CANNOT_OVERRIDE_NAME:
		return syscall.EEXIST
	case msgs.NAME_IS_LOCKED:
		return syscall.EEXIST
	case msgs.MTIME_IS_TOO_RECENT:
		return syscall.EBUSY // reasonable?
	case msgs.MISMATCHING_TARGET:
		return syscall.EINVAL
	case msgs.MISMATCHING_OWNER:
		return syscall.EINVAL
	case msgs.DIRECTORY_NOT_EMPTY:
		return syscall.ENOTEMPTY
	case msgs.FILE_IS_TRANSIENT:
		return syscall.EBUSY // reasonable?
	case msgs.OLD_DIRECTORY_NOT_FOUND:
		return syscall.ENOENT
	case msgs.NEW_DIRECTORY_NOT_FOUND:
		return syscall.ENOENT
	case msgs.LOOP_IN_DIRECTORY_RENAME:
		return syscall.ELOOP
	default:
		logger.Debug("unknown error %v", err)
		return syscall.EIO
	}
}

func inodeTypeToMode(typ msgs.InodeType) uint32 {
	mode := uint32(0)
	// This filesystem is read only, and permissionless.
	mode |= syscall.S_IRUSR | syscall.S_IXUSR
	mode |= syscall.S_IRGRP | syscall.S_IXGRP
	mode |= syscall.S_IROTH | syscall.S_IXOTH
	if typ == msgs.FILE {
		mode |= syscall.S_IFREG
	}
	if typ == msgs.SYMLINK {
		mode |= syscall.S_IFLNK
	}
	if typ == msgs.DIRECTORY {
		mode |= syscall.S_IFDIR
	}
	return mode
}

func shardRequest(shid msgs.ShardId, req msgs.ShardRequest, resp msgs.ShardResponse) syscall.Errno {
	if err := c.ShardRequest(logger, shid, req, resp); err != nil {
		return ternErrToErrno(err)
	}

	return 0
}

func cdcRequest(req msgs.CDCRequest, resp msgs.CDCResponse) syscall.Errno {
	if err := c.CDCRequest(logger, req, resp); err != nil {
		switch ternErr := err.(type) {
		case msgs.TernError:
			return ternErrToErrno(ternErr)
		}
		panic(err)
	}

	return 0
}

type ternNode struct {
	fs.Inode
	id msgs.InodeId
}

func (n *ternNode) getattr(f fs.FileHandle, out *fuse.Attr) syscall.Errno {
	logger.Debug("getattr inode=%v", n.id)

	// We need to check this since we might have to return the size including
	// outstanding writes.
	if tf, isTernFile := f.(*ternFile); isTernFile {
		tf.mu.RLock()
		defer tf.mu.RUnlock()
		if ttf, isTransientFile := tf.body.(*transientFile); isTransientFile {
			logger.Debug("geattr inode=%v trying transient", n.id)
			resp := msgs.StatTransientFileResp{}
			if err := c.ShardRequest(logger, tf.id.Shard(), &msgs.StatTransientFileReq{Id: tf.id}, &resp); err != nil {
				// this might not be transient anymore
				logger.Debug("file=%v is not transient anymore, getting normal file attributes", n.id)
				goto NotTransient
			}
			out.Size = uint64(ttf.written) + uint64(len(ttf.span.Bytes()))
			mtime := uint64(resp.Mtime)
			mtimesec := mtime / 1000000000
			mtimens := uint32(mtime % 1000000000)
			out.Mtime = mtimesec
			out.Mtimensec = mtimens
			out.Atime = mtimesec
			out.Atimensec = mtimens
			out.Owner.Gid = filesGid
			out.Owner.Uid = filesUid
			return 0
		}
	}
NotTransient:

	out.Ino = uint64(n.id)
	out.Mode = inodeTypeToMode(n.id.Type())
	if n.id.Type() == msgs.DIRECTORY {
		var resp msgs.StatDirectoryResp
		if err := shardRequest(n.id.Shard(), &msgs.StatDirectoryReq{Id: n.id}, &resp); err != 0 {
			return err
		}
		mtime := uint64(resp.Mtime)
		mtimesec := mtime / 1000000000
		mtimens := uint32(mtime % 1000000000)
		out.Mtime = mtimesec
		out.Mtimensec = mtimens
	} else {
		var resp msgs.StatFileResp
		err := c.ShardRequest(logger, n.id.Shard(), &msgs.StatFileReq{Id: n.id}, &resp)

		if err != nil {
			return ternErrToErrno(err)
		}
		out.Size = resp.Size
		mtime := uint64(resp.Mtime)
		mtimesec := mtime / 1000000000
		mtimens := uint32(mtime % 1000000000)
		out.Ctime = mtimesec
		out.Ctimensec = mtimens
		out.Mtime = mtimesec
		out.Mtimensec = mtimens
		atime := uint64(resp.Atime)
		out.Atime = atime / 1000000000
		out.Atimensec = uint32(atime % 1000000000)
		out.Owner.Gid = filesGid
		out.Owner.Uid = filesUid
	}
	return 0
}

func (n *ternNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	return n.getattr(f, &out.Attr)
}

func (n *ternNode) Lookup(
	ctx context.Context, name string, out *fuse.EntryOut,
) (*fs.Inode, syscall.Errno) {
	logger.Debug("lookup dir=%v, name=%q", n.id, name)
	resp := msgs.LookupResp{}
	if err := shardRequest(n.id.Shard(), &msgs.LookupReq{DirId: n.id, Name: name}, &resp); err != 0 {
		return nil, err
	}
	mode := uint32(0)
	switch resp.TargetId.Type() {
	case msgs.DIRECTORY:
		mode = syscall.S_IFDIR
	case msgs.FILE:
		mode = syscall.S_IFREG
	case msgs.SYMLINK:
		mode = syscall.S_IFLNK
	default:
		panic(fmt.Errorf("bad type %v", resp.TargetId.Type()))
	}
	newNode := &ternNode{id: resp.TargetId}
	if err := newNode.getattr(nil, &out.Attr); err != 0 {
		return nil, err
	}
	return n.NewInode(ctx, newNode, fs.StableAttr{Ino: uint64(resp.TargetId), Mode: mode}), 0
}

type transientFile struct {
	cookie        [8]byte
	dir           msgs.InodeId
	spanPolicies  msgs.SpanPolicy
	blockPolicies msgs.BlockPolicy
	stripePolicy  msgs.StripePolicy
	name          string
	written       int64        // already flushed
	span          *bufpool.Buf // null if we've already flushed the file, never zero
}

type failedTransientFile struct {
	writeError syscall.Errno
}

type readFile struct {
	wg   sync.WaitGroup
	once sync.Once
	data *[]byte
	err  error
}

type ternFile struct {
	id   msgs.InodeId
	mu   sync.RWMutex
	body any // one of *transientFile/failedTransientFile/*readFile
}

func (n *ternNode) createInternal(name string, flags uint32, mode uint32) (tf *ternFile, errno syscall.Errno) {
	req := msgs.ConstructFileReq{Note: name}
	resp := msgs.ConstructFileResp{}
	if (mode & syscall.S_IFMT) == syscall.S_IFREG {
		req.Type = msgs.FILE
	} else if (mode & syscall.S_IFMT) == syscall.S_IFLNK {
		req.Type = msgs.SYMLINK
	} else {
		panic(fmt.Errorf("bad mode %v", mode))
	}
	if err := shardRequest(n.id.Shard(), &req, &resp); err != 0 {
		return nil, err
	}
	span := bufPool.Get(0)
	transient := &transientFile{
		name:   name,
		dir:    n.id,
		span:   span,
		cookie: resp.Cookie,
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, n.id, &transient.spanPolicies); err != nil {
		return nil, ternErrToErrno(err)
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, n.id, &transient.blockPolicies); err != nil {
		return nil, ternErrToErrno(err)
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, n.id, &transient.stripePolicy); err != nil {
		return nil, ternErrToErrno(err)
	}
	f := ternFile{
		id:   resp.Id,
		body: transient,
	}
	return &f, 0
}

func (n *ternNode) Create(
	ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut,
) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger.Debug("create id=%v, name=%q, flags=0x%08x, mode=0x%08x", n.id, name, flags, mode)
	tf, err := n.createInternal(name, flags, mode)
	if err != 0 {
		return nil, nil, 0, err
	}
	fileNode := ternNode{
		id: tf.id,
	}
	logger.Debug("created id=%v", tf.id)
	return n.NewInode(ctx, &fileNode, fs.StableAttr{Ino: uint64(tf.id), Mode: mode}), tf, 0, 0
}

func (n *ternNode) Mkdir(
	ctx context.Context, name string, mode uint32, out *fuse.EntryOut,
) (*fs.Inode, syscall.Errno) {
	logger.Debug("mkdir dir=%v, name=%q, mode=0x%08x", n.id, name, mode)

	// TODO would probably be better to check the mode and return
	// EINVAL if it doesn't match what we want.
	req := msgs.MakeDirectoryReq{
		OwnerId: n.id,
		Name:    name,
	}
	resp := msgs.MakeDirectoryResp{}
	if err := cdcRequest(&req, &resp); err != 0 {
		return nil, err
	}
	return n.NewInode(ctx, &ternNode{id: resp.Id}, fs.StableAttr{Ino: uint64(resp.Id), Mode: syscall.S_IFDIR}), 0
}

// Writes out current span. Does _not_ replenish f.span. Must be a locked transient file.
func (f *ternFile) writeSpan() syscall.Errno {
	tf := f.body.(*transientFile)
	toWrite := uint32(len(tf.span.Bytes()))
	logger.Debug("%v: writing span from %v, %v bytes", f.id, tf.written, toWrite)
	var err error
	defer func() {
		bufPool.Put(tf.span)
		tf.span = nil
		if err == nil {
			tf.written += int64(toWrite)
		} else {
			f.body = failedTransientFile{ternErrToErrno(err)}
		}
	}()
	if toWrite == 0 { // happens when closing file
		return 0
	}
	_, err = c.CreateSpan(
		logger,
		[]msgs.BlacklistEntry{},
		&tf.spanPolicies,
		&tf.blockPolicies,
		&tf.stripePolicy,
		f.id,
		msgs.NULL_INODE_ID,
		tf.cookie,
		uint64(tf.written),
		uint32(toWrite),
		tf.span.BytesPtr(),
	)
	if err != nil {
		return ternErrToErrno(err)
	}
	return 0
}

// Must be a locked transient file
func (f *ternFile) write(data []byte) (written uint32, errno syscall.Errno) {
	tf := f.body.(*transientFile)
	cursor := 0
	maxSpanSize := tf.spanPolicies.Entries[len(tf.spanPolicies.Entries)-1].MaxSize
	for cursor < len(data) {
		remainingInSpan := maxSpanSize - uint32(len(tf.span.Bytes()))
		toWrite := min(int(remainingInSpan), len(data)-cursor)
		*tf.span.BytesPtr() = append(*tf.span.BytesPtr(), data[cursor:cursor+toWrite]...)
		if len(tf.span.Bytes()) >= int(maxSpanSize) {
			if err := f.writeSpan(); err != 0 {
				return uint32(cursor), err
			}
			tf.span = bufPool.Get(0)
		}
		cursor += toWrite
	}
	return uint32(len(data)), 0
}

func (f *ternFile) rlock(locked *bool) {
	*locked = false
	f.mu.RLock()
}

func (f *ternFile) writeLock(locked *bool) {
	f.mu.RUnlock()
	*locked = true
	f.mu.Lock()
}

func (f *ternFile) unlock(locked *bool) {
	if *locked {
		f.mu.Unlock()
	} else {
		f.mu.RUnlock()
	}
}

func (f *ternFile) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	logger.Debug("write file=%v, off=%v, count=%v", f.id, off, len(data))

	if len(data) == 0 {
		logger.Debug("zero-write, returning early")
		return 0, 0
	}

	var writeLocked bool
	f.rlock(&writeLocked)
	defer f.unlock(&writeLocked)

	switch tf := f.body.(type) {
	case *transientFile:
		if off < tf.written+int64(len(tf.span.Bytes())) {
			logger.Info("refusing to write in the past off=%v written=%v len=%v", off, tf.written, int64(len(tf.span.Bytes())))
			return 0, syscall.EINVAL
		}
		f.writeLock(&writeLocked)
		zeros := off - (tf.written + int64(len(tf.span.Bytes())))
		if zeros > 0 {
			logger.Debug("file=%v writing %v zeros", f.id, zeros)
			_, errno = f.write(make([]byte, zeros))
			if errno != 0 {
				return 0, errno // the zero bytes written do not count
			}
		}
		return f.write(data)
	case *readFile:
		return 0, syscall.ENOTSUP
	case failedTransientFile:
		return 0, tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func readFileSetattr(id msgs.InodeId, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if in.Valid&^(fuse.FATTR_ATIME|fuse.FATTR_MTIME|fuse.FATTR_LOCKOWNER|fuse.FATTR_FH) != 0 {
		logger.Debug("inode=%v bad valid %08x", id, in.Valid)
		return syscall.ENOTSUP
	}
	req := &msgs.SetTimeReq{
		Id: id,
	}
	if atime, ok := in.GetATime(); ok {
		nanos := atime.UnixNano()
		req.Atime = uint64(nanos) | (uint64(1) << 63)
		out.Atime = uint64(nanos / 1000000000)
		out.Atimensec = uint32(nanos % 1000000000)
	}
	if mtime, ok := in.GetMTime(); ok {
		nanos := mtime.UnixNano()
		req.Mtime = uint64(nanos) | (uint64(1) << 63)
		out.Mtime = uint64(nanos / 1000000000)
		out.Mtimensec = uint32(nanos % 1000000000)
	}
	if err := shardRequest(id.Shard(), req, &msgs.SetTimeResp{}); err != 0 {
		return err
	}
	return 0
}

func (f *ternFile) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	logger.Debug("setattr inode=%v, in=%+v", f.id, in)

	var writeLocked bool
	f.rlock(&writeLocked)
	defer f.unlock(&writeLocked)

	switch tf := f.body.(type) {
	case *transientFile: // we can extend transient files using ftruncate
		f.writeLock(&writeLocked)
		if in.Valid&^(fuse.FATTR_SIZE|fuse.FATTR_LOCKOWNER|fuse.FATTR_FH) != 0 {
			logger.Debug("inode=%v bad valid %08x", f.id, in.Valid)
			return syscall.ENOTSUP
		}
		sz := tf.written + int64(len(tf.span.Bytes()))
		if int64(in.Size) < sz { // can't shorten
			logger.Debug("inode=%v would shorten %v -> %v", f.id, in.Size, sz)
			return syscall.ENOTSUP
		}
		maxSpanSize := tf.spanPolicies.Entries[len(tf.spanPolicies.Entries)-1].MaxSize
		remaining := int64(in.Size) - sz
		buf := make([]byte, min(int64(maxSpanSize), remaining))
		logger.Debug("setattr transientFile size sz=%v in.Size=%v", sz, in.Size)
		for remaining > 0 {
			toWrite := min(remaining, int64(maxSpanSize))
			if _, err := f.write(buf[:toWrite]); err != 0 {
				return err
			}
			remaining -= toWrite
		}
		return 0
	case *readFile: // we can set times for already linked files
		logger.Debug("setattr readFile")
		return readFileSetattr(f.id, in, out)
	case failedTransientFile:
		return syscall.ENOTSUP
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func (f *ternFile) Flush(ctx context.Context) syscall.Errno {
	logger.Debug("flush file=%v", f.id)

	var writeLocked bool
	f.rlock(&writeLocked)
	defer f.unlock(&writeLocked)

	switch tf := f.body.(type) {
	case *transientFile:
		if closeMap != nil {
			var closes uint64
			if err := closeMap.LookupAndDelete(uint64(f.id), &closes); err != nil {
				if errors.Is(err, ebpf.ErrKeyNotExist) {
					logger.Info("flush file=%v no close key", f.id)
					return 0
				}
				logger.RaiseAlert("Could not lookup key in close map for file %v: %v", f.id, err)
				return syscall.EIO
			}
			if closes == 0 {
				logger.RaiseAlert("Unexpected zero close count in close map for file %v", f.id)
				return syscall.EIO
			}
			logger.Debug("Found file %v in close map, will close", f.id)
		}
		// We're committed to closing now
		f.writeLock(&writeLocked)
		if err := f.writeSpan(); err != 0 {
			logger.Debug("tf %v could not write span, %v", f.id, err)
			return err
		}
		req := msgs.LinkFileReq{
			FileId:  f.id,
			Cookie:  tf.cookie,
			OwnerId: tf.dir,
			Name:    tf.name,
		}
		if err := shardRequest(tf.dir.Shard(), &req, &msgs.LinkFileResp{}); err != 0 {
			f.body = failedTransientFile{err}
			return err
		}
		of := &readFile{}
		of.wg.Add(1)
		f.body = of
		return 0
	case *readFile:
		return 0
	case failedTransientFile:
		return tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func (n *ternNode) Rename(ctx context.Context, oldName string, newParent0 fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	oldParent := n.id
	newParent := newParent0.(*ternNode).id

	logger.Debug("rename dir=%v oldParent=%v, oldName=%v, newParent=%v, newName=%v", n, oldParent, oldName, newParent, newName)

	var targetId msgs.InodeId
	var oldCreationTime msgs.TernTime
	{
		req := msgs.LookupReq{DirId: oldParent, Name: oldName}
		resp := msgs.LookupResp{}
		if err := shardRequest(oldParent.Shard(), &req, &resp); err != 0 {
			return err
		}
		targetId = resp.TargetId
		oldCreationTime = resp.CreationTime
	}

	if oldParent == newParent {
		req := msgs.SameDirectoryRenameReq{
			TargetId:        targetId,
			DirId:           oldParent,
			OldName:         oldName,
			OldCreationTime: oldCreationTime,
			NewName:         newName,
		}
		if err := shardRequest(oldParent.Shard(), &req, &msgs.SameDirectoryRenameResp{}); err != 0 {
			return err
		}
	} else if targetId.Type() == msgs.DIRECTORY {
		req := msgs.RenameDirectoryReq{
			TargetId:        targetId,
			OldOwnerId:      oldParent,
			OldName:         oldName,
			OldCreationTime: oldCreationTime,
			NewOwnerId:      newParent,
			NewName:         newName,
		}
		if err := cdcRequest(&req, &msgs.RenameDirectoryResp{}); err != 0 {
			return err
		}
	} else {
		req := msgs.RenameFileReq{
			TargetId:        targetId,
			OldOwnerId:      oldParent,
			OldName:         oldName,
			OldCreationTime: oldCreationTime,
			NewOwnerId:      newParent,
			NewName:         newName,
		}
		if err := cdcRequest(&req, &msgs.RenameFileResp{}); err != 0 {
			return err
		}
	}

	return 0
}

func (n *ternNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger.Debug("open file=%v flags=%08x", n.id, flags)

	of := &readFile{}
	of.wg.Add(1)
	f := ternFile{
		id:   n.id,
		body: of,
	}

	if flags|syscall.O_NOATIME != 0 {
		c.ShardRequestDontWait(logger, n.id.Shard(), &msgs.SetTimeReq{
			Id:    n.id,
			Atime: uint64(time.Now().UnixNano()) | (uint64(1) << 63),
		})
	}

	return &f, fuse.FOPEN_CACHE_DIR | fuse.FOPEN_KEEP_CACHE, 0
}

func (n *ternNode) OpendirHandle(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger.Debug("open dir=%v flags=%08x", n.id, flags)

	statResp := &msgs.StatDirectoryResp{}
	if err := shardRequest(n.id.Shard(), &msgs.StatDirectoryReq{Id: n.id}, statResp); err != 0 {
		return nil, 0, ternErrToErrno(err)
	}
	parent := n.id
	if statResp.Owner != msgs.NULL_INODE_ID {
		parent = statResp.Owner
	}

	od := &openDirectory{
		id:     n.id,
		parent: parent,
		cursor: -1,
	}
	return od, fuse.FOPEN_CACHE_DIR | fuse.FOPEN_KEEP_CACHE, 0
}

func (f *ternFile) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	switch tf := f.body.(type) {
	case *transientFile:
		return nil, syscall.ENOTSUP
	case *readFile:
		// Obviously we shouldn't read all at once...
		tf.once.Do(func() {
			defer tf.wg.Done()
			buf, err := c.FetchFile(logger, bufPool, f.id)
			if err != nil {
				tf.err = err
				return
			}
			bufData := buf.Bytes()
			tf.data = &bufData
		})
		tf.wg.Wait()
		if tf.err != nil {
			logger.ErrorNoAlert("read error: %v", tf.err)
			return fuse.ReadResultData([]byte{}), syscall.EIO
		}
		data := *tf.data
		logger.Debug("read id=%v off=%v count=%v len=%v", f.id, off, len(dest), len(data))
		if off > int64(len(data)) {
			return fuse.ReadResultData([]byte{}), 0
		}
		r := copy(dest, data[off:])
		return fuse.ReadResultData(dest[:r]), 0
	case failedTransientFile:
		return nil, tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func (n *ternNode) Unlink(ctx context.Context, name string) syscall.Errno {
	logger.Debug("unlink dir=%v, name=%q", n.id, name)

	lookupResp := msgs.LookupResp{}
	if err := shardRequest(n.id.Shard(), &msgs.LookupReq{DirId: n.id, Name: name}, &lookupResp); err != 0 {
		return err
	}
	unlinkReq := msgs.SoftUnlinkFileReq{
		OwnerId:      n.id,
		FileId:       lookupResp.TargetId,
		Name:         name,
		CreationTime: lookupResp.CreationTime,
	}
	return shardRequest(n.id.Shard(), &unlinkReq, &msgs.SoftUnlinkFileResp{})
}

func (n *ternNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	logger.Debug("rmdir dir=%v, name=%q", n.id, name)
	lookupResp := msgs.LookupResp{}
	if err := shardRequest(n.id.Shard(), &msgs.LookupReq{DirId: n.id, Name: name}, &lookupResp); err != 0 {
		return err
	}
	unlinkReq := msgs.SoftUnlinkDirectoryReq{
		OwnerId:      n.id,
		TargetId:     lookupResp.TargetId,
		Name:         name,
		CreationTime: lookupResp.CreationTime,
	}
	return cdcRequest(&unlinkReq, &msgs.SoftUnlinkDirectoryResp{})
}

func (n *ternNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger.Debug("symlink dir=%v, target=%v, name=%q", n.id, target, name)
	tf, err := n.createInternal(name, 0, syscall.S_IFLNK)
	if err != 0 {
		return nil, err
	}
	if _, err := tf.Write(ctx, []byte(target), 0); err != 0 {
		return nil, err
	}
	if err := tf.Flush(ctx); err != 0 {
		return nil, err
	}
	return n.NewInode(ctx, &ternNode{id: tf.id}, fs.StableAttr{Ino: uint64(tf.id), Mode: inodeTypeToMode(tf.id.Type())}), 0
}

func (n *ternNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	logger.Debug("readlink file=%v", n.id)

	if n.id.Type() != msgs.SYMLINK {
		return nil, syscall.EINVAL
	}

	fileReader, err := c.ReadFile(logger, bufPool, n.id)
	if err != nil {
		return nil, ternErrToErrno(err)
	}
	bs, err := io.ReadAll(fileReader)
	if err != nil {
		return nil, ternErrToErrno(err)
	}
	return bs, 0
}

func (n *ternNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if f, ok := f.(*ternFile); ok {
		return f.Setattr(ctx, in, out)
	}
	return readFileSetattr(n.id, in, out)
}

type openDirectory struct {
	mu      sync.Mutex
	id      msgs.InodeId
	parent  msgs.InodeId
	off     uint64
	entries []fuse.DirEntry
	cursor  int // If >=0, we can just read from resp at that index
}

func (d *openDirectory) Seekdir(ctx context.Context, off uint64) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()

	logger.Debug("dir=%v seek %v -> %v", d.id, d.off, off)
	if d.off != off {
		d.off = off
		d.cursor = -1
	}
	return 0
}

func (d *openDirectory) hasNext() (bool, syscall.Errno) {
	// If we already have a cursor...
	if d.cursor > 0 {
		// If we've stepped out of bounds...
		if d.cursor >= len(d.entries) {
			logger.Debug("dir=%v stepped out of bounds (%v >= %v)", d.id, d.cursor, len(d.entries))
			// ...and we were at the last one, we're done
			if d.entries[len(d.entries)-1].Off == 0 {
				logger.Debug("dir=%v finished entries", d.id)
				return false, 0
			}
			// ...otherwise we'll just get a new batch
			d.cursor = -1
			goto NoCursor
		}
		// If we're in bounds, just return the thing and switch to next element
		logger.Debug("dir=%v have entry at cursor %v", d.id, d.cursor)
		return true, 0
	}
NoCursor:

	// If we don't have a cursor, get at the current offset
	logger.Debug("dir=%v fetching entries at %v", d.id, d.off)
	var resp msgs.ReadDirResp
	if err := shardRequest(d.id.Shard(), &msgs.ReadDirReq{DirId: d.id, StartHash: msgs.NameHash(d.off)}, &resp); err != 0 {
		return false, ternErrToErrno(err)
	}
	d.entries = make([]fuse.DirEntry, len(resp.Results))
	for i := range resp.Results {
		result := &resp.Results[i]
		d.entries[i] = fuse.DirEntry{
			Mode: inodeTypeToMode(result.TargetId.Type()),
			Name: result.Name,
			Ino:  uint64(result.TargetId),
			Off:  uint64(result.NameHash),
		}
	}
	// If we're seeking at off 0 or 1, add "." and "..". Note that we don't use
	// CreationTime.
	if d.off <= 1 {
		var parentIndex int
		if d.off == 0 {
			d.entries = append(
				[]fuse.DirEntry{
					{Mode: syscall.S_IFDIR, Name: ".", Ino: uint64(d.id), Off: 0},
					{Mode: syscall.S_IFDIR, Name: "..", Ino: uint64(d.parent), Off: 1},
				},
				d.entries...,
			)
			parentIndex = 1
		} else {
			d.entries = append(
				[]fuse.DirEntry{
					{Mode: syscall.S_IFDIR, Name: "..", Ino: uint64(d.parent), Off: 1},
				},
				d.entries...,
			)
			parentIndex = 0
		}
		// It's very unlikely, but we might have other zero-valued hashes
		// apart from ".", they must come before ".."
		for parentIndex+1 < len(d.entries) && d.entries[parentIndex].Off > d.entries[parentIndex+1].Off {
			d.entries[parentIndex], d.entries[parentIndex+1] = d.entries[parentIndex+1], d.entries[parentIndex]
			parentIndex++
		}
	}
	// Fixup offsets (they're the "next" offset)
	for i := range d.entries {
		if i == len(d.entries)-1 {
			if resp.NextHash == 0 {
				d.entries[i].Off = (uint64(1) << 63) - 1
			} else {
				d.entries[i].Off = uint64(resp.NextHash)
			}
		} else {
			d.entries[i].Off = d.entries[i+1].Off
		}
	}
	// If wee couldn't find what anything, we're out of bounds
	if len(d.entries) == 0 {
		logger.Debug("dir=%v could not find anything at %v", d.id, d.off)
		d.cursor = -1
		return false, 0
	}
	// Otherwise search for the cursor and return entry
	d.cursor = sort.Search(len(d.entries), func(i int) bool {
		return uint64(d.entries[i].Off) >= d.off
	})
	logger.Debug("dir=%v found off %v", d.id, d.off)
	return true, 0
}

func (d *openDirectory) Readdirent(ctx context.Context) (*fuse.DirEntry, syscall.Errno) {
	logger.Debug("dir=%v readdirrent", d.id)

	d.mu.Lock()
	defer d.mu.Unlock()

	found, err := d.hasNext()
	if err != 0 {
		return nil, err
	}
	if !found {
		return nil, 0
	}

	de := &d.entries[d.cursor]
	d.off = de.Off
	d.cursor++
	return de, 0
}

func (d *openDirectory) Close() {}

var _ = (fs.InodeEmbedder)((*ternNode)(nil))
var _ = (fs.NodeLookuper)((*ternNode)(nil))
var _ = (fs.NodeMkdirer)((*ternNode)(nil))
var _ = (fs.NodeGetattrer)((*ternNode)(nil))
var _ = (fs.NodeCreater)((*ternNode)(nil))
var _ = (fs.NodeRenamer)((*ternNode)(nil))
var _ = (fs.NodeOpener)((*ternNode)(nil))
var _ = (fs.NodeOpendirHandler)((*ternNode)(nil))
var _ = (fs.NodeUnlinker)((*ternNode)(nil))
var _ = (fs.NodeRmdirer)((*ternNode)(nil))
var _ = (fs.NodeSymlinker)((*ternNode)(nil))
var _ = (fs.NodeReadlinker)((*ternNode)(nil))
var _ = (fs.NodeSetattrer)((*ternNode)(nil))

var _ = (fs.FileWriter)((*ternFile)(nil))
var _ = (fs.FileFlusher)((*ternFile)(nil))
var _ = (fs.FileSetattrer)((*ternFile)(nil))
var _ = (fs.FileReader)((*ternFile)(nil))

var _ = (fs.FileSeekdirer)((*openDirectory)(nil))
var _ = (fs.FileReaddirenter)((*openDirectory)(nil))

func initializeCloseMap(closeTrackerObj string, mountPoint string) {
	logger.Info("Will use BPF object %q to setup close map", closeTrackerObj)

	// It's often required to remove memory lock limits for BPF programs
	if err := rlimit.RemoveMemlock(); err != nil {
		panic(err)
	}

	// Load the compiled BPF ELF file
	spec, err := ebpf.LoadCollectionSpec(closeTrackerObj)
	if err != nil {
		panic(err)
	}

	// Get the device ID for the given path
	var stat syscall.Stat_t
	if err := syscall.Stat(mountPoint, &stat); err != nil {
		panic(err)
	}
	// dev_t is 4-byte in the kernel
	if stat.Dev > uint64(^uint32(0)) {
		panic(fmt.Errorf("overlong device id for FUSE mount %016x", stat.Dev))
	}

	// Rewrite the 'target_dev' constant in the BPF program
	if err := spec.RewriteConstants(map[string]interface{}{
		"target_dev": uint32(stat.Dev),
	}); err != nil {
		panic(err)
	}

	// Load the BPF programs and maps into the kernel
	closeMapCollection, err = ebpf.NewCollection(spec)
	if err != nil {
		panic(err)
	}

	// Find the BPF program by its section name from the C code
	prog := closeMapCollection.Programs["handle_close"]
	if prog == nil {
		panic(fmt.Errorf("could not find BPF program handle_close"))
	}

	// Attach the BPF program to the tracepoint
	// The group is "syscalls" and the name is "sys_enter_close"
	closeMapTracepoint, err = link.Tracepoint("syscalls", "sys_enter_close", prog, nil)
	if err != nil {
		panic(err)
	}

	// Find the map by its name from the C code
	closeMap = closeMapCollection.Maps["closed_inodes_map"]
	if closeMap == nil {
		panic(fmt.Errorf("BPF map 'closed_inodes_map' not found"))
	}
}

func cleanupCloseMap() {
	if closeMapCollection != nil {
		closeMapCollection.Close()
	}
	if closeMapTracepoint != nil {
		closeMapTracepoint.Close()
	}
}

func terminate(server *fuse.Server, terminated *bool) {
	logger.Info("terminating")
	if *terminated {
		logger.Info("already terminated")
		return
	}
	logger.Info("stopping cpu profile")
	pprof.StopCPUProfile()
	logger.Info("about to terminate")
	if err := server.Unmount(); err != nil {
		logger.Info("could not unmount: %v\n", err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <mountpoint>\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	verbose := flag.Bool("verbose", false, "Enables debug logging.")
	trace := flag.Bool("trace", false, "")
	logFile := flag.String("log-file", "", "Redirect logging output to given file.")
	signalParent := flag.Bool("signal-parent", false, "If passed, will send USR1 to parent when ready -- useful for tests.")
	registryAddress := flag.String("registry", "", "Registry address (host:port).")
	var addresses flags.StringArrayFlags
	flag.Var(&addresses, "addr", "Local addresses (up to two) to connect from.")
	profileFile := flag.String("profile-file", "", "If set, will write CPU profile here.")
	syslog := flag.Bool("syslog", false, "")
	allowOther := flag.Bool("allow-other", false, "")
	initialShardTimeout := flag.Duration("initial-shard-timeout", 0, "")
	maxShardTimeout := flag.Duration("max-shard-timeout", 0, "")
	overallShardTimeout := flag.Duration("overall-shard-timeout", 0, "")
	initialCDCTimeout := flag.Duration("initial-cdc-timeout", 0, "")
	maxCDCTimeout := flag.Duration("max-cdc-timeout", 0, "")
	overallCDCTimeout := flag.Duration("overall-cdc-timeout", 0, "")
	initialBlockTimeout := flag.Duration("initial-block-timeout", 0, "")
	maxBlockTimeout := flag.Duration("max-block-timeout", 0, "")
	overallBlockTimeout := flag.Duration("overall-block-timeout", 0, "")
	mtu := flag.Uint64("mtu", msgs.DEFAULT_UDP_MTU, "mtu used talking to shards and cdc")
	fileAttrCacheTimeFlag := flag.Duration("file-attr-cache-time", time.Millisecond*250, "time to cache file attributes for read-only files before rechecking with the server. Set to 0 to disable caching. (default: 250ms)")
	dirAttrCacheTimeFlag := flag.Duration("dir-attr-cache-time", time.Millisecond*250, "time to cache directory attributes before rechecking with the server. Set to 0 to disable caching. (default: 250ms)")
	closeTrackerObject := flag.String("close-tracker-object", "", "Compiled BPF object to track explicitly closed files")
	setUid := flag.Bool("set-uid", false, "")
	flag.Usage = usage
	flag.Parse()

	if *registryAddress == "" {
		fmt.Fprintf(os.Stderr, "You need to specify -registry\n")
		os.Exit(2)
	}

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	mountPoint := flag.Args()[0]

	client.SetMTU(*mtu)

	if mountPoint == "" {
		fmt.Fprintf(os.Stderr, "Please specify mountpoint with -mountpoint\n")
		os.Exit(2)
	}

	logOut := os.Stdout
	if *logFile != "" {
		var err error
		logOut, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open file %v: %v", logFile, err)
			os.Exit(1)
		}
		defer logOut.Close()
	}
	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	logger = log.NewLogger(logOut, &log.LoggerOptions{Level: level, Syslog: *syslog, PrintQuietAlerts: true})

	if *profileFile != "" {
		f, err := os.Create(*profileFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open profile file %v", *profileFile)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f) // we stop in terminate()
	}

	var localAddresses msgs.AddrsInfo
	if len(addresses) > 0 {
		ownIp1, port1, err := flags.ParseIPV4Addr(addresses[0])
		if err != nil {
			panic(err)
		}
		localAddresses.Addr1 = msgs.IpPort{Addrs: ownIp1, Port: port1}
		var ownIp2 [4]byte
		var port2 uint16
		if len(addresses) == 2 {
			ownIp2, port2, err = flags.ParseIPV4Addr(addresses[1])
			if err != nil {
				panic(err)
			}
		}
		localAddresses.Addr2 = msgs.IpPort{Addrs: ownIp2, Port: port2}
	}

	counters := client.NewClientCounters()

	var err error
	c, err = client.NewClient(logger, nil, *registryAddress, localAddresses)
	if err != nil {
		panic(err)
	}
	c.SetCounters(counters)

	shardTimeouts := client.DefaultShardTimeout
	if *initialShardTimeout != 0 {
		shardTimeouts.Initial = *initialShardTimeout
	}
	if *maxShardTimeout != 0 {
		shardTimeouts.Max = *maxShardTimeout
	}
	if *overallShardTimeout != 0 {
		shardTimeouts.Overall = *overallShardTimeout
	}
	c.SetShardTimeouts(&shardTimeouts)
	cdcTimeouts := client.DefaultCDCTimeout
	if *initialCDCTimeout != 0 {
		cdcTimeouts.Initial = *initialCDCTimeout
	}
	if *maxCDCTimeout != 0 {
		cdcTimeouts.Max = *maxCDCTimeout
	}
	if *overallCDCTimeout != 0 {
		cdcTimeouts.Overall = *overallCDCTimeout
	}
	c.SetCDCTimeouts(&cdcTimeouts)
	blockTimeouts := client.DefaultBlockTimeout
	if *initialBlockTimeout != 0 {
		blockTimeouts.Initial = *initialBlockTimeout
	}
	if *maxBlockTimeout != 0 {
		blockTimeouts.Max = *maxBlockTimeout
	}
	if *overallBlockTimeout != 0 {
		blockTimeouts.Overall = *overallBlockTimeout
	}
	c.SetBlockTimeouts(&blockTimeouts)

	dirInfoCache = client.NewDirInfoCache()

	bufPool = bufpool.NewBufPool()

	if *setUid {
		filesUid = uint32(os.Geteuid())
		filesGid = uint32(os.Getegid())
	}

	root := ternNode{
		id: msgs.ROOT_DIR_INODE_ID,
	}
	fuseOptions := &fs.Options{
		Logger:       golog.New(os.Stderr, "fuse", golog.Ldate|golog.Ltime|golog.Lmicroseconds|golog.Lshortfile),
		AttrTimeout:  fileAttrCacheTimeFlag,
		EntryTimeout: dirAttrCacheTimeFlag,
		MountOptions: fuse.MountOptions{
			FsName:             "ternfs",
			Name:               "ternfuse" + mountPoint,
			MaxWrite:           1 << 20,
			MaxReadAhead:       1 << 20,
			DisableXAttrs:      true,
			Debug:              *verbose,
			DisableReadDirPlus: true,
		},
	}
	if *allowOther {
		fuseOptions.MountOptions.Options = append(fuseOptions.MountOptions.Options, "allow_other")
	}
	server, err := fs.Mount(mountPoint, &root, fuseOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not mount: %v", err)
		os.Exit(1)
	}

	logger.Info("mounted at %v", mountPoint)

	// We need to run this _after_ we've mounted so that we can get the dev id
	if *closeTrackerObject != "" {
		initializeCloseMap(*closeTrackerObject, mountPoint)
		defer cleanupCloseMap()
	}

	if *signalParent {
		logger.Info("sending USR1 to parent")
		if err := syscall.Kill(os.Getppid(), syscall.SIGUSR1); err != nil {
			panic(err)
		}
	}

	// print out stats when sent USR1
	{
		statsChan := make(chan os.Signal, 1)
		signal.Notify(statsChan, syscall.SIGUSR1)
		go func() {
			for {
				<-statsChan
				counters.Log(logger)
			}
		}()
	}

	terminated := false
	defer terminate(server, &terminated)
	// Cleanup if we get killed with a signal. Obviously we can't do much
	// in the case of SIGKILL or SIGQUIT.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)
	go func() {
		sig := <-signalChan
		signal.Stop(signalChan)
		terminate(server, &terminated)
		cleanupCloseMap()
		syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
	}()

	server.Wait()
}
