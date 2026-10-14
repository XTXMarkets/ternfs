// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package inspect

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Resolution is the answer to /v1/lookup for one id. kind is the id's kind;
// epoch is the issuing process's epoch when the kind has one, and
// issued_by_this_process compares it with ours. known is whether this
// process has state for the id. If incomplete is true, a negative known
// cannot rule out staging whose metadata was unavailable without waiting.
// note explains the result for the reader.
type Resolution struct {
	Envelope
	ID                  string   `json:"id"`
	Kind                string   `json:"kind"`
	Epoch               string   `json:"epoch,omitempty"`
	IssuedByThisProcess *bool    `json:"issued_by_this_process,omitempty"`
	Known               bool     `json:"known"`
	Incomplete          bool     `json:"incomplete,omitempty"`
	State               string   `json:"state,omitempty"`
	Incarnation         string   `json:"incarnation,omitempty"`
	WireClientID        string   `json:"wire_clientid,omitempty"`
	InodeType           string   `json:"inode_type,omitempty"`
	Note                string   `json:"note"`
	Links               []string `json:"links"`
}

// Resolve answers /v1/lookup for one id. The kind prefix chooses the
// resolver and the value is parsed for that kind alone; a zero wire
// clientid, incarnation or inode is an error. In store-only mode the HTTP
// layer answers 503 before calling this.
func Resolve(src Source, text string) (Resolution, error) {
	kind, _, _ := strings.Cut(text, ":")
	var (
		r   Resolution
		raw []byte
		err error
	)
	switch kind {
	case KindWireClientID, KindIncarnation, KindInode:
		v, perr := ParseScalar(kind, text)
		if perr != nil {
			return Resolution{}, perr
		}
		raw = binary.BigEndian.AppendUint64(nil, v)
		switch kind {
		case KindWireClientID:
			r, err = src.ResolveWireClientID(v)
		case KindIncarnation:
			r, err = src.ResolveIncarnation(v)
		default:
			r, err = src.ResolveInode(v)
		}
	case KindStateid:
		if raw, err = ParseID(kind, text); err == nil {
			r, err = src.ResolveStateid([12]byte(raw))
		}
	case KindSession:
		if raw, err = ParseID(kind, text); err == nil {
			r, err = src.ResolveSession([16]byte(raw))
		}
	case KindIdentityHash:
		if raw, err = ParseID(kind, text); err == nil {
			r = Resolution{
				Note:  "the SHA-256 of an EXCHANGE_ID client owner, which names its identity directory in the client store; the store report describes it",
				Links: []string{"/v1/store?identity-hash=" + ID(kind, raw)},
			}
		}
	default:
		return Resolution{}, fmt.Errorf("unknown id kind %q: want one of %s", kind, strings.Join(Kinds, ", "))
	}
	if err != nil {
		return Resolution{}, err
	}
	r.Envelope, r.ID, r.Kind = src.Envelope(), ID(kind, raw), kind
	if r.Links == nil {
		r.Links = []string{}
	}
	return r, nil
}
