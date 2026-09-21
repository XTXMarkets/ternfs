#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0015-gzip-round-trip" "$@"
functional_test_require gzip
functional_test_require gunzip

path="$FUNCTIONAL_TEST_DIR/data.txt"
decoded="$FUNCTIONAL_TEST_DIR/decoded.txt"
printf 'data to compress\n' >"$path"
gzip -- "$path"
if [[ -e $path || ! -e $path.gz ]]; then
	echo "gzip did not replace the input with a compressed file" >&2
	exit 1
fi
gzip -cd -- "$path.gz" >"$decoded"
functional_test_assert_contents "$decoded" $'data to compress\n'
rm -- "$decoded"

gunzip -- "$path.gz"
if [[ -e $path.gz ]]; then
	echo "gunzip left the compressed input behind" >&2
	exit 1
fi
functional_test_assert_contents "$path" $'data to compress\n'
