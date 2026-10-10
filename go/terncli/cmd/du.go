// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
	"path"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

func formatSize(bytes uint64) string {
	bytesf := float64(bytes)
	if bytes == 0 {
		return "0"
	}
	if bytes < 1e6 {
		return fmt.Sprintf("%.2fKB", bytesf/1e3)
	}
	if bytes < 1e9 {
		return fmt.Sprintf("%.2fMB", bytesf/1e6)
	}
	if bytes < 1e12 {
		return fmt.Sprintf("%.2fGB", bytesf/1e9)
	}
	if bytes < 1e15 {
		return fmt.Sprintf("%.2fTB", bytesf/1e12)
	}
	return fmt.Sprintf("%.2fPB", bytesf/1e15)
}

func NewDu() Command {
	duCmd := flag.NewFlagSet("du", flag.ExitOnError)
	duDir := duCmd.String("path", "/", "")
	duHisto := duCmd.String("histogram", "", "Filepath in which to write size histogram (in CSV) to")
	duPhysical := duCmd.Bool("physical", false, "Also measure physical space (slower)")
	duSnapshot := duCmd.Bool("snapshot", false, "Also count snapshot files")
	duWorkersPerSshard := duCmd.Int("workers-per-shard", 5, "")
	duPattern := duCmd.String("pattern", "", "If set only measure files matching this regex pattern")
	duRun := func(runtime *Runtime) {
		l := runtime.Log
		re, err := regexp.Compile(*duPattern)
		if err != nil {
			fmt.Println("failed to compile regex pattern:", err)
			return
		}
		var numDirectories uint64
		var numFiles uint64
		var totalLogicalSize uint64
		var totalPhysicalSize uint64
		var numSnapshotFiles uint64
		var totalSnapshotLogicalSize uint64
		var totalSnapshotPhysicalSize uint64

		// Track physical size per (location, storage_class) when measuring physical space
		type locClassKey struct {
			location msgs.Location
			storage  msgs.StorageClass
		}
		groupTotals := make(map[locClassKey]struct {
			current  uint64
			snapshot uint64
		})
		groupTotalsMutex := sync.Mutex{}
		histogram := timing.NewHistogram(256, 255, 1.15) // max: ~900PB
		histoLogicalSizeBins := make([]uint64, 256)
		histoPhysicalSizeBins := make([]uint64, 256)
		histoCountBins := make([]uint64, 256)
		startedAt := time.Now()
		c := runtime.getClient()
		printReport := func() {
			if *duSnapshot {
				if *duPhysical {
					l.Info("went through %v files (%v current logical, %v current physical, %v snapshot logical, %v snapshot physical, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalPhysicalSize), formatSize(totalSnapshotLogicalSize), formatSize(totalSnapshotPhysicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				} else {
					l.Info("went through %v files (%v current, %v snapshot, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalSnapshotLogicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				}
			} else {
				if *duPhysical {
					l.Info("went through %v files (%v logical, %v physical, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalPhysicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				} else {
					l.Info("went through %v files (%v, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				}
			}
		}
		pool := client.NewParwalkPool(l, c, *duWorkersPerSshard)
		defer pool.Close()
		err = pool.Walk(
			context.Background(),
			&client.ParwalkOptions{
				Snapshot: *duSnapshot,
			},
			*duDir,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if !owned {
					return nil
				}
				if id.Type() == msgs.DIRECTORY {
					atomic.AddUint64(&numDirectories, 1)
					return nil
				}
				fullPath := path.Join(parentPath, name)
				if !re.MatchString(fullPath) {
					return nil
				}
				atomic.AddUint64(&numFiles, 1)
				resp := msgs.StatFileResp{}
				if err := c.ShardRequest(l, id.Shard(), &msgs.StatFileReq{Id: id}, &resp); err != nil {
					return err
				}
				if current {
					atomic.AddUint64(&totalLogicalSize, resp.Size)
				} else {
					atomic.AddUint64(&totalSnapshotLogicalSize, resp.Size)
				}
				bin := histogram.WhichBin(resp.Size)
				atomic.AddUint64(&histoCountBins[bin], 1)
				atomic.AddUint64(&histoLogicalSizeBins[bin], resp.Size)
				if *duPhysical {
					fileSpansReq := msgs.FileSpansReq{
						FileId:     id,
						ByteOffset: 0,
					}
					fileSpansResp := msgs.FileSpansResp{}
					physicalSize := uint64(0)
					for {
						if err := c.ShardRequest(l, id.Shard(), &fileSpansReq, &fileSpansResp); err != nil {
							return err
						}
						for spanIx := range fileSpansResp.Spans {
							span := &fileSpansResp.Spans[spanIx]
							if span.Header.IsInline {
								continue
							}

							locBody := span.Body.(*msgs.FetchedLocations)
							for idx, loc := range locBody.Locations {
								physical := uint64(loc.CellSize) * uint64(loc.Parity.Blocks()) * uint64(loc.Stripes)
								key := locClassKey{location: loc.LocationId, storage: loc.StorageClass}
								groupTotalsMutex.Lock()
								entry := groupTotals[key]
								if current {
									entry.current += physical
								} else {
									entry.snapshot += physical
								}
								groupTotals[key] = entry
								groupTotalsMutex.Unlock()
								if idx == 0 {
									physicalSize += physical
								}
							}
						}
						if fileSpansResp.NextOffset == 0 {
							break
						}
						fileSpansReq.ByteOffset = fileSpansResp.NextOffset
					}
					if current {
						atomic.AddUint64(&totalPhysicalSize, physicalSize)
					} else {
						atomic.AddUint64(&totalSnapshotPhysicalSize, physicalSize)
					}
					atomic.AddUint64(&histoPhysicalSizeBins[bin], physicalSize)
				}
				var currFiles uint64
				if current {
					currFiles = atomic.AddUint64(&numFiles, 1)
				} else {
					currFiles = atomic.AddUint64(&numSnapshotFiles, 1)
				}
				if currFiles%uint64(1_000_000) == 0 {
					printReport()
				}
				return nil
			},
		)
		if err != nil {
			panic(err)
		}
		printReport()
		if *duPhysical {
			l.Info("physical size per location and storage_class:")
			for key, totals := range groupTotals {
				l.Info("location=%v storage=%v current=%v snapshot=%v",
					uint(key.location), key.storage.String(), formatSize(totals.current), formatSize(totals.snapshot))
			}
		}
		if *duHisto != "" {
			l.Info("writing size histogram to %q", *duHisto)
			histoCsvBuf := bytes.NewBuffer([]byte{})
			if *duPhysical {
				fmt.Fprintf(histoCsvBuf, "logical_upper_bound,file_count,total_logical_size,total_physical_size\n")
			} else {
				fmt.Fprintf(histoCsvBuf, "upper_bound,file_count,total_size\n")
			}
			for i, upperBound := range histogram.Bins() {
				if *duPhysical {
					fmt.Fprintf(histoCsvBuf, "%v,%v,%v,%v\n", upperBound, histoCountBins[i], histoLogicalSizeBins[i], histoPhysicalSizeBins[i])
				} else {
					fmt.Fprintf(histoCsvBuf, "%v,%v,%v\n", upperBound, histoCountBins[i], histoLogicalSizeBins[i])
				}
			}
			if err := os.WriteFile(*duHisto, histoCsvBuf.Bytes(), 0644); err != nil {
				l.ErrorNoAlert("could not write histo file %q, will print histogram here: %v", *duHisto, err)
				fmt.Print(histoCsvBuf.Bytes())
				panic(err)
			}
		}
	}
	return Command{
		Flags: duCmd,
		Run:   duRun,
	}
}
