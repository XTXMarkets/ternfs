// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"regexp"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/flags"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

var errGrepInterrupted = errors.New("grep interrupted")

type grepParams struct {
	roots            []string
	expression       string
	nameExpression   string
	workersPerShard  int
	fileWorkers      int
	maxFileSize      uint64
	maxLineSize      int
	progressInterval time.Duration
}

func includeGrepName(pattern *regexp.Regexp, filePath string, id msgs.InodeId) bool {
	return id.Type() != msgs.FILE || pattern.MatchString(path.Base(filePath))
}

func runGrep(
	logger *log.Logger,
	ternClient *client.Client,
	params *grepParams,
	stdout io.Writer,
	stderr io.Writer,
) error {
	pattern, err := regexp.Compile(params.expression)
	if err != nil {
		return fmt.Errorf("compile regexp: %w", err)
	}
	var namePattern *regexp.Regexp
	if params.nameExpression != "" {
		namePattern, err = regexp.Compile(params.nameExpression)
		if err != nil {
			return fmt.Errorf("compile name regexp: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	parwalkPool := client.NewParwalkPool(logger, ternClient, params.workersPerShard)
	defer parwalkPool.Close()
	grep := client.NewRecursiveGrep(parwalkPool, params.fileWorkers)

	options := &client.RecursiveGrepOptions{
		Roots:            params.roots,
		Pattern:          pattern,
		MaxFileSize:      params.maxFileSize,
		MaxLineSize:      params.maxLineSize,
		ProgressInterval: params.progressInterval,
	}
	if namePattern != nil {
		options.Filter = func(filePath string, id msgs.InodeId) (bool, error) {
			return includeGrepName(namePattern, filePath, id), nil
		}
	}
	stats, grepErr := grep.Grep(
		ctx,
		options,
		client.RecursiveGrepCallbacks{
			Match: func(match client.GrepMatch) error {
				_, err := fmt.Fprintf(stdout, "%s:%d:%d:%s\n",
					match.Path,
					match.LineNumber,
					match.ByteOffset,
					match.Line,
				)
				return err
			},
			FileError: func(fileErr client.RecursiveGrepFileError) error {
				_, err := fmt.Fprintf(stderr, "grep: %s: %v\n",
					fileErr.Path, fileErr.Err)
				return err
			},
			Progress: func(stats client.RecursiveGrepStats) error {
				return writeGrepStats(stderr, "progress", stats)
			},
		},
	)
	if err := writeGrepStats(stderr, "done", stats); err != nil &&
		grepErr == nil {
		grepErr = err
	}
	if errors.Is(grepErr, context.Canceled) && ctx.Err() != nil {
		return errGrepInterrupted
	}
	return grepErr
}

func writeGrepStats(
	output io.Writer,
	state string,
	stats client.RecursiveGrepStats,
) error {
	_, err := fmt.Fprintf(
		output,
		"grep: %s: files_found=%d files_scanned=%d files_skipped_by_size=%d file_errors=%d bytes_scanned=%d matches_found=%d\n",
		state,
		stats.FilesFound,
		stats.FilesScanned,
		stats.FilesSkippedBySize,
		stats.FileErrors,
		stats.BytesScanned,
		stats.MatchesFound,
	)
	return err
}

func NewGrep() Command {
	grepCmd := flag.NewFlagSet("grep", flag.ExitOnError)
	expression := grepCmd.String(
		"regexp",
		"",
		"Go regular expression to search for. Matches are written as path:line:match-byte-offset:text.",
	)
	var roots flags.StringArrayFlags
	grepCmd.Var(&roots, "root", "TernFS root path to search. May be repeated.")
	nameExpression := grepCmd.String("name", "",
		"Go regular expression matched against each file's base name.",
	)
	workersPerShard := grepCmd.Int("workers-per-shard", 5,
		"Shared Parwalk workers per shard.",
	)
	fileWorkers := grepCmd.Int("file-workers", 8, "Maximum concurrent file readers.")
	maxFileSize := grepCmd.Uint64("max-file-size", 0,
		"Skip files larger than this many bytes; zero means unlimited.",
	)
	maxLineSize := grepCmd.Int("max-line-size", 0,
		"Fail a file when an encoded line exceeds this many bytes; zero means the library default (1 MiB).",
	)
	progressInterval := grepCmd.Duration("progress-interval", 10*time.Second,
		"Progress reporting interval; non-positive disables progress reports.",
	)
	run := func(runtime *Runtime) {
		if *expression == "" {
			fmt.Fprintln(os.Stderr, "grep: -regexp is required")
			os.Exit(2)
		}
		if len(roots) == 0 {
			fmt.Fprintln(os.Stderr, "grep: at least one -root is required")
			os.Exit(2)
		}
		if *workersPerShard < 1 || *fileWorkers < 1 {
			fmt.Fprintln(os.Stderr,
				"grep: -workers-per-shard and -file-workers must each be at least 1",
			)
			os.Exit(2)
		}
		err := runGrep(
			runtime.Log,
			runtime.getClient(),
			&grepParams{
				roots:            []string(roots),
				expression:       *expression,
				nameExpression:   *nameExpression,
				workersPerShard:  *workersPerShard,
				fileWorkers:      *fileWorkers,
				maxFileSize:      *maxFileSize,
				maxLineSize:      *maxLineSize,
				progressInterval: *progressInterval,
			},
			os.Stdout,
			os.Stderr,
		)
		if errors.Is(err, errGrepInterrupted) {
			os.Exit(130)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "grep: %v\n", err)
			os.Exit(2)
		}
	}
	return Command{Flags: grepCmd, Run: run}
}
