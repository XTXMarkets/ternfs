// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

func NewResurrect() SpecWithClient {
	resurrectFileCmd := flag.NewFlagSet("resurrect", flag.ExitOnError)
	resurrectFilePath := resurrectFileCmd.String("path", "", "The file to resurrect")
	resurrectFileList := resurrectFileCmd.String("list", "", "File with files to resurrect (one per line)")
	resurrectFileWorkers := resurrectFileCmd.Int("workers", 256, "")
	resurrectFileRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		if (*resurrectFilePath == "" && *resurrectFileList == "") || (*resurrectFilePath != "" && *resurrectFileList != "") {
			panic(fmt.Errorf("must provide -path or -list"))
		}
		if *resurrectFilePath == "" && *resurrectFileList == "" {
			panic(fmt.Errorf("must provide -path or -list"))
		}
		if *resurrectFileWorkers < 1 {
			panic(fmt.Errorf("workers must be > 0"))
		}
		c := runtime.Client
		t0 := time.Now()
		ch := make(chan string, *resurrectFileWorkers*4)
		var wg sync.WaitGroup
		wg.Add(*resurrectFileWorkers)
		for i := 0; i < *resurrectFileWorkers; i++ {
			go func() {
				defer wg.Done()
				for {
					p, more := <-ch
					if !more {
						return
					}
					dirId, err := c.ResolvePath(l, path.Dir(p))
					if err != nil {
						panic(err)
					}
					req := msgs.FullReadDirReq{
						DirId:     dirId,
						Flags:     msgs.FULL_READ_DIR_CURRENT | msgs.FULL_READ_DIR_BACKWARDS | msgs.FULL_READ_DIR_SAME_NAME,
						StartName: path.Base(p),
					}
					resp := msgs.FullReadDirResp{}
					if err := c.ShardRequest(l, dirId.Shard(), &req, &resp); err != nil {
						panic(err)
					}
					if len(resp.Results) < 2 {
						l.Info("%q: found < 2 edges, skipping: %+v", p, resp.Results)
						continue
					}

					if resp.Results[0].Current {
						l.Info("%q: a current edge already exists, skipping", p)
						continue
					}

					if resp.Results[0].TargetId.Id() != msgs.NULL_INODE_ID {
						l.Info("%q: last edge is not a deletion edge, skipping: %+v", p, resp.Results[0])
						continue
					}
					if !resp.Results[1].TargetId.Extra() {
						l.Info("%q: second to last edge is not an owned edge, skipping: %+v", p, resp.Results[1])
						continue
					}

					resurrectReq := msgs.SameDirectoryRenameSnapshotReq{
						TargetId:        resp.Results[1].TargetId.Id(),
						DirId:           dirId,
						OldName:         path.Base(p),
						OldCreationTime: resp.Results[1].CreationTime,
						NewName:         path.Base(p),
					}
					if err := c.ShardRequest(l, dirId.Shard(), &resurrectReq, &msgs.SameDirectoryRenameSnapshotResp{}); err != nil {
						panic(fmt.Errorf("could not resurrect %q: %w", p, err))
					}
					l.Info("%q: resurrected", p)
				}
			}()
		}
		if *resurrectFilePath != "" {
			ch <- *resurrectFilePath
		}
		seenFiles := 0
		if *resurrectFileList != "" {
			file, err := os.Open(*resurrectFileList)
			if err != nil {
				panic(err)
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if seenFiles%30_000 == 0 {
					l.Info("Went through %v files (%0.2f files/sec)", seenFiles, 1000.0*float64(seenFiles)/float64(time.Since(t0).Milliseconds()))
				}
				seenFiles++
				if strings.TrimSpace(scanner.Text()) == "" {
					continue
				}
				ch <- scanner.Text()
			}
		}
		close(ch)
		wg.Wait()
	}
	return SpecWithClient{
		Flags: resurrectFileCmd,
		Run:   resurrectFileRun,
	}
}
