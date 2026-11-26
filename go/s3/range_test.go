// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/core/assert"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestParseRange(t *testing.T) {
	cases := []struct {
		header     string
		size       int64
		wantStart  int64
		wantLength int64
		wantErr    bool
	}{
		// The single-range examples from RFC 9110 section 14.1.2, for a 10000
		// byte representation.
		{"bytes=0-499", 10000, 0, 500, false},
		{"bytes=500-999", 10000, 500, 500, false},
		{"bytes=-500", 10000, 9500, 500, false},
		{"bytes=9500-", 10000, 9500, 500, false},

		{"bytes=0-0", 10000, 0, 1, false},
		{"bytes=-1", 10000, 9999, 1, false},
		{"bytes=9999-9999", 10000, 9999, 1, false},

		// Suffix ranges much shorter than the object, e.g. reading a file footer.
		{"bytes=-8", 10 << 20, 10<<20 - 8, 8, false},
		{"bytes=-65536", 10 << 20, 10<<20 - 65536, 65536, false},
		// A suffix at least as long as the object selects the whole object.
		{"bytes=-10000", 10000, 0, 10000, false},
		{"bytes=-20000", 10000, 0, 10000, false},
		// A last byte past the end of the object selects up to the last byte.
		{"bytes=9990-10009", 10000, 9990, 10, false},
		{"bytes=0-16777215", 10000, 0, 10000, false},

		// Not satisfiable: the range selects no byte of the object.
		{"bytes=10000-", 10000, 0, 0, true},
		{"bytes=10000-10005", 10000, 0, 0, true},
		{"bytes=-0", 10000, 0, 0, true},
		{"bytes=0-", 0, 0, 0, true},
		{"bytes=0-5", 0, 0, 0, true},
		{"bytes=-5", 0, 0, 0, true},

		// Malformed, or not supported.
		{"items=0-5", 10000, 0, 0, true},
		{"bytes=", 10000, 0, 0, true},
		{"bytes=-", 10000, 0, 0, true},
		{"bytes=5", 10000, 0, 0, true},
		{"bytes=a-5", 10000, 0, 0, true},
		{"bytes=5-2", 10000, 0, 0, true},
		{"bytes=0-1,3-4", 10000, 0, 0, true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s size=%d", tc.header, tc.size), func(t *testing.T) {
			start, length, err := parseRange(tc.header, tc.size)
			if tc.wantErr {
				assert.True(t, err != nil, "expected an error, got start=%d length=%d", start, length)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantLength, length)
		})
	}
}

func TestWriteObjectBody(t *testing.T) {
	s := &S3Server{log: log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})}
	object := make([]byte, 1000)
	for i := range object {
		object[i] = byte(i % 251)
	}

	cases := []struct {
		name         string
		method       string
		object       []byte
		rangeHeader  string
		wantStatus   int
		contentRange string
		// For 200 and 206, the expected body is object[from:to].
		from, to int
	}{
		{"whole object", http.MethodGet, object, "", http.StatusOK, "", 0, 1000},
		{"first bytes", http.MethodGet, object, "bytes=0-9", http.StatusPartialContent, "bytes 0-9/1000", 0, 10},
		{"open ended", http.MethodGet, object, "bytes=995-", http.StatusPartialContent, "bytes 995-999/1000", 995, 1000},
		{"suffix", http.MethodGet, object, "bytes=-8", http.StatusPartialContent, "bytes 992-999/1000", 992, 1000},
		{"suffix longer than object", http.MethodGet, object, "bytes=-2000", http.StatusPartialContent, "bytes 0-999/1000", 0, 1000},
		{"end past EOF", http.MethodGet, object, "bytes=990-4095", http.StatusPartialContent, "bytes 990-999/1000", 990, 1000},
		{"head suffix", http.MethodHead, object, "bytes=-8", http.StatusPartialContent, "bytes 992-999/1000", 992, 1000},
		{"start at EOF", http.MethodGet, object, "bytes=1000-", http.StatusRequestedRangeNotSatisfiable, "bytes */1000", 0, 0},
		{"empty suffix", http.MethodGet, object, "bytes=-0", http.StatusRequestedRangeNotSatisfiable, "bytes */1000", 0, 0},
		{"empty object", http.MethodGet, []byte{}, "", http.StatusOK, "", 0, 0},
		{"empty object suffix", http.MethodGet, []byte{}, "bytes=-8", http.StatusRequestedRangeNotSatisfiable, "bytes */0", 0, 0},
		{"empty object open ended", http.MethodGet, []byte{}, "bytes=0-", http.StatusRequestedRangeNotSatisfiable, "bytes */0", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/bucket/obj", nil)
			if tc.rangeHeader != "" {
				r.Header.Set("Range", tc.rangeHeader)
			}
			w := httptest.NewRecorder()
			// Errors are written the way ServeHTTP writes them.
			if err := s.writeObjectBody(w, r, parseObjectPath("/obj"), bytes.NewReader(tc.object), int64(len(tc.object))); err != nil {
				var s3Err *S3Error
				if !errors.As(err, &s3Err) {
					t.Fatalf("unexpected error: %v", err)
				}
				s.writeXML(w, s3Err.StatusCode, s3Err)
			}
			if !assert.Equal(t, tc.wantStatus, w.Code) {
				return
			}
			assert.Equal(t, tc.contentRange, w.Header().Get("Content-Range"))
			if tc.wantStatus == http.StatusRequestedRangeNotSatisfiable {
				assert.True(t, bytes.Contains(w.Body.Bytes(), []byte("<Code>InvalidRange</Code>")), "unexpected body %q", w.Body.String())
				return
			}
			assert.Equal(t, strconv.Itoa(tc.to-tc.from), w.Header().Get("Content-Length"))
			if tc.method == http.MethodHead {
				assert.Equal(t, 0, w.Body.Len())
			} else {
				assert.EqualBytes(t, tc.object[tc.from:tc.to], w.Body.Bytes())
			}
		})
	}
}
