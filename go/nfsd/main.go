// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/server"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func main() {
	addr := flag.String("addr", ":2049", "listen address")
	root := flag.String("root", "", "local root directory to export (for testing)")
	registry := flag.String("registry", "", "TernFS registry address (for production)")
	stagingDir := flag.String("staging", "", "staging directory for writes (omit for read-only)")
	verbose := flag.Bool("v", false, "verbose logging of NFS requests/responses")
	allowMinor2 := flag.Bool("allow-minor-version-2", false,
		"also accept NFSv4.2 COMPOUNDs, served as NFSv4.1 with every 4.2 operation unsupported")
	maxInFlight := flag.Int("max-in-flight-per-connection", server.DefaultMaxInFlightPerConn,
		"maximum requests admitted per TCP connection")
	statusAddr := flag.String("status-addr", "",
		"serve the read-only diagnostics API (JSON over HTTP) on this address; off by default")
	storeOnly := flag.Bool("store-only", false,
		"serve only the diagnostics API, reading the client store and -staging without\n"+
			"starting the NFS server or touching anything; requires -status-addr")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options]\n       %s -store-only -status-addr ADDR [options]\n\nOptions:\n",
			os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	// nfsd takes no positional arguments. Rejecting them here, before any
	// directory or listener is touched, keeps a mistyped command line from
	// starting a server.
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", flag.Args())
		os.Exit(2)
	}
	if *maxInFlight <= 0 {
		fmt.Fprintln(os.Stderr, "-max-in-flight-per-connection must be positive")
		os.Exit(2)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if *storeOnly {
		if *statusAddr == "" {
			fmt.Fprintln(os.Stderr, "-store-only requires -status-addr")
			os.Exit(2)
		}
		runStoreOnly(slogger, *root, *registry, *stagingDir, *statusAddr, *verbose)
		return
	}

	// Set up local state before dialling the cluster, so that a bad -staging
	// path or an unusable -root fails without a round trip to the registry.
	var ss staging.Store
	if *stagingDir != "" {
		store, err := staging.NewLocalStore(*stagingDir, slogger)
		if err != nil {
			slogger.Error("creating staging store", "err", err)
			os.Exit(1)
		}
		ss = store
		slogger.Info("staging directory configured", "path", *stagingDir)
	} else {
		ss = staging.ReadOnlyStore{}
		slogger.Info("no staging directory — read-only mode")
	}

	fs, target, err := openVFS(*root, *registry, *verbose)
	if err != nil {
		slogger.Error("opening filesystem", "err", err)
		os.Exit(1)
	}
	slogger.Info("filesystem opened", "target", target)

	srv, err := server.NewServer(fs, ss, slogger)
	if err != nil {
		slogger.Error("creating server", "err", err)
		os.Exit(1)
	}
	srv.MaxInFlightPerConn = *maxInFlight
	srv.AllowMinorVersion2 = *allowMinor2
	srv.NFSAddr = *addr
	if *statusAddr != "" {
		ln, err := net.Listen("tcp", *statusAddr)
		if err != nil {
			slogger.Error("diagnostics listen", "addr", *statusAddr, "err", err)
			os.Exit(1)
		}
		go func() {
			err := http.Serve(ln, inspect.Handler(srv, srv))
			slogger.Error("diagnostics server stopped", "err", err)
		}()
		slogger.Info("diagnostics listening", "addr", ln.Addr().String())
	}
	slogger.Info("NFS server listening", "addr", *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		slogger.Error("server error", "err", err)
		os.Exit(1)
	}
}

// runStoreOnly serves the diagnostics API for a process that is not an NFS
// server, for when nfsd will not start or a host's store should be read
// without disturbing it. It opens the client store read-only and never
// opens the staging store: staging.NewLocalStore would quarantine and
// truncate during recovery, so sidecars are only read by the store report,
// on request.
func runStoreOnly(slogger *slog.Logger, root, registry, stagingDir, statusAddr string, verbose bool) {
	fs, target, err := openVFS(root, registry, verbose)
	if err != nil {
		slogger.Error("opening filesystem", "err", err)
		os.Exit(1)
	}
	slogger.Info("filesystem opened", "target", target)
	store, err := clientstore.OpenReader(fs)
	if err != nil {
		slogger.Error("opening client store", "err", err)
		os.Exit(1)
	}
	if stagingDir != "" {
		if info, err := os.Stat(stagingDir); err != nil || !info.IsDir() {
			slogger.Error("staging directory", "path", stagingDir, "err", err)
			os.Exit(1)
		}
	}
	ln, err := net.Listen("tcp", statusAddr)
	if err != nil {
		slogger.Error("diagnostics listen", "addr", statusAddr, "err", err)
		os.Exit(1)
	}
	slogger.Info("diagnostics listening in store-only mode", "addr", ln.Addr().String(), "staging", stagingDir)
	if err := http.Serve(ln, inspect.Handler(nil, server.StoreOnlySource{ClientStore: store, StagingDir: stagingDir})); err != nil {
		slogger.Error("diagnostics server stopped", "err", err)
		os.Exit(1)
	}
}

// openVFS opens the local directory or the TernFS cluster named by exactly
// one of root and registry. It also returns a description of what it opened,
// with a local root resolved to an absolute path.
func openVFS(root string, registry string, verbose bool) (vfs.TernVFS, string, error) {
	if (root == "") == (registry == "") {
		return nil, "", fmt.Errorf(
			"exactly one of -root (local) or -registry (TernFS) must be specified")
	}
	if root != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, "", fmt.Errorf("resolving root path: %w", err)
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			return nil, "", fmt.Errorf("root must be a directory: %s", absRoot)
		}
		return vfs.NewLocalTernVFS(absRoot), "local root " + absRoot, nil
	}
	logLevel := log.INFO
	if verbose {
		logLevel = log.DEBUG
	}
	ternLogger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: logLevel})
	c, err := client.NewClient(ternLogger, nil, registry, msgs.AddrsInfo{})
	if err != nil {
		return nil, "", fmt.Errorf("connecting to TernFS registry: %w", err)
	}
	return vfs.NewRemoteTernVFS(c, ternLogger, bufpool.NewBufPool()),
		"TernFS registry " + registry, nil
}
