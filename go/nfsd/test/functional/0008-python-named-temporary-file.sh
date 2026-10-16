#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0008-python-named-temporary-file" "$@"
functional_test_require python3

python3 - "$FUNCTIONAL_TEST_DIR" <<'PY'
import pathlib
import sys
import tempfile

root = pathlib.Path(sys.argv[1])
f = tempfile.NamedTemporaryFile(dir=root, delete=False)
path = pathlib.Path(f.name)
assert path.exists()
try:
    f.write(b"temporary data")
finally:
    f.close()

data = path.read_bytes()
assert data == b"temporary data", (
    f"pathname read returned {data!r}; expected closed file data"
)
path.unlink()
assert not path.exists()
PY

if [[ -n $(find "$FUNCTIONAL_TEST_DIR" -mindepth 1 -print -quit) ]]; then
	echo "named temporary file left a directory entry behind" >&2
	exit 1
fi
