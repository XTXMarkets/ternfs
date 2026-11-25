// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestIntentRoundTrip(t *testing.T) {
	for i, in := range []any{
		renameIntent{srcDir: 1, srcName: "a", src: vfs.Edge{ID: 2, CreationTime: 3}, dstDir: 4, dstName: "b", dst: 5},
		renameIntent{srcDir: 1, srcName: "", src: vfs.Edge{ID: 2}, dstDir: 1, dstName: "same dir, no target"},
		removeIntent{dir: 6, name: "gone", edge: vfs.Edge{ID: 7, CreationTime: 8}},
		createIntent{dir: 9, name: "d", objType: nfs4.NF4DIR},
		createIntent{dir: 9, name: "l", objType: nfs4.NF4LNK, target: "../elsewhere"},
	} {
		op := 1 + i*31 // up to maxCompoundOperations
		var b []byte
		switch v := in.(type) {
		case renameIntent:
			b = v.encode(op)
		case removeIntent:
			b = v.encode(op)
		case createIntent:
			b = v.encode(op)
		}
		gotOp, got, err := decodeIntent(b)
		if err != nil || got != in || gotOp != op {
			t.Fatalf("%+v at %d -> %+v at %d %v", in, op, got, gotOp, err)
		}
		for cut := 0; cut < len(b); cut++ {
			if _, _, err := decodeIntent(b[:cut]); !errors.Is(err, errBadIntent) {
				t.Fatalf("%+v truncated to %d bytes decoded: %v", in, cut, err)
			}
		}
		if _, _, err := decodeIntent(append(b, 0)); !errors.Is(err, errBadIntent) {
			t.Fatalf("%+v with a trailing byte decoded: %v", in, err)
		}
	}
	if _, _, err := decodeIntent([]byte{9, 0, 1, 0}); !errors.Is(err, errBadIntent) {
		t.Fatalf("unknown kind decoded: %v", err)
	}
	if _, _, err := decodeIntent(createIntent{dir: 1, name: "x", objType: nfs4.NF4REG}.encode(2)); !errors.Is(err, errBadIntent) {
		t.Fatalf("create of a regular file decoded: %v", err)
	}
}

// The intents of the longest names and target TernFS takes fit the record
// a session's slot area is sized for, whatever reply size it caches.
func TestLongestIntentsFitSlotArea(t *testing.T) {
	name := strings.Repeat("n", maxTernNameLength)
	target := strings.Repeat("t", maxTernBytesLength)
	for _, b := range [][]byte{
		renameIntent{srcName: name, dstName: name}.encode(maxCompoundOperations - 1),
		removeIntent{name: name}.encode(maxCompoundOperations - 1),
		createIntent{name: name, objType: nfs4.NF4LNK, target: target}.encode(maxCompoundOperations - 1),
	} {
		if len(b) > maxIntentRecord {
			t.Fatalf("intent of %d bytes, area holds %d", len(b), maxIntentRecord)
		}
	}
}

func TestSessionRecordRoundTrip(t *testing.T) {
	in := sessionRecord{
		incarnation: 0x2000000000000385, wireClientID: 0x6ad2271d00000001,
		fore:      channelAttrs{headerPad: 0, maxReq: 1 << 20, maxResp: 1 << 20, maxCached: 64 << 10, maxOps: 16, maxReqs: 64},
		back:      channelAttrs{maxReq: 4096, maxResp: 4096, maxOps: 2, maxReqs: 1},
		cbProgram: 0x40000000, backRequested: true,
		cbSec: callbackSec{flavor: nfs4.AUTH_SYS, body: []byte("cred body")},
	}
	got, err := decodeSessionRecord(in.encode())
	if err != nil || got.incarnation != in.incarnation || got.fore != in.fore || got.back != in.back ||
		got.cbProgram != in.cbProgram || got.backRequested != in.backRequested ||
		got.cbSec.flavor != in.cbSec.flavor || string(got.cbSec.body) != string(in.cbSec.body) {
		t.Fatalf("%+v -> %+v %v", in, got, err)
	}
	if _, err := decodeSessionRecord(append(in.encode(), 1)); !errors.Is(err, errBadSessionRecord) {
		t.Fatalf("record with a trailing byte decoded: %v", err)
	}
	if _, err := decodeSessionRecord(in.encode()[:10]); !errors.Is(err, errBadSessionRecord) {
		t.Fatalf("truncated record decoded: %v", err)
	}
	u := effectiveUser{flavor: nfs4.AUTH_SYS, uid: 1000, gid: 1000, groups: "4,24,27"}
	if got, ok := decodeUser(encodeUser(u)); !ok || got != u {
		t.Fatalf("user %+v -> %+v %v", u, got, ok)
	}
}
