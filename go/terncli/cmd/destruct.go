// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func NewDestruct() Command {
	destructCmd := flag.NewFlagSet("destruct", flag.ExitOnError)
	destructFileIdU64 := destructCmd.Uint64("file", 0, "Transient file id to destruct. If not present, they'll all be destructed.")
	destrutcFileShardId := destructCmd.Int("shard", -1, "Shard to destruct into. Will destruct all of them if -1.")
	destructFileCookieU64 := destructCmd.Uint64("cookie", 0, "Transient file cookie. Must be present if file is specified.")
	destructRun := func(runtime *Runtime) {
		l := runtime.Log
		if *destructFileIdU64 == 0 {
			state := &cleanup.DestructFilesState{}
			opts := &cleanup.DestructFilesOptions{NumWorkersPerShard: 10, WorkersQueueSize: 100}
			if *destrutcFileShardId < 0 {
				if err := cleanup.DestructFilesInAllShards(l, runtime.getClient(), opts, state); err != nil {
					panic(err)
				}
			} else {
				if err := cleanup.DestructFiles(l, runtime.getClient(), opts, state, msgs.ShardId(*destrutcFileShardId)); err != nil {
					panic(err)
				}
			}
		} else {
			fileId := msgs.InodeId(*destructFileIdU64)
			if fileId.Type() == msgs.DIRECTORY {
				panic(fmt.Errorf("inode id %v is not a file/symlink", fileId))
			}
			stats := cleanup.DestructFilesStats{}
			var destructFileCookie [8]byte
			binary.LittleEndian.PutUint64(destructFileCookie[:], *destructFileCookieU64)
			if err := cleanup.DestructFile(l, runtime.getClient(), &stats, fileId, 0, destructFileCookie); err != nil {
				panic(fmt.Errorf("could not destruct %v, stats: %+v, err: %v", fileId, stats, err))
			}
			l.Info("finished destructing %v, stats: %+v", fileId, stats)
		}
	}
	return Command{
		Flags: destructCmd,
		Run:   destructRun,
	}
}
