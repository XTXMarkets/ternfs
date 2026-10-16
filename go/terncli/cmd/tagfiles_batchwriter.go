// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// batchWriter is a per-(tag, shard) rotating TSV writer. One goroutine
// per writer is the sole owner of the file/buffer state; AppendRow is a
// buffered channel send.
//
// When ternfsTransient is true, files are created under a TernFS path
// where an open file is not listed by readdir until close. Otherwise the
// writer creates "<name>.tmp" and renames to "<name>" on rotation.
type batchWriter struct {
	dir             string
	tag             string
	shard           uint8
	rotationRows    int
	rotationAge     time.Duration
	ternfsTransient bool

	rows chan string
	done chan struct{}

	// Owned by the loop goroutine.
	w        *bufio.Writer
	f        *os.File
	openPath string
	row      int
	openedAt time.Time
	seq      int
	err      error
}

const batchStreamQueueDepth = 1024

type batchKey struct {
	tag   string
	shard uint8
}

type batchWriters struct {
	writers map[batchKey]*batchWriter
}

// newBatchWriters preallocates one writer per (tag, shard). AppendRow
// errors if it gets a tag that wasn't supplied here.
func newBatchWriters(outDir string, rotationRows int, rotationAge time.Duration, ternfsTransient bool, tags []string) (*batchWriters, error) {
	if rotationRows <= 0 {
		return nil, fmt.Errorf("rotationRows must be > 0")
	}
	if rotationAge <= 0 {
		return nil, fmt.Errorf("rotationAge must be > 0")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %q: %w", outDir, err)
	}
	bws := &batchWriters{writers: make(map[batchKey]*batchWriter, len(tags)*256)}
	for _, tag := range tags {
		for shard := 0; shard < 256; shard++ {
			bw := &batchWriter{
				dir:             outDir,
				tag:             tag,
				shard:           uint8(shard),
				rotationRows:    rotationRows,
				rotationAge:     rotationAge,
				ternfsTransient: ternfsTransient,
				rows:            make(chan string, batchStreamQueueDepth),
				done:            make(chan struct{}),
			}
			bws.writers[batchKey{tag: tag, shard: uint8(shard)}] = bw
			go bw.loop()
		}
	}
	return bws, nil
}

// AppendRow appends one row to the (tag, shard) writer. The row must not
// include a trailing newline.
func (b *batchWriters) AppendRow(tag string, shard uint8, row string) (err error) {
	bw, ok := b.writers[batchKey{tag: tag, shard: shard}]
	if !ok {
		return fmt.Errorf("batchWriters: unknown tag %q", tag)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("batchWriters: AppendRow after Close (%v)", r)
		}
	}()
	bw.rows <- row
	return nil
}

// Close drains every writer and returns the first error.
func (b *batchWriters) Close() error {
	for _, bw := range b.writers {
		close(bw.rows)
	}
	var firstErr error
	for _, bw := range b.writers {
		<-bw.done
		if bw.err != nil && firstErr == nil {
			firstErr = bw.err
		}
	}
	return firstErr
}

func (b *batchWriter) loop() {
	defer close(b.done)

	ticker := time.NewTicker(b.rotationAge)
	defer ticker.Stop()

	for {
		select {
		case row, ok := <-b.rows:
			if !ok {
				if err := b.closeFile(); err != nil {
					b.recordErr(err)
				}
				return
			}
			if err := b.ensureOpen(); err != nil {
				b.recordErr(err)
				continue
			}
			if _, err := b.w.WriteString(row); err != nil {
				b.recordErr(err)
				continue
			}
			if err := b.w.WriteByte('\n'); err != nil {
				b.recordErr(err)
				continue
			}
			b.row++
			if b.row >= b.rotationRows {
				if err := b.closeFile(); err != nil {
					b.recordErr(err)
				}
			}
		case <-ticker.C:
			if b.f != nil && time.Since(b.openedAt) >= b.rotationAge {
				if err := b.closeFile(); err != nil {
					b.recordErr(err)
				}
			}
		}
	}
}

func (b *batchWriter) recordErr(err error) {
	if b.err == nil {
		b.err = err
	}
}

func (b *batchWriter) ensureOpen() error {
	if b.f != nil {
		return nil
	}
	finalBase := batchFinalName(b.tag, b.shard, b.seq)
	openBase := finalBase
	if !b.ternfsTransient {
		openBase = finalBase + ".tmp"
	}
	openPath := filepath.Join(b.dir, openBase)
	f, err := os.OpenFile(openPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open %q: %w", openPath, err)
	}
	b.f = f
	b.w = bufio.NewWriterSize(f, 64*1024)
	b.openPath = openPath
	b.row = 0
	b.openedAt = time.Now()
	return nil
}

func (b *batchWriter) closeFile() error {
	if b.f == nil {
		return nil
	}
	flushErr := b.w.Flush()
	closeErr := b.f.Close()
	openPath := b.openPath
	finalBase := batchFinalName(b.tag, b.shard, b.seq)
	b.f = nil
	b.w = nil
	b.openPath = ""
	if flushErr != nil {
		return flushErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !b.ternfsTransient {
		finalPath := filepath.Join(b.dir, finalBase)
		if err := os.Rename(openPath, finalPath); err != nil {
			return fmt.Errorf("rename %q -> %q: %w", openPath, finalPath, err)
		}
	}
	b.seq++
	return nil
}

func batchFinalName(tag string, shard uint8, seq int) string {
	return fmt.Sprintf("%s-%02x-%06d.tsv", tag, shard, seq)
}
