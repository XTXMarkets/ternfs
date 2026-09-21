#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0002-shell-append" "$@"

path="$FUNCTIONAL_TEST_DIR/append.txt"
printf 'first\n' >"$path"
printf 'second\n' >>"$path"

functional_test_assert_contents "$path" $'first\nsecond\n'
