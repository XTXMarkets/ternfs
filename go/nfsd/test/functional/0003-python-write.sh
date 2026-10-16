#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0003-python-write" "$@"
functional_test_require python3

path="$FUNCTIONAL_TEST_DIR/python.txt"
python3 - "$path" <<'PY'
import sys

with open(sys.argv[1], "w") as f:
    f.write("written by python\n")
PY

functional_test_assert_contents "$path" $'written by python\n'
