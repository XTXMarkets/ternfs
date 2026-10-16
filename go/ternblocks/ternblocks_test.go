// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"os"
	"testing"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestInitBlockServicesInfoReturnsStorageReadErrors(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "ternblocks-test-log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	logger := log.NewLogger(logFile, &log.LoggerOptions{Level: log.ERROR})
	missingPath := t.TempDir() + "/missing"
	blockServices := map[msgs.BlockServiceId]*blockService{
		1: {path: missingPath},
	}

	err = initBlockServicesInfo(
		&env{},
		logger,
		0,
		msgs.AddrsInfo{},
		[16]byte{},
		blockServices,
		0,
		0,
	)
	if err == nil {
		t.Fatal("initBlockServicesInfo() succeeded for a missing storage path")
	}
}
