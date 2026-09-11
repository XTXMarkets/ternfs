// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package wyhash

import (
	"bytes"
	"encoding/hex"
	"testing"
)

const testSeed = 0x1234567890abcdef

// Recorded from the original unsafe implementation on an 8-byte aligned
// buffer, so this test also pins output compatibility with data generated
// before the rewrite.
const want100 = "07f1728368f6793f1753298e89eef75eba9b43bd9c090b18b3db1366adfd13187aec184b840d33d45af822b5973bf3b54155d04514e2fbbb6a72b40a3ca6ffbec79b7df48346de87117e15a54088cb047df1c937531e7c4af4fbde3f06ad7881d7e8403f"

func TestReadKnownSequence(t *testing.T) {
	want, err := hex.DecodeString(want100)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(want))
	n, err := New(testSeed).Read(got)
	if err != nil || n != len(got) {
		t.Fatalf("Read = %d, %v; want %d, nil", n, err, len(got))
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Read = %x\nwant   %s", got, want100)
	}
}

func TestReadIndependentOfAlignment(t *testing.T) {
	aligned := make([]byte, 100)
	New(testSeed).Read(aligned)
	for offset := 1; offset < 8; offset++ {
		big := make([]byte, 100+offset)
		New(testSeed).Read(big[offset:])
		if !bytes.Equal(big[offset:], aligned) {
			t.Fatalf("offset %d: Read = %x\nwant %x", offset, big[offset:], aligned)
		}
	}
}

func TestReadShortAndEmpty(t *testing.T) {
	// Buffers shorter than a word take the byte-at-a-time path. The original
	// implementation on an aligned 5-byte buffer produced these bytes.
	want, _ := hex.DecodeString("0717bab37a")
	got := make([]byte, 5)
	New(testSeed).Read(got)
	if !bytes.Equal(got, want) {
		t.Fatalf("Read = %x, want %x", got, want)
	}
	n, err := New(testSeed).Read(nil)
	if n != 0 || err != nil {
		t.Fatalf("Read(nil) = %d, %v; want 0, nil", n, err)
	}
}

func BenchmarkRead(b *testing.B) {
	r := New(1)
	buf := make([]byte, 1<<20)
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		r.Read(buf)
	}
}
