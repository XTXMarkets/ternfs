#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0014-tar-extract" "$@"
functional_test_require tar

source_dir="$FUNCTIONAL_TEST_DIR/source"
target_dir="$FUNCTIONAL_TEST_DIR/target"
archive="$FUNCTIONAL_TEST_DIR/archive.tar"
mkdir -p "$source_dir/nested" "$target_dir"
printf 'top level\n' >"$source_dir/top.txt"
printf 'nested\n' >"$source_dir/nested/data.txt"
ln -s nested/data.txt "$source_dir/link"

tar -cf "$archive" -C "$source_dir" .
tar -xf "$archive" -C "$target_dir" \
	--no-same-owner --no-same-permissions --touch

functional_test_assert_contents "$target_dir/top.txt" $'top level\n'
functional_test_assert_contents "$target_dir/nested/data.txt" $'nested\n'
if [[ ! -L $target_dir/link ||
	$(readlink -- "$target_dir/link") != nested/data.txt ]]; then
	echo "tar did not restore the relative symlink" >&2
	exit 1
fi
