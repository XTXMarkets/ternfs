// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "inspect" {
		runInspect(os.Args[2:])
		return
	}

	addr := flag.String("addr", ":2049", "listen address")
	root := flag.String("root", "", "local root directory to export (for testing)")
	registry := flag.String("registry", "", "TernFS registry address (for production)")
	staging := flag.String("staging", "", "staging directory for writes (omit for read-only)")
	verbose := flag.Bool("v", false, "verbose logging of NFS requests/responses")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options]\n       %s inspect [options]\n\nOptions:\n",
			os.Args[0], os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(),
			"\nRun %s inspect -h for the client store inspector.\n", os.Args[0])
	}
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

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

	fs, err := openVFS(*root, *registry, *verbose)
	if err != nil {
		slogger.Error("opening filesystem", "err", err)
		os.Exit(1)
	}
	if *root != "" {
		slogger.Info("local VFS mode", "root", *root)
	} else {
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

// openVFS opens the local directory or the TernFS cluster named by exactly
// one of root and registry.
func openVFS(root string, registry string, verbose bool) (TernVFS, error) {
	if (root == "") == (registry == "") {
		return nil, fmt.Errorf(
			"exactly one of -root (local) or -registry (TernFS) must be specified")
	}
	if root != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolving root path: %w", err)
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("root must be a directory: %s", absRoot)
		}
		return NewLocalTernVFS(absRoot), nil
	}
	logLevel := log.INFO
	if verbose {
		logLevel = log.DEBUG
	}
	ternLogger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: logLevel})
	c, err := client.NewClient(ternLogger, nil, registry, msgs.AddrsInfo{})
	if err != nil {
		return nil, fmt.Errorf("connecting to TernFS registry: %w", err)
	}
	return NewRemoteTernVFS(c, ternLogger, bufpool.NewBufPool()), nil
}

// runInspect implements "nfsd inspect", a read-only report of the persistent
// client store.
func runInspect(args []string) {
	fs := flag.NewFlagSet("nfsd inspect", flag.ExitOnError)
	root := fs.String("root", "", "local root directory (for testing)")
	registry := fs.String("registry", "", "TernFS registry address")
	identity := fs.String("identity", "",
		"raw SETCLIENTID identity, for example \"Linux NFSv4.0 host/10.0.0.1\"")
	identityHash := fs.String("identity-hash", "",
		"hexadecimal identity directory name (SHA-256 of the identity)")
	clientID := fs.String("clientid", "",
		"clientid as returned to the client, 0x-prefixed hex or decimal")
	stateID := fs.String("stateid", "",
		"stateid \"other\" field as 24 hex digits")
	staging := fs.String("staging", "",
		"local staging directory; its sidecars are joined to open markers")
	jsonOut := fs.Bool("json", false, "write the report as JSON")
	verbose := fs.Bool("v", false, "verbose TernFS client logging")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(),
			"Usage: %s inspect (-root DIR | -registry ADDR) [filter] [-staging DIR] [-json]\n\n",
			os.Args[0])
		fmt.Fprintf(fs.Output(),
			"Reports every client identity in /%s/clients, or the one selected by\n"+
				"-identity, -identity-hash, -clientid or -stateid. It never modifies the store.\n\n",
			nfsDirName)
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", fs.Args())
		os.Exit(2)
	}

	var opts inspectOptions
	filters := 0
	if *identity != "" {
		opts.Identity = []byte(*identity)
		filters++
	}
	if *identityHash != "" {
		opts.IdentityHash = *identityHash
		filters++
	}
	if *clientID != "" {
		id, err := strconv.ParseUint(*clientID, 0, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -clientid %q: %v\n", *clientID, err)
			os.Exit(2)
		}
		opts.ClientID = id
		filters++
	}
	if *stateID != "" {
		raw, err := hex.DecodeString(*stateID)
		if err != nil || len(raw) != len(StateID{}) {
			fmt.Fprintf(os.Stderr, "invalid -stateid %q: want %d hex digits\n",
				*stateID, 2*len(StateID{}))
			os.Exit(2)
		}
		var sid StateID
		copy(sid[:], raw)
		opts.StateID = &sid
		filters++
	}
	if filters > 1 {
		fmt.Fprintf(os.Stderr, "use at most one of -identity, -identity-hash, -clientid and -stateid\n")
		os.Exit(2)
	}
	opts.StagingDir = *staging

	vfs, err := openVFS(*root, *registry, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	store, err := openClientStoreReader(vfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	report, err := store.Inspect(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		if err := report.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}
	report.WriteText(os.Stdout)
}
