// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package log

import (
	"os"
	"strings"
	"testing"
)

func TestLogNoSpaceFallback(t *testing.T) {
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	originalStderr := os.Stderr
	defer func() { os.Stderr = originalStderr }()
	os.Stderr = stderr

	logger := NewLogger(full, &LoggerOptions{StderrOnNoSpace: true})
	logger.Info("still serving reads")
	output, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "still serving reads") {
		t.Fatalf("missing fallback log: %q", output)
	}
	// Even when stderr shares the full filesystem, do not panic.
	os.Stderr = full
	logger.Info("stderr is full too")

	// Once the original destination recovers, resume logging there.
	logger.out = stderr
	logger.Info("space recovered")
	output, err = os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "space recovered") {
		t.Fatalf("logging did not recover: %q", output)
	}
}
