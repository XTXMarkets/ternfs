// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestScanGrepReader(t *testing.T) {
	const contents = "zero\nfoo one\r\nmiddle foo foo\nlast foo"
	type foundMatch struct {
		lineNumber uint64
		byteOffset uint64
		line       string
	}
	var matches []foundMatch
	bytesScanned, err := scanGrepReader(
		context.Background(),
		strings.NewReader(contents),
		regexp.MustCompile("foo"),
		1024,
		false,
		func(lineNumber uint64, byteOffset uint64, line []byte) error {
			matches = append(matches, foundMatch{
				lineNumber: lineNumber,
				byteOffset: byteOffset,
				line:       string(line),
			})
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if bytesScanned != uint64(len(contents)) {
		t.Fatalf("bytes scanned = %d, want %d", bytesScanned, len(contents))
	}
	want := []foundMatch{
		{lineNumber: 2, byteOffset: 5, line: "foo one"},
		{lineNumber: 3, byteOffset: 21, line: "middle foo foo"},
		{lineNumber: 4, byteOffset: 34, line: "last foo"},
	}
	if len(matches) != len(want) {
		t.Fatalf("matches = %#v, want %#v", matches, want)
	}
	for i := range want {
		if matches[i] != want[i] {
			t.Fatalf("match %d = %#v, want %#v", i, matches[i], want[i])
		}
	}
}

func TestScanGrepReaderRejectsLongLine(t *testing.T) {
	bytesScanned, err := scanGrepReader(
		context.Background(),
		strings.NewReader("12345\n"),
		regexp.MustCompile("."),
		4,
		false,
		func(uint64, uint64, []byte) error {
			t.Fatal("match callback was called")
			return nil
		},
	)
	if !errors.Is(err, ErrGrepLineTooLong) {
		t.Fatalf("error = %v, want %v", err, ErrGrepLineTooLong)
	}
	if bytesScanned != 6 {
		t.Fatalf("bytes scanned = %d, want 6", bytesScanned)
	}
}

var errGrepRead = errors.New("read failed")

type failingGrepReader struct {
	contents string
}

func (reader *failingGrepReader) Read(p []byte) (int, error) {
	n := copy(p, reader.contents)
	reader.contents = reader.contents[n:]
	return n, errGrepRead
}

func TestScanGrepReaderDoesNotMatchPartialLineBeforeError(t *testing.T) {
	bytesScanned, err := scanGrepReader(
		context.Background(),
		&failingGrepReader{contents: "partial foo"},
		regexp.MustCompile("foo"),
		1024,
		false,
		func(uint64, uint64, []byte) error {
			t.Fatal("match callback was called")
			return nil
		},
	)
	if !errors.Is(err, errGrepRead) {
		t.Fatalf("error = %v, want %v", err, errGrepRead)
	}
	if bytesScanned != uint64(len("partial foo")) {
		t.Fatalf("bytes scanned = %d, want %d", bytesScanned, len("partial foo"))
	}
}

type cancelAfterFirstRead struct {
	reader io.Reader
	cancel context.CancelFunc
	read   bool
}

func (reader *cancelAfterFirstRead) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	if !reader.read {
		reader.read = true
		reader.cancel()
	}
	return n, err
}

func TestScanGrepReaderCancellationBetweenBufferRefills(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	contents := strings.Repeat("a", grepReadBufferSize+1)
	reader := &cancelAfterFirstRead{
		reader: strings.NewReader(contents),
		cancel: cancel,
	}

	bytesScanned, err := scanGrepReader(
		ctx,
		reader,
		regexp.MustCompile("needle"),
		len(contents),
		// Keep this cancellation at the large scan-buffer refill.
		true,
		func(uint64, uint64, []byte) error {
			t.Fatal("match callback was called")
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
	if bytesScanned != grepReadBufferSize {
		t.Fatalf(
			"bytes scanned = %d, want %d",
			bytesScanned,
			grepReadBufferSize,
		)
	}
}

func TestScanGrepReaderFragmentedLine(t *testing.T) {
	// A line longer than the read buffer, forcing the fragmented path.
	longLine := strings.Repeat("a", grepReadBufferSize+500_000)
	contents := "before\n" + longLine + " needle\nafter\n"
	var matched []string
	bytesScanned, err := scanGrepReader(
		context.Background(),
		strings.NewReader(contents),
		regexp.MustCompile("needle"),
		2*grepReadBufferSize,
		false,
		func(lineNumber uint64, byteOffset uint64, line []byte) error {
			if lineNumber != 2 {
				t.Fatalf("match on line %d, want 2", lineNumber)
			}
			matched = append(matched, string(line))
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if bytesScanned != uint64(len(contents)) {
		t.Fatalf("bytes scanned = %d, want %d", bytesScanned, len(contents))
	}
	if len(matched) != 1 {
		t.Fatalf("matches = %d, want 1", len(matched))
	}
	if !strings.HasSuffix(matched[0], " needle") || len(matched[0]) != len(longLine)+7 {
		t.Fatalf("matched line has length %d, want %d", len(matched[0]), len(longLine)+7)
	}
}

func TestScanGrepReaderTextDetection(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		binary  bool
	}{
		{"ascii", "needle\r\n", false},
		{"utf8", "日本語 café needle\n", false},
		{"empty", "", false},
		// Only NUL counts, as in git: these are all text.
		{"latin1", "% temperature in \xb0C\nneedle\n", false},
		{"bmp-like prefix", "BM_return,weight\nneedle\n", false},
		{"control bytes", "\x0b\x1a\x1bneedle\n", false},
		{"nul after probe", strings.Repeat("a\n", grepBinaryProbeBytes/2) + "\x00 needle\n", false},
		{"nul", "needle\x00\n", true},
		{"nul at end of probe", strings.Repeat("a\n", grepBinaryProbeBytes/2-1) + "a\x00\nneedle\n", true},
		{"png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR needle", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			matches := 0
			scanned, err := scanGrepReader(t.Context(), strings.NewReader(test.content),
				regexp.MustCompile("needle"), 4096, false,
				func(_ uint64, offset uint64, _ []byte) error {
					matches++
					if want := strings.Index(test.content, "needle"); offset != uint64(want) {
						t.Errorf("offset = %d, want %d", offset, want)
					}
					return nil
				})
			if test.binary {
				if !errors.Is(err, ErrGrepBinaryFile) || matches != 0 {
					t.Fatalf("error = %v, matches = %d; want binary skip without matches", err, matches)
				}
				return
			}
			wantMatches := 1
			if test.content == "" {
				wantMatches = 0
			}
			if err != nil || scanned != uint64(len(test.content)) || matches != wantMatches {
				t.Fatalf("error = %v, bytes = %d, matches = %d", err, scanned, matches)
			}
		})
	}
}

type countingGrepReader struct {
	io.Reader
	read int
}

func (r *countingGrepReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestScanGrepReaderSkipsBinaryWithoutReadingWholeFile(t *testing.T) {
	reader := &countingGrepReader{Reader: strings.NewReader("\x00" + strings.Repeat("needle\n", 1<<18))}
	_, err := scanGrepReader(t.Context(), reader, regexp.MustCompile("needle"), 1024, false,
		func(uint64, uint64, []byte) error {
			t.Fatal("binary file emitted a match")
			return nil
		})
	if !errors.Is(err, ErrGrepBinaryFile) || reader.read > grepBinaryProbeBytes {
		t.Fatalf("error = %v, read = %d; want binary skip after the prefix", err, reader.read)
	}
}
