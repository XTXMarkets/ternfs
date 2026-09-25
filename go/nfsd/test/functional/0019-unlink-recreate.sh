#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0019-unlink-recreate" "$@"
functional_test_require rm

path="$FUNCTIONAL_TEST_DIR/data"
exec 3>"$path"
printf 'old data before unlink\n' >&3
rm -- "$path"
printf 'replacement data\n' >"$path"
printf 'old data after unlink\n' >&3
exec 3>&-

functional_test_assert_contents "$path" $'replacement data\n'
