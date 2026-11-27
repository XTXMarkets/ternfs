// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func parseTestConfig(t *testing.T, args ...string) (config, error) {
	t.Helper()
	fs := flag.NewFlagSet("nfsd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return parseConfig(fs, args)
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nfsd.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Keys the file leaves out keep the defaults deployments rely on.
func TestConfigDefaults(t *testing.T) {
	cfg, err := parseTestConfig(t, "-config", writeConfigFile(t, `{"registry": "reg:10001", "staging": "/s"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := config{
		Addr:            ":2049",
		Registry:        "reg:10001",
		Staging:         "/s",
		Owner:           "nobody",
		OwnerGroup:      "nogroup",
		ReplyWorkMiB:    8192,
		MaxInFlight:     64,
		MetricsInterval: duration(10 * time.Second),
		XmonAppType:     "restech_eggsfs.daytime",
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestConfigFile(t *testing.T) {
	path := writeConfigFile(t, `{
		"registry": "reg:10001",
		"staging": "/s",
		"verbose": true,
		"read_only": true,
		"reply_work_budget_mib": 1024,
		"metrics_interval": "30s"
	}`)
	cfg, err := parseTestConfig(t, "-config", path)
	if err != nil {
		t.Fatal(err)
	}
	want := defaultConfig()
	want.Registry = "reg:10001"
	want.Staging = "/s"
	want.Verbose = true
	want.ReadOnly = true
	want.ReplyWorkMiB = 1024
	want.MetricsInterval = duration(30 * time.Second)
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

// The command line is -config FILE and nothing else.
func TestConfigCommandLine(t *testing.T) {
	path := writeConfigFile(t, `{"registry": "reg:10001", "staging": "/s"}`)
	for _, args := range [][]string{
		nil,
		{"-staging", "/s"},
		{"-config", path, "-addr", ":3049"},
		{"-config", path, "extra"},
	} {
		if _, err := parseTestConfig(t, args...); err == nil {
			t.Errorf("%q accepted", args)
		}
	}
}

// An error names the key and the config file.
func TestConfigFileRejected(t *testing.T) {
	for _, tc := range []struct {
		name, contents string
		want           string // PATH stands for the config file's path
	}{
		{"unknown key", `{"staging": "/s", "registry": "r", "v": true}`, `config file PATH: json: unknown field "v"`},
		{"wrong type", `{"reply_work_budget_mib": "1024"}`, "config file PATH: json: cannot unmarshal string into Go struct field config.reply_work_budget_mib"},
		{"bad duration", `{"metrics_interval": "30"}`, "config file PATH: json: cannot unmarshal \"30\" into Go struct field config.metrics_interval"},
		{"invalid json", `{"addr": ":2049",}`, "config file PATH: invalid character"},
		{"trailing data", `{} {}`, "config file PATH: data after the top-level object"},
		{"invalid value", `{"staging": "/s", "registry": "r", "max_in_flight_per_connection": 0}`,
			"config file PATH: max_in_flight_per_connection must be positive"},
		{"missing required key", `{"registry": "r"}`, "config file PATH: staging is required"},
		{"neither root nor registry", `{"staging": "/s"}`,
			"config file PATH: exactly one of root (a local directory) and registry (TernFS) is required"},
		{"both root and registry", `{"staging": "/s", "root": "/r", "registry": "r"}`,
			"config file PATH: exactly one of root (a local directory) and registry (TernFS) is required"},
		{"key another needs", `{"staging": "/s", "registry": "r", "influx_db_origin": "http://metrics"}`,
			"config file PATH: filesystem is required with influx_db_origin"},
		{"unknown xmon app type", `{"staging": "/s", "registry": "r", "xmon_app_type": "restech_eggsfs.sometimes"}`,
			`config file PATH: xmon_app_type "restech_eggsfs.sometimes" is not`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfigFile(t, tc.contents)
			_, err := parseTestConfig(t, "-config", path)
			want := strings.ReplaceAll(tc.want, "PATH", path)
			if err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("got error %v, want one starting %q", err, want)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "missing.json")
	if _, err := parseTestConfig(t, "-config", path); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("got error %v for a missing file", err)
	}
}
