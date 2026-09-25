#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0017-find-delete" "$@"
functional_test_require find

tree="$FUNCTIONAL_TEST_DIR/tree"
mkdir -p "$tree/one/two" "$tree/other"
printf 'one\n' >"$tree/one/file"
printf 'two\n' >"$tree/one/two/file"
ln -s ../one/file "$tree/other/link"

find "$tree" -depth -delete

if [[ -e $tree ]]; then
	echo "find -delete left the tree behind" >&2
	exit 1
fi
