// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"xtx/ternfs/core/log"
)

type preadddirOpts struct {
	filesPerDir int
	loops       int
	threads     int
	numDirs     int
}

func preaddirCheck(
	log *log.Logger,
	thread int,
	mountPoint string,
	opts *preadddirOpts,
) []string {
	errors := []string{}
	for i := 0; i < opts.numDirs; i++ {
		dpath := path.Join(mountPoint, strconv.Itoa(i))
		entries, err := os.ReadDir(dpath)
		if err != nil {
			panic(err)
		}
		found := make([]bool, opts.filesPerDir)
		for _, entry := range entries {
			i, err := strconv.Atoi(entry.Name())
			if err != nil {
				panic(err)
			}
			found[i] = true
		}
		for i, b := range found {
			if !b {
				err := fmt.Sprintf("[%v] %v/%v", thread, dpath, i)
				log.RaiseAlert(err)
				errors = append(errors, err)
			}
		}
	}
	return errors
}

func preaddirTest(
	log *log.Logger,
	mountPoint string,
	opts *preadddirOpts,
) {
	for i := 0; i < opts.numDirs; i++ {
		dpath := path.Join(mountPoint, strconv.Itoa(i))
		log.Debug("creating directory %v", dpath)
		if err := os.Mkdir(dpath, 0777); err != nil {
			panic(err)
		}
		for i := 0; i < opts.filesPerDir; i++ {
			fpath := path.Join(dpath, strconv.Itoa(i))
			log.Debug("creating file %v", fpath)
			f, err := os.Create(fpath)
			if err != nil {
				panic(err)
			}
			if err := f.Close(); err != nil {
				panic(err)
			}
		}
	}
	errsChans := make([](chan any), opts.threads)
	errors := make([][]string, opts.threads)
	for i := 0; i < opts.threads; i++ {
		errsChans[i] = make(chan any)
		go func(thread int) {
			defer func() { errsChans[thread] <- recover() }()
			errors[thread] = preaddirCheck(log, thread, mountPoint, opts)
		}(i)
	}
	log.Info("waiting for threads")
	for i := 0; i < opts.threads; i++ {
		log.Debug("reading from %v", i)
		err := <-errsChans[i]
		if err != nil {
			panic(fmt.Errorf("checking thread %v failed: %v", i, err))
		}
	}
	log.Info("checking errors")
	var flatErrors string
	for i := range errors {
		for j := range errors[i] {
			if flatErrors == "" {
				flatErrors = errors[i][j]
			} else {
				flatErrors = flatErrors + ", " + errors[i][j]
			}
		}
	}
	if flatErrors != "" {
		panic(fmt.Sprintf("missing %s", flatErrors))
	}
}
