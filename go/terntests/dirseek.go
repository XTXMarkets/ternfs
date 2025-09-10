// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"unsafe"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"
)

// #include <string.h>
// #include <fcntl.h>
// #include <errno.h>
// #include <stdlib.h>
// #include <sys/syscall.h>
// #include <unistd.h>
// #include <dirent.h>
//
// int open_dir(const char* path, int* err) {
//     int fd = open(path, O_RDONLY|O_DIRECTORY);
//     if (fd < 0) {
//         *err = errno;
//         return fd;
//     }
//     *err = 0;
//     return fd;
// }
//
// off_t dir_seek(int fd, off_t offset, int whence, int* err) {
//     off_t res = lseek(fd, offset, whence);
//     if (res < 0) {
//         *err = errno;
//         return 0;
//     }
//     *err = 0;
//     return res;
// }
//
// ssize_t get_dents(int fd, void* buf, size_t count, int* err) {
//     ssize_t nread = syscall(SYS_getdents64, fd, buf, count);
//     if (nread < 0) {
//         *err = errno;
//         return nread;
//     }
//     *err = 0;
//     return nread;
// }
import "C"

func openDir(path string) (C.int, error) {
	cPath := unsafe.Pointer(C.CString(path))
	defer C.free(cPath)
	var err C.int
	fd := C.open_dir((*C.char)(cPath), &err)
	if fd < 0 {
		return 0, fmt.Errorf("opening %q failed: %v", path, err)
	} else {
		return fd, nil
	}
}

type dent struct {
	ino        msgs.InodeId
	nextOffset C.off_t
	reclen     C.ushort
	typ        C.char
	name       string
}

func getDents(bufsize int, fd C.int, dents []dent) ([]dent, error) {
	cStr := (*C.char)(C.malloc(C.ulong(bufsize)))
	defer C.free(unsafe.Pointer(cStr))
	var err C.int
	r := C.get_dents(fd, unsafe.Pointer(cStr), C.ulong(bufsize), &err)
	if r < 0 {
		return nil, fmt.Errorf("getting dents failed: %v", err)
	}
	dentsStr := C.GoStringN(cStr, C.int(r))
	dentsR := bytes.NewReader([]byte(dentsStr))
	for {
		dent := dent{}
		err := binary.Read(dentsR, binary.LittleEndian, &dent.ino)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := binary.Read(dentsR, binary.LittleEndian, &dent.nextOffset); err != nil {
			panic(err)
		}
		if err := binary.Read(dentsR, binary.LittleEndian, &dent.reclen); err != nil {
			panic(err)
		}
		if err := binary.Read(dentsR, binary.LittleEndian, &dent.typ); err != nil {
			panic(err)
		}
		name := make([]byte, int(dent.reclen)-(8+8+2+1))
		if _, err := io.ReadFull(dentsR, name); err != nil {
			panic(err)
		}
		for i := 0; i < len(name); i++ {
			if name[i] == 0 {
				name = name[:i]
				break
			}
		}
		dent.name = string(name)
		switch dent.typ {
		case C.DT_REG:
			if dent.ino.Type() != msgs.FILE {
				panic(fmt.Errorf("mismatching d_type %v and ino type %v", dent.typ, dent.ino.Type()))
			}
		case C.DT_DIR:
			if dent.ino.Type() != msgs.DIRECTORY {
				panic(fmt.Errorf("mismatching d_type %v and ino type %v", dent.typ, dent.ino.Type()))
			}
		case C.DT_LNK:
			if dent.ino.Type() != msgs.SYMLINK {
				panic(fmt.Errorf("mismatching d_type %v and ino type %v", dent.typ, dent.ino.Type()))
			}
		default:
			panic(fmt.Errorf("bad d_type %v", dent.typ))
		}
		dents = append(dents, dent)
	}
	return dents, nil
}

func getAllDents(bufsize int, fd C.int) ([]dent, error) {
	dents := []dent{}
	for {
		var err error
		lenBefore := len(dents)
		dents, err = getDents(bufsize, fd, dents)
		if err != nil {
			return nil, err
		}
		if len(dents) == lenBefore {
			break
		}
	}
	return dents, nil
}

func dirSeek(fd C.int, off C.long, whence C.int) (C.long, error) {
	var err C.int
	off = C.dir_seek(fd, off, whence, &err)
	if off < 0 {
		return 0, fmt.Errorf("could not seek dir: %v", err)
	}
	return off, nil
}

func dirSeekTest(log *log.Logger, registryAddress string, mountPoint string, numPaths int) {
	log.Info("creating %v paths", numPaths)
	for i := 0; i < numPaths; i++ {
		path := path.Join(mountPoint, fmt.Sprintf("%v", i))
		if i%10 == 0 { // dirs, not too many since they're more expensive to create
			if err := os.Mkdir(path, 0777); err != nil {
				panic(err)
			}
		} else if i%3 == 0 {
			if err := os.Symlink(mountPoint, path); err != nil {
				panic(err)
			}
		} else {
			f, err := os.Create(path)
			if err != nil {
				panic(err)
			}
			if err := f.Close(); err != nil {
				panic(err)
			}
		}
	}
	// open root dir
	dirFd, err := openDir(mountPoint)
	if err != nil {
		panic(err)
	}
	defer C.close(dirFd)
	// get dents
	bufsize := 500
	dents, err := getAllDents(bufsize, dirFd)
	if err != nil {
		panic(err)
	}
	// verify dents with what we get straight from the server
	{
		c := newTestClient(log, registryAddress, nil)
		defer c.Close()
		ix := 2 // . and ..
		req := msgs.ReadDirReq{
			DirId: msgs.ROOT_DIR_INODE_ID,
		}
		resp := msgs.ReadDirResp{}
		for {
			if err := c.ShardRequest(log, 0, &req, &resp); err != nil {
				panic(err)
			}
			for i := range resp.Results {
				dent := &dents[ix+i]
				edge := &resp.Results[i]
				if dent.name != edge.Name || dent.ino != edge.TargetId || dents[ix+i-1].nextOffset != C.off_t(edge.NameHash) {
					panic(fmt.Errorf("mismatching edge %+v and dent %+v dent at index %+v", edge, dent, ix+i))
				}
			}
			ix += len(resp.Results)
			req.StartHash = resp.NextHash
			if req.StartHash == 0 {
				break
			}
		}
	}
	r := wyhash.New(42)
	verifyOffset := func(ix int, offset C.long) {
		log.Debug("seeking dir at %v (%v out of %v)", offset, ix, len(dents))
		whence := C.int(C.SEEK_SET)
		if r.Uint64()%2 == 0 {
			// seek at random, then use SEEK_CUR
			tmpOffset := C.long(r.Uint64() & ^(uint64(1) << 63))
			if _, err := dirSeek(dirFd, tmpOffset, C.SEEK_SET); err != nil {
				panic(err)
			}
			offset = offset - tmpOffset
			whence = C.SEEK_CUR
		}
		if _, err := dirSeek(dirFd, offset, whence); err != nil {
			panic(err)
		}
		postfixDents, err := getAllDents(bufsize, dirFd)
		if err != nil {
			panic(err)
		}
		if len(postfixDents) != len(dents)-ix {
			panic(fmt.Errorf("expected postfix of length %v, got length %v", len(dents)-ix, len(postfixDents)))
		}
		for j := ix; j < len(dents); j++ {
			if dents[j] != postfixDents[j-ix] {
				log.Info("dents[%v] %+v postfixDents[%v] %+v", j, dents[j], j-ix, postfixDents[j-ix])
				panic(fmt.Errorf("mistmatching dents %+v != %+v", dents[j], postfixDents[j-ix]))
			}
		}
	}
	// seek from random, existing offsets, get the dents again,
	// see if they're what we expect.
	for i := 0; i < 100; i++ {
		ix := int(r.Uint32()) % len(dents)
		var offset C.long
		if ix > 0 {
			offset = C.long(dents[ix-1].nextOffset)
		} else {
			offset = 0
		}
		verifyOffset(ix, offset)
	}
	// seek from random, blind offsets
	for i := 0; i < 100; i++ {
		offset := C.long(r.Uint64() & ^(uint64(1) << 63))
		ix := sort.Search(len(dents), func(i int) bool {
			if i < 2 {
				return false // . ..
			}
			return C.long(dents[i-1].nextOffset) >= offset
		})
		verifyOffset(ix, offset)
	}
	// seek at the end
	if _, err := dirSeek(dirFd, C.long((uint64(1)<<63)-1), C.SEEK_SET); err != nil {
		panic(err)
	}
	if dents, err := getAllDents(bufsize, dirFd); err != nil {
		panic(err)
	} else if len(dents) > 0 {
		panic(fmt.Errorf("unexpected length %v", len(dents)))
	}
}
