#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0020-rename-file-open" "$@"
functional_test_require mv

source_path="$FUNCTIONAL_TEST_DIR/before"
target_path="$FUNCTIONAL_TEST_DIR/after"

exec 3>"$source_path"
mv -- "$source_path" "$target_path"
if [[ -e $source_path || ! -e $target_path ]]; then
	echo "rename did not move the open file to $target_path" >&2
	exit 1
fi
printf 'written after rename\n' >&3
exec 3>&-

functional_test_assert_contents "$target_path" $'written after rename\n'
