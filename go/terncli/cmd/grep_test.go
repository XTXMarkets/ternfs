// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestIncludeGrepDepth(t *testing.T) {
	file := msgs.MakeInodeId(msgs.FILE, 1, 1)
	directory := msgs.MakeInodeId(msgs.DIRECTORY, 1, 2)
	for _, test := range []struct {
		roots    []string
		maxDepth int
		path     string
		id       msgs.InodeId
		want     bool
	}{
		// a root which is a file
		{[]string{"/src/main.go"}, 1, "/src/main.go", file, true},
		// only the files directly in the root
		{[]string{"/src"}, 1, "/src", directory, true},
		{[]string{"/src"}, 1, "/src/main.go", file, true},
		{[]string{"/src"}, 1, "/src/cmd", directory, false},
		// one level of subdirectories
		{[]string{"/src"}, 2, "/src/cmd", directory, true},
		{[]string{"/src"}, 2, "/src/cmd/main.go", file, true},
		{[]string{"/src"}, 2, "/src/cmd/tool", directory, false},
		{[]string{"/src"}, 2, "/src/cmd/tool/main.go", file, false},
		// the root directory has no name to strip
		{[]string{"/"}, 1, "/", directory, true},
		{[]string{"/"}, 1, "/main.go", file, true},
		{[]string{"/"}, 1, "/src", directory, false},
		// a name which only starts like the root is under the other root
		{[]string{"/src", "/"}, 1, "/srcs", directory, false},
		// nested roots: the nearest one counts
		{[]string{"/src", "/src/cmd/tool"}, 1, "/src/cmd/tool", directory, true},
		{[]string{"/src", "/src/cmd/tool"}, 1, "/src/cmd/tool/main.go", file, true},
		{[]string{"/src", "/src/cmd/tool"}, 1, "/src/cmd", directory, false},
	} {
		got := includeGrepDepth(test.roots, test.maxDepth, test.path, test.id)
		if got != test.want {
			t.Errorf("roots %v max depth %d: include %s (%v) = %v, want %v",
				test.roots, test.maxDepth, test.path, test.id.Type(), got, test.want)
		}
	}
}
