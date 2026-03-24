// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

// #include <unistd.h>
// #include <fcntl.h>
// #include <errno.h>
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// #include <sys/syscall.h>
//
// struct linux_dirent {
//     unsigned long  d_ino;
//     off_t          d_off;
//     unsigned short d_reclen;
//     char           d_name[];
// };
//
// // If negative, it's an error code.
// ssize_t count_blocks(char* base_path) {
//     int base_fd = -1;
//     int dir_fd = -1;
//     ssize_t blocks = 0;
//     base_fd = open(base_path, O_RDONLY|O_DIRECTORY);
//     if (base_fd < 0) {
//         fprintf(stderr, "could not open %s: %d\n", base_path, errno);
//         blocks = -errno;
//         goto out;
//     }
//     char buf[1024];
//     for (int i = 0; i < 256; i++) {
//         char dir_path[3];
//         snprintf(dir_path, 3, "%02x", i);
//         if (dir_fd >= 0) { close(dir_fd); }
//         dir_fd = openat(base_fd, dir_path, O_RDONLY|O_DIRECTORY);
//         if (dir_fd < 0) {
//             if (errno == ENOENT) {
//                 continue;
//             }
//             fprintf(stderr, "could not open dir %s/%s: %d\n", base_path, dir_path, errno);
//             blocks = -errno;
//             goto out;
//         }
//         for (;;) {
//             long read = syscall(SYS_getdents, dir_fd, buf, sizeof(buf));
//             if (read < 0) {
//                 fprintf(stderr, "could not read direntries in %s/%s: %d\n", base_path, dir_path, errno);
//                 blocks = -errno;
//                 goto out;
//             }
//             if (read == 0) { break; }
//             long bpos = 0;
//             while( bpos < read) {
//                 struct linux_dirent *entry = (struct linux_dirent *)(buf + bpos);
//                 bpos += entry->d_reclen;
//                 if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0 ||
//                     strncmp(entry->d_name, "tmp.", 4) == 0) {
//                         continue;
//                 }
//                 blocks++;
//             }
//         }
//     }
// out:
//     if (dir_fd >= 0) { close(dir_fd); }
//     if (base_fd >= 0) { close(base_fd); }
//     return blocks;
// }
import "C"

import (
	"syscall"
	"unsafe"
)

// CountBlocks counts the number of block files in a block service directory.
// Done in C to minimize syscall overhead and avoid starving goroutines
// waiting on network I/O.
func CountBlocks(basePath string) (uint64, error) {
	cBasePath := C.CString(basePath)
	defer C.free(unsafe.Pointer(cBasePath))
	blocks := C.count_blocks(cBasePath)
	if blocks < 0 {
		return 0, syscall.Errno(-blocks)
	}
	return uint64(blocks), nil
}
