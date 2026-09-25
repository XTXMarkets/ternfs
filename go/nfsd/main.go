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
	"strings"

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
	maxInFlight := flag.Int("max-in-flight-per-connection", defaultMaxInFlightPerConn,
		"maximum requests admitted per TCP connection")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options]\n       %s inspect [options]\n\nOptions:\n",
			os.Args[0], os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(),
			"\nRun %s inspect -h for the client store inspector.\n", os.Args[0])
	}
	flag.Parse()
	if *maxInFlight <= 0 {
		fmt.Fprintln(os.Stderr, "-max-in-flight-per-connection must be positive")
		os.Exit(2)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Set up local state before dialling the cluster, so that a bad -staging
	// path or an unusable -root fails without a round trip to the registry.
	var ss StagingStore
	if *staging != "" {
		store, err := NewLocalStagingStore(*staging, slogger)
		if err != nil {
			slogger.Error("creating staging store", "err", err)
			os.Exit(1)
		}
		ss = store
		slogger.Info("staging directory configured", "path", *staging)
	} else {
		ss = readOnlyStagingStore{}
		slogger.Info("no staging directory — read-only mode")
	}

	fs, target, err := openVFS(*root, *registry, *verbose)
	if err != nil {
		slogger.Error("opening filesystem", "err", err)
		os.Exit(1)
	}
	slogger.Info("filesystem opened", "target", target)

	srv, err := NewServer(fs, ss, slogger)
	if err != nil {
		slogger.Error("creating server", "err", err)
		os.Exit(1)
	}
	srv.maxInFlightPerConn = *maxInFlight
	slogger.Info("NFS server listening", "addr", *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		slogger.Error("server error", "err", err)
		os.Exit(1)
	}
}

// openVFS opens the local directory or the TernFS cluster named by exactly
// one of root and registry. It also returns a description of what it opened,
// with a local root resolved to an absolute path.
func openVFS(root string, registry string, verbose bool) (TernVFS, string, error) {
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
		return NewLocalTernVFS(absRoot), "local root " + absRoot, nil
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
	return NewRemoteTernVFS(c, ternLogger, bufpool.NewBufPool()),
		"TernFS registry " + registry, nil
}

// Exit statuses of "nfsd inspect". A report which found problems is still a
// successful report, so it gets its own status rather than reusing the one
// that means the inspector could not run.
const (
	inspectExitUsage    = 2
	inspectExitProblems = 3
)

// runInspect implements "nfsd inspect", a read-only report of the persistent
// client store.
func runInspect(args []string) {
	fs := flag.NewFlagSet("nfsd inspect", flag.ExitOnError)
	root := fs.String("root", "", "local root directory (for testing)")
	registry := fs.String("registry", "", "TernFS registry address")
	identity := fs.String("identity", "",
		"the report's `identity` field: the raw SETCLIENTID identity,\n"+
			"for example \"Linux NFSv4.0 host/10.0.0.1\"")
	identityHash := fs.String("identity-hash", "",
		"the report's `identity-hash` field: 64 hex digits naming the\n"+
			"identity directory (SHA-256 of the identity)")
	clientID := fs.String("clientid", "",
		"the report's `clientid` field: the value returned to the client,\n"+
			"0x-prefixed hex or decimal. Not the directory inode")
	stateID := fs.String("stateid", "",
		"the report's `stateid` field: the 12-byte \"other\" field as\n"+
			"24 hex digits")
	staging := fs.String("staging", "",
		"local staging directory; report file integrity and join opens")
	jsonOut := fs.Bool("json", false, "write the report as JSON")
	verbose := fs.Bool("v", false, "verbose TernFS client logging")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(),
			"Usage: %s inspect (-root DIR | -registry ADDR) [filter] [-staging DIR] [-json]\n\n",
			os.Args[0])
		fmt.Fprintf(fs.Output(),
			"Reports every client identity in /%s/clients, or the one selected by\n"+
				"-identity, -identity-hash, -clientid or -stateid. It never modifies the\n"+
				"store. Each field the report prints is labelled with the flag which\n"+
				"selects it, so any value in the output can be pasted back here.\n\n",
			nfsDirName)
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(),
			"\nExit status: 0 if the report found no problems, %d if it found any,\n"+
				"%d for a usage error, 1 if the report could not be produced. Expired\n"+
				"leases, stale opens and unreachable incarnations are ordinary states\n"+
				"and are not problems.\n",
			inspectExitProblems, inspectExitUsage)
	}
	fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", fs.Args())
		os.Exit(inspectExitUsage)
	}

	opts, err := parseInspectOptions(*identity, *identityHash, *clientID, *stateID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(inspectExitUsage)
	}
	opts.StagingDir = *staging

	vfs, _, err := openVFS(*root, *registry, *verbose)
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
	} else {
		report.WriteText(os.Stdout)
	}
	if report.Summary.Problems > 0 {
		os.Exit(inspectExitProblems)
	}
}

func parseInspectOptions(identity, identityHash, clientID, stateID string) (
	inspectOptions, error,
) {
	var opts inspectOptions
	filters := 0
	if identity != "" {
		opts.Identity = []byte(identity)
		filters++
	}
	if identityHash != "" {
		raw, err := hex.DecodeString(identityHash)
		if err != nil || len(raw) != 32 {
			return opts, fmt.Errorf(
				"invalid -identity-hash %q: want 64 hex digits", identityHash)
		}
		opts.IdentityHash = strings.ToLower(identityHash)
		filters++
	}
	if clientID != "" {
		id, err := strconv.ParseUint(clientID, 0, 64)
		if err != nil || !isDirectoryInodeID(InodeID(id)) {
			return opts, fmt.Errorf(
				"invalid -clientid %q: want a 0x-prefixed hex or decimal "+
					"directory inode", clientID)
		}
		opts.ClientID = id
		filters++
	}
	if stateID != "" {
		raw, err := hex.DecodeString(stateID)
		if err != nil || len(raw) != len(StateID{}) {
			return opts, fmt.Errorf("invalid -stateid %q: want %d hex digits",
				stateID, 2*len(StateID{}))
		}
		var sid StateID
		copy(sid[:], raw)
		opts.StateID = &sid
		filters++
	}
	if filters > 1 {
		return opts, fmt.Errorf(
			"use at most one of -identity, -identity-hash, -clientid and -stateid")
	}
	return opts, nil
}
