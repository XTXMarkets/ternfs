#!/usr/bin/env python3

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""Read and stamp the GNU build ID of TernFS binaries.

A binary that a change does not touch is byte-identical across builds, so the
linker gives it the same GNU build ID. Debug symbol packages install files at
paths derived from the build ID, and identical paths stop packages for
different builds from being installed together. Stamping replaces each build
ID with a hash of the build number and the linker's build ID, which keeps it
unique to both the binary and the build. This is what rpm's
`debugedit --build-id-seed` does for the same reason.
"""

import argparse
import hashlib
import subprocess
import tempfile
from pathlib import Path
from typing import Optional, Sequence


BUILD_ID_SECTION = ".note.gnu.build-id"
# The note is namesz, descsz and type (4 bytes each), the name "GNU\0", and
# then the build ID.
BUILD_ID_OFFSET = 16


def _read_note(path: Path, note: Path) -> bytes:
    subprocess.run(
        [
            "objcopy",
            "--output-target=binary",
            f"--only-section={BUILD_ID_SECTION}",
            str(path),
            str(note),
        ],
        check=True,
    )
    contents = note.read_bytes()
    if len(contents) <= BUILD_ID_OFFSET or contents[12:BUILD_ID_OFFSET] != b"GNU\0":
        raise ValueError(f"binary has no GNU build ID: {path}")
    return contents


def gnu_build_id(path: Path) -> str:
    """Return the GNU build ID of an ELF file as hex."""
    with tempfile.TemporaryDirectory(prefix="ternfs-build-id-") as temporary:
        note = _read_note(path, Path(temporary) / "note")
    return note[BUILD_ID_OFFSET:].hex()


def stamp_build_id(path: Path, build_number: int) -> str:
    """Replace the GNU build ID in place and return the new one as hex."""
    if build_number < 1:
        raise ValueError("build number must be positive")
    with tempfile.TemporaryDirectory(prefix="ternfs-build-id-") as temporary:
        note_path = Path(temporary) / "note"
        note = _read_note(path, note_path)
        build_id = note[BUILD_ID_OFFSET:]
        digest = hashlib.sha256(
            f"ternfs-build-number-{build_number}\0".encode() + build_id
        ).digest()
        if len(build_id) > len(digest):
            raise ValueError(f"build ID is longer than {len(digest)} bytes: {path}")
        stamped = digest[: len(build_id)]
        note_path.write_bytes(note[:BUILD_ID_OFFSET] + stamped)
        subprocess.run(
            [
                "objcopy",
                f"--update-section={BUILD_ID_SECTION}={note_path}",
                str(path),
            ],
            check=True,
        )
    return stamped.hex()


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        description="Stamp a build number into the GNU build ID of binaries",
    )
    parser.add_argument("--build-number", required=True, type=int)
    parser.add_argument("binaries", nargs="+", type=Path)
    args = parser.parse_args(argv)
    for binary in args.binaries:
        stamp_build_id(binary, args.build_number)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
