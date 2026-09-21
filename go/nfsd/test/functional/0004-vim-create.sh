#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0004-vim-create" "$@"
functional_test_require vim

path="$FUNCTIONAL_TEST_DIR/vim-create.txt"
vim -Nu NONE -i NONE -es \
	-c 'call setline(1, "created by vim")' \
	-c write -c quit "$path"

functional_test_assert_contents "$path" $'created by vim\n'
