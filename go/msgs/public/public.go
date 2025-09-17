// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package public

import "fmt"

// Types for querying the registry from external services

const (
	StorageHostStatus string = "storage_host_status"
)

func MkPublicMessage(k string) (any, any, error) {
	switch {
	case k == StorageHostStatus:
		return &StorageHostStatusReq{}, &StorageHostStatusResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

type StorageHostStatusReq struct {
	Hostname string
}

type StorageHostStatusResp struct {
	IsIdle    bool // all blockservices NR|NW or DECOMMISSIONED
	IsDrained bool // all blockservices DECOMMISSIOND and migrated away
}
