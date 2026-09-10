// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"path"
	"regexp"
	"time"
)

func NewFind() SpecWithClient {
	findCmd := flag.NewFlagSet("find", flag.ExitOnError)
	findDir := findCmd.String("path", "/", "")
	findName := findCmd.String("name", "", "Regex to match the name against.")
	findSnapshot := findCmd.Bool("snapshot", false, "If set, will search through snapshot directory entries too.")
	findOnlySnapshot := findCmd.Bool("only-snapshot", false, "If set, will return _only_ snapshot edges.")
	findOnlyOwned := findCmd.Bool("only-owned", false, "If true and -snapshot is set, only owned files will be searched.")
	findBeforeSpec := findCmd.String("before", "", "If set, only directory entries created before this duration/date will be searched.")
	findBlockId := findCmd.Uint64("block-id", 0, "If specified, only files which contain the given block will be returned.")
	findMinSize := findCmd.Uint64("min-size", 0, "If specified, only files of at least this size will be returned.")
	findCheckBlocks := findCmd.Bool("check-blocks", false, "If true check all blocks in file")
	findWorkersPerShard := findCmd.Int("workers-per-shard", 5, "")
	findRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		re := regexp.MustCompile(`.*`)
		if *findName != "" {
			re = regexp.MustCompile(*findName)
		}
		findBefore := msgs.TernTime(^uint64(0))
		if *findBeforeSpec != "" {
			d, durErr := time.ParseDuration(*findBeforeSpec)
			if durErr != nil {
				t, tErr := time.Parse(time.RFC3339Nano, *findBeforeSpec)
				if tErr != nil {
					panic(fmt.Errorf("could not parse %q as duration or time: %v, %v", *findBeforeSpec, durErr, tErr))
				}
				findBefore = msgs.MakeTernTime(t)
			} else {
				findBefore = msgs.MakeTernTime(time.Now().Add(-d))
			}
		}
		c := runtime.Client
		err := client.Parwalk(
			l,
			c,
			&client.ParwalkOptions{
				WorkersPerShard: *findWorkersPerShard,
				Snapshot:        *findSnapshot,
			},
			*findDir,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if !owned && *findOnlyOwned {
					return nil
				}
				if current && *findOnlySnapshot {
					return nil
				}
				if creationTime > findBefore {
					return nil
				}
				if !re.MatchString(name) {
					return nil
				}
				if *findMinSize > 0 {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					statReq := msgs.StatFileReq{Id: id}
					statResp := msgs.StatFileResp{}
					if err := c.ShardRequest(l, id.Shard(), &statReq, &statResp); err != nil {
						if err == msgs.FILE_NOT_FOUND {

							l.Info("file %q disappeared", path.Join(parentPath, name))
						} else {
							return err
						}
					}
					if statResp.Size < *findMinSize {
						return nil
					}
				}
				if *findBlockId != 0 {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					req := msgs.LocalFileSpansReq{
						FileId: id,
					}
					resp := msgs.LocalFileSpansResp{}
					found := false
					for {
						if err := c.ShardRequest(l, id.Shard(), &req, &resp); err != nil {
							return err
						}
						for _, span := range resp.Spans {
							if span.Header.StorageClass == msgs.INLINE_STORAGE {
								continue
							}
							body := span.Body.(*msgs.FetchedBlocksSpan)
							for _, block := range body.Blocks {
								if block.BlockId == msgs.BlockId(*findBlockId) {
									found = true
									break
								}
							}
							if found {
								break
							}
						}
						req.ByteOffset = resp.NextOffset
						if req.ByteOffset == 0 || found {
							break
						}
					}
					if !found {
						return nil
					}
				}
				if *findCheckBlocks {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					req := msgs.LocalFileSpansReq{
						FileId: id,
					}
					resp := msgs.LocalFileSpansResp{}
					for {
						if err := c.ShardRequest(l, id.Shard(), &req, &resp); err != nil {
							return err
						}
						for _, span := range resp.Spans {
							if span.Header.StorageClass == msgs.INLINE_STORAGE {
								continue
							}
							body := span.Body.(*msgs.FetchedBlocksSpan)
							for _, block := range body.Blocks {
								blockService := &resp.BlockServices[block.BlockServiceIx]
								if err := c.CheckBlock(l, blockService, block.BlockId, body.CellSize*uint32(body.Stripes), block.Crc); err != nil {
									l.ErrorNoAlert("while checking block %v in file %v got error %v", block.BlockId, path.Join(parentPath, name), err)
								}
							}
						}
						req.ByteOffset = resp.NextOffset
						if req.ByteOffset == 0 {
							break
						}
					}
				}
				l.Info("%v %q", id, path.Join(parentPath, name))
				return nil
			},
		)
		if err != nil {
			panic(err)
		}
	}
	return SpecWithClient{
		Flags: findCmd,
		Run:   findRun,
	}
}
