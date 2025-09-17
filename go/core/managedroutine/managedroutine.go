// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package managedroutine

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

type ManagedRoutines struct {
	panicChan chan<- error
	logMutex  sync.Mutex
}

func New(panicChan chan<- error) *ManagedRoutines {
	return &ManagedRoutines{
		panicChan: panicChan,
	}
}

func (mr *ManagedRoutines) Start(name string, body func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				mr.logMutex.Lock()
				fmt.Fprintf(os.Stderr, "%s: PANIC %v. Stacktrace:", name, err)
				for _, line := range strings.Split(string(debug.Stack()), "\n") {
					fmt.Fprintf(os.Stderr, "%s: %s", name, line)
				}
				mr.logMutex.Unlock()
				mr.panicChan <- fmt.Errorf("%s: PANIC %v", name, err)
			}
		}()
		body()
	}()
}
