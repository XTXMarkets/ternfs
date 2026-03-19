// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

// RemoteTernVFS implements TernVFS backed by a real TernFS cluster.
type RemoteTernVFS struct {
	client       *client.Client
	log          *log.Logger
	bufPool      *bufpool.BufPool
	dirInfoCache *client.DirInfoCache

	mu      sync.Mutex
	parents map[InodeID]InodeID            // child → parent, populated by Lookup
	readers map[msgs.InodeId]*cachedReader // file reader cache
}

type cachedReader struct {
	reader   *client.FileReader
	fileSize uint64
}

func NewRemoteTernVFS(c *client.Client, logger *log.Logger, bufPool *bufpool.BufPool) *RemoteTernVFS {
	return &RemoteTernVFS{
		client:       c,
		log:          logger,
		bufPool:      bufPool,
		dirInfoCache: client.NewDirInfoCache(),
		parents:      make(map[InodeID]InodeID),
		readers:      make(map[msgs.InodeId]*cachedReader),
	}
}

func (t *RemoteTernVFS) RootID() InodeID {
	return InodeID(msgs.ROOT_DIR_INODE_ID)
}

func (t *RemoteTernVFS) Stat(id InodeID) (NodeInfo, error) {
	mid := msgs.InodeId(id)
	switch mid.Type() {
	case msgs.DIRECTORY:
		var resp msgs.StatDirectoryResp
		if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.StatDirectoryReq{Id: mid}, &resp); err != nil {
			return NodeInfo{}, ternToOSError(err)
		}
		mt := resp.Mtime.Time()
		return NodeInfo{Size: 0, Mtime: mt, Atime: mt}, nil
	default:
		var resp msgs.StatFileResp
		if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.StatFileReq{Id: mid}, &resp); err != nil {
			return NodeInfo{}, ternToOSError(err)
		}
		return NodeInfo{
			Size:  resp.Size,
			Mtime: resp.Mtime.Time(),
			Atime: resp.Atime.Time(),
		}, nil
	}
}

func (t *RemoteTernVFS) Lookup(dirID InodeID, name string) (InodeID, error) {
	mid := msgs.InodeId(dirID)
	var resp msgs.LookupResp
	if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.LookupReq{DirId: mid, Name: name}, &resp); err != nil {
		return 0, ternToOSError(err)
	}
	childID := InodeID(resp.TargetId)
	t.mu.Lock()
	t.parents[childID] = dirID
	t.mu.Unlock()
	return childID, nil
}

func (t *RemoteTernVFS) LookupParent(id InodeID) (InodeID, error) {
	// Check the parent cache first.
	t.mu.Lock()
	pid, ok := t.parents[id]
	t.mu.Unlock()
	if ok {
		return pid, nil
	}
	// For directories, StatDirectory returns the Owner (= parent directory).
	mid := msgs.InodeId(id)
	if mid.Type() == msgs.DIRECTORY {
		var resp msgs.StatDirectoryResp
		if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.StatDirectoryReq{Id: mid}, &resp); err != nil {
			return 0, ternToOSError(err)
		}
		if resp.Owner == msgs.NULL_INODE_ID {
			// Root directory or snapshot directory — parent is itself.
			return id, nil
		}
		return InodeID(resp.Owner), nil
	}
	// For files/symlinks without a cached parent, we have no way to find it.
	return 0, os.ErrNotExist
}

func (t *RemoteTernVFS) Readdir(dirID InodeID, startHash uint64) ([]DirEntry, uint64, error) {
	mid := msgs.InodeId(dirID)
	var resp msgs.ReadDirResp
	if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.ReadDirReq{
		DirId:     mid,
		StartHash: msgs.NameHash(startHash),
	}, &resp); err != nil {
		return nil, 0, ternToOSError(err)
	}
	entries := make([]DirEntry, len(resp.Results))
	t.mu.Lock()
	for i, e := range resp.Results {
		entries[i] = DirEntry{
			Name:     e.Name,
			ID:       InodeID(e.TargetId),
			NameHash: uint64(e.NameHash),
		}
		t.parents[InodeID(e.TargetId)] = dirID
	}
	t.mu.Unlock()
	return entries, uint64(resp.NextHash), nil
}

func (t *RemoteTernVFS) Read(fileID InodeID, offset uint64, dest []byte) (int, bool, error) {
	mid := msgs.InodeId(fileID)
	cr, err := t.getOrCreateReader(mid)
	if err != nil {
		return 0, false, ternToOSError(err)
	}
	n, err := cr.reader.Read(t.log, t.client, nil, t.bufPool, offset, dest)
	if errors.Is(err, io.EOF) {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	eof := offset+uint64(n) >= cr.fileSize
	return n, eof, nil
}

func (t *RemoteTernVFS) getOrCreateReader(mid msgs.InodeId) (*cachedReader, error) {
	t.mu.Lock()
	cr, ok := t.readers[mid]
	t.mu.Unlock()
	if ok {
		return cr, nil
	}
	fr, err := t.client.NewFileReader(t.log, mid)
	if err != nil {
		return nil, err
	}
	// Get file size from Stat.
	var resp msgs.StatFileResp
	if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.StatFileReq{Id: mid}, &resp); err != nil {
		return nil, err
	}
	cr = &cachedReader{reader: fr, fileSize: resp.Size}
	t.mu.Lock()
	t.readers[mid] = cr
	t.mu.Unlock()
	return cr, nil
}

func (t *RemoteTernVFS) Readlink(fileID InodeID) (string, error) {
	mid := msgs.InodeId(fileID)
	buf, err := t.client.FetchFile(t.log, t.bufPool, mid)
	if err != nil {
		return "", ternToOSError(err)
	}
	target := string(buf.Bytes())
	t.bufPool.Put(buf)
	return target, nil
}

func (t *RemoteTernVFS) Mkdir(dirID InodeID, name string) (InodeID, error) {
	mid := msgs.InodeId(dirID)
	var resp msgs.MakeDirectoryResp
	if err := t.client.CDCRequest(t.log, &msgs.MakeDirectoryReq{
		OwnerId: mid,
		Name:    name,
	}, &resp); err != nil {
		return 0, ternToOSError(err)
	}
	childID := InodeID(resp.Id)
	t.mu.Lock()
	t.parents[childID] = dirID
	t.mu.Unlock()
	return childID, nil
}

func (t *RemoteTernVFS) Symlink(dirID InodeID, name string, target string) (InodeID, error) {
	mid := msgs.InodeId(dirID)
	// Create transient symlink inode.
	var constructResp msgs.ConstructFileResp
	if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.ConstructFileReq{
		Type: msgs.SYMLINK,
		Note: name,
	}, &constructResp); err != nil {
		return 0, ternToOSError(err)
	}
	fileId := constructResp.Id
	cookie := constructResp.Cookie
	// Write symlink target as inline span.
	body := []byte(target)
	crc := msgs.Crc(crc32c.Sum(0, body))
	if err := t.client.ShardRequest(t.log, fileId.Shard(), &msgs.AddInlineSpanReq{
		FileId:       fileId,
		Cookie:       cookie,
		StorageClass: msgs.INLINE_STORAGE,
		ByteOffset:   0,
		Size:         uint32(len(body)),
		Crc:          crc,
		Body:         body,
	}, &msgs.AddInlineSpanResp{}); err != nil {
		return 0, ternToOSError(err)
	}
	// Link into directory.
	if err := t.client.ShardRequest(t.log, mid.Shard(), &msgs.LinkFileReq{
		FileId:  fileId,
		Cookie:  cookie,
		OwnerId: mid,
		Name:    name,
	}, &msgs.LinkFileResp{}); err != nil {
		return 0, ternToOSError(err)
	}
	childID := InodeID(fileId)
	t.mu.Lock()
	t.parents[childID] = dirID
	t.mu.Unlock()
	return childID, nil
}

func (t *RemoteTernVFS) ConstructFile(dirID InodeID) (InodeID, Cookie, error) {
	dirMid := msgs.InodeId(dirID)
	var resp msgs.ConstructFileResp
	if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.ConstructFileReq{
		Type: msgs.FILE,
	}, &resp); err != nil {
		return 0, Cookie{}, ternToOSError(err)
	}
	return InodeID(resp.Id), Cookie(resp.Cookie), nil
}

func (t *RemoteTernVFS) LinkFile(fileID InodeID, cookie Cookie, dirID InodeID, name string, data io.Reader) error {
	mid := msgs.InodeId(fileID)
	dirMid := msgs.InodeId(dirID)
	// Write file data as spans.
	if data != nil {
		if err := t.client.WriteFile(t.log, t.bufPool, t.dirInfoCache, dirMid, mid, msgs.Cookie(cookie), data); err != nil {
			return ternToOSError(err)
		}
	}
	// Link the file into the directory.
	if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.LinkFileReq{
		FileId:  mid,
		Cookie:  msgs.Cookie(cookie),
		OwnerId: dirMid,
		Name:    name,
	}, &msgs.LinkFileResp{}); err != nil {
		return ternToOSError(err)
	}
	childID := InodeID(mid)
	t.mu.Lock()
	t.parents[childID] = dirID
	t.mu.Unlock()
	return nil
}

func (t *RemoteTernVFS) CreateFile(dirID InodeID, name string, data io.Reader) (InodeID, error) {
	dirMid := msgs.InodeId(dirID)
	// ConstructFile on the same shard as the directory.
	var constructResp msgs.ConstructFileResp
	if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.ConstructFileReq{
		Type: msgs.FILE,
		Note: name,
	}, &constructResp); err != nil {
		return 0, ternToOSError(err)
	}
	fileId := constructResp.Id
	cookie := constructResp.Cookie
	// Write data.
	if data != nil {
		if err := t.client.WriteFile(t.log, t.bufPool, t.dirInfoCache, dirMid, fileId, cookie, data); err != nil {
			return 0, ternToOSError(err)
		}
	}
	// Link.
	if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.LinkFileReq{
		FileId:  fileId,
		Cookie:  cookie,
		OwnerId: dirMid,
		Name:    name,
	}, &msgs.LinkFileResp{}); err != nil {
		return 0, ternToOSError(err)
	}
	childID := InodeID(fileId)
	t.mu.Lock()
	t.parents[childID] = dirID
	t.mu.Unlock()
	return childID, nil
}

func (t *RemoteTernVFS) Remove(dirID InodeID, name string) error {
	dirMid := msgs.InodeId(dirID)
	// Lookup to get target ID and creation time.
	var lookupResp msgs.LookupResp
	if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.LookupReq{
		DirId: dirMid,
		Name:  name,
	}, &lookupResp); err != nil {
		return ternToOSError(err)
	}
	targetId := lookupResp.TargetId
	creationTime := lookupResp.CreationTime
	switch targetId.Type() {
	case msgs.DIRECTORY:
		// Directory removal goes through CDC.
		if err := t.client.CDCRequest(t.log, &msgs.SoftUnlinkDirectoryReq{
			OwnerId:      dirMid,
			TargetId:     targetId,
			CreationTime: creationTime,
			Name:         name,
		}, &msgs.SoftUnlinkDirectoryResp{}); err != nil {
			return ternToOSError(err)
		}
	default:
		// File/symlink removal is a shard-local operation.
		if err := t.client.ShardRequest(t.log, dirMid.Shard(), &msgs.SoftUnlinkFileReq{
			OwnerId:      dirMid,
			FileId:       targetId,
			Name:         name,
			CreationTime: creationTime,
		}, &msgs.SoftUnlinkFileResp{}); err != nil {
			return ternToOSError(err)
		}
	}
	// Evict cached reader for removed files.
	t.mu.Lock()
	delete(t.readers, targetId)
	delete(t.parents, InodeID(targetId))
	t.mu.Unlock()
	return nil
}

func (t *RemoteTernVFS) Rename(srcDirID InodeID, srcName string, dstDirID InodeID, dstName string) error {
	srcMid := msgs.InodeId(srcDirID)
	dstMid := msgs.InodeId(dstDirID)
	// Lookup source to get target ID and creation time.
	var lookupResp msgs.LookupResp
	if err := t.client.ShardRequest(t.log, srcMid.Shard(), &msgs.LookupReq{
		DirId: srcMid,
		Name:  srcName,
	}, &lookupResp); err != nil {
		return ternToOSError(err)
	}
	targetId := lookupResp.TargetId
	creationTime := lookupResp.CreationTime
	if srcDirID == dstDirID && targetId.Type() != msgs.DIRECTORY {
		// Same-directory file/symlink rename — shard-local.
		if err := t.client.ShardRequest(t.log, srcMid.Shard(), &msgs.SameDirectoryRenameReq{
			TargetId:        targetId,
			DirId:           srcMid,
			OldName:         srcName,
			OldCreationTime: creationTime,
			NewName:         dstName,
		}, &msgs.SameDirectoryRenameResp{}); err != nil {
			return ternToOSError(err)
		}
	} else if targetId.Type() == msgs.DIRECTORY {
		// Directory rename — always through CDC.
		if err := t.client.CDCRequest(t.log, &msgs.RenameDirectoryReq{
			TargetId:        targetId,
			OldOwnerId:      srcMid,
			OldName:         srcName,
			OldCreationTime: creationTime,
			NewOwnerId:      dstMid,
			NewName:         dstName,
		}, &msgs.RenameDirectoryResp{}); err != nil {
			return ternToOSError(err)
		}
	} else {
		// Cross-directory file rename — through CDC.
		if err := t.client.CDCRequest(t.log, &msgs.RenameFileReq{
			TargetId:        targetId,
			OldOwnerId:      srcMid,
			OldName:         srcName,
			OldCreationTime: creationTime,
			NewOwnerId:      dstMid,
			NewName:         dstName,
		}, &msgs.RenameFileResp{}); err != nil {
			return ternToOSError(err)
		}
	}
	// Update parent cache.
	t.mu.Lock()
	t.parents[InodeID(targetId)] = dstDirID
	t.mu.Unlock()
	return nil
}

func (t *RemoteTernVFS) SetTime(id InodeID, mtime *time.Time, atime *time.Time) error {
	mid := msgs.InodeId(id)
	var req msgs.SetTimeReq
	req.Id = mid
	// MSB of Mtime/Atime indicates "set this field".
	if mtime != nil {
		req.Mtime = (1 << 63) | uint64(mtime.UnixNano())
	}
	if atime != nil {
		req.Atime = (1 << 63) | uint64(atime.UnixNano())
	}
	if err := t.client.ShardRequest(t.log, mid.Shard(), &req, &msgs.SetTimeResp{}); err != nil {
		return ternToOSError(err)
	}
	return nil
}

// ternToOSError maps TernFS errors to os-level errors so that the NFS
// server's errToNFS can translate them to NFS status codes.
func ternToOSError(err error) error {
	var te msgs.TernError
	if !errors.As(err, &te) {
		return err
	}
	switch te {
	case msgs.EDGE_NOT_FOUND, msgs.FILE_NOT_FOUND, msgs.DIRECTORY_NOT_FOUND,
		msgs.NAME_NOT_FOUND, msgs.OLD_DIRECTORY_NOT_FOUND, msgs.NEW_DIRECTORY_NOT_FOUND:
		return os.ErrNotExist
	case msgs.NOT_AUTHORISED:
		return os.ErrPermission
	case msgs.CANNOT_OVERRIDE_NAME:
		return os.ErrExist
	case msgs.DIRECTORY_NOT_EMPTY:
		return nfsError(NFS4ERR_NOTEMPTY)
	case msgs.EDGE_IS_LOCKED, msgs.NAME_IS_LOCKED:
		return nfsError(NFS4ERR_LOCKED)
	default:
		return err
	}
}
