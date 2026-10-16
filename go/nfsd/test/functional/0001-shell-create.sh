#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0001-shell-create" "$@"

path="$FUNCTIONAL_TEST_DIR/create.txt"
printf 'created\n' >"$path"

functional_test_assert_contents "$path" $'created\n'
