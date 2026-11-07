// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"

	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

const defaultGrepMaxLineSize = 1024 * 1024

// grepReadBufferSize is large enough that virtually every line comes back
// from ReadSlice as a single zero-copy fragment.
const grepReadBufferSize = 1 << 20

// ErrGrepLineTooLong reports a file containing a line over MaxLineSize.
var ErrGrepLineTooLong = errors.New("grep line is too long")

// ErrGrepFileTooLarge reports a file over MaxFileSize.
var ErrGrepFileTooLarge = errors.New("grep file is too large")

// GrepMatch describes the first match on one line. ByteOffset is the
// offset of the match within the file, not the offset of the line.
type GrepMatch struct {
	Path       string
	Inode      msgs.InodeId
	LineNumber uint64
	ByteOffset uint64
	Line       string
}

// GrepFile searches one file for pattern, invoking onMatch for the first
// match on each matching line, and returns the number of bytes read. A
// maxFileSize of zero is unlimited; a maxLineSize <= 0 uses a sensible
// default. Size-limit errors wrap ErrGrepFileTooLarge or
// ErrGrepLineTooLong and should be tested with errors.Is. Cancelling ctx
// or returning an error from onMatch stops the scan early. Callers that drive
// their own traversal can use this directly instead of RecursiveGrep.Grep.
func GrepFile(
	ctx context.Context,
	logger *log.Logger,
	client *Client,
	bufPool *bufpool.BufPool,
	filePath string,
	id msgs.InodeId,
	pattern *regexp.Regexp,
	maxFileSize uint64,
	maxLineSize int,
	onMatch func(GrepMatch) error,
) (uint64, error) {
	if maxLineSize <= 0 {
		maxLineSize = defaultGrepMaxLineSize
	}
	reader, err := client.ReadFile(
		logger,
		bufPool,
		id,
	)
	if err != nil {
		return 0, err
	}
	if maxFileSize > 0 {
		size, seekErr := reader.Seek(0, io.SeekEnd)
		if seekErr == nil {
			_, seekErr = reader.Seek(0, io.SeekStart)
		}
		if seekErr != nil {
			return 0, errors.Join(seekErr, reader.Close())
		}
		if uint64(size) > maxFileSize {
			return 0, errors.Join(ErrGrepFileTooLarge, reader.Close())
		}
	}
	bytesScanned, err := scanGrepReader(
		ctx,
		reader,
		pattern,
		maxLineSize,
		func(
			lineNumber uint64,
			byteOffset uint64,
			line []byte,
		) error {
			return onMatch(GrepMatch{
				Path:       filePath,
				Inode:      id,
				LineNumber: lineNumber,
				ByteOffset: byteOffset,
				Line:       string(line),
			})
		},
	)
	closeErr := reader.Close()
	if err == nil {
		err = closeErr
	}
	return bytesScanned, err
}

type grepContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader grepContextReader) Read(p []byte) (int, error) {
	if err := context.Cause(reader.ctx); err != nil {
		return 0, err
	}
	return reader.reader.Read(p)
}

// scanGrepReader reads reader line by line and invokes match for the
// first pattern match on each line. The line slice passed to match aliases
// the internal read buffer and is invalidated by the next match call, so
// implementations must copy it if they need to retain it (GrepFile
// converts it to a string for exactly this reason).
func scanGrepReader(
	ctx context.Context,
	reader io.Reader,
	pattern *regexp.Regexp,
	maxLineSize int,
	match func(lineNumber uint64, byteOffset uint64, line []byte) error,
) (uint64, error) {
	buffered := bufio.NewReaderSize(
		grepContextReader{ctx: ctx, reader: reader},
		grepReadBufferSize,
	)
	var bytesScanned uint64
	for lineNumber := uint64(1); ; lineNumber++ {
		lineStart := bytesScanned
		line, rawSize, err := readGrepLine(buffered, maxLineSize)
		bytesScanned += uint64(rawSize)
		if len(line) > 0 && (err == nil || errors.Is(err, io.EOF)) {
			text := bytes.TrimSuffix(line, []byte{'\n'})
			text = bytes.TrimSuffix(text, []byte{'\r'})
			if location := pattern.FindIndex(text); location != nil {
				if emitErr := match(
					lineNumber,
					lineStart+uint64(location[0]),
					text,
				); emitErr != nil {
					return bytesScanned, emitErr
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return bytesScanned, nil
		}
		if err != nil {
			return bytesScanned, err
		}
	}
}

// readGrepLine returns one line, including the trailing newline. When the
// whole line fits in the read buffer -- the common case -- the result is
// ReadSlice's zero-copy view of that buffer; only a line spanning a buffer
// refill is copied into a fresh allocation.
func readGrepLine(
	reader *bufio.Reader,
	maxLineSize int,
) ([]byte, int, error) {
	var line []byte
	rawSize := 0
	for {
		fragment, err := reader.ReadSlice('\n')
		rawSize += len(fragment)
		if rawSize > maxLineSize {
			return nil, rawSize, ErrGrepLineTooLong
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			line = append(line, fragment...)
			continue
		}
		if line == nil {
			return fragment, rawSize, err
		}
		return append(line, fragment...), rawSize, err
	}
}
