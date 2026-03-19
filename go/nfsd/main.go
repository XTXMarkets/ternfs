// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

func main() {
	addr := flag.String("addr", ":2049", "listen address")
	root := flag.String("root", "", "local root directory to export (for testing)")
	registry := flag.String("registry", "", "TernFS registry address (for production)")
	staging := flag.String("staging", "", "staging directory for writes (omit for read-only)")
	verbose := flag.Bool("v", false, "verbose logging of NFS requests/responses")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if (*root == "") == (*registry == "") {
		slogger.Error("exactly one of -root (local) or -registry (TernFS) must be specified")
		os.Exit(1)
	}

	var ss StagingStore
	var err error
	if *staging != "" {
		ss, err = NewLocalStagingStore(*staging, slogger)
		if err != nil {
			slogger.Error("creating staging store", "err", err)
			os.Exit(1)
		}
		slogger.Info("staging directory configured", "path", *staging)
	} else {
		ss = readOnlyStagingStore{}
		slogger.Info("no staging directory — read-only mode")
	}

	var fs TernVFS
	if *root != "" {
		absRoot, err := filepath.Abs(*root)
		if err != nil {
			slogger.Error("resolving root path", "err", err)
			os.Exit(1)
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			slogger.Error("root must be a directory", "path", absRoot)
			os.Exit(1)
		}
		fs = NewLocalTernVFS(absRoot)
		slogger.Info("local VFS mode", "root", absRoot)
	} else {
		logLevel := log.INFO
		if *verbose {
			logLevel = log.DEBUG
		}
		ternLogger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: logLevel})
		c, err := client.NewClient(ternLogger, nil, *registry, msgs.AddrsInfo{})
		if err != nil {
			slogger.Error("connecting to TernFS registry", "err", err)
			os.Exit(1)
		}
		bp := bufpool.NewBufPool()
		fs = NewRemoteTernVFS(c, ternLogger, bp)
		slogger.Info("TernFS mode", "registry", *registry)
	}

	srv, err := NewServer(fs, ss, slogger)
	if err != nil {
		slogger.Error("creating server", "err", err)
		os.Exit(1)
	}
	slogger.Info("NFS server listening", "addr", *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		slogger.Error("server error", "err", err)
		os.Exit(1)
	}
}
