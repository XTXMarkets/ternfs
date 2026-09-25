#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0018-python-temporary-file" "$@"
functional_test_require python3

python3 - "$FUNCTIONAL_TEST_DIR" <<'PY'
import pathlib
import sys
import tempfile

root = pathlib.Path(sys.argv[1])
with tempfile.TemporaryFile(dir=root) as f:
    f.write(b"temporary data")
    f.seek(0)
    assert f.read() == b"temporary data"
PY

if [[ -n $(find "$FUNCTIONAL_TEST_DIR" -mindepth 1 -print -quit) ]]; then
	echo "temporary file left a directory entry behind" >&2
	exit 1
fi
