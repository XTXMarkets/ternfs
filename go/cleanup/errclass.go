// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"context"
	"errors"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"net"
	"os"
	"syscall"
)

// errIsTolerable reports whether a block-service error should be silently
// skipped rather than alerted on. The block service's flags are checked
// against the local cache first; on a miss we force one registry refresh.
func errIsTolerable(c *client.Client, bsId msgs.BlockServiceId, err error) bool {
	if err == nil {
		return false
	}
	if os.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}

	expected := func(flags msgs.BlockServiceFlags) bool {
		if flags.HasAny(msgs.TERNFS_BLOCK_SERVICE_STALE | msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) {
			return true
		}
		// Single NR or NW alone is half-maintenance and not excused.
		return flags.HasAll(msgs.TERNFS_BLOCK_SERVICE_NO_READ | msgs.TERNFS_BLOCK_SERVICE_NO_WRITE)
	}
	if bs, ok := c.GetBlockService(bsId); ok && expected(bs.Flags) {
		return true
	}
	if err := c.RefreshBlockServices(); err != nil {
		return false
	}
	if bs, ok := c.GetBlockService(bsId); ok && expected(bs.Flags) {
		return true
	}
	return false
}
