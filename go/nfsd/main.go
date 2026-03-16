// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	addr := flag.String("addr", ":2049", "listen address")
	root := flag.String("root", ".", "root directory to export")
	staging := flag.String("staging", "", "staging directory for writes (omit for read-only)")
	verbose := flag.Bool("v", false, "verbose logging of NFS requests/responses")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		logger.Error("resolving root path", "err", err)
		os.Exit(1)
	}
	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		logger.Error("root must be a directory", "path", absRoot)
		os.Exit(1)
	}

	var ss StagingStore
	if *staging != "" {
		ss, err = NewLocalStagingStore(*staging, logger)
		if err != nil {
			logger.Error("creating staging store", "err", err)
			os.Exit(1)
		}
		logger.Info("staging directory configured", "path", *staging)
	} else {
		ss = readOnlyStagingStore{}
		logger.Info("no staging directory — read-only mode")
	}

	fs := NewLocalTernVFS(absRoot)
	srv, err := NewServer(fs, ss, logger)
	if err != nil {
		logger.Error("creating server", "err", err)
		os.Exit(1)
	}
	logger.Info("NFS server listening", "addr", *addr, "root", absRoot)
	if err := srv.ListenAndServe(*addr); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
