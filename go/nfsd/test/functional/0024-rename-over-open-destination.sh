#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0024-rename-over-open-destination" "$@"
functional_test_require mv

source_path="$FUNCTIONAL_TEST_DIR/source"
target_path="$FUNCTIONAL_TEST_DIR/target"
printf 'replacement\n' >"$source_path"

exec 3>>"$target_path"
printf 'old destination before rename\n' >&3
mv -f -- "$source_path" "$target_path"
printf 'old destination after rename\n' >&3
exec 3>&-

functional_test_assert_contents "$target_path" $'replacement\n'
