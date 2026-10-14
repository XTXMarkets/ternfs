// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// rpcRequest holds the parsed ONC RPC call header.
type rpcRequest struct {
	xid        uint32
	prog       uint32
	vers       uint32
	proc       uint32
	credFlavor uint32
	credBody   []byte
	verfFlavor uint32
	verfBody   []byte
	body       []byte // remaining bytes after header
	wireSize   int    // full reassembled XDR call, excluding TCP record markers
	connection *rpcConnection
	replyWork  *replyWork
	hold       *requestHold // an admitted SEQUENCE's hold, until releaseReply
	// afterReply runs once the reply is written or will never be: a new
	// back connection is probed only after the reply that created it.
	afterReply []func()
}

// Replay identity is the effective supported user, independently of the
// registration principal. AUTH_SYS host/time and group order are not identity.
type effectiveUser struct {
	flavor   uint32
	uid, gid uint32
	groups   string
}

func (r *rpcRequest) effectiveUser() (effectiveUser, error) {
	switch r.credFlavor {
	case nfs4.AUTH_NONE:
		if len(r.credBody) != 0 {
			return effectiveUser{}, fmt.Errorf("AUTH_NONE body")
		}
		return effectiveUser{flavor: nfs4.AUTH_NONE, uid: 65534, gid: 65534}, nil
	case nfs4.AUTH_SYS:
		body := r.credBody
		if len(body) < 8 {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS header")
		}
		name := uint64(binary.BigEndian.Uint32(body[4:8]))
		if name > 255 {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS machine name")
		}
		off := uint64(8) + (name+3)&^3
		if off+12 > uint64(len(body)) {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS user")
		}
		uid := binary.BigEndian.Uint32(body[off:])
		gid := binary.BigEndian.Uint32(body[off+4:])
		count := uint64(binary.BigEndian.Uint32(body[off+8:]))
		off += 12
		if count > 16 || off+4*count != uint64(len(body)) {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS groups")
		}
		groups := make([]uint32, 0, count)
		for i := uint64(0); i < count; i++ {
			groups = append(groups, binary.BigEndian.Uint32(body[off+4*i:]))
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
		var canonical []byte
		var prev uint32
		first := true
		for _, group := range groups {
			if group == gid || !first && group == prev {
				continue
			}
			canonical = binary.BigEndian.AppendUint32(canonical, group)
			prev, first = group, false
		}
		return effectiveUser{r.credFlavor, uid, gid, string(canonical)}, nil
	default:
		return effectiveUser{}, fmt.Errorf("unsupported credential flavor %d", r.credFlavor)
	}
}

func (r *rpcRequest) fullSize() int {
	if r.wireSize != 0 {
		return r.wireSize
	}
	// Direct in-process tests also get honest header/opaque padding accounting.
	return 40 + ((len(r.credBody) + 3) &^ 3) + ((len(r.verfBody) + 3) &^ 3) + len(r.body)
}

func (r *rpcRequest) principal() clientstore.RPCPrincipal {
	body := r.credBody
	// AUTH_SYS starts with a caller-supplied timestamp. It is not part of
	// the principal and may change between requests from the same caller.
	if r.credFlavor == nfs4.AUTH_SYS && len(body) >= 4 {
		body = body[4:]
		// The machine name is caller-supplied too. Linux nfsd identifies
		// AUTH_SYS callers by uid, gid and auxiliary groups instead.
		if len(body) >= 4 {
			nameLen := int(binary.BigEndian.Uint32(body[:4]))
			nameEnd := 4 + (nameLen+3)&^3
			if nameEnd <= len(body) {
				body = body[nameEnd:]
			}
		}
	}
	return clientstore.RPCPrincipal{Flavor: r.credFlavor, Body: string(body)}
}
