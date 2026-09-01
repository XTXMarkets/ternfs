// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build libnfs

package main

/*
#cgo CFLAGS: -I${SRCDIR}/.deps/libnfs-install/include
#cgo LDFLAGS: -L${SRCDIR}/.deps/libnfs-install/lib -lnfs -Wl,-rpath,${SRCDIR}/.deps/libnfs-install/lib
#include <stdlib.h>
#include <fcntl.h>
#include <nfsc/libnfs.h>

static int tern_nfs_mount_url(struct nfs_context *nfs, const char *url)
{
	struct nfs_url *parsed = nfs_parse_url_dir(nfs, url);
	int ret;

	if (parsed == NULL) {
		return -1;
	}
	ret = nfs_mount(nfs, parsed->server, parsed->path);
	nfs_destroy_url(parsed);
	return ret;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type libnfsClient struct {
	nfs *C.struct_nfs_context
}

func libnfsConnect(host string, port int) (*libnfsClient, error) {
	nfs := C.nfs_init_context()
	if nfs == nil {
		return nil, fmt.Errorf("nfs_init_context failed")
	}
	if ret := C.nfs_set_version(nfs, 4); ret != 0 {
		msg := C.GoString(C.nfs_get_error(nfs))
		C.nfs_destroy_context(nfs)
		return nil, fmt.Errorf("nfs_set_version: %s (ret=%d)", msg, ret)
	}
	C.nfs_set_timeout(nfs, 5000)
	C.nfs_set_debug(nfs, 2)

	mountURL := fmt.Sprintf("nfs://%s/?nfsport=%d", host, port)
	curl := C.CString(mountURL)
	defer C.free(unsafe.Pointer(curl))

	ret := C.tern_nfs_mount_url(nfs, curl)
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(nfs))
		C.nfs_destroy_context(nfs)
		return nil, fmt.Errorf("nfs_mount %q: %s (ret=%d)", mountURL, msg, ret)
	}
	return &libnfsClient{nfs: nfs}, nil
}

func (c *libnfsClient) Close() {
	if c.nfs != nil {
		C.nfs_destroy_context(c.nfs)
		c.nfs = nil
	}
}

type libnfsStat struct {
	Mode  uint64
	Size  uint64
	Nlink uint64
}

func (c *libnfsClient) Stat(path string) (libnfsStat, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var st C.struct_nfs_stat_64
	ret := C.nfs_stat64(c.nfs, cpath, &st)
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(c.nfs))
		return libnfsStat{}, fmt.Errorf("nfs_stat64(%q): %s", path, msg)
	}
	return libnfsStat{
		Mode:  uint64(st.nfs_mode),
		Size:  uint64(st.nfs_size),
		Nlink: uint64(st.nfs_nlink),
	}, nil
}

func (c *libnfsClient) ReadFile(path string) ([]byte, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var fh *C.struct_nfsfh
	ret := C.nfs_open(c.nfs, cpath, C.O_RDONLY, &fh)
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(c.nfs))
		return nil, fmt.Errorf("nfs_open(%q): %s", path, msg)
	}
	defer C.nfs_close(c.nfs, fh)

	var result []byte
	buf := make([]byte, 64*1024)
	for {
		n := C.nfs_read(c.nfs, fh, C.uint64_t(len(buf)), unsafe.Pointer(&buf[0]))
		if n < 0 {
			msg := C.GoString(C.nfs_get_error(c.nfs))
			return nil, fmt.Errorf("nfs_read(%q): %s", path, msg)
		}
		if n == 0 {
			break
		}
		result = append(result, buf[:n]...)
	}
	return result, nil
}

func (c *libnfsClient) ReadDir(path string) ([]string, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var dir *C.struct_nfsdir
	ret := C.nfs_opendir(c.nfs, cpath, &dir)
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(c.nfs))
		return nil, fmt.Errorf("nfs_opendir(%q): %s", path, msg)
	}
	defer C.nfs_closedir(c.nfs, dir)

	var names []string
	for {
		ent := C.nfs_readdir(c.nfs, dir)
		if ent == nil {
			break
		}
		name := C.GoString(ent.name)
		if name != "." && name != ".." {
			names = append(names, name)
		}
	}
	return names, nil
}

func (c *libnfsClient) Readlink(path string) (string, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	buf := make([]byte, 4096)
	ret := C.nfs_readlink(c.nfs, cpath, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(c.nfs))
		return "", fmt.Errorf("nfs_readlink(%q): %s", path, msg)
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

func (c *libnfsClient) Lstat(path string) (libnfsStat, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var st C.struct_nfs_stat_64
	ret := C.nfs_lstat64(c.nfs, cpath, &st)
	if ret != 0 {
		msg := C.GoString(C.nfs_get_error(c.nfs))
		return libnfsStat{}, fmt.Errorf("nfs_lstat64(%q): %s", path, msg)
	}
	return libnfsStat{
		Mode:  uint64(st.nfs_mode),
		Size:  uint64(st.nfs_size),
		Nlink: uint64(st.nfs_nlink),
	}, nil
}
