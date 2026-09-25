#!/usr/bin/env bash

set -uo pipefail

if (( $# != 1 )); then
	echo "usage: $0 TEST_ROOT" >&2
	exit 2
fi

root=$1
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
shopt -s nullglob
tests=("$script_dir"/[0-9][0-9][0-9][0-9]-*.sh)
if (( ${#tests[@]} == 0 )); then
	echo "no functional tests found in $script_dir" >&2
	exit 2
fi

status=0
for test_path in "${tests[@]}"; do
	test_name=$(basename -- "$test_path")
	printf '%s: ' "$test_name"
	if "$test_path" "$root"; then
		echo PASS
	else
		echo FAIL
		status=1
	fi
done
exit "$status"
