// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package bincode

import (
	"bytes"
	"io"
	"math"
	"testing"
)

type oversizedPackable struct{}

func (oversizedPackable) Pack(w io.Writer) error {
	return PackBytes(w, bytes.Repeat([]byte{'x'}, math.MaxUint8+1))
}

func TestPackBytesLengthBounds(t *testing.T) {
	if err := PackBytes(io.Discard, bytes.Repeat([]byte{'x'}, math.MaxUint8)); err != nil {
		t.Fatalf("maximum-length bytes: %v", err)
	}
	if err := PackBytes(io.Discard, bytes.Repeat([]byte{'x'}, math.MaxUint8+1)); err == nil {
		t.Fatal("oversized bytes succeeded")
	}
}

func TestPackBlobLengthBounds(t *testing.T) {
	if err := PackBlob(io.Discard, bytes.Repeat([]byte{'x'}, math.MaxUint16)); err != nil {
		t.Fatalf("maximum-length blob: %v", err)
	}
	if err := PackBlob(io.Discard, bytes.Repeat([]byte{'x'}, math.MaxUint16+1)); err == nil {
		t.Fatal("oversized blob succeeded")
	}
}

func TestPackLengthBounds(t *testing.T) {
	if err := PackLength(io.Discard, math.MaxUint16); err != nil {
		t.Fatalf("maximum length: %v", err)
	}
	if err := PackLength(io.Discard, math.MaxUint16+1); err == nil {
		t.Fatal("oversized length succeeded")
	}
	if err := PackLength(io.Discard, -1); err == nil {
		t.Fatal("negative length succeeded")
	}
}

func TestPackReturnsPackingErrors(t *testing.T) {
	if _, err := Pack(oversizedPackable{}); err == nil {
		t.Fatal("oversized value packed successfully")
	}
}
