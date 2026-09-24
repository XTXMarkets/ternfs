// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/test/internal/harness"
)

type pynfsCaseTiming struct {
	code     string
	name     string
	duration time.Duration
}

var pynfsCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*[a-z]?$`)

const defaultPynfsSkipFile = "pynfs/pynfs_unsupported.txt"

func pynfsSkipSelectors(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var selectors []string
	seen := make(map[string]int)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !pynfsCodePattern.MatchString(line) {
			return nil, fmt.Errorf("%s:%d: invalid pynfs test code %q",
				path, lineNumber, line)
		}
		if previousLine, ok := seen[line]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate pynfs test code %q (first on line %d)",
				path, lineNumber, line, previousLine)
		}
		seen[line] = lineNumber
		selectors = append(selectors, "no"+line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return selectors, nil
}

func main() {
	var opts harness.Options
	opts.Flags(flag.CommandLine)
	short := flag.Bool("short", false, "skip pynfs timed cases")
	timeout := flag.Duration("timeout", time.Hour, "overall run timeout")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if err := run(ctx, opts, *short); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts harness.Options, short bool) (err error) {
	source := os.Getenv("PYNFS_SOURCE")
	if source == "" {
		source = filepath.Join(".deps", "pynfs")
	}
	source = harness.SourcePath(source)

	python := os.Getenv("PYNFS_PYTHON")
	if python == "" {
		python = "python3"
	}

	testServer := filepath.Join(source, "nfs4.0", "testserver.py")
	if _, err := os.Stat(testServer); err != nil {
		return fmt.Errorf("pynfs runner not found: %w; run make fetch-pynfs", err)
	}

	suite, err := harness.New(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, suite.Close(err == nil)) }()
	server, err := suite.Server(ctx)
	if err != nil {
		return err
	}

	resultsPath := filepath.Join(server.Dir, "pynfs-results.json")
	args := []string{
		"-u",
		testServer,
		fmt.Sprintf("nfs://%s/%s", server.Addr, server.Name),
		"--maketree",
		"--rundeps",
		"--verbose",
	}
	args = append(args, strings.Fields(os.Getenv("PYNFS_ARGS"))...)

	selectors := strings.Fields(os.Getenv("PYNFS_TESTS"))
	if len(selectors) == 0 {
		selectors = []string{"all", "noblock", "nochar", "nofifo", "nosocket", "nogss", "noacl", "nomode000"}
	}
	skipFile, configured := os.LookupEnv("PYNFS_SKIP_FILE")
	if !configured {
		skipFile = defaultPynfsSkipFile
	}
	if skipFile != "" {
		skipFile = harness.SourcePath(skipFile)
	}
	skipSelectors, err := pynfsSkipSelectors(skipFile)
	if err != nil {
		return fmt.Errorf("read pynfs unsupported-test manifest: %w", err)
	}
	selectors = append(selectors, skipSelectors...)
	if short {
		selectors = append(selectors, "notimed")
	}
	// Keep failure evidence; the shared fixture removes this tree on success.
	args = append(args, "--nocleanup", "--jsonout", resultsPath)
	args = append(args, selectors...)

	cmd := exec.Command(python, args...)
	cmd.Dir = filepath.Join(source, "nfs4.0")
	console, err := os.Create(filepath.Join(server.Dir, "pynfs.out"))
	if err != nil {
		return err
	}
	defer console.Close()
	cmd.Stdout = io.MultiWriter(os.Stdout, console)
	runner, err := harness.StartProcess(cmd, filepath.Join(server.Dir, "pynfs-stderr.log"))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, runner.Stop(syscall.SIGTERM)) }()
	if err := runner.Wait(ctx); err != nil {
		return fmt.Errorf("pynfs runner failed: %w", err)
	}

	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("read pynfs results: %w", err)
	}
	var results struct {
		Tests    int `json:"tests"`
		Errors   int `json:"errors"`
		Failures int `json:"failures"`
		Skipped  int `json:"skipped"`
		Testcase []struct {
			Code string `json:"code"`
			Name string `json:"name"`
			Time string `json:"time"`
		} `json:"testcase"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("decode pynfs results: %w", err)
	}
	var slowTests []pynfsCaseTiming
	for _, testCase := range results.Testcase {
		seconds, err := strconv.ParseFloat(testCase.Time, 64)
		if err != nil {
			return fmt.Errorf("decode duration for pynfs test %s: %w", testCase.Code, err)
		}
		duration := time.Duration(seconds * float64(time.Second))
		if duration >= time.Second {
			slowTests = append(slowTests, pynfsCaseTiming{
				code:     testCase.Code,
				name:     testCase.Name,
				duration: duration,
			})
		}
	}
	sort.Slice(slowTests, func(i, j int) bool {
		return slowTests[i].duration > slowTests[j].duration
	})
	for _, testCase := range slowTests {
		fmt.Printf("slow pynfs test: code=%s duration=%s name=%s\n",
			testCase.code, testCase.duration.Round(time.Millisecond), testCase.name)
	}
	fmt.Printf("pynfs results: tests=%d failures=%d errors=%d skipped=%d\n",
		results.Tests, results.Failures, results.Errors, results.Skipped)
	if results.Tests <= results.Skipped {
		return fmt.Errorf("pynfs ran no tests")
	}
	if results.Failures != 0 || results.Errors != 0 {
		return fmt.Errorf("pynfs reported %d failures and %d errors",
			results.Failures, results.Errors)
	}
	return nil
}
