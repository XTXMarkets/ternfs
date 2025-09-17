// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
	"xtx/ternfs/s3"
)

// bucketFlag is a custom flag type to handle multiple "-bucket" arguments.
type bucketFlag []string

func (b *bucketFlag) String() string {
	return fmt.Sprintf("%v", *b)
}

func (b *bucketFlag) Set(value string) error {
	*b = append(*b, value)
	return nil
}

func main() {
	var buckets bucketFlag
	flag.Var(&buckets, "bucket", "Bucket mapping in format <bucket-name>:<root-path>. Can be repeated.")
	virtualHost := flag.String("virtual", "", "Domain for virtual host-style requests, e.g., 's3.example.com'")
	addr := flag.String("addr", ":8080", "Address and port to listen on")
	ternfsAddr := flag.String("ternfs", "localhost:10001", "Address of the TernFS metaserver")
	verbose := flag.Bool("verbose", false, "")
	trace := flag.Bool("trace", false, "")
	flag.Parse()

	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}

	l := log.NewLogger(os.Stdout, &log.LoggerOptions{
		Level:            level,
		AppInstance:      "eggss3",
		AppType:          "restech_eggsfs.daytime",
		PrintQuietAlerts: true,
	})

	if len(buckets) == 0 {
		fmt.Fprintf(os.Stderr, "At least one -bucket flag is required.")
		os.Exit(2)
	}

	c, err := client.NewClient(
		l,
		nil,
		*ternfsAddr,
		msgs.AddrsInfo{},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create TernFS client: %v", err)
		os.Exit(1)
	}

	bucketPaths := make(map[string]string)
	for _, b := range buckets {
		parts := strings.SplitN(b, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			fmt.Fprintf(os.Stderr, "Invalid bucket format %q. Expected <bucket-name>:<root-path>", b)
			os.Exit(2)
		}
		bucketName, rootPath := parts[0], parts[1]
		l.Info("Mapping bucket %q to path %q", bucketName, rootPath)
		bucketPaths[bucketName] = rootPath
	}

	s3Server := s3.NewS3Server(
		l,
		c,
		bufpool.NewBufPool(),
		client.NewDirInfoCache(),
		bucketPaths,
		*virtualHost,
	)

	server := &http.Server{
		Addr:    *addr,
		Handler: s3Server,
	}
	l.Info("Starting S3 gateway on %q", *addr)
	if *virtualHost != "" {
		l.Info("Virtual host routing enabled for domain: %q", *virtualHost)
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
