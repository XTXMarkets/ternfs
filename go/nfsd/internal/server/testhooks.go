// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import "time"

// Test hooks: other packages' tests feed the server's counters directly.

// ObserveOpForTest records an operation as the compound loop would.
func (s *Server) ObserveOpForTest(op, status uint32, d time.Duration) {
	s.metrics.observeOp(op, status, d)
}
