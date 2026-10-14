// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package inspect

import (
	"bytes"
	"strings"
	"testing"
)

// Every kind round-trips through its one canonical text form, and a
// representative set of malformed inputs is rejected for the kind each is
// aimed at: the kind prefix, not the width, selects the parse.
func TestIDs(t *testing.T) {
	eight := []byte{0x40, 0, 0, 1, 0, 0, 0, 1}
	for _, tc := range []struct {
		kind string
		raw  []byte
		text string
	}{
		{KindWireClientID, eight, "wire-clientid:4000000100000001"},
		{KindIncarnation, []byte{0x20, 0, 0, 0, 0, 0, 0, 0x0e}, "incarnation:200000000000000e"},
		{KindInode, eight, "inode:4000000100000001"},
		{KindStateid, []byte{0x88, 0x31, 0x6c, 0x9d, 0, 0, 0, 0, 0, 0, 0, 0xab}, "stateid:88316c9d00000000000000ab"},
		{KindSession, []byte{0x88, 0x31, 0x6c, 0x9d, 0x20, 0, 0, 0, 0x0f, 0xb8, 0x29, 0x07, 0, 0, 0, 1}, "session:88316c9d200000000fb8290700000001"},
		{KindIdentityHash, append(append([]byte{0xaa}, make([]byte, 30)...), 0x01), "identity-hash:aa" + strings.Repeat("00", 30) + "01"},
	} {
		if got := ID(tc.kind, tc.raw); got != tc.text {
			t.Errorf("ID(%s) = %q, want %q", tc.kind, got, tc.text)
		}
		for _, in := range []string{tc.text, strings.ToUpper(tc.text[len(tc.kind)+1:])} {
			if !strings.HasPrefix(in, tc.kind) {
				in = tc.kind + ":" + in
			}
			raw, err := ParseID(tc.kind, in)
			if err != nil || !bytes.Equal(raw, tc.raw) {
				t.Errorf("ParseID(%s, %q) = %x, %v", tc.kind, in, raw, err)
			}
		}
	}

	// Representative rejections, each for the kind it is aimed at: a wrong
	// width, an untyped or 0x value, bad digits, an equal-width value of
	// another kind, and the empty string.
	for _, tc := range []struct{ kind, bad string }{
		{KindWireClientID, "wire-clientid:400000010000000"},
		{KindWireClientID, "4000000100000001"},
		{KindWireClientID, "0x4000000100000001"},
		{KindIncarnation, "incarnation:zz00000000000001"},
		{KindIncarnation, "wire-clientid:4000000100000001"},
		{KindInode, "inode:200000000000000e00"},
		{KindStateid, "stateid:4000000100000001"},
		{KindStateid, "stateid:" + strings.Repeat("0", 26)},
		{KindSession, "stateid:" + strings.Repeat("0", 24)},
		{KindIdentityHash, ""},
		{"filehandle", "filehandle:4000000100000001"},
	} {
		if _, err := ParseID(tc.kind, tc.bad); err == nil {
			t.Errorf("ParseID(%s, %q) accepted", tc.kind, tc.bad)
		}
	}
	// Zero is rejected for the eight-byte kinds; an all-zero stateid is a
	// value, distinct from absence.
	if _, err := ParseScalar(KindIncarnation, "incarnation:"+strings.Repeat("0", 16)); err == nil {
		t.Error("zero accepted as an incarnation")
	}
	if v, err := ParseScalar(KindInode, "inode:4000000100000001"); err != nil || v != 0x4000000100000001 {
		t.Errorf("ParseScalar = %#x, %v", v, err)
	}
	// A wider kind is not a scalar, even when its leading bytes are.
	if v, err := ParseScalar(KindStateid, "stateid:000000000000000100000000"); err == nil {
		t.Errorf("ParseScalar accepted a stateid as %#x", v)
	}
	if _, err := ParseID(KindStateid, "stateid:"+strings.Repeat("0", 24)); err != nil {
		t.Errorf("all-zero stateid rejected: %v", err)
	}
}
