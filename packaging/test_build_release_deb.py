#!/usr/bin/env python3

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import hashlib
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import build_release_deb


class BuildReleaseDebTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.build_dir = self.root / "build"
        self.build_dir.mkdir()
        for index, name in enumerate(build_release_deb.RELEASE_BINARIES):
            binary = self.build_dir / name
            binary.write_bytes(f"fake binary {index}: {name}\n".encode())
            binary.chmod(0o755)

    def build(self, build_number: int = 42, revision: str = "a" * 40) -> Path:
        return build_release_deb.build_release_deb(
            build_dir=self.build_dir,
            output_dir=self.root / "dist",
            build_number=build_number,
            revision=revision,
            version="0.1.0",
            created_epoch=1_700_000_000,
        )

    def test_package_contains_versioned_binaries_and_manifest(self) -> None:
        package = self.build()
        self.assertEqual(
            package.name,
            "ternfs-release-r42-gaaaaaaaaaaaa_0.1.0-1_amd64.deb",
        )
        self.assertEqual(
            subprocess.check_output(
                ["dpkg-deb", "--field", package, "Package"],
                text=True,
            ).strip(),
            "ternfs-release-r42-gaaaaaaaaaaaa",
        )
        self.assertEqual(
            subprocess.check_output(
                ["dpkg-deb", "--field", package, "Version"],
                text=True,
            ).strip(),
            "0.1.0-1",
        )

        extracted = self.root / "extracted"
        subprocess.run(["dpkg-deb", "--extract", package, extracted], check=True)
        release_root = (
            extracted / "usr/lib/ternfs/releases/r42-gaaaaaaaaaaaa"
        )
        manifest = json.loads((release_root / "release.json").read_text())
        self.assertEqual(manifest["schema_version"], 1)
        self.assertEqual(manifest["source_revision"], "a" * 40)
        self.assertFalse(manifest["source_dirty"])
        self.assertEqual(manifest["build_variant"], "alpine")
        self.assertEqual(manifest["build_number"], 42)
        self.assertEqual(manifest["package"]["version"], "0.1.0-1")
        self.assertEqual(
            [entry["name"] for entry in manifest["binaries"]],
            list(build_release_deb.RELEASE_BINARIES),
        )

        for entry in manifest["binaries"]:
            binary = release_root / "bin" / entry["name"]
            self.assertTrue(os.access(binary, os.X_OK))
            self.assertEqual(
                hashlib.sha256(binary.read_bytes()).hexdigest(),
                entry["sha256"],
            )

    def test_different_releases_are_coinstallable_by_path_and_package_name(self) -> None:
        first = self.build(build_number=42, revision="a" * 40)
        second = self.build(build_number=42, revision="b" * 40)
        self.assertNotEqual(first.name, second.name)
        self.assertNotEqual(
            subprocess.check_output(["dpkg-deb", "--field", first, "Package"]),
            subprocess.check_output(["dpkg-deb", "--field", second, "Package"]),
        )

        first_contents = subprocess.check_output(
            ["dpkg-deb", "--contents", first],
            text=True,
        )
        second_contents = subprocess.check_output(
            ["dpkg-deb", "--contents", second],
            text=True,
        )
        self.assertIn("/r42-gaaaaaaaaaaaa/bin/ternshard", first_contents)
        self.assertIn("/r42-gbbbbbbbbbbbb/bin/ternshard", second_contents)
        self.assertNotIn("/r42-gaaaaaaaaaaaa/", second_contents)

    def test_package_has_no_maintainer_scripts(self) -> None:
        package = self.build()
        control_dir = self.root / "control"
        subprocess.run(
            ["dpkg-deb", "--control", package, control_dir],
            check=True,
        )
        self.assertEqual(
            sorted(path.name for path in control_dir.iterdir()),
            ["control"],
        )

    def test_same_release_inputs_produce_identical_package(self) -> None:
        first = self.build()
        second = build_release_deb.build_release_deb(
            build_dir=self.build_dir,
            output_dir=self.root / "other-dist",
            build_number=42,
            revision="a" * 40,
            version="0.1.0",
            created_epoch=1_700_000_000,
        )
        self.assertEqual(first.read_bytes(), second.read_bytes())

    def test_missing_binary_fails_before_creating_package(self) -> None:
        (self.build_dir / "ternweb").unlink()
        with self.assertRaisesRegex(FileNotFoundError, "ternweb"):
            self.build()
        self.assertEqual(list((self.root / "dist").iterdir()), [])

    def test_failed_dpkg_does_not_publish_a_partial_package(self) -> None:
        def fail_after_writing(args, **kwargs):
            Path(args[-1]).write_bytes(b"partial package")
            raise subprocess.CalledProcessError(1, args)

        with (
            patch.object(build_release_deb, "_normalize_mtimes"),
            patch.object(build_release_deb.subprocess, "run", side_effect=fail_after_writing),
        ):
            with self.assertRaises(subprocess.CalledProcessError):
                self.build()
        self.assertEqual(list((self.root / "dist").iterdir()), [])

    def test_publish_preserves_package_created_during_build(self) -> None:
        output = self.root / "dist" / "ternfs-release-r42-gaaaaaaaaaaaa_0.1.0-1_amd64.deb"

        def finish_after_competing_publish(args, **kwargs):
            output.write_bytes(b"first publisher")
            Path(args[-1]).write_bytes(b"second publisher")

        with (
            patch.object(build_release_deb, "_normalize_mtimes"),
            patch.object(build_release_deb.subprocess, "run", side_effect=finish_after_competing_publish),
        ):
            with self.assertRaises(FileExistsError):
                self.build()
        self.assertEqual(output.read_bytes(), b"first publisher")
        self.assertEqual(list(output.parent.iterdir()), [output])

    def test_release_identity_rejects_invalid_inputs(self) -> None:
        with self.assertRaisesRegex(ValueError, "positive"):
            build_release_deb.release_id(0, "a" * 40)
        with self.assertRaisesRegex(ValueError, "revision"):
            build_release_deb.release_id(1, "not-a-revision")
        with self.assertRaisesRegex(ValueError, "project version"):
            build_release_deb.package_version("main")
        with self.assertRaisesRegex(ValueError, "architecture"):
            build_release_deb.build_release_deb(
                build_dir=self.build_dir,
                output_dir=self.root / "dist",
                build_number=1,
                revision="a" * 40,
                version="0.1.0",
                architecture="amd64\nDepends: bad",
            )
        with self.assertRaisesRegex(ValueError, "build variant"):
            build_release_deb.build_release_deb(
                build_dir=self.build_dir,
                output_dir=self.root / "dist",
                build_number=1,
                revision="a" * 40,
                version="0.1.0",
                build_variant="alpine\nuntrusted",
            )


if __name__ == "__main__":
    unittest.main()
