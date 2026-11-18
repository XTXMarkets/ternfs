// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func s3FileTypeRoundTrip(log *log.Logger, c *client.Client, harness *s3TestHarness) {
	dirName := fmt.Sprintf("s3-file-types-%d", time.Now().UnixNano())
	direct := &apiFsTestHarness{client: c}
	dirId, creationTime := direct.createDirectory(log, msgs.ROOT_DIR_INODE_ID, dirName)
	defer deleteDir(log, c, msgs.ROOT_DIR_INODE_ID, dirName, creationTime, dirId)
	options := harness.client.Options()

	for _, tc := range []struct {
		name string
		mode string
		body string
		typ  msgs.InodeType
	}{
		{"link", "0120777", " ../dir//target/\n", msgs.SYMLINK},
		{"directory", "0040755", "", msgs.DIRECTORY},
	} {
		key := dirName + "/" + tc.name
		_, err := harness.client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(harness.bucket),
			Key:         aws.String(key),
			Body:        strings.NewReader(tc.body),
			ContentType: aws.String("application/octet-stream"),
			Metadata:    map[string]string{"file-permissions": tc.mode},
		})
		if err != nil {
			panic(err)
		}
		lookup := msgs.LookupResp{}
		shardReq(log, c, dirId.Shard(), &msgs.LookupReq{DirId: dirId, Name: tc.name}, &lookup)
		if lookup.TargetId.Type() != tc.typ {
			panic(fmt.Errorf("%s: expected inode type %v, got %v", key, tc.typ, lookup.TargetId.Type()))
		}
		if tc.typ == msgs.SYMLINK {
			stat := msgs.StatFileResp{}
			shardReq(log, c, lookup.TargetId.Shard(), &msgs.StatFileReq{Id: lookup.TargetId}, &stat)
			data := readFile(log, harness.bufPool, c, lookup.TargetId, stat.Size)
			defer harness.bufPool.Put(data)
			checkFileData(lookup.TargetId, 0, len(tc.body), data.Bytes(), []byte(tc.body))
		}

		reads := []struct {
			method string
			suffix string
		}{{http.MethodGet, ""}, {http.MethodHead, ""}, {http.MethodGet, "?attributes"}}
		if tc.typ == msgs.DIRECTORY {
			reads = reads[:1]
			reads[0].method = http.MethodHead
			reads[0].suffix = "/"
		}
		for _, read := range reads {
			func() {
				requestURL := *options.BaseEndpoint + "/" + harness.bucket + "/" + key + read.suffix
				req, err := http.NewRequest(read.method, requestURL, nil)
				if err != nil {
					panic(err)
				}
				if read.suffix == "?attributes" {
					req.Header.Set("X-Amz-Object-Attributes", "ETag,ObjectSize")
				}
				resp, err := options.HTTPClient.Do(req)
				if err != nil {
					panic(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					panic(fmt.Errorf("%s %s: expected 200, got %s", read.method, requestURL, resp.Status))
				}
				if mode := resp.Header.Get("X-Amz-Meta-File-Permissions"); mode != tc.mode {
					panic(fmt.Errorf("%s %s: expected mode %s, got %q", read.method, requestURL, tc.mode, mode))
				}
				if len(resp.Header.Values("X-Amz-Meta-Mode")) != 0 {
					panic(fmt.Errorf("%s %s: unexpected Mode header", read.method, requestURL))
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					panic(err)
				}
				if read.method == http.MethodGet && read.suffix == "" {
					checkFileData(key, 0, len(tc.body), body, []byte(tc.body))
				}
			}()
		}
	}
	log.Info("S3 file type round trip passed")
}
