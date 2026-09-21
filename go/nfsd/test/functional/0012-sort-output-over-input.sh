#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0012-sort-output-over-input" "$@"
functional_test_require sort

path="$FUNCTIONAL_TEST_DIR/data"
printf 'charlie\nalpha\nbravo\n' >"$path"
sort -o "$path" "$path"

functional_test_assert_contents "$path" $'alpha\nbravo\ncharlie\n'
