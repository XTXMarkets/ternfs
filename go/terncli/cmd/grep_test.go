// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestRunGrepRejectsInvalidRegexpBeforeUsingClient(t *testing.T) {
	err := runGrep(
		nil,
		nil,
		&grepParams{expression: "["},
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "compile regexp") {
		t.Fatalf("error = %v, want regexp compilation error", err)
	}
}

func TestRunGrepRejectsInvalidNameRegexpBeforeUsingClient(t *testing.T) {
	err := runGrep(
		nil,
		nil,
		&grepParams{expression: ".", nameExpression: "["},
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "compile name regexp") {
		t.Fatalf("error = %v, want name regexp compilation error", err)
	}
}

func TestIncludeGrepName(t *testing.T) {
	pattern := regexp.MustCompile(`\.go$`)
	file := msgs.MakeInodeId(msgs.FILE, 1, 1)
	directory := msgs.MakeInodeId(msgs.DIRECTORY, 1, 2)

	if !includeGrepName(pattern, "/src/main.go", file) {
		t.Fatal("matching file was excluded")
	}
	if includeGrepName(pattern, "/src/main.cc", file) {
		t.Fatal("non-matching file was included")
	}
	if !includeGrepName(pattern, "/src/vendor", directory) {
		t.Fatal("directory was excluded")
	}
}

func TestWriteGrepStats(t *testing.T) {
	var output bytes.Buffer
	err := writeGrepStats(
		&output,
		"progress",
		client.RecursiveGrepStats{
			FilesFound:         8,
			FilesScanned:       5,
			BytesScanned:       1234,
			MatchesFound:       3,
			FilesSkippedBySize: 2,
			FileErrors:         1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	const want = "grep: progress: files_found=8 files_scanned=5 files_skipped_by_size=2 file_errors=1 bytes_scanned=1234 matches_found=3\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}
