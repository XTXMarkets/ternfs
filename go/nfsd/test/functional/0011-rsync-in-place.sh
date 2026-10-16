#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0011-rsync-in-place" "$@"
functional_test_require rsync

source_path="$FUNCTIONAL_TEST_DIR/source"
target_path="$FUNCTIONAL_TEST_DIR/target"
printf 'replacement from rsync --inplace\n' >"$source_path"
printf 'old target\n' >"$target_path"
rsync --inplace --no-perms --no-owner --no-group --no-times \
	-- "$source_path" "$target_path"

functional_test_assert_contents \
	"$target_path" $'replacement from rsync --inplace\n'
