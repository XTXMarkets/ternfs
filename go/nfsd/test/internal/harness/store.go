// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux

package harness

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/crc32c"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

type store struct {
	client *client.Client
	logger *log.Logger
	log    *os.File
	bufs   *bufpool.BufPool
	dirs   *client.DirInfoCache
}

func newStore(registry, logPath string) (*store, error) {
	out, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	logger := log.NewLogger(out, &log.LoggerOptions{Level: log.INFO})
	c, err := client.NewClient(logger, nil, registry, msgs.AddrsInfo{})
	if err != nil {
		out.Close()
		return nil, err
	}
	return &store{client: c, logger: logger, log: out,
		bufs: bufpool.NewBufPool(), dirs: client.NewDirInfoCache()}, nil
}

func (s *store) close() {
	s.client.Close()
	s.log.Close()
}

func (s *store) root() msgs.InodeId { return msgs.ROOT_DIR_INODE_ID }

func (s *store) mkdir(parent msgs.InodeId, name string) (msgs.InodeId, error) {
	var resp msgs.MakeDirectoryResp
	err := s.client.CDCRequest(s.logger,
		&msgs.MakeDirectoryReq{OwnerId: parent, Name: name}, &resp)
	return resp.Id, err
}

func (s *store) symlink(parent msgs.InodeId, name, target string) error {
	var file msgs.ConstructFileResp
	if err := s.client.ShardRequest(s.logger, parent.Shard(),
		&msgs.ConstructFileReq{Type: msgs.SYMLINK, Note: name}, &file); err != nil {
		return err
	}
	body := []byte(target)
	if err := s.client.ShardRequest(s.logger, file.Id.Shard(), &msgs.AddInlineSpanReq{
		FileId: file.Id, Cookie: file.Cookie, StorageClass: msgs.INLINE_STORAGE,
		Size: uint32(len(body)), Crc: msgs.Crc(crc32c.Sum(0, body)), Body: body,
	}, &msgs.AddInlineSpanResp{}); err != nil {
		return err
	}
	return s.client.ShardRequest(s.logger, parent.Shard(), &msgs.LinkFileReq{
		FileId: file.Id, Cookie: file.Cookie, OwnerId: parent, Name: name,
	}, &msgs.LinkFileResp{})
}

// Only traverse the test's allocated directory; never follow symlinks.
func (s *store) remove(parent msgs.InodeId, name string) error {
	var lookup msgs.LookupResp
	if err := s.client.ShardRequest(s.logger, parent.Shard(),
		&msgs.LookupReq{DirId: parent, Name: name}, &lookup); err != nil {
		return err
	}
	id := lookup.TargetId
	if id.Type() == msgs.DIRECTORY {
		var edges []msgs.Edge
		var cursor client.DirEdgesCursor
		for {
			page, err := s.client.ReadCurrentDirEdgesPage(s.logger, id, cursor)
			if err != nil {
				return err
			}
			edges = append(edges, page.Results...)
			if page.Next == nil {
				break
			}
			cursor = *page.Next
		}
		for _, edge := range edges {
			if err := s.remove(id, edge.Name); err != nil {
				return err
			}
		}
		return s.client.CDCRequest(s.logger, &msgs.SoftUnlinkDirectoryReq{
			OwnerId: parent, TargetId: id, CreationTime: lookup.CreationTime, Name: name,
		}, &msgs.SoftUnlinkDirectoryResp{})
	}
	return s.client.ShardRequest(s.logger, parent.Shard(), &msgs.SoftUnlinkFileReq{
		OwnerId: parent, FileId: id, CreationTime: lookup.CreationTime, Name: name,
	}, &msgs.SoftUnlinkFileResp{})
}

func (s *Server) Seed(root string) error {
	id, err := s.store.client.ResolvePath(s.store.logger, "/"+s.Name)
	if err != nil {
		return err
	}
	dirs := map[string]msgs.InodeId{".": id}
	return filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil || rel == "." {
			return err
		}
		parent := dirs[filepath.Dir(rel)]
		switch {
		case entry.IsDir():
			id, err := s.store.mkdir(parent, entry.Name())
			dirs[rel] = id
			return err
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(filePath)
			if err != nil {
				return err
			}
			return s.store.symlink(parent, entry.Name(), target)
		case entry.Type().IsRegular():
			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = s.store.client.CreateFile(s.store.logger, s.store.bufs, s.store.dirs,
				path.Join("/", s.Name, filepath.ToSlash(rel)), f)
			return err
		default:
			return fmt.Errorf("unsupported fixture file %s", filePath)
		}
	})
}

func (s *Server) ReadFile(name string) ([]byte, error) {
	id, err := s.store.client.ResolvePath(s.store.logger, path.Join("/", s.Name, name))
	if err != nil {
		return nil, err
	}
	reader, err := s.store.client.ReadFile(s.store.logger, s.store.bufs, id)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
