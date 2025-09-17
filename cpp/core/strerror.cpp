// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <string.h>
#include <stdio.h>

thread_local static char strerror_buf[128];

// Using the POSIX-compliant strerror_r is annoying with glibc, because
// libstdc++ sets _GNU_SOURCE. So we just define a safe version here
// to avoid repeated annoyances.
//
// Testing for _GNU_SOURCE does not work, because the alpine build
// has that set, too.

#ifdef TERN_ALPINE
const char* safe_strerror(int errnum) {
    int res = strerror_r(errnum, strerror_buf, sizeof(strerror_buf));
    if (res > 0) {
        fprintf(stderr, "strerror_r failed: %d\n", res);
        snprintf(strerror_buf, sizeof(strerror_buf), "%d", errnum);
    }
    return strerror_buf;
}
#else
const char* safe_strerror(int errnum) {
    return strerror_r(errnum, strerror_buf, sizeof(strerror_buf));
}
#endif
