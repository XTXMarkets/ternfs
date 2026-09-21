#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0005-vim-edit" "$@"
functional_test_require vim

path="$FUNCTIONAL_TEST_DIR/vim-edit.txt"
printf 'before\n' >"$path"
vim -Nu NONE -i NONE -es \
	-c '%s/before/after/' \
	-c write -c quit "$path"

functional_test_assert_contents "$path" $'after\n'
