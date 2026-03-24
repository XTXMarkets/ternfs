// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"encoding/binary"
	"fmt"
	"io"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/msgs"
)

type pageCrcReader struct {
	r         io.Reader
	streamCrc [4]byte
	fullCrc   uint32
	pageCrc   uint32
	stripCrc  bool
	totalRead int
}

// NewPageCrcReader creates a reader that validates page CRCs and optionally strips them.
// Pages are expected to be TERN_PAGE_SIZE bytes followed by 4-byte CRC.
// When stripCrc is true, CRC bytes are removed from the output stream.
func NewPageCrcReader(r io.Reader, stripCrc bool) *pageCrcReader {
	return &pageCrcReader{
		r:        r,
		stripCrc: stripCrc,
	}
}

// GetCrc returns the CRC of everything read so far.
// Valid only when called after reading a complete page (including CRC bytes), error otherwise.
func (c *pageCrcReader) GetCrc() (uint32, error) {
	if c.totalRead%int(msgs.TERN_PAGE_WITH_CRC_SIZE) != 0 {
		return c.fullCrc, fmt.Errorf("not at the end of a page")
	}
	return c.fullCrc, nil
}

// Read implements io.Reader while validating page CRCs. Returns BAD_BLOCK_CRC on mismatch.
// When stripping CRCs, handles cases where caller's buffer doesn't have space for CRC bytes
// by reading and validating them internally without copying to output.
func (c *pageCrcReader) Read(p []byte) (int, error) {
	var read int
	var readFromSource int
	var err error
	for read < len(p) {
		readFromSource, err = c.r.Read(p[read:])
		for readFromSource > 0 {
			offsetInPage := c.totalRead % int(msgs.TERN_PAGE_WITH_CRC_SIZE)
			if offsetInPage < int(msgs.TERN_PAGE_SIZE) {
				availableInPage := int(msgs.TERN_PAGE_SIZE) - offsetInPage
				dataLen := min(availableInPage, readFromSource)
				c.pageCrc = crc32c.Sum(c.pageCrc, p[read:read+dataLen])
				c.fullCrc = crc32c.Sum(c.fullCrc, p[read:read+dataLen])
				read += dataLen
				readFromSource -= dataLen
				c.totalRead += dataLen
				continue
			}
			offsetInCrc := offsetInPage - int(msgs.TERN_PAGE_SIZE)
			availableCrcBytes := min(4, readFromSource)

			copy(c.streamCrc[offsetInCrc:], p[read:read+availableCrcBytes])
			read += availableCrcBytes
			readFromSource -= availableCrcBytes
			c.totalRead += availableCrcBytes
			if c.stripCrc {
				read -= availableCrcBytes
				copy(p[read:], p[read+availableCrcBytes:read+availableCrcBytes+readFromSource])
			}
			if offsetInCrc+availableCrcBytes == 4 {
				streamCrc := binary.LittleEndian.Uint32(c.streamCrc[:])
				if streamCrc != c.pageCrc {
					return read, msgs.BAD_BLOCK_CRC
				}
				c.pageCrc = 0
			}
		}
		if err != nil {
			break
		}
	}
	// Handle case where the reader wants to strip CRCs and will provide a buffer where
	// the last CRC won't fit. We still need to read and validate it.
	if c.stripCrc && c.totalRead%int(msgs.TERN_PAGE_WITH_CRC_SIZE) == int(msgs.TERN_PAGE_SIZE) {
		if _, err := io.ReadFull(c.r, c.streamCrc[:]); err != nil {
			return read, msgs.BAD_BLOCK_CRC
		}
		c.totalRead += 4
		streamCrc := binary.LittleEndian.Uint32(c.streamCrc[:])
		if streamCrc != c.pageCrc {
			return read, msgs.BAD_BLOCK_CRC
		}
		c.pageCrc = 0
	}
	return read, err
}

func (c *pageCrcReader) Close() error {
	if closer, ok := c.r.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	if c.totalRead%int(msgs.TERN_PAGE_WITH_CRC_SIZE) != 0 {
		return fmt.Errorf("incomplete page read")
	}
	return nil
}

type pageCrcWriter struct {
	w          io.Writer
	fullCrc    uint32
	pageCrc    uint32
	pageOffset int
}

func NewPageCrcWriter(w io.Writer) *pageCrcWriter {
	return &pageCrcWriter{w: w}
}

// Write implements io.Writer while appending CRCs after each full page.
func (w *pageCrcWriter) Write(p []byte) (int, error) {
	var written int
	for len(p) > 0 {
		toWrite := min(len(p), int(msgs.TERN_PAGE_SIZE)-w.pageOffset)
		n, err := w.w.Write(p[:toWrite])
		if err != nil {
			return written, err
		}
		w.pageCrc = crc32c.Sum(w.pageCrc, p[:n])
		w.fullCrc = crc32c.Sum(w.fullCrc, p[:n])
		w.pageOffset += n
		written += n
		p = p[n:]
		if w.pageOffset != int(msgs.TERN_PAGE_SIZE) {
			continue
		}
		var calculatedCrc [4]byte
		binary.LittleEndian.PutUint32(calculatedCrc[:], w.pageCrc)
		crcSlice := calculatedCrc[:]
		for len(crcSlice) > 0 {
			m, err := w.w.Write(crcSlice)
			if err != nil {
				return written, err
			}
			crcSlice = crcSlice[m:]
		}
		w.pageOffset = 0
		w.pageCrc = 0
	}
	return written, nil
}

// GetCrc returns the full data CRC when no partial page exists.
func (w *pageCrcWriter) GetCrc() (uint32, error) {
	if w.pageOffset != 0 {
		return w.fullCrc, fmt.Errorf("incomplete page, cannot get full CRC")
	}
	return w.fullCrc, nil
}
