#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

cd "$(dirname "$0")"

if [[ -v IN_TERN_BUILD_CONTAINER ]]; then
  sanitized="ubuntusanitized"
  valgrind="ubuntuvalgrind"
else
  sanitized="sanitized"
  valgrind="valgrind"
fi

echo "$(tput bold)C++ tests, sanitized$(tput sgr0)"
set -x
./build.py $sanitized rs/rs-tests crc32c/crc32c-tests tests/tests tests/logsdbtests
UBSAN_OPTIONS=print_stacktrace=1 ./build/$sanitized/rs/rs-tests
UBSAN_OPTIONS=print_stacktrace=1 ./build/$sanitized/crc32c/crc32c-tests
UBSAN_OPTIONS=print_stacktrace=1 ./build/$sanitized/tests/tests
UBSAN_OPTIONS=print_stacktrace=1 ./build/$sanitized/tests/logsdbtests
set +x

# valgrind doesn't support fnctl F_SET_RW_HINT (1036), and as far as I can
# tell there isn't a way to programmatically filter those.

echo "$(tput bold)C++ tests, valgrind$(tput sgr0)"
set -x
./build.py $valgrind rs/rs-tests crc32c/crc32c-tests tests/tests tests/logsdbtests
valgrind --exit-on-first-error=yes -q --error-exitcode=1 ./build/$valgrind/rs/rs-tests
valgrind --exit-on-first-error=yes -q --error-exitcode=1 ./build/$valgrind/crc32c/crc32c-tests
valgrind --exit-on-first-error=yes -q --suppressions=valgrind-suppressions --error-exitcode=1 ./build/$valgrind/tests/tests 2> >(grep -v "Warning: unimplemented fcntl command: 1036")
valgrind --exit-on-first-error=yes -q --suppressions=valgrind-suppressions --error-exitcode=1 ./build/$valgrind/tests/logsdbtests 2> >(grep -v "Warning: unimplemented fcntl command: 1036")
set +x
