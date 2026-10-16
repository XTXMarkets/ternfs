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

func TestPackLength32Bounds(t *testing.T) {
	if err := PackLength32(io.Discard, math.MaxUint32); err != nil {
		t.Fatalf("maximum length: %v", err)
	}
	if err := PackLength32(io.Discard, math.MaxUint32+1); err == nil {
		t.Fatal("oversized length succeeded")
	}
	if err := PackLength32(io.Discard, -1); err == nil {
		t.Fatal("negative length succeeded")
	}
}

func TestLength32ListRoundTrip(t *testing.T) {
	xs := make([]uint64, math.MaxUint16+2)
	for i := range xs {
		xs[i] = uint64(i)
	}
	buf := new(bytes.Buffer)
	if err := PackLength32(buf, len(xs)); err != nil {
		t.Fatal(err)
	}
	for _, x := range xs {
		if err := PackScalar(buf, x); err != nil {
			t.Fatal(err)
		}
	}
	r := bytes.NewReader(buf.Bytes())
	var l int
	if err := UnpackLength32(r, &l); err != nil {
		t.Fatal(err)
	}
	var out []uint64
	EnsureLength(&out, l)
	for i := range out {
		if err := UnpackScalar(r, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	if r.Len() != 0 || len(out) != len(xs) || out[len(out)-1] != xs[len(xs)-1] {
		t.Fatal("list mismatch")
	}
}

func TestBlob32RoundTrip(t *testing.T) {
	blob := Blob(bytes.Repeat([]byte{'x'}, math.MaxUint16+1))
	buf := new(bytes.Buffer)
	if err := PackBlob32(buf, blob); err != nil {
		t.Fatal(err)
	}
	var out Blob
	r := bytes.NewReader(buf.Bytes())
	if err := UnpackBlob32(r, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob, out) || r.Len() != 0 {
		t.Fatal("blob mismatch")
	}
}
