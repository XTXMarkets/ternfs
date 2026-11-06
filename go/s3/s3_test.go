// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"encoding/xml"
	"github.com/XTXMarkets/ternfs/go/core/assert"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestFileTypeFromPermissions(t *testing.T) {
	cases := []struct {
		name  string
		perms string
		want  msgs.InodeType
	}{
		{"no header", "", msgs.FILE},
		{"symlink", "0120777", msgs.SYMLINK},
		{"regular file", "0100664", msgs.FILE},
		{"decimal is not octal", "41471", msgs.FILE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/bucket/key", nil)
			r.Header.Set("X-Amz-Meta-File-Permissions", tc.perms)
			assert.Equal(t, tc.want, fileTypeFromPermissions(r))
		})
	}
}

func TestPutRejectsBadSymlink(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	// A nil client panics if the handler gets as far as creating "dir".
	server := NewS3Server(logger, nil, nil, nil, map[string]string{"bucket": "/"}, "")
	cases := []struct {
		name string
		body string
	}{
		{"too long", strings.Repeat("a", 4096)},
		{"empty", ""},
		{"NUL", "a\x00b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/bucket/dir/key", strings.NewReader(tc.body))
			req.Header.Set("X-Amz-Meta-File-Permissions", "0120777")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, req)
			assert.Equal(t, http.StatusBadRequest, response.Code)
			var result S3Error
			assert.NoError(t, xml.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, "InvalidArgument", result.Code)
		})
	}
}

func TestSetPermissionsHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	setPermissionsHeader(rec, msgs.SYMLINK)
	assert.Equal(t, "0120777", rec.Header().Get("X-Amz-Meta-File-Permissions"))
	for _, typ := range []msgs.InodeType{msgs.FILE, msgs.DIRECTORY} {
		rec := httptest.NewRecorder()
		setPermissionsHeader(rec, typ)
		assert.Equal(t, 0, len(rec.Header()))
	}
}
