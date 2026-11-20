// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"encoding/xml"
	"errors"
	"github.com/XTXMarkets/ternfs/go/core/assert"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/iotest"
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

func fileTypeOf(t *testing.T, headers map[string]string) msgs.InodeType {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/bucket/key", nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	typ, err := fileTypeFromHeaders(r)
	assert.NoError(t, err)
	return typ
}

func TestFileTypeFromFilePermissions(t *testing.T) {
	cases := []struct {
		name  string
		perms string
		want  msgs.InodeType
	}{
		{"symlink", "0120777", msgs.SYMLINK},
		{"symlink without leading zero", "120777", msgs.SYMLINK},
		{"regular file", "0100644", msgs.FILE},
		{"directory", "040755", msgs.DIRECTORY},
		{"decimal is not octal", "41471", msgs.DIRECTORY},
		{"padded symlink", " \t0120777 ", msgs.SYMLINK},
		{"sticky directory", "041777", msgs.DIRECTORY},
		{"setuid file", "0104644", msgs.FILE},
		{"permissions only", "0777", msgs.FILE},
		{"garbage", "rwxrwxrwx", msgs.FILE},
		{"overflow", "7777777777777777777777", msgs.FILE},
		{"empty", "", msgs.FILE},
		{"zero", "0", msgs.FILE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fileTypeOf(t, map[string]string{"X-Amz-Meta-File-Permissions": tc.perms})
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFileTypeWithoutHeaders(t *testing.T) {
	assert.Equal(t, msgs.FILE, fileTypeOf(t, nil))
}

func TestFileTypeIgnoresModeHeader(t *testing.T) {
	assert.Equal(t, msgs.FILE, fileTypeOf(t, map[string]string{
		"X-Amz-Meta-File-Permissions": "garbage",
		"X-Amz-Meta-Mode":             "41471",
	}))
}

func TestFileTypeDirectoryHintsAndFirstValues(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		headers http.Header
		want    msgs.InodeType
	}{
		{"slash without headers", "/bucket/key/", nil, msgs.DIRECTORY},
		{"directory MIME", "/bucket/key", http.Header{"Content-Type": {"application/x-directory"}}, msgs.DIRECTORY},
		{"apache MIME", "/bucket/key", http.Header{"Content-Type": {"httpd/unix-directory; charset=utf-8"}}, msgs.DIRECTORY},
		{"MIME overrides file mode", "/bucket/key", http.Header{"Content-Type": {"application/x-directory"}, "X-Amz-Meta-File-Permissions": {"0100644"}}, msgs.DIRECTORY},
		{"slash permissions", "/bucket/key/", http.Header{"X-Amz-Meta-File-Permissions": {" 0040755 "}}, msgs.DIRECTORY},
		{"first link", "/bucket/key", http.Header{"X-Amz-Meta-File-Permissions": {"0120777", "garbage"}}, msgs.SYMLINK},
		{"first malformed", "/bucket/key", http.Header{"X-Amz-Meta-File-Permissions": {"garbage", "0120777"}}, msgs.FILE},
		{"first empty", "/bucket/key", http.Header{"X-Amz-Meta-File-Permissions": {"", "0120777"}}, msgs.FILE},
		{"slash first directory", "/bucket/key/", http.Header{"X-Amz-Meta-File-Permissions": {"0040755", "0120777"}}, msgs.DIRECTORY},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, tc.path, nil)
			r.Header = tc.headers
			typ, err := fileTypeFromHeaders(r)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, typ)
		})
	}
}

func TestPutObjectRejectsTypeConflictsBeforeParents(t *testing.T) {
	server := NewS3Server(log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR}), nil, nil, nil, map[string]string{"bucket": "/"}, "")
	cases := []struct {
		name    string
		key     string
		headers http.Header
	}{
		{"slash first empty", "key/", http.Header{"X-Amz-Meta-File-Permissions": {"", "0040755"}}},
		{"slash first malformed", "key/", http.Header{"X-Amz-Meta-File-Permissions": {"garbage", "0040755"}}},
	}
	for _, tc := range []struct {
		name  string
		key   string
		perms string
		mime  string
	}{
		{"combined type", "key", "0160000", ""},
		{"fifo", "key", "0010644", ""},
		{"character device", "key", "0020644", ""},
		{"reserved 0030000", "key", "0030000", ""},
		{"reserved 0050000", "key", "0050000", ""},
		{"block device", "key", "0060644", ""},
		{"reserved 0070000", "key", "0070000", ""},
		{"reserved 0110000", "key", "0110000", ""},
		{"reserved 0130000", "key", "0130000", ""},
		{"socket", "key", "0140644", ""},
		{"reserved 0150000", "key", "0150000", ""},
		{"reserved 0170000", "key", "0170000", ""},
		{"slash combined type", "key/", "0160000", ""},
		{"slash symlink", "key/", "0120777", ""},
		{"slash regular file", "key/", "0100644", ""},
		{"slash permissions only", "key/", "0777", ""},
		{"slash fifo", "key/", "0010644", ""},
		{"slash malformed", "key/", "garbage", ""},
		{"slash empty", "key/", "", ""},
		{"link directory MIME", "key", "0120777", "application/x-directory"},
		{"link apache MIME", "key", "0120777", "httpd/unix-directory; charset=utf-8"},
	} {
		headers := http.Header{}
		headers.Set("X-Amz-Meta-File-Permissions", tc.perms)
		headers.Set("Content-Type", tc.mime)
		cases = append(cases, struct {
			name    string
			key     string
			headers http.Header
		}{tc.name, tc.key, headers})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/bucket/missing/parents/"+tc.key, strings.NewReader("target/"))
			r.Header = tc.headers
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, r)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var response S3Error
			assert.NoError(t, xml.Unmarshal(rec.Body.Bytes(), &response))
			assert.Equal(t, "InvalidArgument", response.Code)
		})
	}
}

func TestReadSymlinkTarget(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"empty", "", true},
		{"one byte", "a", false},
		{"slash target", "dir/", false},
		{"exact bytes", " ../dir//target/\n", false},
		{"maximum length", strings.Repeat("x", 4095), false},
		{"over limit", strings.Repeat("x", 4096), true},
		{"large body", strings.Repeat("x", 8192), true},
		{"NUL byte", "dir\x00target", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.NewReader(tc.body)
			target, err := readSymlinkTarget(source)
			assert.Equal(t, len(tc.body)-min(len(tc.body), 4096), source.Len())
			if tc.wantErr {
				var s3Err *S3Error
				if !assert.True(t, errors.As(err, &s3Err), "want *S3Error, got %v", err) {
					return
				}
				assert.Equal(t, http.StatusBadRequest, s3Err.StatusCode)
				assert.Equal(t, "InvalidArgument", s3Err.Code)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.body, string(target))
		})
	}
	t.Run("read error", func(t *testing.T) {
		want := errors.New("body read failed")
		_, err := readSymlinkTarget(iotest.ErrReader(want))
		assert.True(t, errors.Is(err, want), "want read error, got %v", err)
	})
}

func TestPutObjectSymlinkBodyBeforeParents(t *testing.T) {
	server := NewS3Server(log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR}), nil, nil, nil, map[string]string{"bucket": "/"}, "")
	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"over limit", strings.Repeat("x", 4096)},
		{"NUL byte", "dir\x00target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.NewReader(tc.body)
			r := httptest.NewRequest(http.MethodPut, "/bucket/missing/parents/link", source)
			r.ContentLength = -1
			r.Header.Set("X-Amz-Meta-File-Permissions", "0120777")
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, r)
			var response S3Error
			assert.NoError(t, xml.Unmarshal(rec.Body.Bytes(), &response))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "InvalidArgument", response.Code)
			assert.Equal(t, len(tc.body)-min(len(tc.body), 4096), source.Len())
		})
	}
}

func TestSetModeHeaders(t *testing.T) {
	cases := []struct {
		name      string
		typ       msgs.InodeType
		wantPerms string
	}{
		{"symlink", msgs.SYMLINK, "0120777"},
		{"directory", msgs.DIRECTORY, "0040755"},
		{"regular file", msgs.FILE, "0100644"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			setModeHeaders(rec, tc.typ)
			assert.Equal(t, tc.wantPerms, rec.Header().Get("X-Amz-Meta-File-Permissions"))
			assert.Equal(t, 0, len(rec.Header().Values("X-Amz-Meta-Mode")))
		})
	}
}
