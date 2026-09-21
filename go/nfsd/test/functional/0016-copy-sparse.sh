#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0016-copy-sparse" "$@"
functional_test_require cp
functional_test_require dd
functional_test_require truncate

source_path="$FUNCTIONAL_TEST_DIR/source"
target_path="$FUNCTIONAL_TEST_DIR/target"
size=$((1024 * 1024))
truncate -s "$size" "$source_path"
printf 'begin' | dd of="$source_path" bs=1 seek=0 conv=notrunc status=none
printf 'end' |
	dd of="$source_path" bs=1 seek=$((size - 3)) conv=notrunc status=none

cp --sparse=always --no-preserve=mode,ownership,timestamps \
	-- "$source_path" "$target_path"

if ! cmp -s "$source_path" "$target_path"; then
	echo "sparse copy differs from its source" >&2
	exit 1
fi
