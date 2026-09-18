// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build libnfs

package libnfs

/*
#cgo CFLAGS: -I${SRCDIR}/../.deps/libnfs-install/include
#cgo LDFLAGS: -L${SRCDIR}/../.deps/libnfs-install/lib -lnfs -Wl,-rpath,${SRCDIR}/../.deps/libnfs-install/lib
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
	"syscall"
	"unsafe"
)

type libnfsClient struct {
	nfs *C.struct_nfs_context
}

type libnfsFile struct {
	client *libnfsClient
	fh     *C.struct_nfsfh
}

func (c *libnfsClient) OpenFile(path string, flags int) (*libnfsFile, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var fh *C.struct_nfsfh
	if ret := C.nfs_open(c.nfs, cpath, C.int(flags), &fh); ret != 0 {
		return nil, c.error("nfs_open", ret)
	}
	return &libnfsFile{client: c, fh: fh}, nil
}

func (f *libnfsFile) Close() error {
	if f.fh == nil {
		return nil
	}
	fh := f.fh
	f.fh = nil
	if ret := C.nfs_close(f.client.nfs, fh); ret != 0 {
		return f.client.error("nfs_close", ret)
	}
	return nil
}

func (f *libnfsFile) WriteAt(data []byte, offset uint64) error {
	if len(data) == 0 {
		return nil
	}
	n := C.nfs_pwrite(f.client.nfs, f.fh, unsafe.Pointer(&data[0]), C.size_t(len(data)), C.uint64_t(offset))
	if n < 0 {
		return f.client.error("nfs_pwrite", n)
	}
	if int(n) != len(data) {
		return fmt.Errorf("nfs_pwrite: count=%d: %s", n, C.GoString(C.nfs_get_error(f.client.nfs)))
	}
	return nil
}

func (f *libnfsFile) Read() ([]byte, error) {
	buf := make([]byte, 4096)
	n := C.nfs_pread(f.client.nfs, f.fh, unsafe.Pointer(&buf[0]), C.size_t(len(buf)), 0)
	if n < 0 {
		return nil, f.client.error("nfs_pread", n)
	}
	return buf[:int(n)], nil
}

func (f *libnfsFile) SyncSize() (uint64, error) {
	if ret := C.nfs_fsync(f.client.nfs, f.fh); ret != 0 {
		return 0, f.client.error("nfs_fsync", ret)
	}
	var st C.struct_nfs_stat_64
	if C.nfs_fstat64(f.client.nfs, f.fh, &st) != 0 {
		return 0, fmt.Errorf("nfs_fstat64: %s", C.GoString(C.nfs_get_error(f.client.nfs)))
	}
	return uint64(st.nfs_size), nil
}

func libnfsConnectAt(host string, port int, root, identity string) (*libnfsClient, error) {
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
	if identity != "" {
		id := C.CString(identity)
		C.nfs4_set_client_name(nfs, id)
		C.free(unsafe.Pointer(id))
		C.nfs_set_autoreconnect(nfs, 2)
	}

	mountURL := fmt.Sprintf("nfs://%s%s?nfsport=%d", host, root, port)
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

func (c *libnfsClient) error(op string, ret C.int) error {
	return fmt.Errorf("%s: %s: %w", op, C.GoString(C.nfs_get_error(c.nfs)), syscall.Errno(-ret))
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
		n := C.nfs_read(c.nfs, fh, unsafe.Pointer(&buf[0]), C.size_t(len(buf)))
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
