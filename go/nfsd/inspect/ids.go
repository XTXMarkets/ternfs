// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package inspect

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// The six identifier kinds the API routes on. Every id the API prints for
// one of them is kind:hex, lowercase and of the kind's width, and every id
// it accepts is parsed for the one kind the parameter names, so a wire
// clientid, an incarnation and an inode, all eight bytes, never
// interchange. A value copied from a response or from the session-created
// log line pastes back unchanged. Uppercase hex is accepted on input.
//
// These are presentation forms. The wire protocol, the store's directory
// names and the sidecar bytes are unchanged; the server formats observed
// values when it reports and leaves an unavailable id as the empty string.
const (
	KindWireClientID = "wire-clientid"
	KindIncarnation  = "incarnation"
	KindInode        = "inode"
	KindStateid      = "stateid"
	KindSession      = "session"
	KindIdentityHash = "identity-hash"
)

// Kinds lists them, for error messages and the index.
var Kinds = []string{KindWireClientID, KindIncarnation, KindInode, KindStateid, KindSession, KindIdentityHash}

var kindWidth = map[string]int{
	KindWireClientID: 8, KindIncarnation: 8, KindInode: 8,
	KindStateid: 12, KindSession: 16, KindIdentityHash: 32,
}

// ID formats an observed id. raw must have the kind's width.
func ID(kind string, raw []byte) string { return kind + ":" + hex.EncodeToString(raw) }

// ParseID parses kind:hex, requiring exactly that kind and width.
func ParseID(kind, text string) ([]byte, error) {
	width, known := kindWidth[kind]
	h, ok := strings.CutPrefix(text, kind+":")
	raw, err := hex.DecodeString(h)
	if !known || !ok || err != nil || len(raw) != width {
		return nil, fmt.Errorf("want %s:<%d hex digits>, got %q", kind, 2*width, text)
	}
	return raw, nil
}

// ParseScalar parses one of the three eight-byte kinds and rejects zero,
// which no record has: a zero must never widen a filter or be looked up.
func ParseScalar(kind, text string) (uint64, error) {
	raw, err := ParseID(kind, text)
	if err != nil {
		return 0, err
	}
	if len(raw) != 8 {
		return 0, fmt.Errorf("%s is not an eight-byte kind", kind)
	}
	v := binary.BigEndian.Uint64(raw)
	if v == 0 {
		return 0, fmt.Errorf("%s: zero is not an id", text)
	}
	return v, nil
}

// IncarnationLinks are the resources that describe an incarnation, given
// its formatted id.
func IncarnationLinks(incarnation string) []string {
	return []string{
		"/v1/incarnations?id=" + incarnation,
		"/v1/sessions?incarnation=" + incarnation,
		"/v1/store?incarnation=" + incarnation,
	}
}

// Scalar formats an eight-byte id, or returns "" for zero, which everywhere
// it appears means "not observed": a missing base inode, an unowned staging
// file, a group with no wire record.
func Scalar(kind string, id uint64) string {
	if id == 0 {
		return ""
	}
	return ID(kind, binary.BigEndian.AppendUint64(nil, id))
}

// IdentityHash formats an identity directory name, which is the hash as 64
// hex digits when the store is well formed, or "" when it is not.
func IdentityHash(name string) string {
	raw, err := hex.DecodeString(name)
	if err != nil || len(raw) != 32 {
		return ""
	}
	return ID(KindIdentityHash, raw)
}

// NewIncarnation is the empty /v1/incarnations record each store fills its
// own part of.
func NewIncarnation(id uint64) *Incarnation {
	text := Scalar(KindIncarnation, id)
	return &Incarnation{ID: text, Sessions: []string{}, Links: IncarnationLinks(text)}
}
