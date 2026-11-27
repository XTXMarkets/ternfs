// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/server"
)

// usage is nfsd's command line: every setting is in the config file.
const usage = "usage: nfsd -config FILE"

// config holds every setting of the daemon, read from the JSON file named
// by -config.
type config struct {
	// Addr is the NFS listen address.
	Addr string `json:"addr"`
	// Root is a local directory to export, for testing. Exactly one of
	// Root and Registry is set.
	Root string `json:"root"`
	// Registry is the address of the TernFS registry to export.
	Registry string `json:"registry"`
	// Staging is the staging directory: writes, sessions and client
	// records. Required.
	Staging string `json:"staging"`
	// Verbose logs NFS requests and responses.
	Verbose bool `json:"verbose"`
	// ReadOnly refuses every change to the filesystem; staging then holds
	// only sessions and client records.
	ReadOnly bool `json:"read_only"`
	// Owner and OwnerGroup are the owner and group every file reports:
	// user@domain or group@domain, with the clients' idmap domain, or a
	// numeric id.
	Owner      string `json:"owner"`
	OwnerGroup string `json:"owner_group"`
	// AllowMinor2 also accepts NFSv4.2 COMPOUNDs, served as NFSv4.1 with
	// every 4.2 operation unsupported.
	AllowMinor2 bool `json:"allow_minor_version_2"`
	// ReplyWorkMiB is the memory reserved for replies in flight across all
	// connections, in MiB; requests beyond it wait.
	ReplyWorkMiB int `json:"reply_work_budget_mib"`
	// MaxInFlight is the most requests admitted per TCP connection.
	MaxInFlight int `json:"max_in_flight_per_connection"`
	// InfluxOrigin is the base URL of the metrics store accepting InfluxDB
	// line protocol; empty disables metrics. InfluxOrg and InfluxBucket
	// are the org and bucket.
	InfluxOrigin string `json:"influx_db_origin"`
	InfluxOrg    string `json:"influx_db_org"`
	InfluxBucket string `json:"influx_db_bucket"`
	// Filesystem names the served filesystem, the filesystem tag on every
	// metric; required with InfluxOrigin.
	Filesystem string `json:"filesystem"`
	// Instance is the instance tag on every metric, and the xmon app
	// instance; the short hostname and NFS port if empty.
	Instance string `json:"instance"`
	// MetricsInterval is how often metrics are pushed.
	MetricsInterval duration `json:"metrics_interval"`
	// StatusAddr serves the read-only diagnostics API (JSON over HTTP);
	// off if empty.
	StatusAddr string `json:"status_addr"`
	// Xmon is the xmon address; none if empty.
	Xmon string `json:"xmon"`
	// XmonAppType picks the xmon rota and schedule.
	XmonAppType string `json:"xmon_app_type"`
}

// duration is a time.Duration written in JSON as a Go duration string,
// such as "30s".
type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if v, err := time.ParseDuration(s); err == nil {
			*d = duration(v)
			return nil
		}
	}
	// The decoder names the key only in an UnmarshalTypeError.
	return &json.UnmarshalTypeError{Value: string(b), Type: reflect.TypeFor[time.Duration]()}
}

func defaultConfig() config {
	return config{
		Addr:            ":2049",
		Owner:           "nobody",
		OwnerGroup:      "nogroup",
		ReplyWorkMiB:    server.DefaultReplyWorkBudgetMiB,
		MaxInFlight:     server.DefaultMaxInFlightPerConn,
		MetricsInterval: duration(defaultMetricsInterval),
		XmonAppType:     string(log.XMON_DAYTIME),
	}
}

// parseConfig parses args, which hold only -config FILE, and returns the
// defaults overridden by the file.
func parseConfig(fs *flag.FlagSet, args []string) (config, error) {
	path := fs.String("config", "", "JSON file holding the settings; see README.md")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), usage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	// Rejecting a mistyped command line here, before any directory or
	// listener is touched, keeps it from starting a server.
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("unexpected arguments: %v\n%s", fs.Args(), usage)
	}
	if *path == "" {
		return config{}, errors.New(usage)
	}
	cfg := defaultConfig()
	if err := cfg.load(*path); err != nil {
		return cfg, err
	}
	return cfg, cfg.validate(*path)
}

// load sets the settings the JSON file at path names. An unknown key is an
// error.
func (c *config) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("config file %s: data after the top-level object", path)
	}
	return nil
}

// validate rejects settings the server cannot run with, naming the key and
// the config file at path.
func (c *config) validate(path string) error {
	errorf := func(format string, args ...any) error {
		return fmt.Errorf("config file %s: %s", path, fmt.Sprintf(format, args...))
	}
	switch {
	case c.Staging == "":
		return errorf("staging is required")
	case (c.Root == "") == (c.Registry == ""):
		return errorf("exactly one of root (a local directory) and registry (TernFS) is required")
	case c.ReplyWorkMiB <= 0:
		return errorf("reply_work_budget_mib must be positive")
	case c.MaxInFlight <= 0:
		return errorf("max_in_flight_per_connection must be positive")
	case c.Owner == "":
		return errorf("owner must not be empty")
	case c.OwnerGroup == "":
		return errorf("owner_group must not be empty")
	case c.InfluxOrigin != "" && c.Filesystem == "":
		return errorf("filesystem is required with influx_db_origin")
	}
	switch log.XmonAppType(c.XmonAppType) {
	case log.XMON_NEVER, log.XMON_DAYTIME, log.XMON_CRITICAL:
	default:
		return errorf("xmon_app_type %q is not %s, %s or %s",
			c.XmonAppType, log.XMON_NEVER, log.XMON_DAYTIME, log.XMON_CRITICAL)
	}
	return nil
}
