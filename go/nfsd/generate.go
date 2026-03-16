// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

// Generate NFS protocol types from XDR specification.
// Pipeline: nfs.x → xdr2msg.py → nfs.msg → msgparse → nfs.go

//go:generate sh -c "python3 ../msgparse/xdr2msg.py nfs.x > nfs.msg"
//go:generate go run ../msgparse -pkg main -o nfs.go nfs.msg
