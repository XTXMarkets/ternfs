// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package crc32c

// #cgo LDFLAGS: -L${SRCDIR}/../../../cpp/build/alpine/crc32c -lcrc32c
// #include "../../../cpp/crc32c/crc32c.h"
import "C"
import (
	"fmt"
	"unsafe"
)

func Sum(crc uint32, buf []byte) uint32 {
	if len(buf) == 0 { // otherwise the buf[0] won't work below
		return crc
	}
	return (uint32)(C.crc32c(C.uint(crc), (*C.char)(unsafe.Pointer(&buf[0])), C.ulong(len(buf))))
}

func Xor(crc1 uint32, crc2 uint32, len int) uint32 {
	if len < 0 {
		panic(fmt.Errorf("negative len %v", len))
	}
	return (uint32)(C.crc32c_xor(C.uint(crc1), C.uint(crc2), C.ulong(len)))
}

func Append(crc1 uint32, crc2 uint32, len2 int) uint32 {
	if len2 < 0 {
		panic(fmt.Errorf("negative len %v", len2))
	}
	return (uint32)(C.crc32c_append(C.uint(crc1), C.uint(crc2), C.ulong(len2)))
}

func ZeroExtend(crc uint32, zeros int) uint32 {
	return (uint32)(C.crc32c_zero_extend(C.uint(crc), C.long(zeros)))
}
