// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/core/log"
)

func renameRequest(source, destination string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/bucket/new?renameObject", nil)
	r.URL.Path = "/bucket/" + destination
	r.Header.Set("X-Amz-Rename-Source", source)
	return r
}

func renameServer(c storageClient) *S3Server {
	return &S3Server{
		log: log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR}),
		c:   c, bucketPaths: map[string]string{"bucket": "/"},
	}
}

func checkRenameHTTP(t *testing.T, s *S3Server, r *http.Request, status int, code string) S3Error {
	t.Helper()
	r.Body = unreadRenameBody{t}
	response := httptest.NewRecorder()
	s.ServeHTTP(response, r)
	if response.Code != status {
		t.Fatalf("%s %s %v: status=%d body=%s", r.Method, r.URL, r.Header, response.Code, response.Body)
	}
	if code == "" {
		if response.Body.Len() != 0 || response.Header().Get("ETag") != FileEtag(renameFile) {
			t.Fatalf("body=%s headers=%v", response.Body, response.Header())
		}
		return S3Error{}
	}
	var result S3Error
	if err := xml.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != code || result.BucketName != "bucket" ||
		!strings.HasPrefix(response.Header().Get("Content-Type"), "application/xml") {
		t.Fatalf("error=%+v headers=%v", result, response.Header())
	}
	return result
}

type unreadRenameBody struct{ t *testing.T }

func (b unreadRenameBody) Read([]byte) (int, error) { b.t.Fatal("body read"); return 0, nil }
func (b unreadRenameBody) Close() error             { b.t.Fatal("body closed"); return nil }

func TestServeHTTPRejectsUnsupportedRename(t *testing.T) {
	server := renameServer(&renameTestClient{t: t})
	check := func(r *http.Request, status int, code string) {
		t.Helper()
		checkRenameHTTP(t, server, r, status, code)
	}
	for _, source := range []string{
		"", "key", "bucket", "/bucket", "//bucket/a", "bucket//a", "bucket/a/",
		"bucket/a%2F", "bucket/./a", "bucket/a/..", "bucket/%2e%2e", "bucket/%zz",
		"bucket/a%2", "bucket/%ff", " ",
	} {
		check(renameRequest(source, "new"), 400, "InvalidRequest")
	}
	t.Run("foreign_bucket", func(t *testing.T) {
		checkRenameHTTP(t, renameServer(&renameTestClient{t: t}),
			renameRequest("other/old", "new"), 403, "AccessDenied")
	})
	for _, name := range []string{
		"", ".", "..", "\xff", "a\x00b", "a\x01b", "a\tb", "a\x7fb",
		strings.Repeat("a", 256), strings.Repeat("\u00e9", 128),
	} {
		check(renameRequest("bucket/"+url.PathEscape(name), "new"), 400, "InvalidRequest")
		check(renameRequest("bucket/old", name), 400, "InvalidRequest")
	}
	for _, path := range []string{"/bucket//new", "//bucket/new", "/bucket/./new", "/bucket/../new", "/bucket/new/"} {
		r := renameRequest("bucket/old", "new")
		r.URL.Path = path
		check(r, 400, "InvalidRequest")
	}
	for _, query := range []string{
		"renameObject=1", "renameObject&renameObject", "renameObject&%zz",
		"renameObject=%", "renameObject;uploads", "renameObject=;",
		"renameObject;uploads&uploadId=x&partNumber=1",
		"renameObject&uploads", "renameObject&uploadId=", "renameObject&partNumber=1",
		"renameObject&list-type=2", "renameObject&attributes", "renameObject&delete",
	} {
		r := renameRequest("bucket/old", "new")
		r.URL.RawQuery = query
		check(r, 400, "InvalidRequest")
	}
	for _, method := range []string{"GET", "POST", "DELETE", "HEAD", "put"} {
		r := renameRequest("bucket/old", "new")
		r.Method = method
		check(r, 405, "MethodNotAllowed")
	}
	for _, headers := range []http.Header{
		{}, {"X-Amz-Rename-Source": nil},
		{"X-Amz-Rename-Source": {"bucket/a", "bucket/a"}},
		{"X-Amz-Rename-Source": {"bucket/a"}, "x-amz-rename-source": {"bucket/b"}},
	} {
		r := renameRequest("bucket/old", "new")
		r.Header = headers
		check(r, 400, "InvalidRequest")
	}
	for _, length := range []int64{7, -1, 0} {
		r := renameRequest("bucket/old", "new")
		r.ContentLength = length
		if length == 0 {
			r.TransferEncoding = []string{"chunked"}
		}
		check(r, 400, "InvalidRequest")
	}
}
