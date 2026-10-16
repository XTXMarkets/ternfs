// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"github.com/XTXMarkets/ternfs/go/core/assert"
	"testing"
)

func TestIsDirectoryContentType(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		want        bool
	}{
		// The conventions we treat as directory markers.
		{"s3fs x-directory", "application/x-directory", true},
		{"apache unix-directory", "httpd/unix-directory", true},

		// Parameters (charset etc.) must be stripped before comparison.
		{"x-directory with charset", "application/x-directory; charset=utf-8", true},
		{"x-directory with trailing param", "application/x-directory;", true},
		{"unix-directory with param and spaces", "httpd/unix-directory ; q=1", true},
		{"surrounding whitespace", "  application/x-directory  ", true},

		// Regular object content types are not directories.
		{"octet-stream", "application/octet-stream", false},
		{"text plain", "text/plain", false},
		{"text plain with charset", "text/plain; charset=utf-8", false},
		{"empty", "", false},

		// Match is exact on the media type, not a prefix/substring.
		{"superstring", "application/x-directory-ish", false},
		{"substring", "x-directory", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isDirectoryContentType(tc.contentType))
		})
	}
}
