// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/server"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func main() {
	cfg, err := parseConfig(flag.CommandLine, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}
	slogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if cfg.Instance == "" {
		hostname, _ := os.Hostname()
		cfg.Instance = defaultInstance(hostname, cfg.Addr)
	}
	// The xmon app instance is the metrics instance.
	ternLevel := log.INFO
	if cfg.Verbose {
		ternLevel = log.DEBUG
	}
	ternLogger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: ternLevel, XmonAddr: cfg.Xmon, AppInstance: cfg.Instance, AppType: log.XmonAppType(cfg.XmonAppType)})

	// Set up local state before dialling the cluster, so that a bad staging
	// path or an unusable root fails without a round trip to the registry.
	ss, err := staging.NewLocalStore(cfg.Staging, slogger)
	if err != nil {
		slogger.Error("creating staging store", "err", err)
		os.Exit(1)
	}
	slogger.Info("staging directory configured", "path", cfg.Staging)
	if n := len(ss.Entries()); cfg.ReadOnly && n > 0 {
		// A read-only server can never publish them.
		slogger.Error("staging holds staged writers; start the server writable to publish them",
			"path", cfg.Staging, "writers", n)
		os.Exit(1)
	}

	fs, target, counters, err := openVFS(cfg.Root, cfg.Registry, ternLogger)
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
	srv.MaxInFlightPerConn = cfg.MaxInFlight
	srv.SetReplyWorkBudget(cfg.ReplyWorkMiB << 20)
	srv.AllowMinorVersion2 = cfg.AllowMinor2
	srv.ReadOnly = cfg.ReadOnly
	srv.SetOwner(cfg.Owner, cfg.OwnerGroup)
	if cfg.ReadOnly {
		slogger.Info("read-only: every change to the filesystem is refused")
	}
	srv.NFSAddr = cfg.Addr
	if cfg.StatusAddr != "" {
		ln, err := net.Listen("tcp", cfg.StatusAddr)
		if err != nil {
			slogger.Error("diagnostics listen", "addr", cfg.StatusAddr, "err", err)
			os.Exit(1)
		}
		go func() {
			err := http.Serve(ln, inspect.Handler(srv))
			slogger.Error("diagnostics server stopped", "err", err)
		}()
		slogger.Info("diagnostics listening", "addr", ln.Addr().String())
	}
	if cfg.InfluxOrigin != "" {
		pusher := &metricsPusher{
			srv: srv, counters: counters,
			db:         &log.InfluxDB{Origin: cfg.InfluxOrigin, Org: cfg.InfluxOrg, Bucket: cfg.InfluxBucket},
			filesystem: cfg.Filesystem, instance: cfg.Instance, started: time.Now(), log: slogger,
		}
		go pusher.run(time.Duration(cfg.MetricsInterval), make(chan struct{}))
		slogger.Info("metrics enabled", "origin", cfg.InfluxOrigin, "filesystem", cfg.Filesystem, "instance", cfg.Instance, "interval", time.Duration(cfg.MetricsInterval))
	}
	slogger.Info("NFS server listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(cfg.Addr); err != nil {
		slogger.Error("server error", "err", err)
		os.Exit(1)
	}
}

// openVFS opens the local directory root, or else the TernFS cluster of
// registry. It also returns a description of what it opened,
// with a local root resolved to an absolute path, and for a TernFS backend
// the client's request counters for the metrics exporter.
func openVFS(root string, registry string, ternLogger *log.Logger) (vfs.TernVFS, string, *client.ClientCounters, error) {
	if root != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, "", nil, fmt.Errorf("resolving root path: %w", err)
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			return nil, "", nil, fmt.Errorf("root must be a directory: %s", absRoot)
		}
		if err := vfs.CheckLocalRoot(absRoot); err != nil {
			return nil, "", nil, err
		}
		return vfs.NewLocalTernVFS(absRoot), "local root " + absRoot, nil, nil
	}
	c, err := client.NewClient(ternLogger, nil, registry, msgs.AddrsInfo{})
	if err != nil {
		return nil, "", nil, fmt.Errorf("connecting to TernFS registry: %w", err)
	}
	counters := client.NewClientCounters()
	c.SetCounters(counters)
	return vfs.NewRemoteTernVFS(c, ternLogger, bufpool.NewBufPool()),
		"TernFS registry " + registry, counters, nil
}
