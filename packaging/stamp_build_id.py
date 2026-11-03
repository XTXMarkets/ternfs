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
import struct
from pathlib import Path
from typing import BinaryIO, Optional, Sequence, Tuple


SHT_NOTE = 7
NT_GNU_BUILD_ID = 3


def _round_up(size: int, alignment: int) -> int:
    return (size + alignment - 1) // alignment * alignment


def _find_gnu_build_id(elf: BinaryIO, path: Path) -> Tuple[int, bytes]:
    """Return the file offset and contents of the NT_GNU_BUILD_ID descriptor."""
    header = elf.read(64)
    if len(header) < 64 or header[:4] != b"\x7fELF":
        raise ValueError(f"not an ELF file: {path}")
    if header[4] != 2 or header[5] != 1:
        raise ValueError(f"not a little-endian ELF64 file: {path}")
    shoff = struct.unpack_from("<Q", header, 0x28)[0]
    shentsize, shnum = struct.unpack_from("<HH", header, 0x3A)
    for index in range(shnum):
        elf.seek(shoff + index * shentsize)
        section = elf.read(64)
        sh_type = struct.unpack_from("<I", section, 4)[0]
        if sh_type != SHT_NOTE:
            continue
        offset, size, _, _, alignment = struct.unpack_from("<QQIIQ", section, 0x18)
        padding = 8 if alignment == 8 else 4
        elf.seek(offset)
        notes = elf.read(size)
        position = 0
        while position + 12 <= len(notes):
            namesz, descsz, note_type = struct.unpack_from("<III", notes, position)
            position += 12
            name = notes[position : position + namesz]
            position += _round_up(namesz, padding)
            desc = notes[position : position + descsz]
            if name == b"GNU\0" and note_type == NT_GNU_BUILD_ID:
                return offset + position, desc
            position += _round_up(descsz, padding)
    raise ValueError(f"binary has no GNU build ID: {path}")


def gnu_build_id(path: Path) -> str:
    """Return the GNU build ID of a little-endian ELF64 file as hex."""
    with path.open("rb") as elf:
        return _find_gnu_build_id(elf, path)[1].hex()


def stamp_build_id(path: Path, build_number: int) -> str:
    """Replace the GNU build ID in place and return the new one as hex."""
    if build_number < 1:
        raise ValueError("build number must be positive")
    with path.open("r+b") as elf:
        offset, build_id = _find_gnu_build_id(elf, path)
        digest = hashlib.sha256(
            f"ternfs-build-number-{build_number}\0".encode() + build_id
        ).digest()
        if len(build_id) > len(digest):
            raise ValueError(f"build ID is longer than {len(digest)} bytes: {path}")
        stamped = digest[: len(build_id)]
        elf.seek(offset)
        elf.write(stamped)
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
