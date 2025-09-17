// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/wyhash"
)

type largeFileTestOpts struct {
	fileSize uint64
}

func largeFileTest(
	log *log.Logger,
	opts *largeFileTestOpts,
	mountPoint string,
) {
	// create single 1GiB file
	log.Info("creating %vGB file", float64(opts.fileSize)/1e9)
	fpath := path.Join(mountPoint, "big")
	f, err := os.Create(fpath)
	defer f.Close()
	if err != nil {
		panic(err)
	}
	rand := wyhash.New(0)
	buf := make([]byte, 1<<20)
	expectedSha := sha256.New()
	written := uint64(0)
	for written < opts.fileSize {
		rand.Read(buf)
		expectedSha.Write(buf)
		if _, err := f.Write(buf); err != nil {
			panic(err)
		}
		written += uint64(len(buf))
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	log.Info("expected sha: %x", expectedSha.Sum(nil))
	// first using our own reading
	f, err = os.Open(fpath)
	if err != nil {
		panic(err)
	}
	actualSha := sha256.New()
	for {
		read, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		actualSha.Write(buf[:read])
	}
	log.Info("actual sha: %x", actualSha.Sum(nil))
	if !bytes.Equal(expectedSha.Sum(nil), actualSha.Sum(nil)) {
		panic(fmt.Errorf("expected sha %v, got %v", expectedSha.Sum(nil), actualSha.Sum(nil)))
	}
	// Purposefully using an external program to have somebody else issue syscalls.
	// This one is currently interesting because sha256sum looks ahead in unpredicatble
	// ways, which means that it stimulates the "unhappy" reading path in interesting ways.
	// That said, it' be much better to just stimulate it with our code, given that we
	// have no control on how `sha256sum` works.
	sha256Out, err := exec.Command("sha256sum", fpath).Output()
	if err != nil {
		panic(err)
	}
	log.Info("sha256 out: %s", strings.TrimSpace(string(sha256Out)))
	expectedSha256Out := fmt.Sprintf("%x  %s\n", expectedSha.Sum(nil), fpath)
	if string(sha256Out) != expectedSha256Out {
		panic(fmt.Errorf("expected sha256 output %q, got %q", expectedSha256Out, string(sha256Out)))
	}
}
