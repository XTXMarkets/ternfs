#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0023-read-unlinked-open-file" "$@"
functional_test_require rm

# Detaching a pathname leaves the filehandle usable for reads as well as
# writes. Reading a range the writer never touched has to come from the
# published version the staged file is based on, after its name is gone.
path="$FUNCTIONAL_TEST_DIR/data"
printf '0123456789\n' >"$path"

exec 3<>"$path"
printf 'AB' >&3
rm -- "$path"
# Read through the descriptor itself: reopening /dev/fd/3 would look the
# removed name up again and fail with ESTALE.
tail=$(head -c 8 <&3)
if [[ $tail != '23456789' ]]; then
	echo "unwritten range after unlink = $(printf %q "$tail")" >&2
	exit 1
fi
exec 3>&-

if [[ -e $path ]]; then
	echo "unlinked name reappeared after CLOSE" >&2
	exit 1
fi
