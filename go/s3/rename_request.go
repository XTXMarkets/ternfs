// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

func validRenameName(s string) bool {
	return s != "" && s != "." && s != ".." && len(s) <= 255 && utf8.ValidString(s) &&
		!strings.ContainsFunc(s, func(r rune) bool {
			return r < 0x20 || r == 0x7f || r == '/'
		})
}

func parseRenameRequest(r *http.Request, bucket string, destination *objectPath) (*objectPath, error) {
	invalid := &S3Error{
		Code: "InvalidRequest", StatusCode: http.StatusBadRequest,
		Message: "RenameObject requires bucket/key or /bucket/key paths and an empty request.",
	}
	if r == nil || r.URL == nil {
		return nil, invalid
	}
	if r.Method != http.MethodPut {
		return nil, &S3Error{
			Code: "MethodNotAllowed", StatusCode: http.StatusMethodNotAllowed,
			Message: "RenameObject requires PUT.",
		}
	}
	if destination == nil || len(destination.segments) == 0 ||
		destination.isDirectory || !strings.HasPrefix(r.URL.Path, "/") {
		return nil, invalid
	}
	// parseObjectPath drops empty components.
	for _, parts := range [][]string{destination.segments, strings.Split(r.URL.Path[1:], "/")} {
		for _, name := range parts {
			if !validRenameName(name) {
				return nil, invalid
			}
		}
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if flag := query["renameObject"]; err != nil || len(flag) != 1 || flag[0] != "" {
		return nil, invalid
	}
	for _, key := range strings.Fields("uploads uploadId partNumber list-type attributes delete") {
		if _, ok := query[key]; ok {
			return nil, invalid
		}
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		return nil, invalid
	}
	var sources []string
	for key, values := range r.Header {
		if strings.ToLower(key) == "x-amz-rename-source" {
			sources = append(sources, values...)
		}
	}
	if len(sources) != 1 || sources[0] == "" {
		return nil, invalid
	}
	decoded, err := url.PathUnescape(sources[0])
	parts := strings.Split(strings.TrimPrefix(decoded, "/"), "/")
	if err != nil || len(parts) < 2 {
		return nil, invalid
	}
	for _, name := range parts {
		if !validRenameName(name) {
			return nil, invalid
		}
	}
	if parts[0] != bucket {
		return nil, &S3Error{
			Code: "AccessDenied", StatusCode: http.StatusForbidden,
			Message: "Rename source must be in the same bucket.",
		}
	}
	return &objectPath{segments: parts[1:]}, nil
}
