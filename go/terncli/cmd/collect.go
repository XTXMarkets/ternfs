// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"sync/atomic"
	"time"
)

func NewCollect() Command {
	collectCmd := flag.NewFlagSet("collect", flag.ExitOnError)
	collectDirPath := collectCmd.String("path", "", "Directory path to collect. If -follow-subdirs is set, all subdirectories will also be collected.")
	collectDirId := collectCmd.Uint64("dir-id", 0, "Directory inode id to collect directly.")
	collectDirMinEdgeAge := collectCmd.Duration("min-edge-age", time.Hour, "Minimum age of edges to be collected")
	collectDirForcePolicyStr := collectCmd.String("force-policy", "", "If set, will ignore existing directory info and use this SNAPSHOT policy for all directories")
	collectDirFollowSnapshot := collectCmd.Bool("follow-snapshot", false, "Whether to follow snapshot edges. Use with care, it can follow moved directories")
	collectDirFollowSubdirs := collectCmd.Bool("follow-subdirs", false, "Whether to also collect subdirectories.")
	collectRun := func(runtime *Runtime) {
		l := runtime.Log
		if (*collectDirPath == "" && *collectDirId == 0) || (*collectDirPath != "" && *collectDirId != 0) {
			panic("You need to specify -path xor -dir-id.\n")
		}
		if *collectDirFollowSubdirs && *collectDirId != 0 {
			panic("Cannot use -follow-subdirs with -dir-id.\n")
		}
		var forcePolicy *msgs.SnapshotPolicy
		if collectDirForcePolicyStr != nil && *collectDirForcePolicyStr != "" {
			forcePolicy = &msgs.SnapshotPolicy{}
			if err := forcePolicy.UnmarshalJSON([]byte(*collectDirForcePolicyStr)); err != nil {
				panic(fmt.Errorf("could not parse force policy: %v", err))
			}
		}
		dirInfoCache := client.NewDirInfoCache()

		var err error
		if *collectDirFollowSubdirs {
			var stats cleanup.CollectDirectoriesStats
			stopStatsPrinter := make(chan struct{})
			go func() {
				ticker := time.NewTicker(60 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						l.Info("collect stats so far: visited=%v edges=%v collected=%v destructed=%v",
							atomic.LoadUint64(&stats.VisitedDirectories),
							atomic.LoadUint64(&stats.VisitedEdges),
							atomic.LoadUint64(&stats.CollectedEdges),
							atomic.LoadUint64(&stats.DestructedDirectories),
						)
					case <-stopStatsPrinter:
						return
					}
				}
			}()
			err = client.Parwalk(
				l,
				runtime.getClient(),
				&client.ParwalkOptions{
					WorkersPerShard: 100,
					Snapshot:        *collectDirFollowSnapshot,
				},
				*collectDirPath,
				func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
					if id.Type() != msgs.DIRECTORY {
						return nil
					}
					var localStats cleanup.CollectDirectoriesStats
					if err := cleanup.CollectDirectory(l, runtime.getClient(), dirInfoCache, &localStats, id, *collectDirMinEdgeAge, forcePolicy); err != nil {
						print(fmt.Errorf("could not collect %v, err: %v", id, err))
					} else {
						atomic.AddUint64(&stats.VisitedDirectories, localStats.VisitedDirectories)
						atomic.AddUint64(&stats.VisitedEdges, localStats.VisitedEdges)
						atomic.AddUint64(&stats.CollectedEdges, localStats.CollectedEdges)
						atomic.AddUint64(&stats.DestructedDirectories, localStats.DestructedDirectories)
					}
					return nil
				},
			)
			close(stopStatsPrinter)
			l.Info("finished collecting %v, stats: %+v", *collectDirPath, stats)
		} else {
			dirId := msgs.InodeId(*collectDirId)
			c := runtime.getClient()
			if dirId == 0 {
				if dirId, err = c.ResolvePath(l, *collectDirPath); err != nil {
					panic(fmt.Errorf("could not resolve path %v: %v", *collectDirPath, err))
				}
			}
			if dirId.Type() != msgs.DIRECTORY {
				panic(fmt.Errorf("inode id %v is not a directory", dirId))
			}
			var stats cleanup.CollectDirectoriesStats
			if err := cleanup.CollectDirectory(l, runtime.getClient(), dirInfoCache, &stats, dirId, *collectDirMinEdgeAge, forcePolicy); err != nil {
				panic(fmt.Errorf("could not collect %v, stats: %+v, err: %v", dirId, stats, err))
			}
			l.Info("finished collecting %v, stats: %+v", dirId, stats)
			return
		}
		if err != nil {
			panic(err)
		}
	}
	return Command{
		Flags: collectCmd,
		Run:   collectRun,
	}
}
