#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0006-dd-create" "$@"
functional_test_require dd

path="$FUNCTIONAL_TEST_DIR/dd"
dd if=/dev/zero of="$path" bs=4096 count=1 status=none

functional_test_assert_zero_file "$path" 4096
