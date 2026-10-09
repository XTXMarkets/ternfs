// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"strings"
)

// leaseTemp reports a refresher lease temp: the publication of this
// process's lease uses ConstructFile and LinkFile like a file publication,
// so fixtures that fault or count file publications skip these.
func leaseTemp(name string) bool { return strings.HasPrefix(name, tempPrefix) }
