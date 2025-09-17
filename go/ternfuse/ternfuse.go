// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	golog "log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var c *client.Client
var logger *log.Logger
var dirInfoCache *client.DirInfoCache
var bufPool *bufpool.BufPool

func ternErrToErrno(err error) syscall.Errno {
	switch err {
	case msgs.INTERNAL_ERROR:
		return syscall.EIO
	case msgs.FATAL_ERROR:
		return syscall.EIO
	case msgs.TIMEOUT:
		return syscall.EIO
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

func (n *ternNode) getattr(allowTransient bool, out *fuse.Attr) syscall.Errno {
	logger.Debug("getattr inode=%v", n.id)

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

		// if we tolerate transient files, try that
		if ternErr, ok := err.(msgs.TernError); ok && ternErr == msgs.FILE_NOT_FOUND && allowTransient {
			resp := msgs.StatTransientFileResp{}
			if newErr := c.ShardRequest(logger, n.id.Shard(), &msgs.StatTransientFileReq{Id: n.id}, &resp); newErr != nil {
				logger.Debug("ignoring transient stat error %v", newErr)
				return ternErrToErrno(err) // use original error
			}
			out.Size = resp.Size
			mtime := uint64(resp.Mtime)
			mtimesec := mtime / 1000000000
			mtimens := uint32(mtime % 1000000000)
			out.Mtime = mtimesec
			out.Mtimensec = mtimens
			out.Atime = mtimesec
			out.Atimensec = mtimens
			return 0
		}

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
	}
	return 0
}

func (n *ternNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	return n.getattr(true, &out.Attr)
}

func (n *ternNode) Lookup(
	ctx context.Context, name string, out *fuse.EntryOut,
) (*fs.Inode, syscall.Errno) {
	logger.Debug("lookup dir=%v, name=%v", n.id, name)
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
	if err := newNode.getattr(false, &out.Attr); err != 0 {
		return nil, err
	}
	return n.NewInode(ctx, newNode, fs.StableAttr{Ino: uint64(resp.TargetId), Mode: mode}), 0
}

type dirStream struct {
	dirId  msgs.InodeId
	cursor int
	resp   msgs.ReadDirResp
}

func (ds *dirStream) refresh() syscall.Errno {
	if err := shardRequest(ds.dirId.Shard(), &msgs.ReadDirReq{DirId: ds.dirId, StartHash: ds.resp.NextHash}, &ds.resp); err != 0 {
		return err
	}
	ds.cursor = 0
	return 0
}

func (ds *dirStream) ensureNext() (bool, syscall.Errno) {
	if ds.cursor < len(ds.resp.Results) { // we have the result right here
		return true, 0
	}
	if ds.resp.NextHash == 0 { // there's nothing more
		return false, 0
	}
	// refresh and recurse
	if err := ds.refresh(); err != 0 {
		return false, err
	}
	return ds.ensureNext()
}

func (ds *dirStream) HasNext() bool {
	hasNext, err := ds.ensureNext()
	if err != 0 {
		logger.RaiseAlert("dropping err in HasNext(): %v", err)
	}
	return hasNext
}

func (ds *dirStream) Next() (fuse.DirEntry, syscall.Errno) {
	hasNext, err := ds.ensureNext()
	var de fuse.DirEntry
	if err != 0 {
		return de, err
	}
	if !hasNext {
		panic(fmt.Errorf("expecting next, possible race?"))
	}
	edge := ds.resp.Results[ds.cursor]
	de.Ino = uint64(edge.TargetId)
	de.Mode = inodeTypeToMode(edge.TargetId.Type())
	de.Name = edge.Name
	ds.cursor++
	return de, 0
}

func (ds *dirStream) Close() {}

func (n *ternNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	logger.Debug("readdir dir=%v", n.id)
	ds := dirStream{dirId: n.id}
	if err := ds.refresh(); err != 0 {
		return nil, err
	}
	return &ds, 0

}

type transientFile struct {
	mu       sync.Mutex
	id       msgs.InodeId
	cookie   [8]byte
	dir      msgs.InodeId
	name     string
	data     *bufpool.Buf // null if it has been flushed
	size     uint64
	writeErr syscall.Errno
}

func (n *ternNode) createInternal(name string, flags uint32, mode uint32) (tf *transientFile, errno syscall.Errno) {
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
	data := bufPool.Get(0)
	transient := transientFile{
		name:   name,
		dir:    n.id,
		data:   data,
		id:     resp.Id,
		cookie: resp.Cookie,
	}
	return &transient, 0
}

func (n *ternNode) Create(
	ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut,
) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger.Debug("create id=%v, name=%v, flags=0x%08x, mode=0x%08x", n.id, name, flags, mode)

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
	logger.Debug("mkdir dir=%v, name=%v, mode=0x%08x", n.id, name, mode)

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

func (f *transientFile) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	logger.Debug("write file=%v, off=%v, count=%v", f.id, off, len(data))

	if len(data) == 0 {
		logger.Debug("zero-write, returning early")
		return 0, 0
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writeErr != 0 {
		logger.Debug("file already errored")
		return 0, f.writeErr
	}

	if f.data == nil {
		logger.Debug("file already flushed")
		return 0, syscall.EPERM // flushed already
	}

	if off != int64(len(f.data.Bytes())) {
		logger.Info("refusing to write in the past off=%v len=%v", off, len(f.data.Bytes()))
		return 0, syscall.EINVAL
	}

	*f.data.BytesPtr() = append(*f.data.BytesPtr(), data...)
	f.size += uint64(len(data))

	return uint32(len(data)), 0
}

func (f *transientFile) Flush(ctx context.Context) syscall.Errno {
	logger.Debug("flush file=%v", f.id)

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writeErr != 0 {
		logger.Debug("tf %v has already errored", f.id)
		return f.writeErr
	}

	if f.data == nil {
		logger.Debug("tf %v has already been flushed", f.id)
		return 0
	}

	defer func() {
		bufPool.Put(f.data)
		f.data = nil
	}()

	if err := c.WriteFile(logger, bufPool, dirInfoCache, f.dir, f.id, f.cookie, bytes.NewReader(f.data.Bytes())); err != nil {
		f.writeErr = ternErrToErrno(err)
		return f.writeErr
	}

	req := msgs.LinkFileReq{
		FileId:  f.id,
		Cookie:  f.cookie,
		OwnerId: f.dir,
		Name:    f.name,
	}
	if err := shardRequest(f.dir.Shard(), &req, &msgs.LinkFileResp{}); err != 0 {
		f.writeErr = err
		return err
	}

	return 0
}

func (n *ternNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	logger.Debug("setattr inode=%v, in=%+v", n.id, in)

	if n.id.Type() == msgs.DIRECTORY {
		return syscall.EPERM
	}

	if in.Valid&^(fuse.FATTR_ATIME|fuse.FATTR_MTIME) != 0 {
		return syscall.EPERM
	}

	req := &msgs.SetTimeReq{
		Id: n.id,
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

	if err := shardRequest(n.id.Shard(), req, &msgs.SetTimeResp{}); err != 0 {
		return err
	}

	return 0
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

type openFile struct {
	data *[]byte
	err  error
	wg   sync.WaitGroup
}

func (n *ternNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger.Debug("open file=%v flags=%08x", n.id, flags)

	of := openFile{}

	of.wg.Add(1)
	go func() {
		defer of.wg.Done()
		buf, err := c.FetchFile(logger, bufPool, n.id)
		if err != nil {
			of.err = err
			return
		}
		bufData := buf.Bytes()
		of.data = &bufData
	}()

	if flags|syscall.O_NOATIME != 0 {
		c.ShardRequestDontWait(logger, n.id.Shard(), &msgs.SetTimeReq{
			Id:    n.id,
			Atime: uint64(time.Now().UnixNano()) | (uint64(1) << 63),
		})
	}

	return &of, fuse.FOPEN_CACHE_DIR | fuse.FOPEN_KEEP_CACHE, 0
}

func (of *openFile) Flush(ctx context.Context) syscall.Errno {
	return 0
}

func (of *openFile) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	of.wg.Wait()
	if of.err != nil {
		logger.ErrorNoAlert("read error: %v", of.err)
		return fuse.ReadResultData([]byte{}), syscall.EIO
	}
	logger.Debug("read off=%v, count=%v", off, len(dest))
	data := *of.data

	if off > int64(len(data)) {
		return fuse.ReadResultData([]byte{}), 0
	}

	r := copy(dest, data[off:])
	return fuse.ReadResultData(dest[:r]), 0
}

func (n *ternNode) Unlink(ctx context.Context, name string) syscall.Errno {
	logger.Debug("unlink dir=%v, name=%v", n.id, name)

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
	logger.Debug("rmdir dir=%v, name=%v", n.id, name)
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
	logger.Debug("symlink dir=%v, target=%v, name=%v", n.id, target, name)
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
	return n.NewInode(ctx, &ternNode{id: tf.id}, fs.StableAttr{Ino: uint64(tf.id), Mode: syscall.S_IFLNK}), 0
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

var _ = (fs.InodeEmbedder)((*ternNode)(nil))
var _ = (fs.NodeLookuper)((*ternNode)(nil))
var _ = (fs.NodeReaddirer)((*ternNode)(nil))
var _ = (fs.NodeMkdirer)((*ternNode)(nil))
var _ = (fs.NodeGetattrer)((*ternNode)(nil))
var _ = (fs.NodeCreater)((*ternNode)(nil))
var _ = (fs.NodeSetattrer)((*ternNode)(nil))
var _ = (fs.NodeRenamer)((*ternNode)(nil))
var _ = (fs.NodeOpener)((*ternNode)(nil))
var _ = (fs.NodeUnlinker)((*ternNode)(nil))
var _ = (fs.NodeRmdirer)((*ternNode)(nil))
var _ = (fs.NodeSymlinker)((*ternNode)(nil))
var _ = (fs.NodeReadlinker)((*ternNode)(nil))

var _ = (fs.FileWriter)((*transientFile)(nil))
var _ = (fs.FileFlusher)((*transientFile)(nil))

var _ = (fs.FileReader)((*openFile)(nil))
var _ = (fs.FileFlusher)((*openFile)(nil))

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
	initialCDCTimeout := flag.Duration("initial-cdc-timeout", 0, "")
	mtu := flag.Uint64("mtu", msgs.DEFAULT_UDP_MTU, "mtu used talking to shards and cdc")
	fileAttrCacheTimeFlag := flag.Duration("file-attr-cache-time", time.Millisecond*250, "time to cache file attributes for read-only files before rechecking with the server. Set to 0 to disable caching. (default: 250ms)")
	dirAttrCacheTimeFlag := flag.Duration("dir-attr-cache-time", time.Millisecond*250, "time to cache directory attributes before rechecking with the server. Set to 0 to disable caching. (default: 250ms)")
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
	c.SetShardTimeouts(&shardTimeouts)
	cdcTimeouts := client.DefaultCDCTimeout
	if *initialCDCTimeout != 0 {
		shardTimeouts.Initial = *initialCDCTimeout
	}
	c.SetCDCTimeouts(&cdcTimeouts)

	dirInfoCache = client.NewDirInfoCache()

	bufPool = bufpool.NewBufPool()

	root := ternNode{
		id: msgs.ROOT_DIR_INODE_ID,
	}
	fuseOptions := &fs.Options{
		Logger:       golog.New(os.Stderr, "fuse", golog.Ldate|golog.Ltime|golog.Lmicroseconds|golog.Lshortfile),
		AttrTimeout:  fileAttrCacheTimeFlag,
		EntryTimeout: dirAttrCacheTimeFlag,
		MountOptions: fuse.MountOptions{
			FsName:        "ternfs",
			Name:          "ternfuse" + mountPoint,
			MaxWrite:      1 << 20,
			MaxReadAhead:  1 << 20,
			DisableXAttrs: true,
		},
	}
	if *allowOther {
		fuseOptions.MountOptions.Options = append(fuseOptions.MountOptions.Options, "allow_other")
	}
	// fuseOptions.Debug = *trace
	server, err := fs.Mount(mountPoint, &root, fuseOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not mount: %v", err)
		os.Exit(1)
	}

	logger.Info("mounted at %v", mountPoint)

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
		syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
	}()

	server.Wait()
}
