// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"io"
	"os"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/msgs"
)

var buf []byte

func printCrc32(name string, r io.Reader) {
	crc := uint32(0)
	for {
		read, err := r.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if read == 0 {
			break
		}
		crc = crc32c.Sum(crc, buf[:read])
	}
	fmt.Printf("%v %v\n", msgs.Crc(crc), name)
}

func main() {
	buf = make([]byte, 1<<20)

	if len(os.Args) == 1 {
		printCrc32("-", os.Stdin)
	} else {
		for _, fname := range os.Args[1:] {
			f, err := os.Open(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not open file %v: %v\n", fname, err)
				os.Exit(1)
			}
			printCrc32(fname, f)
			f.Close()
		}
	}
}
