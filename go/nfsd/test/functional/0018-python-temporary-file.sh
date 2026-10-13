#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0018-python-temporary-file" "$@"
functional_test_require python3

python3 - "$FUNCTIONAL_TEST_DIR" <<'PY'
import pathlib
import sys
import tempfile
import time

root = pathlib.Path(sys.argv[1])
with tempfile.TemporaryFile(dir=root) as f:
    f.write(b"temporary data")
    f.seek(0)
    assert f.read() == b"temporary data"

# Linux NFS removes silly-renamed .nfs* files asynchronously after close.
deadline = time.monotonic() + 5
while True:
    entries = sorted(p.name for p in root.iterdir())
    if not entries:
        break
    if any(not name.startswith(".nfs") for name in entries):
        sys.exit(f"temporary file left unexpected directory entries: {entries!r}")
    if time.monotonic() >= deadline:
        sys.exit(f"temporary file cleanup timed out after 5 seconds: {entries!r}")
    time.sleep(0.05)
PY
