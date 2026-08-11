// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build ternnfs && pynfs

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type pynfsCaseTiming struct {
	code     string
	name     string
	duration time.Duration
}

func TestPynfs(t *testing.T) {
	source := os.Getenv("PYNFS_SOURCE")
	if source == "" {
		source = filepath.Join(".deps", "pynfs")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}

	python := os.Getenv("PYNFS_PYTHON")
	if python == "" {
		python = "python3"
	}

	testServer := filepath.Join(source, "nfs4.0", "testserver.py")
	if _, err := os.Stat(testServer); err != nil {
		t.Fatalf("pynfs runner not found: %v; run make fetch-pynfs", err)
	}

	addr, cleanup := startTernTestServer(t)
	defer cleanup()

	resultsPath := filepath.Join(t.TempDir(), "pynfs-results.json")
	args := []string{
		"-u",
		testServer,
		fmt.Sprintf("nfs://%s/", addr),
		"--maketree",
		"--rundeps",
		"--verbose",
		"--jsonout", resultsPath,
	}
	args = append(args, strings.Fields(os.Getenv("PYNFS_ARGS"))...)

	selectors := strings.Fields(os.Getenv("PYNFS_TESTS"))
	if len(selectors) == 0 {
		selectors = []string{"all"}
	}
	args = append(args, selectors...)

	cmd := exec.Command(python, args...)
	cmd.Dir = filepath.Join(source, "nfs4.0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pynfs runner failed: %v", err)
	}

	data, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("read pynfs results: %v", err)
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
		t.Fatalf("decode pynfs results: %v", err)
	}
	var slowTests []pynfsCaseTiming
	for _, testCase := range results.Testcase {
		seconds, err := strconv.ParseFloat(testCase.Time, 64)
		if err != nil {
			t.Fatalf("decode duration for pynfs test %s: %v", testCase.Code, err)
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
		t.Logf("slow pynfs test: code=%s duration=%s name=%s",
			testCase.code, testCase.duration.Round(time.Millisecond), testCase.name)
	}
	t.Logf("pynfs results: tests=%d failures=%d errors=%d skipped=%d",
		results.Tests, results.Failures, results.Errors, results.Skipped)
	if results.Failures != 0 || results.Errors != 0 {
		t.Fatalf("pynfs reported %d failures and %d errors",
			results.Failures, results.Errors)
	}
}
