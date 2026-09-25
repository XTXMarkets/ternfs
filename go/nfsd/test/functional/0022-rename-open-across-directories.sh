#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0022-rename-open-across-directories" "$@"
functional_test_require mv

# This is a rejection test: mv must fail with EBUSY for the test to pass.
# An active staged inode must publish in its construction directory, so nfsd
# refuses to move its pathname to another directory. The open descriptor must
# remain writable and CLOSE must publish at the original name.
# A retired writer can be discarded to allow a move; fd 3 keeps this one active.
mkdir -- "$FUNCTIONAL_TEST_DIR/logs" "$FUNCTIONAL_TEST_DIR/old"
log_path="$FUNCTIONAL_TEST_DIR/logs/app.log"
rotated_path="$FUNCTIONAL_TEST_DIR/old/app.log"

exec 3>>"$log_path"
printf 'before rotation\n' >&3
if error=$(LC_ALL=C mv -- "$log_path" "$rotated_path" 2>&1); then
	echo "rename across directories unexpectedly succeeded" >&2
	exit 1
fi
if [[ $error != *"busy"* ]]; then
	echo "unexpected rename error: $error" >&2
	exit 1
fi
printf 'after rotation\n' >&3
exec 3>&-

functional_test_assert_contents \
	"$log_path" $'before rotation\nafter rotation\n'
if [[ -e $rotated_path ]]; then
	echo "rejected rename created $rotated_path" >&2
	exit 1
fi
