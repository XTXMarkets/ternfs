// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"syscall"
	"time"

	"xtx/ternfs/core/log"
)

func nfsMutationTest(l *log.Logger, mnt string) {
	dir := path.Join(mnt, "nfs-mutations")
	if err := os.Mkdir(dir, 0755); err != nil {
		panic(fmt.Errorf("mkdir %v: %w", dir, err))
	}

	nfsCreateWriteReadback(l, dir)
	nfsOutOfOrderWrites(l, dir)
	nfsRename(l, dir)
	nfsDelete(l, dir)
	nfsSetTimes(l, dir)
	nfsRejectInPlaceModify(l, dir)
}

func nfsOutOfOrderWrites(l *log.Logger, dir string) {
	l.Info("nfs mutation: out-of-order writes")
	p := path.Join(dir, "ooo.bin")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Errorf("open %v: %w", p, err))
	}

	want := make([]byte, 3000)
	for i := range want {
		want[i] = byte('a' + i%26)
	}

	if _, err := f.WriteAt(want[2000:3000], 2000); err != nil {
		panic(fmt.Errorf("write tail: %w", err))
	}
	if _, err := f.WriteAt(want[0:1000], 0); err != nil {
		panic(fmt.Errorf("write head: %w", err))
	}
	if _, err := f.WriteAt(want[1000:2000], 1000); err != nil {
		panic(fmt.Errorf("write middle: %w", err))
	}
	if err := f.Close(); err != nil {
		panic(fmt.Errorf("close %v: %w", p, err))
	}

	got, err := os.ReadFile(p)
	if err != nil {
		panic(fmt.Errorf("read %v: %w", p, err))
	}
	if !bytes.Equal(got, want) {
		panic(fmt.Errorf("out-of-order readback mismatch: got %d bytes, want %d", len(got), len(want)))
	}
}

func nfsCreateWriteReadback(l *log.Logger, dir string) {
	l.Info("nfs mutation: create+write+readback")
	p := path.Join(dir, "created.txt")
	want := []byte("hello from nfs\n")
	if err := os.WriteFile(p, want, 0644); err != nil {
		panic(fmt.Errorf("write %v: %w", p, err))
	}
	got, err := os.ReadFile(p)
	if err != nil {
		panic(fmt.Errorf("read %v: %w", p, err))
	}
	if !bytes.Equal(got, want) {
		panic(fmt.Errorf("readback mismatch: got %q want %q", got, want))
	}
}

func nfsRename(l *log.Logger, dir string) {
	l.Info("nfs mutation: rename")
	src := path.Join(dir, "rename-src.txt")
	dst := path.Join(dir, "rename-dst.txt")
	want := []byte("rename me")
	if err := os.WriteFile(src, want, 0644); err != nil {
		panic(fmt.Errorf("write %v: %w", src, err))
	}
	if err := os.Rename(src, dst); err != nil {
		panic(fmt.Errorf("rename %v -> %v: %w", src, dst, err))
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		panic(fmt.Errorf("source still present after rename, stat err = %v", err))
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		panic(fmt.Errorf("read %v: %w", dst, err))
	}
	if !bytes.Equal(got, want) {
		panic(fmt.Errorf("renamed content mismatch: got %q want %q", got, want))
	}
}

func nfsDelete(l *log.Logger, dir string) {
	l.Info("nfs mutation: delete")
	p := path.Join(dir, "delete-me.txt")
	if err := os.WriteFile(p, []byte("transient"), 0644); err != nil {
		panic(fmt.Errorf("write %v: %w", p, err))
	}
	if err := os.Remove(p); err != nil {
		panic(fmt.Errorf("remove %v: %w", p, err))
	}
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		panic(fmt.Errorf("file still present after delete, stat err = %v", err))
	}
}

func nfsSetTimes(l *log.Logger, dir string) {
	l.Info("nfs mutation: setattr times")
	p := path.Join(dir, "times.txt")
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		panic(fmt.Errorf("write %v: %w", p, err))
	}
	mtime := time.Unix(1_600_000_000, 0)
	atime := time.Unix(1_600_000_500, 0)
	if err := os.Chtimes(p, atime, mtime); err != nil {
		panic(fmt.Errorf("chtimes %v: %w", p, err))
	}
	info, err := os.Stat(p)
	if err != nil {
		panic(fmt.Errorf("stat %v: %w", p, err))
	}
	if !info.ModTime().Equal(mtime) {
		panic(fmt.Errorf("mtime not set: got %v want %v", info.ModTime(), mtime))
	}
}

func nfsRejectInPlaceModify(l *log.Logger, dir string) {
	l.Info("nfs mutation: reject in-place modify")
	p := path.Join(dir, "immutable.txt")
	if err := os.WriteFile(p, []byte("original contents"), 0644); err != nil {
		panic(fmt.Errorf("write %v: %w", p, err))
	}

	f, err := os.OpenFile(p, os.O_WRONLY, 0644)
	if err != nil {
		if isPermErr(err) {
			return // rejected at open, as expected
		}
		panic(fmt.Errorf("unexpected open error for %v: %w", p, err))
	}
	_, werr := f.Write([]byte("OVERWRITTEN"))
	cerr := f.Close()
	if werr == nil && cerr == nil {
		panic(fmt.Errorf("in-place modify of %v unexpectedly succeeded", p))
	}
	if werr != nil && !isPermErr(werr) {
		panic(fmt.Errorf("unexpected write error for %v: %w", p, werr))
	}
	if werr == nil && cerr != nil && !isPermErr(cerr) {
		panic(fmt.Errorf("unexpected close error for %v: %w", p, cerr))
	}
}

func isPermErr(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)
}
