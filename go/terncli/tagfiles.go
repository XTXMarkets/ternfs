// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"os"
	"path"
	"sync/atomic"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type tagFilesParams struct {
	rulesPath           string
	roots               []string
	outputDir           string
	outputOnTernFS      bool
	rotationRows        int
	rotationInterval    time.Duration
	workersPerShard     int
	dryRun              bool
	creationTimePreskip bool
}

type tagFilesStats struct {
	files          atomic.Uint64
	dirs           atomic.Uint64
	preskipped     atomic.Uint64
	dirsEmptyCheck atomic.Uint64
	tags           []string
	rowsByTag      map[string]*atomic.Uint64
	bytesByTag     map[string]*atomic.Uint64
	statErrors     atomic.Uint64
	matchErrors    atomic.Uint64
}

func newTagFilesStats(rules []*Rule) *tagFilesStats {
	s := &tagFilesStats{
		rowsByTag:  make(map[string]*atomic.Uint64),
		bytesByTag: make(map[string]*atomic.Uint64),
	}
	for _, r := range rules {
		if _, ok := s.rowsByTag[r.Tag]; ok {
			continue
		}
		s.tags = append(s.tags, r.Tag)
		s.rowsByTag[r.Tag] = new(atomic.Uint64)
		s.bytesByTag[r.Tag] = new(atomic.Uint64)
	}
	return s
}

func runTagFiles(l *log.Logger, c *client.Client, p *tagFilesParams) error {
	rulesBytes, err := os.ReadFile(p.rulesPath)
	if err != nil {
		return fmt.Errorf("read rules: %w", err)
	}
	rules, err := LoadRules(rulesBytes)
	if err != nil {
		return fmt.Errorf("parse rules: %w", err)
	}
	fileRules := make([]*Rule, 0, len(rules))
	dirRules := make([]*Rule, 0, len(rules))
	for _, r := range rules {
		if r.AppliesTo == "directory" {
			dirRules = append(dirRules, r)
		} else {
			fileRules = append(fileRules, r)
		}
	}
	l.Info("loaded %d rules (%d file, %d directory)", len(rules), len(fileRules), len(dirRules))

	stats := newTagFilesStats(rules)

	var bw *batchWriters
	if !p.dryRun {
		bw, err = newBatchWriters(p.outputDir, p.rotationRows, p.rotationInterval, p.outputOnTernFS, stats.tags)
		if err != nil {
			return fmt.Errorf("batch writer: %w", err)
		}
	}

	now := msgs.Now()
	startedAt := time.Now()

	cb := func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
		if id.Type() == msgs.DIRECTORY {
			stats.dirs.Add(1)
			if !owned || !current || len(dirRules) == 0 {
				return nil
			}
			fullPath := path.Join(parentPath, name)
			if !AnyMaybeMatches(dirRules, fullPath, creationTime, now) {
				return nil
			}
			stats.dirsEmptyCheck.Add(1)
			rdResp := msgs.ReadDirResp{}
			if err := c.ShardRequest(l, id.Shard(), &msgs.ReadDirReq{DirId: id}, &rdResp); err != nil {
				l.ErrorNoAlert("readdir %s: %v", fullPath, err)
				stats.statErrors.Add(1)
				return nil
			}
			if len(rdResp.Results) > 0 {
				return nil
			}
			statResp := msgs.StatDirectoryResp{}
			if err := c.ShardRequest(l, id.Shard(), &msgs.StatDirectoryReq{Id: id}, &statResp); err != nil {
				l.ErrorNoAlert("statdir %s: %v", fullPath, err)
				stats.statErrors.Add(1)
				return nil
			}
			// Directories have no atime; reuse mtime so workers can drift-
			// check uniformly with file rows.
			fired := FirstMatch(dirRules, fullPath, 0, statResp.Mtime, statResp.Mtime, now)
			if fired == nil {
				return nil
			}
			stats.rowsByTag[fired.Tag].Add(1)
			if p.dryRun {
				return nil
			}
			row := fmt.Sprintf(
				"%s\t%d\t%d\t%d\t%s\t%s",
				id.String(),
				0,
				uint64(statResp.Mtime),
				uint64(statResp.Mtime),
				fired.Name,
				fullPath,
			)
			if err := bw.AppendRow(fired.Tag, uint8(id.Shard()), row); err != nil {
				l.ErrorNoAlert("append row %s tag=%s: %v", fullPath, fired.Tag, err)
				stats.matchErrors.Add(1)
			}
			return nil
		}
		if !fileRulesApplyTo(id) {
			return nil
		}
		if !owned || !current {
			return nil
		}
		stats.files.Add(1)

		fullPath := path.Join(parentPath, name)

		if p.creationTimePreskip && !AnyMaybeMatches(fileRules, fullPath, creationTime, now) {
			stats.preskipped.Add(1)
			return nil
		}

		resp := msgs.StatFileResp{}
		if err := c.ShardRequest(l, id.Shard(), &msgs.StatFileReq{Id: id}, &resp); err != nil {
			l.ErrorNoAlert("stat %s: %v", fullPath, err)
			stats.statErrors.Add(1)
			return nil
		}

		fired := FirstMatch(fileRules, fullPath, resp.Size, resp.Atime, resp.Mtime, now)
		if fired == nil {
			return nil
		}

		stats.rowsByTag[fired.Tag].Add(1)
		stats.bytesByTag[fired.Tag].Add(resp.Size)

		if p.dryRun {
			return nil
		}

		// Row schema: inode_hex \t size \t atime_ns \t mtime_ns \t rule \t path
		row := fmt.Sprintf(
			"%s\t%d\t%d\t%d\t%s\t%s",
			id.String(),
			resp.Size,
			uint64(resp.Atime),
			uint64(resp.Mtime),
			fired.Name,
			fullPath,
		)
		if err := bw.AppendRow(fired.Tag, uint8(id.Shard()), row); err != nil {
			l.ErrorNoAlert("append row %s tag=%s: %v", fullPath, fired.Tag, err)
			stats.matchErrors.Add(1)
		}
		return nil
	}

	walkErr := client.ParwalkMany(
		l,
		c,
		&client.ParwalkOptions{WorkersPerShard: p.workersPerShard},
		p.roots,
		cb,
	)
	if walkErr != nil {
		l.ErrorNoAlert("walk: %v", walkErr)
	}

	var closeErr error
	if bw != nil {
		closeErr = bw.Close()
		if closeErr != nil {
			l.ErrorNoAlert("close batches: %v", closeErr)
		}
	}

	elapsed := time.Since(startedAt)
	files := stats.files.Load()
	rate := float64(files) / elapsed.Seconds()
	l.Info("tag-files done in %s: %d files visited (%d dirs), %.0f files/s, %d stat-preskipped, %d dir-empty-checks, %d stat errors, %d write errors",
		elapsed.Truncate(time.Second), files, stats.dirs.Load(), rate, stats.preskipped.Load(), stats.dirsEmptyCheck.Load(), stats.statErrors.Load(), stats.matchErrors.Load())
	fmt.Fprintln(os.Stderr, "tag\trows\ttotal_bytes")
	for _, tag := range stats.tags {
		rows := stats.rowsByTag[tag].Load()
		bytes := stats.bytesByTag[tag].Load()
		fmt.Fprintf(os.Stderr, "%s\t%d\t%d\n", tag, rows, bytes)
	}

	if walkErr != nil {
		return walkErr
	}
	return closeErr
}

func fileRulesApplyTo(id msgs.InodeId) bool {
	return id.Type() == msgs.FILE
}
