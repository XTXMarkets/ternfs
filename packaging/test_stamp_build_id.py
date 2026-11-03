#!/usr/bin/env python3

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import stamp_build_id


def compile_binary(path: Path, source: str, build_id: str = "sha1") -> None:
    source_path = path.with_suffix(".c")
    source_path.write_text(source)
    subprocess.run(
        ["cc", "-g", "-O0", f"-Wl,--build-id={build_id}", "-o", path, source_path],
        check=True,
    )
    source_path.unlink()


def readelf_build_id(path: Path) -> str:
    notes = subprocess.check_output(
        ["readelf", "--wide", "--notes", path],
        text=True,
        env={"LC_ALL": "C"},
    )
    for line in notes.splitlines():
        if "Build ID:" in line:
            return line.split()[-1]
    raise AssertionError(f"readelf found no build ID in {path}")


@unittest.skipUnless(shutil.which("cc"), "needs a C compiler")
class StampBuildIdTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.binary = self.root / "binary"
        compile_binary(self.binary, "int main(void) { return 7; }\n")

    def copy(self, name: str) -> Path:
        path = self.root / name
        shutil.copy2(self.binary, path)
        return path

    def test_reads_build_id(self) -> None:
        self.assertEqual(
            stamp_build_id.gnu_build_id(self.binary),
            readelf_build_id(self.binary),
        )

    def test_stamp_depends_only_on_build_number_and_original_build_id(self) -> None:
        original = stamp_build_id.gnu_build_id(self.binary)
        first = self.copy("first")
        again = self.copy("again")
        other = self.copy("other")

        stamped = stamp_build_id.stamp_build_id(first, 42)
        self.assertEqual(stamp_build_id.stamp_build_id(again, 42), stamped)
        self.assertNotEqual(stamp_build_id.stamp_build_id(other, 43), stamped)
        self.assertNotEqual(stamped, original)
        self.assertEqual(len(stamped), len(original))
        self.assertEqual(readelf_build_id(first), stamped)
        self.assertEqual(first.read_bytes(), again.read_bytes())

    def test_stamp_changes_only_the_build_id(self) -> None:
        stamped = self.copy("stamped")
        stamp_build_id.stamp_build_id(stamped, 42)
        original_bytes = self.binary.read_bytes()
        stamped_bytes = stamped.read_bytes()
        self.assertEqual(len(original_bytes), len(stamped_bytes))
        differing = [
            index
            for index, (a, b) in enumerate(zip(original_bytes, stamped_bytes))
            if a != b
        ]
        self.assertTrue(differing)
        build_id_length = len(stamp_build_id.gnu_build_id(self.binary)) // 2
        self.assertLess(differing[-1] - differing[0], build_id_length)
        self.assertEqual(subprocess.run([stamped]).returncode, 7)

    def test_command_line_stamps_every_binary(self) -> None:
        first = self.copy("first")
        second = self.copy("second")
        stamp_build_id.main(["--build-number", "42", str(first), str(second)])
        expected = stamp_build_id.stamp_build_id(self.copy("expected"), 42)
        self.assertEqual(stamp_build_id.gnu_build_id(first), expected)
        self.assertEqual(stamp_build_id.gnu_build_id(second), expected)

    def test_rejects_invalid_inputs(self) -> None:
        with self.assertRaisesRegex(ValueError, "positive"):
            stamp_build_id.stamp_build_id(self.copy("zero"), 0)
        no_build_id = self.root / "no-build-id"
        compile_binary(no_build_id, "int main(void) { return 0; }\n", build_id="none")
        with self.assertRaisesRegex(ValueError, "no GNU build ID"):
            stamp_build_id.stamp_build_id(no_build_id, 1)
        text = self.root / "text"
        text.write_text("#!/bin/sh\n")
        with self.assertRaisesRegex(ValueError, "not an ELF file"):
            stamp_build_id.gnu_build_id(text)


if __name__ == "__main__":
    unittest.main()
