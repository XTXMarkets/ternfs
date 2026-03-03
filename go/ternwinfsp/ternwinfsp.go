// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"xtx/ternfs/cleanup"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"github.com/winfsp/cgofuse/fuse"
)

var c *client.Client
var logger *log.Logger
var dirInfoCache *client.DirInfoCache
var bufPool *bufpool.BufPool
var readdirBatchSize int

// Handle management
var nextHandle uint64 = 1
var handleMu sync.RWMutex
var fileHandles = make(map[uint64]*fileHandle)
var dirHandles = make(map[uint64]*dirHandle)

func allocHandle() uint64 {
	return atomic.AddUint64(&nextHandle, 1)
}

func ternErrToFuseErr(err error) int {
	switch err {
	case nil:
		return 0
	case msgs.INTERNAL_ERROR:
		return -fuse.EIO
	case msgs.FATAL_ERROR:
		return -fuse.EIO
	case msgs.TIMEOUT:
		return -fuse.ETIMEDOUT
	case msgs.NOT_AUTHORISED:
		return -fuse.EACCES
	case msgs.UNRECOGNIZED_REQUEST:
		return -fuse.EIO
	case msgs.FILE_NOT_FOUND:
		return -fuse.ENOENT
	case msgs.DIRECTORY_NOT_FOUND:
		return -fuse.ENOENT
	case msgs.NAME_NOT_FOUND:
		return -fuse.ENOENT
	case msgs.TYPE_IS_DIRECTORY:
		return -fuse.EISDIR
	case msgs.TYPE_IS_NOT_DIRECTORY:
		return -fuse.ENOTDIR
	case msgs.BAD_COOKIE:
		return -fuse.EACCES
	case msgs.INCONSISTENT_STORAGE_CLASS_PARITY:
		return -fuse.EINVAL
	case msgs.LAST_SPAN_STATE_NOT_CLEAN:
		return -fuse.EBUSY
	case msgs.COULD_NOT_PICK_BLOCK_SERVICES:
		return -fuse.EIO
	case msgs.BAD_SPAN_BODY:
		return -fuse.EINVAL
	case msgs.SPAN_NOT_FOUND:
		return -fuse.EINVAL
	case msgs.BLOCK_SERVICE_NOT_FOUND:
		return -fuse.EIO
	case msgs.CANNOT_CERTIFY_BLOCKLESS_SPAN:
		return -fuse.EINVAL
	case msgs.BAD_BLOCK_PROOF:
		return -fuse.EINVAL
	case msgs.CANNOT_OVERRIDE_NAME:
		return -fuse.EEXIST
	case msgs.NAME_IS_LOCKED:
		return -fuse.EEXIST
	case msgs.MTIME_IS_TOO_RECENT:
		return -fuse.EBUSY
	case msgs.MISMATCHING_TARGET:
		return -fuse.EINVAL
	case msgs.MISMATCHING_OWNER:
		return -fuse.EINVAL
	case msgs.DIRECTORY_NOT_EMPTY:
		return -fuse.ENOTEMPTY
	case msgs.FILE_IS_TRANSIENT:
		return -fuse.EBUSY
	case msgs.OLD_DIRECTORY_NOT_FOUND:
		return -fuse.ENOENT
	case msgs.NEW_DIRECTORY_NOT_FOUND:
		return -fuse.ENOENT
	case msgs.LOOP_IN_DIRECTORY_RENAME:
		return -fuse.ELOOP
	default:
		logger.Debug("unknown error %v", err)
		return -fuse.EIO
	}
}

func inodeTypeToMode(typ msgs.InodeType) uint32 {
	mode := uint32(0)
	mode |= fuse.S_IRUSR | fuse.S_IXUSR
	mode |= fuse.S_IRGRP | fuse.S_IXGRP
	mode |= fuse.S_IROTH | fuse.S_IXOTH
	if typ == msgs.FILE {
		mode |= fuse.S_IFREG
	}
	if typ == msgs.SYMLINK {
		mode |= fuse.S_IFLNK
	}
	if typ == msgs.DIRECTORY {
		mode |= fuse.S_IFDIR
	}
	return mode
}

func shardRequest(shid msgs.ShardId, req msgs.ShardRequest, resp msgs.ShardResponse) int {
	if err := c.ShardRequest(logger, shid, req, resp); err != nil {
		return ternErrToFuseErr(err)
	}
	return 0
}

func cdcRequest(req msgs.CDCRequest, resp msgs.CDCResponse) int {
	if err := c.CDCRequest(logger, req, resp); err != nil {
		switch ternErr := err.(type) {
		case msgs.TernError:
			return ternErrToFuseErr(ternErr)
		}
		panic(err)
	}
	return 0
}

// Path resolution: walk from root to resolve path to InodeId
func resolvePath(path string) (id msgs.InodeId, parent msgs.InodeId, name string, errno int) {
	// Normalize path
	if path == "/" || path == "" {
		return msgs.ROOT_DIR_INODE_ID, msgs.NULL_INODE_ID, "", 0
	}

	// Remove leading slash and split
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	currentId := msgs.ROOT_DIR_INODE_ID
	var parentId msgs.InodeId

	for i, part := range parts {
		if part == "" {
			continue
		}

		parentId = currentId
		resp := msgs.LookupResp{}
		if err := shardRequest(currentId.Shard(), &msgs.LookupReq{DirId: currentId, Name: part}, &resp); err != 0 {
			return msgs.NULL_INODE_ID, msgs.NULL_INODE_ID, "", err
		}

		currentId = resp.TargetId

		// If this is the last component, return the name too
		if i == len(parts)-1 {
			name = part
		}
	}

	return currentId, parentId, name, 0
}

// resolveParentPath resolves the parent directory of a path
func resolveParentPath(path string) (parentId msgs.InodeId, name string, errno int) {
	if path == "/" || path == "" {
		return msgs.NULL_INODE_ID, "", -fuse.EINVAL
	}

	path = strings.TrimPrefix(path, "/")
	lastSlash := strings.LastIndex(path, "/")

	if lastSlash == -1 {
		// File in root directory
		return msgs.ROOT_DIR_INODE_ID, path, 0
	}

	parentPath := "/" + path[:lastSlash]
	name = path[lastSlash+1:]

	parentId, _, _, errno = resolvePath(parentPath)
	return parentId, name, errno
}

// transientFile represents a file being written
type transientFile struct {
	cookie        [8]byte
	dir           msgs.InodeId
	spanPolicies  msgs.SpanPolicy
	blockPolicies msgs.BlockPolicy
	stripePolicy  msgs.StripePolicy
	name          string
	written       int64
	span          *bufpool.Buf
}

// failedTransientFile represents a file that failed to write
type failedTransientFile struct {
	writeError int
}

// fileHandle tracks open file state
type fileHandle struct {
	id   msgs.InodeId
	mu   sync.RWMutex
	body any // *transientFile, failedTransientFile, or *client.FileReader
}

// dirHandle tracks open directory state
type dirHandle struct {
	mu      sync.Mutex
	id      msgs.InodeId
	parent  msgs.InodeId
	entries []dirEntry
	cursor  int
}

type dirEntry struct {
	name    string
	id      msgs.InodeId
	mode    uint32
	size    uint64
	mtime   msgs.TernTime
	atime   msgs.TernTime
	namehash msgs.NameHash
}

// TernFS implements fuse.FileSystemInterface
type TernFS struct {
	fuse.FileSystemBase
}

func (fs *TernFS) Init() {
	logger.Info("TernFS initialized")
}

func (fs *TernFS) Destroy() {
	logger.Info("TernFS destroyed")
}

func (fs *TernFS) Statfs(path string, stat *fuse.Statfs_t) int {
	stat.Bsize = 4096
	stat.Frsize = 4096
	stat.Blocks = 1 << 30
	stat.Bfree = 1 << 30
	stat.Bavail = 1 << 30
	stat.Files = 1 << 20
	stat.Ffree = 1 << 20
	stat.Favail = 1 << 20
	stat.Namemax = 255
	return 0
}

func fillStat(id msgs.InodeId, stat *fuse.Stat_t) int {
	stat.Ino = uint64(id)
	stat.Mode = inodeTypeToMode(id.Type())
	stat.Nlink = 1

	if id.Type() == msgs.DIRECTORY {
		var resp msgs.StatDirectoryResp
		if err := shardRequest(id.Shard(), &msgs.StatDirectoryReq{Id: id}, &resp); err != 0 {
			return err
		}
		mtime := uint64(resp.Mtime)
		stat.Mtim.Sec = int64(mtime / 1000000000)
		stat.Mtim.Nsec = int64(mtime % 1000000000)
		stat.Atim = stat.Mtim
		stat.Ctim = stat.Mtim
	} else {
		var resp msgs.StatFileResp
		if err := c.ShardRequest(logger, id.Shard(), &msgs.StatFileReq{Id: id}, &resp); err != nil {
			return ternErrToFuseErr(err)
		}
		stat.Size = int64(resp.Size)
		mtime := uint64(resp.Mtime)
		stat.Mtim.Sec = int64(mtime / 1000000000)
		stat.Mtim.Nsec = int64(mtime % 1000000000)
		stat.Ctim = stat.Mtim
		atime := uint64(resp.Atime)
		stat.Atim.Sec = int64(atime / 1000000000)
		stat.Atim.Nsec = int64(atime % 1000000000)
	}
	return 0
}

func (fs *TernFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	logger.Debug("getattr path=%q fh=%v", path, fh)

	// If we have a file handle, use it
	if fh != ^uint64(0) {
		handleMu.RLock()
		f, ok := fileHandles[fh]
		handleMu.RUnlock()
		if ok {
			f.mu.RLock()
			defer f.mu.RUnlock()
			if ttf, isTransient := f.body.(*transientFile); isTransient {
				resp := msgs.StatTransientFileResp{}
				if err := c.ShardRequest(logger, f.id.Shard(), &msgs.StatTransientFileReq{Id: f.id}, &resp); err == nil {
					stat.Ino = uint64(f.id)
					stat.Mode = inodeTypeToMode(f.id.Type())
					stat.Size = int64(ttf.written) + int64(len(ttf.span.Bytes()))
					mtime := uint64(resp.Mtime)
					stat.Mtim.Sec = int64(mtime / 1000000000)
					stat.Mtim.Nsec = int64(mtime % 1000000000)
					stat.Atim = stat.Mtim
					stat.Ctim = stat.Mtim
					stat.Nlink = 1
					return 0
				}
			}
			return fillStat(f.id, stat)
		}
	}

	id, _, _, errno := resolvePath(path)
	if errno != 0 {
		return errno
	}
	return fillStat(id, stat)
}

func (fs *TernFS) Opendir(path string) (int, uint64) {
	logger.Debug("opendir path=%q", path)

	id, _, _, errno := resolvePath(path)
	if errno != 0 {
		return errno, ^uint64(0)
	}

	if id.Type() != msgs.DIRECTORY {
		return -fuse.ENOTDIR, ^uint64(0)
	}

	statResp := &msgs.StatDirectoryResp{}
	if err := shardRequest(id.Shard(), &msgs.StatDirectoryReq{Id: id}, statResp); err != 0 {
		return err, ^uint64(0)
	}

	parent := id
	if statResp.Owner != msgs.NULL_INODE_ID {
		parent = statResp.Owner
	}

	dh := &dirHandle{
		id:     id,
		parent: parent,
		cursor: 0,
	}

	handle := allocHandle()
	handleMu.Lock()
	dirHandles[handle] = dh
	handleMu.Unlock()

	return 0, handle
}

func (fs *TernFS) Releasedir(path string, fh uint64) int {
	logger.Debug("releasedir path=%q fh=%v", path, fh)

	handleMu.Lock()
	delete(dirHandles, fh)
	handleMu.Unlock()

	return 0
}

type statDirentryResp struct {
	ix    int
	size  uint64
	mtime msgs.TernTime
	atime msgs.TernTime
	err   any
}

func (fs *TernFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	logger.Debug("readdir path=%q ofst=%v fh=%v", path, ofst, fh)

	handleMu.RLock()
	dh, ok := dirHandles[fh]
	handleMu.RUnlock()
	if !ok {
		return -fuse.EBADF
	}

	dh.mu.Lock()
	defer dh.mu.Unlock()

	// Load entries if needed
	if dh.entries == nil || ofst == 0 {
		dh.entries = []dirEntry{}
		respCh := make(chan statDirentryResp)

		req := &msgs.ReadDirReq{DirId: dh.id, StartHash: 0}
		for len(dh.entries) < readdirBatchSize {
			var resp msgs.ReadDirResp
			if err := shardRequest(dh.id.Shard(), req, &resp); err != 0 {
				return err
			}
			lenBefore := len(dh.entries)
			dh.entries = append(dh.entries, make([]dirEntry, len(resp.Results))...)
			for i := lenBefore; i < len(dh.entries); i++ {
				ix := i
				result := &resp.Results[ix-lenBefore]
				targetId := result.TargetId
				dh.entries[ix] = dirEntry{
					name:     result.Name,
					id:       targetId,
					mode:     inodeTypeToMode(targetId.Type()),
					namehash: result.NameHash,
				}
				go func() {
					attrResp := statDirentryResp{ix: ix}
					defer func() {
						err := recover()
						if err != nil && attrResp.err == nil {
							attrResp.err = err
						}
						respCh <- attrResp
					}()
					if targetId.Type() == msgs.DIRECTORY {
						resp := &msgs.StatDirectoryResp{}
						if err := c.ShardRequest(logger, targetId.Shard(), &msgs.StatDirectoryReq{Id: targetId}, resp); err != nil {
							attrResp.err = err
						} else {
							attrResp.mtime = resp.Mtime
						}
					} else {
						resp := &msgs.StatFileResp{}
						if err := c.ShardRequest(logger, targetId.Shard(), &msgs.StatFileReq{Id: targetId}, resp); err != nil {
							attrResp.err = err
						} else {
							attrResp.size = resp.Size
							attrResp.mtime = resp.Mtime
							attrResp.atime = resp.Atime
						}
					}
				}()
			}
			// Get all stat responses
			for range resp.Results {
				r := <-respCh
				dh.entries[r.ix].size = r.size
				dh.entries[r.ix].mtime = r.mtime
				dh.entries[r.ix].atime = r.atime
			}
			req.StartHash = resp.NextHash
			if req.StartHash == 0 {
				break
			}
		}
	}

	// Add . and ..
	if ofst == 0 {
		stat := &fuse.Stat_t{}
		stat.Mode = fuse.S_IFDIR | 0755
		stat.Ino = uint64(dh.id)
		if !fill(".", stat, 1) {
			return 0
		}
		stat.Ino = uint64(dh.parent)
		if !fill("..", stat, 2) {
			return 0
		}
	}

	// Offset 1 and 2 are . and ..
	entryOffset := int64(0)
	if ofst > 2 {
		entryOffset = ofst - 2
	} else if ofst > 0 {
		entryOffset = 0
	}

	for i := int(entryOffset); i < len(dh.entries); i++ {
		e := &dh.entries[i]
		stat := &fuse.Stat_t{}
		stat.Mode = e.mode
		stat.Ino = uint64(e.id)
		stat.Size = int64(e.size)
		mtime := uint64(e.mtime)
		stat.Mtim.Sec = int64(mtime / 1000000000)
		stat.Mtim.Nsec = int64(mtime % 1000000000)
		atime := uint64(e.atime)
		stat.Atim.Sec = int64(atime / 1000000000)
		stat.Atim.Nsec = int64(atime % 1000000000)

		if !fill(e.name, stat, int64(i)+3) {
			break
		}
	}

	return 0
}

func (fs *TernFS) Mkdir(path string, mode uint32) int {
	logger.Debug("mkdir path=%q mode=0x%08x", path, mode)

	parentId, name, errno := resolveParentPath(path)
	if errno != 0 {
		return errno
	}

	req := msgs.MakeDirectoryReq{
		OwnerId: parentId,
		Name:    name,
	}
	resp := msgs.MakeDirectoryResp{}
	return cdcRequest(&req, &resp)
}

func (fs *TernFS) Rmdir(path string) int {
	logger.Debug("rmdir path=%q", path)

	parentId, name, errno := resolveParentPath(path)
	if errno != 0 {
		return errno
	}

	lookupResp := msgs.LookupResp{}
	if err := shardRequest(parentId.Shard(), &msgs.LookupReq{DirId: parentId, Name: name}, &lookupResp); err != 0 {
		return err
	}

	unlinkReq := msgs.SoftUnlinkDirectoryReq{
		OwnerId:      parentId,
		TargetId:     lookupResp.TargetId,
		Name:         name,
		CreationTime: lookupResp.CreationTime,
	}
	return cdcRequest(&unlinkReq, &msgs.SoftUnlinkDirectoryResp{})
}

func (fs *TernFS) Open(path string, flags int) (int, uint64) {
	logger.Debug("open path=%q flags=%08x", path, flags)

	id, _, _, errno := resolvePath(path)
	if errno != 0 {
		return errno, ^uint64(0)
	}

	if id.Type() == msgs.DIRECTORY {
		return -fuse.EISDIR, ^uint64(0)
	}

	fr, err := c.NewFileReader(logger, id)
	if err != nil {
		return ternErrToFuseErr(err), ^uint64(0)
	}

	f := &fileHandle{
		id:   id,
		body: fr,
	}

	// Update access time asynchronously
	c.ShardRequestDontWait(logger, id.Shard(), &msgs.SetTimeReq{
		Id:    id,
		Atime: uint64(time.Now().UnixNano()) | (uint64(1) << 63),
	})

	handle := allocHandle()
	handleMu.Lock()
	fileHandles[handle] = f
	handleMu.Unlock()

	return 0, handle
}

func (fs *TernFS) Release(path string, fh uint64) int {
	logger.Debug("release path=%q fh=%v", path, fh)

	handleMu.Lock()
	delete(fileHandles, fh)
	handleMu.Unlock()

	return 0
}

func createFile(parentId msgs.InodeId, name string, mode uint32) (*fileHandle, int) {
	req := msgs.ConstructFileReq{Note: name}
	resp := msgs.ConstructFileResp{}
	if (mode & fuse.S_IFMT) == fuse.S_IFREG {
		req.Type = msgs.FILE
	} else if (mode & fuse.S_IFMT) == fuse.S_IFLNK {
		req.Type = msgs.SYMLINK
	} else {
		return nil, -fuse.EINVAL
	}
	if err := shardRequest(parentId.Shard(), &req, &resp); err != 0 {
		return nil, err
	}
	span := bufPool.Get(0)
	transient := &transientFile{
		name:   name,
		dir:    parentId,
		span:   span,
		cookie: resp.Cookie,
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, parentId, &transient.spanPolicies); err != nil {
		return nil, ternErrToFuseErr(err)
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, parentId, &transient.blockPolicies); err != nil {
		return nil, ternErrToFuseErr(err)
	}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, parentId, &transient.stripePolicy); err != nil {
		return nil, ternErrToFuseErr(err)
	}
	f := &fileHandle{
		id:   resp.Id,
		body: transient,
	}
	return f, 0
}

func (fs *TernFS) Create(path string, flags int, mode uint32) (int, uint64) {
	logger.Debug("create path=%q flags=%08x mode=0x%08x", path, flags, mode)

	parentId, name, errno := resolveParentPath(path)
	if errno != 0 {
		return errno, ^uint64(0)
	}

	f, err := createFile(parentId, name, fuse.S_IFREG)
	if err != 0 {
		return err, ^uint64(0)
	}

	handle := allocHandle()
	handleMu.Lock()
	fileHandles[handle] = f
	handleMu.Unlock()

	logger.Debug("created id=%v", f.id)
	return 0, handle
}

// writeSpan writes out current span. Does _not_ replenish f.span.
func (f *fileHandle) writeSpan() int {
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
			f.body = failedTransientFile{ternErrToFuseErr(err)}
		}
	}()
	if toWrite == 0 {
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
		return ternErrToFuseErr(err)
	}
	return 0
}

func (f *fileHandle) write(data []byte) (written int, errno int) {
	tf := f.body.(*transientFile)
	cursor := 0
	maxSpanSize := tf.spanPolicies.Entries[len(tf.spanPolicies.Entries)-1].MaxSize
	for cursor < len(data) {
		remainingInSpan := maxSpanSize - uint32(len(tf.span.Bytes()))
		toWrite := min(int(remainingInSpan), len(data)-cursor)
		*tf.span.BytesPtr() = append(*tf.span.BytesPtr(), data[cursor:cursor+toWrite]...)
		if len(tf.span.Bytes()) >= int(maxSpanSize) {
			if err := f.writeSpan(); err != 0 {
				return cursor, err
			}
			tf.span = bufPool.Get(0)
		}
		cursor += toWrite
	}
	return len(data), 0
}

func (fs *TernFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	logger.Debug("write path=%q off=%v count=%v fh=%v", path, ofst, len(buff), fh)

	if len(buff) == 0 {
		return 0
	}

	handleMu.RLock()
	f, ok := fileHandles[fh]
	handleMu.RUnlock()
	if !ok {
		return -fuse.EBADF
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch tf := f.body.(type) {
	case *transientFile:
		if ofst < tf.written+int64(len(tf.span.Bytes())) {
			logger.Info("refusing to write in the past off=%v written=%v len=%v", ofst, tf.written, int64(len(tf.span.Bytes())))
			return -fuse.EINVAL
		}
		zeros := ofst - (tf.written + int64(len(tf.span.Bytes())))
		if zeros > 0 {
			logger.Debug("file=%v writing %v zeros", f.id, zeros)
			_, errno := f.write(make([]byte, zeros))
			if errno != 0 {
				return errno
			}
		}
		written, errno := f.write(buff)
		if errno != 0 {
			return errno
		}
		return written
	case *client.FileReader:
		return -fuse.ENOTSUP
	case failedTransientFile:
		return tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func (fs *TernFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	logger.Debug("read path=%q off=%v count=%v fh=%v", path, ofst, len(buff), fh)

	handleMu.RLock()
	f, ok := fileHandles[fh]
	handleMu.RUnlock()
	if !ok {
		return -fuse.EBADF
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	switch tf := f.body.(type) {
	case *transientFile:
		return -fuse.ENOTSUP
	case *client.FileReader:
		if ofst < 0 {
			return -fuse.EINVAL
		}
		r := 0
		for r < len(buff) {
			thisR, err := tf.Read(logger, c, nil, bufPool, uint64(ofst)+uint64(r), buff[r:])
			if thisR == 0 || err == io.EOF {
				break
			}
			if err != nil {
				if r > 0 {
					return r
				}
				return ternErrToFuseErr(err)
			}
			r += thisR
		}
		return r
	case failedTransientFile:
		return tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func (fs *TernFS) Flush(path string, fh uint64) int {
	logger.Debug("flush path=%q fh=%v", path, fh)

	handleMu.RLock()
	f, ok := fileHandles[fh]
	handleMu.RUnlock()
	if !ok {
		return -fuse.EBADF
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch tf := f.body.(type) {
	case *transientFile:
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
		resp := &msgs.LinkFileResp{}
		if err := shardRequest(tf.dir.Shard(), &req, resp); err != 0 {
			f.body = failedTransientFile{err}
			return err
		}
		fr, err := c.NewFileReader(logger, f.id)
		if err != nil {
			f.body = failedTransientFile{ternErrToFuseErr(err)}
			return ternErrToFuseErr(err)
		}
		f.body = fr
		return 0
	case *client.FileReader:
		return 0
	case failedTransientFile:
		return tf.writeError
	default:
		panic(fmt.Errorf("bad file type %T", f.body))
	}
}

func canDeleteFileImmediately(dirId msgs.InodeId) (bool, int) {
	policy := &msgs.SnapshotPolicy{}
	if _, err := c.ResolveDirectoryInfoEntry(logger, dirInfoCache, dirId, policy); err != nil {
		return false, ternErrToFuseErr(err)
	}
	delete :=
		!(!policy.DeleteAfterTime.Active() && !policy.DeleteAfterVersions.Active()) &&
			((!policy.DeleteAfterVersions.Active() || policy.DeleteAfterTime.Time() == time.Duration(0)) ||
				(!policy.DeleteAfterTime.Active() || policy.DeleteAfterVersions.Versions() == 0))
	return delete, 0
}

func (fs *TernFS) Unlink(path string) int {
	logger.Debug("unlink path=%q", path)

	parentId, name, errno := resolveParentPath(path)
	if errno != 0 {
		return errno
	}

	lookupResp := msgs.LookupResp{}
	if err := shardRequest(parentId.Shard(), &msgs.LookupReq{DirId: parentId, Name: name}, &lookupResp); err != 0 {
		return err
	}

	unlinkReq := msgs.SoftUnlinkFileReq{
		OwnerId:      parentId,
		FileId:       lookupResp.TargetId,
		Name:         name,
		CreationTime: lookupResp.CreationTime,
	}
	if err := shardRequest(parentId.Shard(), &unlinkReq, &msgs.SoftUnlinkFileResp{}); err != 0 {
		return err
	}

	if canDelete, err := canDeleteFileImmediately(parentId); err != 0 {
		// ignore error
	} else if canDelete {
		if err := cleanup.HardUnlinkFile(logger, c, parentId, lookupResp.TargetId, name, lookupResp.CreationTime); err != nil {
			// ignore error
		}
	}
	return 0
}

func (fs *TernFS) Rename(oldpath string, newpath string) int {
	logger.Debug("rename oldpath=%q newpath=%q", oldpath, newpath)

	oldParent, oldName, errno := resolveParentPath(oldpath)
	if errno != 0 {
		return errno
	}

	newParent, newName, errno := resolveParentPath(newpath)
	if errno != 0 {
		return errno
	}

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
		return shardRequest(oldParent.Shard(), &req, &msgs.SameDirectoryRenameResp{})
	} else if targetId.Type() == msgs.DIRECTORY {
		req := msgs.RenameDirectoryReq{
			TargetId:        targetId,
			OldOwnerId:      oldParent,
			OldName:         oldName,
			OldCreationTime: oldCreationTime,
			NewOwnerId:      newParent,
			NewName:         newName,
		}
		return cdcRequest(&req, &msgs.RenameDirectoryResp{})
	} else {
		req := msgs.RenameFileReq{
			TargetId:        targetId,
			OldOwnerId:      oldParent,
			OldName:         oldName,
			OldCreationTime: oldCreationTime,
			NewOwnerId:      newParent,
			NewName:         newName,
		}
		return cdcRequest(&req, &msgs.RenameFileResp{})
	}
}

func (fs *TernFS) Symlink(target string, newpath string) int {
	logger.Debug("symlink target=%q newpath=%q", target, newpath)

	parentId, name, errno := resolveParentPath(newpath)
	if errno != 0 {
		return errno
	}

	f, err := createFile(parentId, name, fuse.S_IFLNK)
	if err != 0 {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, errno := f.write([]byte(target)); errno != 0 {
		return errno
	}

	tf := f.body.(*transientFile)
	if err := f.writeSpan(); err != 0 {
		return err
	}

	req := msgs.LinkFileReq{
		FileId:  f.id,
		Cookie:  tf.cookie,
		OwnerId: tf.dir,
		Name:    tf.name,
	}
	return shardRequest(tf.dir.Shard(), &req, &msgs.LinkFileResp{})
}

func (fs *TernFS) Readlink(path string) (int, string) {
	logger.Debug("readlink path=%q", path)

	id, _, _, errno := resolvePath(path)
	if errno != 0 {
		return errno, ""
	}

	if id.Type() != msgs.SYMLINK {
		return -fuse.EINVAL, ""
	}

	fileReader, err := c.ReadFile(logger, bufPool, id)
	if err != nil {
		return ternErrToFuseErr(err), ""
	}
	bs, err := io.ReadAll(fileReader)
	if err != nil {
		return ternErrToFuseErr(err), ""
	}
	return 0, string(bs)
}

func (fs *TernFS) Utimens(path string, tmsp []fuse.Timespec) int {
	logger.Debug("utimens path=%q", path)

	id, _, _, errno := resolvePath(path)
	if errno != 0 {
		return errno
	}

	if id.Type() == msgs.DIRECTORY {
		// Directories don't support setting times in the same way
		return 0
	}

	req := &msgs.SetTimeReq{
		Id: id,
	}
	if len(tmsp) >= 1 && tmsp[0].Sec != 0 {
		nanos := tmsp[0].Sec*1000000000 + tmsp[0].Nsec
		req.Atime = uint64(nanos) | (uint64(1) << 63)
	}
	if len(tmsp) >= 2 && tmsp[1].Sec != 0 {
		nanos := tmsp[1].Sec*1000000000 + tmsp[1].Nsec
		req.Mtime = uint64(nanos) | (uint64(1) << 63)
	}
	return shardRequest(id.Shard(), req, &msgs.SetTimeResp{})
}

func (fs *TernFS) Truncate(path string, size int64, fh uint64) int {
	logger.Debug("truncate path=%q size=%v fh=%v", path, size, fh)

	// Only transient files can be truncated (extended)
	if fh != ^uint64(0) {
		handleMu.RLock()
		f, ok := fileHandles[fh]
		handleMu.RUnlock()
		if ok {
			f.mu.Lock()
			defer f.mu.Unlock()

			if tf, isTransient := f.body.(*transientFile); isTransient {
				sz := tf.written + int64(len(tf.span.Bytes()))
				if size < sz {
					return -fuse.ENOTSUP
				}
				maxSpanSize := tf.spanPolicies.Entries[len(tf.spanPolicies.Entries)-1].MaxSize
				remaining := size - sz
				buf := make([]byte, min(int64(maxSpanSize), remaining))
				for remaining > 0 {
					toWrite := min(remaining, int64(maxSpanSize))
					if _, err := f.write(buf[:toWrite]); err != 0 {
						return err
					}
					remaining -= toWrite
				}
				return 0
			}
		}
	}
	return -fuse.ENOTSUP
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <mountpoint>\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	verbose := flag.Bool("verbose", false, "Enables debug logging.")
	logFile := flag.String("log-file", "", "Redirect logging output to given file.")
	registryAddress := flag.String("registry", "", "Registry address (host:port).")
	var addresses flags.StringArrayFlags
	flag.Var(&addresses, "addr", "Local addresses (up to two) to connect from.")
	profileFile := flag.String("profile-file", "", "If set, will write CPU profile here.")
	initialShardTimeout := flag.Duration("initial-shard-timeout", 0, "")
	maxShardTimeout := flag.Duration("max-shard-timeout", 0, "")
	overallShardTimeout := flag.Duration("overall-shard-timeout", 0, "")
	initialCDCTimeout := flag.Duration("initial-cdc-timeout", 0, "")
	maxCDCTimeout := flag.Duration("max-cdc-timeout", 0, "")
	overallCDCTimeout := flag.Duration("overall-cdc-timeout", 0, "")
	initialBlockTimeout := flag.Duration("initial-block-timeout", 0, "")
	maxBlockTimeout := flag.Duration("max-block-timeout", 0, "")
	overallBlockTimeout := flag.Duration("overall-block-timeout", 0, "")
	mtu := flag.Uint64("mtu", msgs.DEFAULT_UDP_MTU, "MTU used talking to shards and CDC")
	readdirBatchSizeFlag := flag.Int("readdir-batch-size", 1000, "How many readdir entries to fetch + stat at once.")
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

	if *readdirBatchSizeFlag <= 0 {
		fmt.Fprintf(os.Stderr, "-readdir-batch-size must be positive\n")
		os.Exit(2)
	}
	readdirBatchSize = *readdirBatchSizeFlag

	logOut := os.Stdout
	if *logFile != "" {
		var err error
		logOut, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open file %v: %v\n", *logFile, err)
			os.Exit(1)
		}
		defer logOut.Close()
	}
	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	logger = log.NewLogger(logOut, &log.LoggerOptions{Level: level})

	if *profileFile != "" {
		f, err := os.Create(*profileFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open profile file %v\n", *profileFile)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
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

	ternfs := &TernFS{}
	host := fuse.NewFileSystemHost(ternfs)

	// Configure options for WinFSP
	host.SetCapReaddirPlus(true)

	logger.Info("Mounting ternfs at %v", mountPoint)

	// Mount and serve
	// The Mount call blocks until unmounted
	if !host.Mount(mountPoint, []string{"-o", "volname=ternfs", "-o", "FileSystemName=ternfs"}) {
		fmt.Fprintf(os.Stderr, "Mount failed\n")
		os.Exit(1)
	}

	// Log counters on exit
	counters.Log(logger)
}
