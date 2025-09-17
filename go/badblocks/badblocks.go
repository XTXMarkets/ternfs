// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

func getPhysicalBlockSize(path string) (int64, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return 0, err
	}
	return fs.Bsize, nil
}

func isIOErr(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err == syscall.EIO {
		return true
	}
	return false
}

func checkFile(path string) {
	blockSize, err := getPhysicalBlockSize(path)
	if err != nil {
		fmt.Printf("could not get block size for %q: %v\n", path, err)
		os.Exit(1)
	}
	buf := make([]byte, blockSize)
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("could not open file %q: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fmt.Printf("could not stat file %q: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("will read %q (%v bytes) with block size %v\n", path, fi.Size(), blockSize)
	badSectionsStart := []int64{}
	badSectionsEnd := []int64{}
	for block := int64(0); block < fi.Size(); block += blockSize {
		// Previous failures might have impeded the file position being bumped,
		// so use SeekStart
		if _, seekErr := f.Seek(block, io.SeekStart); seekErr != nil {
			fmt.Printf("could not seek file %v to %v: %v\n", path, block, seekErr)
			os.Exit(1)
		}
		_, err := f.Read(buf)
		if isIOErr(err) {
			fmt.Printf("got IO error at block %v\n", block)
			if len(badSectionsStart) == len(badSectionsEnd) { // new section
				badSectionsStart = append(badSectionsStart, block)
			}
		} else if err != nil {
			fmt.Printf("could not read block starting at %v: %v\n", block, err)
			os.Exit(1)
		} else {
			if len(badSectionsStart) > len(badSectionsEnd) {
				badSectionsEnd = append(badSectionsEnd, block)
			}
		}
	}
	if len(badSectionsStart) > len(badSectionsEnd) {
		badSectionsEnd = append(badSectionsEnd, fi.Size())
	}
	for i := 0; i < len(badSectionsStart); i++ {
		fmt.Printf("bad section at %v-%v (%v blocks)\n", badSectionsStart[i], badSectionsEnd[i], (badSectionsEnd[i]-badSectionsStart[i]+blockSize-1)/blockSize)
	}
}

func main() {
	checkFile(os.Args[1])
}
