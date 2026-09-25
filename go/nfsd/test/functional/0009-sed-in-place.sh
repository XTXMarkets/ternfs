#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0009-sed-in-place" "$@"
functional_test_require sed

path="$FUNCTIONAL_TEST_DIR/data.txt"
printf 'before\nunchanged\n' >"$path"
sed -i 's/before/after/' "$path"

functional_test_assert_contents "$path" $'after\nunchanged\n'
