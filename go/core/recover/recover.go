// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package recover

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"xtx/ternfs/core/log"
)

var stacktraceLock sync.Mutex

func HandleRecoverChan(log *log.Logger, terminateChan chan any, err any) {
	if err != nil {
		log.RaiseAlert(fmt.Sprintf("caught stray error: %v", err))
		stacktraceLock.Lock()
		fmt.Fprintf(os.Stderr, "PANIC %v. Stacktrace:\n", err)
		for _, line := range strings.Split(string(debug.Stack()), "\n") {
			fmt.Fprintln(os.Stderr, line)
		}
		stacktraceLock.Unlock()
		terminateChan <- err
	}
}

func HandleRecoverPanic(log *log.Logger, err any) {
	if err != nil {
		log.RaiseAlert(fmt.Sprintf("caught stray error: %v", err))
		stacktraceLock.Lock()
		fmt.Fprintf(os.Stderr, "PANIC %v. Stacktrace:\n", err)
		for _, line := range strings.Split(string(debug.Stack()), "\n") {
			fmt.Fprintln(os.Stderr, line)
		}
		stacktraceLock.Unlock()
		panic(err)
	}
}
