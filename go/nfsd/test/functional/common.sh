#!/usr/bin/env bash

set -euo pipefail

functional_test_init() {
	local test_name=$1
	shift
	if (( $# != 1 )); then
		echo "usage: $test_name TEST_ROOT" >&2
		exit 2
	fi
	local root_arg=$1
	if [[ ! -d $root_arg ]]; then
		echo "test root is not a directory: $root_arg" >&2
		exit 2
	fi
	local root
	root=$(cd -- "$root_arg" && pwd -P)

	FUNCTIONAL_TEST_DIR=$(mktemp -d -- "$root/${test_name}.XXXXXX")
	export FUNCTIONAL_TEST_DIR
	trap functional_test_cleanup EXIT
}

functional_test_cleanup() {
	local status=$?
	trap - EXIT
	if (( status == 0 )); then
		if ! rm -rf -- "$FUNCTIONAL_TEST_DIR"; then
			echo "failed to remove test directory: $FUNCTIONAL_TEST_DIR" >&2
			exit 1
		fi
	else
		echo "preserved failed test directory: $FUNCTIONAL_TEST_DIR" >&2
	fi
	exit "$status"
}

functional_test_require() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required command is not installed: $1" >&2
		exit 2
	fi
}

functional_test_assert_contents() {
	local path=$1
	local expected=$2
	if ! cmp -s <(printf '%s' "$expected") "$path"; then
		echo "unexpected contents in $path" >&2
		diff -u <(printf '%s' "$expected") "$path" >&2 || true
		return 1
	fi
}

functional_test_assert_zero_file() {
	local path=$1
	local expected_size=$2
	local actual_size
	actual_size=$(stat -c %s -- "$path")
	if [[ $actual_size != "$expected_size" ]]; then
		echo "$path has size $actual_size; expected $expected_size" >&2
		return 1
	fi
	if ! cmp -s -n "$expected_size" "$path" /dev/zero; then
		echo "$path contains non-zero data" >&2
		return 1
	fi
}
