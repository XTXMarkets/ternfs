// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"bytes"
	"encoding/binary"
	"testing"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/msgs"
)

func TestPageCrcReader(t *testing.T) {
	t.Run("valid full page with crc", func(t *testing.T) {
		data := bytes.Repeat([]byte{0xAA}, int(msgs.TERN_PAGE_SIZE))
		var crcBuf [4]byte
		crc := crc32c.Sum(0, data)
		binary.LittleEndian.PutUint32(crcBuf[:], crc)
		input := append(data, crcBuf[:]...)

		reader := NewPageCrcReader(bytes.NewReader(input), false)
		buf := make([]byte, len(input))

		n, err := reader.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != len(input) {
			t.Fatalf("Expected to read %d bytes, got %d", len(input), n)
		}
		if !bytes.Equal(buf[:len(data)], data) {
			t.Error("Data corrupted during read")
		}
	})

	t.Run("invalid crc should error", func(t *testing.T) {
		data := bytes.Repeat([]byte{0xBB}, int(msgs.TERN_PAGE_SIZE))
		var crcBuf [4]byte
		binary.LittleEndian.PutUint32(crcBuf[:], 0xDEADBEEF)
		input := append(data, crcBuf[:]...)

		reader := NewPageCrcReader(bytes.NewReader(input), false)
		buf := make([]byte, len(input))

		_, err := reader.Read(buf)
		if err != msgs.BAD_BLOCK_CRC {
			t.Fatal("Expected BAD_BLOCK_CRC error")
		}
	})

	t.Run("strip crc from output", func(t *testing.T) {
		data := bytes.Repeat([]byte{0xCC}, int(msgs.TERN_PAGE_SIZE))
		var crcBuf [4]byte
		crc := crc32c.Sum(0, data)
		binary.LittleEndian.PutUint32(crcBuf[:], crc)
		input := append(data, crcBuf[:]...)

		reader := NewPageCrcReader(bytes.NewReader(input), true)
		buf := make([]byte, len(data))

		n, err := reader.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != len(data) {
			t.Fatalf("Expected to read %d bytes, got %d", len(data), n)
		}
		if !bytes.Equal(buf, data) {
			t.Error("Data corrupted during read")
		}
	})

	t.Run("get full crc at page boundary", func(t *testing.T) {
		data := bytes.Repeat([]byte{0x11}, int(msgs.TERN_PAGE_SIZE))
		var crcBuf [4]byte
		crc := crc32c.Sum(0, data)
		binary.LittleEndian.PutUint32(crcBuf[:], crc)
		input := append(data, crcBuf[:]...)

		reader := NewPageCrcReader(bytes.NewReader(input), false)
		buf := make([]byte, len(input))
		_, err := reader.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		gotCrc, err := reader.GetCrc()
		if err != nil {
			t.Fatalf("GetCrc error: %v", err)
		}
		if gotCrc != crc {
			t.Errorf("Expected full CRC %x, got %x", crc, gotCrc)
		}
	})

	t.Run("get crc mid-page returns error", func(t *testing.T) {
		data := bytes.Repeat([]byte{0x22}, 512)
		reader := NewPageCrcReader(bytes.NewReader(data), false)
		buf := make([]byte, 512)
		_, err := reader.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		expectedCrc := crc32c.Sum(0, data)
		gotCrc, err := reader.GetCrc()
		if err == nil {
			t.Error("Expected error when getting CRC mid-page")
		}
		if gotCrc != expectedCrc {
			t.Errorf("Expected partial CRC %x, got %x", expectedCrc, gotCrc)
		}
	})

	t.Run("multiple pages", func(t *testing.T) {
		var input []byte
		var allData []byte
		for i := range 3 {
			data := bytes.Repeat([]byte{byte(i)}, int(msgs.TERN_PAGE_SIZE))
			allData = append(allData, data...)
			var crcBuf [4]byte
			crc := crc32c.Sum(0, data)
			binary.LittleEndian.PutUint32(crcBuf[:], crc)
			input = append(input, data...)
			input = append(input, crcBuf[:]...)
		}

		reader := NewPageCrcReader(bytes.NewReader(input), true)
		buf := make([]byte, len(allData))
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != len(allData) {
			t.Fatalf("Expected to read %d bytes, got %d", len(allData), n)
		}
		if !bytes.Equal(buf, allData) {
			t.Error("Data corrupted during multi-page read with CRC stripping")
		}
	})
}

func TestPageCrcWriter(t *testing.T) {
	t.Run("write full page with crc", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewPageCrcWriter(&buf)
		data := bytes.Repeat([]byte{0xDD}, int(msgs.TERN_PAGE_SIZE))

		n, err := writer.Write(data)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(data) {
			t.Fatalf("Expected to write %d bytes, got %d", len(data), n)
		}

		if buf.Len() != len(data)+4 {
			t.Fatalf("Expected %d bytes written, got %d", len(data)+4, buf.Len())
		}

		writtenCrc := binary.LittleEndian.Uint32(buf.Bytes()[len(data):])
		expectedCrc := crc32c.Sum(0, data)
		if writtenCrc != expectedCrc {
			t.Errorf("CRC mismatch: expected %x, got %x", expectedCrc, writtenCrc)
		}
	})

	t.Run("write partial page", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewPageCrcWriter(&buf)
		data := bytes.Repeat([]byte{0xEE}, 512)

		_, err := writer.Write(data)
		if err != nil {
			t.Fatal(err)
		}

		if buf.Len() != len(data) {
			t.Fatalf("Expected %d bytes written, got %d", len(data), buf.Len())
		}
	})

	t.Run("get crc before completion", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewPageCrcWriter(&buf)
		data := bytes.Repeat([]byte{0xFF}, 512)

		writer.Write(data)
		_, err := writer.GetCrc()
		if err == nil {
			t.Fatal("Expected error when getting CRC from incomplete page")
		}
	})

	t.Run("roundtrip write then read", func(t *testing.T) {
		data := bytes.Repeat([]byte{0x42}, int(msgs.TERN_PAGE_SIZE)*5)

		var buf bytes.Buffer
		writer := NewPageCrcWriter(&buf)
		_, err := writer.Write(data)
		if err != nil {
			t.Fatal(err)
		}

		reader := NewPageCrcReader(bytes.NewReader(buf.Bytes()), true)
		result := make([]byte, len(data))
		n, err := reader.Read(result)
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		if n != len(data) {
			t.Fatalf("Expected %d bytes, got %d", len(data), n)
		}
		if !bytes.Equal(data, result) {
			t.Error("Roundtrip data mismatch")
		}
	})
}
