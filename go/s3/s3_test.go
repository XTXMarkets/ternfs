// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"xtx/ternfs/core/assert"
	"xtx/ternfs/msgs"
)

func TestParseAmzMetaMode(t *testing.T) {
	cases := []struct {
		name   string
		header string // empty means header not set at all
		want   msgs.InodeType
	}{
		// s3fs-fuse emits decimal of full st_mode.
		{"s3fs symlink (decimal 41471)", "41471", msgs.SYMLINK},
		{"s3fs regular file 0644 (decimal 33188)", "33188", msgs.FILE},
		{"s3fs directory 0755 (decimal 16877)", "16877", msgs.FILE},

		// strconv.ParseUint with base=0 must accept octal and hex too,
		// matching s3fs's strtoll(base=0) reader path.
		{"octal symlink (0120777)", "0120777", msgs.SYMLINK},
		{"octal regular file (0100644)", "0100644", msgs.FILE},
		{"hex symlink (0xa1ff)", "0xa1ff", msgs.SYMLINK},
		{"hex regular file (0x81a4)", "0x81a4", msgs.FILE},

		// Setuid/setgid/sticky bits MUST NOT be misread as a type bit.
		// 36388 = S_IFREG | S_ISUID | 0644.
		{"setuid regular file (36388)", "36388", msgs.FILE},
		// 17407 = S_IFDIR | S_ISVTX | 0777 — sticky-bit dir.
		{"sticky directory (17407)", "17407", msgs.FILE},

		// Type bits other than S_IFLNK map to a regular file.
		{"FIFO 0010644", "0010644", msgs.FILE},
		{"socket 0140644", "0140644", msgs.FILE},
		{"char dev 0020644", "0020644", msgs.FILE},
		{"block dev 0060644", "0060644", msgs.FILE},

		// Tolerant fallback: missing or garbage header → regular file, no error.
		{"absent header", "", msgs.FILE},
		{"empty value", "", msgs.FILE},
		{"garbage", "not-a-number", msgs.FILE},
		{"trailing garbage", "41471xyz", msgs.FILE},

		// Whitespace tolerance — clients sometimes pad headers.
		{"leading and trailing space", "  41471 ", msgs.SYMLINK},

		// Edge: zero is parseable but its type bits aren't S_IFLNK.
		{"zero", "0", msgs.FILE},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/bucket/key", nil)
			if tc.header != "" {
				r.Header.Set("X-Amz-Meta-Mode", tc.header)
			}
			assert.Equal(t, tc.want, parseAmzMetaMode(r))
		})
	}
}

func TestParseAmzMetaModeIsCaseInsensitive(t *testing.T) {
	// AWS canonicalizes user-metadata header names to lowercase; s3fs always
	// emits lowercase. Ensure we accept any casing of "x-amz-meta-mode".
	for _, headerName := range []string{
		"x-amz-meta-mode",
		"X-Amz-Meta-Mode",
		"X-AMZ-META-MODE",
		"X-aMz-MeTa-MoDe",
	} {
		t.Run(headerName, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/bucket/key", nil)
			r.Header.Set(headerName, "41471")
			assert.Equal(t, msgs.SYMLINK, parseAmzMetaMode(r))
		})
	}
}

func TestEmitAmzMetaMode(t *testing.T) {
	cases := []struct {
		name string
		typ  msgs.InodeType
		want string
	}{
		{"symlink", msgs.SYMLINK, "41471"},     // S_IFLNK | 0777
		{"directory", msgs.DIRECTORY, "16877"}, // S_IFDIR | 0755
		{"regular file", msgs.FILE, "33188"},   // S_IFREG | 0644
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			emitAmzMetaMode(rec, tc.typ)
			assert.Equal(t, tc.want, rec.Header().Get("X-Amz-Meta-Mode"))
		})
	}
}

// Round-trip: anything we emit for a SYMLINK must be parsed back as a SYMLINK.
// This guards against future edits where someone adjusts only one side
// (e.g. switches the constants to a different value or base) and silently
// breaks s3fs-fuse interop.
func TestSymlinkModeRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	emitAmzMetaMode(rec, msgs.SYMLINK)
	emitted := rec.Header().Get("X-Amz-Meta-Mode")

	r := httptest.NewRequest(http.MethodPut, "/bucket/key", nil)
	r.Header.Set("X-Amz-Meta-Mode", emitted)
	assert.Equal(t, msgs.SYMLINK, parseAmzMetaMode(r))
}

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
