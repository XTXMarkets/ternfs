#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0021-log-rotation" "$@"
functional_test_require mv

log_path="$FUNCTIONAL_TEST_DIR/app.log"
rotated_path="$FUNCTIONAL_TEST_DIR/app.log.1"

exec 3>>"$log_path"
printf 'before rotation\n' >&3
mv -- "$log_path" "$rotated_path"
printf 'new log\n' >"$log_path"
printf 'after rotation\n' >&3
exec 3>&-

functional_test_assert_contents \
	"$rotated_path" $'before rotation\nafter rotation\n'
functional_test_assert_contents "$log_path" $'new log\n'
