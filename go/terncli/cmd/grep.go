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
	"strings"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/flags"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

var errGrepInterrupted = errors.New("grep interrupted")

type grepParams struct {
	roots            flags.StringArrayFlags
	expression       string
	nameExpression   string
	workersPerShard  int
	fileWorkers      int
	maxDepth         int
	maxFileSize      uint64
	maxLineSize      int
	progressInterval time.Duration
}

func includeGrepName(pattern *regexp.Regexp, filePath string, id msgs.InodeId) bool {
	return id.Type() != msgs.FILE || pattern.MatchString(path.Base(filePath))
}

// grepDepth returns how many levels below its root filePath is, zero being
// the root itself. With nested roots the nearest one counts.
func grepDepth(roots []string, filePath string) int {
	depth := -1
	for _, root := range roots {
		below := 0
		switch {
		case filePath == root:
		case root == "/":
			below = strings.Count(filePath, "/")
		case strings.HasPrefix(filePath, root+"/"):
			below = strings.Count(filePath[len(root):], "/")
		default:
			continue
		}
		if depth < 0 || below < depth {
			depth = below
		}
	}
	return max(depth, 0)
}

// includeGrepDepth excludes files more than maxDepth levels below their root,
// and the directories that could only hold such files.
func includeGrepDepth(roots []string, maxDepth int, filePath string, id msgs.InodeId) bool {
	depth := grepDepth(roots, filePath)
	if id.Type() == msgs.DIRECTORY {
		return depth < maxDepth
	}
	return depth <= maxDepth
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
	// a second interrupt kills the process, in case a request is stuck
	context.AfterFunc(ctx, stop)
	grep := client.NewTreeGrep(logger, ternClient, params.fileWorkers)

	// The walk reports clean paths, and -max-depth compares them with the
	// roots.
	roots := make([]string, len(params.roots))
	for i, root := range params.roots {
		roots[i] = path.Clean(root)
	}
	options := &client.TreeGrepOptions{
		Roots:            roots,
		Parwalk:          client.ParwalkOptions{WorkersPerShard: params.workersPerShard},
		Pattern:          pattern,
		MaxFileSize:      params.maxFileSize,
		MaxLineSize:      params.maxLineSize,
		ProgressInterval: params.progressInterval,
	}
	if namePattern != nil || params.maxDepth > 0 {
		options.Filter = func(filePath string, id msgs.InodeId) (bool, error) {
			if params.maxDepth > 0 && !includeGrepDepth(roots, params.maxDepth, filePath, id) {
				return false, nil
			}
			return namePattern == nil || includeGrepName(namePattern, filePath, id), nil
		}
	}
	stats, grepErr := grep.Grep(
		ctx,
		options,
		client.TreeGrepCallbacks{
			Match: func(match client.GrepMatch) error {
				_, err := fmt.Fprintf(stdout, "%s:%d:%d:%s\n",
					match.Path,
					match.LineNumber,
					match.ByteOffset,
					match.Line,
				)
				return err
			},
			FileError: func(fileErr client.TreeGrepFileError) error {
				_, err := fmt.Fprintf(stderr, "grep: %s: %v\n",
					fileErr.Path, fileErr.Err)
				return err
			},
			Progress: func(stats client.TreeGrepStats) error {
				return writeGrepStats(stderr, "progress", stats)
			},
		},
	)
	if err := writeGrepStats(stderr, "done", stats); err != nil &&
		grepErr == nil {
		grepErr = err
	}
	if ctx.Err() != nil && errors.Is(grepErr, context.Cause(ctx)) {
		return errGrepInterrupted
	}
	if grepErr != nil {
		return grepErr
	}
	if stats.FileErrors != 0 {
		return fmt.Errorf("search incomplete: file_errors=%d", stats.FileErrors)
	}
	return nil
}

func writeGrepStats(
	output io.Writer,
	state string,
	stats client.TreeGrepStats,
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
	params := &grepParams{}
	grepCmd.StringVar(&params.expression, "regexp", "",
		"Go regular expression to search for. Matches are written as path:line:match-byte-offset:text.",
	)
	grepCmd.Var(&params.roots, "root", "TernFS root path to search. May be repeated.")
	grepCmd.StringVar(&params.nameExpression, "name", "",
		"Go regular expression matched against each file's base name.",
	)
	grepCmd.IntVar(&params.workersPerShard, "workers-per-shard", 5,
		"Parwalk workers per shard.",
	)
	grepCmd.IntVar(&params.fileWorkers, "file-workers", 8, "Maximum concurrent file readers.")
	grepCmd.IntVar(&params.maxDepth, "max-depth", 0,
		"Search at most this many levels below each root: 1 is only the files directly in a root. Zero means unlimited.",
	)
	grepCmd.Uint64Var(&params.maxFileSize, "max-file-size", 0,
		"Skip files larger than this many bytes; zero means unlimited.",
	)
	grepCmd.IntVar(&params.maxLineSize, "max-line-size", 0,
		"Fail a file when an encoded line exceeds this many bytes; zero means the library default (1 MiB).",
	)
	grepCmd.DurationVar(&params.progressInterval, "progress-interval", 10*time.Second,
		"Progress reporting interval; non-positive disables progress reports.",
	)
	run := func(runtime *Runtime) {
		if params.expression == "" {
			fmt.Fprintln(os.Stderr, "grep: -regexp is required")
			os.Exit(2)
		}
		if len(params.roots) == 0 {
			fmt.Fprintln(os.Stderr, "grep: at least one -root is required")
			os.Exit(2)
		}
		if params.workersPerShard < 1 || params.fileWorkers < 1 {
			fmt.Fprintln(os.Stderr,
				"grep: -workers-per-shard and -file-workers must each be at least 1",
			)
			os.Exit(2)
		}
		if params.maxDepth < 0 {
			fmt.Fprintln(os.Stderr, "grep: -max-depth must not be negative")
			os.Exit(2)
		}
		err := runGrep(runtime.Log, runtime.getClient(), params, os.Stdout, os.Stderr)
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
