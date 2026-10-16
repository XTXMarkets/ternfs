#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0007-dd-direct" "$@"
functional_test_require dd

path="$FUNCTIONAL_TEST_DIR/dd-direct"
dd if=/dev/zero of="$path" bs=4096 count=1 oflag=direct status=none

functional_test_assert_zero_file "$path" 4096
