#!/usr/bin/env python3

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import Optional, Set

import build_release_deb
from stamp_build_id import gnu_build_id, stamp_build_id
from test_stamp_build_id import compile_binary


@unittest.skipUnless(shutil.which("cc"), "needs a C compiler")
class BuildReleaseDebTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.build_dir = self.root / "build"
        self.build_dir.mkdir()
        for index, name in enumerate(build_release_deb.RELEASE_BINARIES):
            compile_binary(
                self.build_dir / name,
                f"int main(void) {{ return {index}; }}\n",
            )

    def build(
        self,
        build_number: int = 42,
        revision: str = "a" * 40,
        build_dir: Optional[Path] = None,
    ) -> build_release_deb.ReleasePackages:
        return build_release_deb.build_release_deb(
            build_dir=build_dir or self.build_dir,
            output_dir=self.root / "dist",
            build_number=build_number,
            revision=revision,
            version="0.1.0",
            created_epoch=1_700_000_000,
        )

    def test_package_contains_versioned_binaries_and_manifest(self) -> None:
        package = self.build().package
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
            self.assertEqual(
                subprocess.run([binary]).returncode,
                build_release_deb.RELEASE_BINARIES.index(entry["name"]),
            )

    def test_binaries_are_stripped_into_debug_package(self) -> None:
        packages = self.build()
        package_name = "ternfs-release-r42-gaaaaaaaaaaaa"
        self.assertEqual(
            packages.debug_package.name,
            f"{package_name}-dbgsym_0.1.0-1_amd64.ddeb",
        )
        build_ids = [
            gnu_build_id(self.build_dir / name)
            for name in build_release_deb.RELEASE_BINARIES
        ]

        def field(name: str) -> str:
            return subprocess.check_output(
                ["dpkg-deb", "--field", packages.debug_package, name],
                text=True,
            ).strip()

        self.assertEqual(field("Package"), f"{package_name}-dbgsym")
        self.assertEqual(field("Version"), "0.1.0-1")
        self.assertEqual(field("Depends"), f"{package_name} (= 0.1.0-1)")
        self.assertEqual(field("Section"), "debug")
        self.assertEqual(field("Auto-Built-Package"), "debug-symbols")
        self.assertEqual(field("Build-Ids").split(), sorted(build_ids))

        extracted = self.root / "extracted"
        for package in packages:
            subprocess.run(["dpkg-deb", "--extract", package, extracted], check=True)
        release_dir = Path("usr/lib/ternfs/releases/r42-gaaaaaaaaaaaa/bin")
        for name, build_id in zip(build_release_deb.RELEASE_BINARIES, build_ids):
            binary = extracted / release_dir / name
            debug_name = f"{build_id[2:]}.debug"
            debug_file = extracted / "usr/lib/debug/.build-id" / build_id[:2] / debug_name
            self.assertEqual(gnu_build_id(binary), build_id)
            self.assertEqual(gnu_build_id(debug_file), build_id)
            binary_sections = subprocess.check_output(
                ["readelf", "--wide", "--section-headers", binary],
                text=True,
            )
            self.assertNotIn(".debug_info", binary_sections)
            self.assertNotIn(".symtab", binary_sections)
            debuglink = subprocess.check_output(
                ["readelf", "--string-dump=.gnu_debuglink", binary],
            )
            self.assertIn(f"[     0]  {debug_name}\n".encode(), debuglink)
            debug_sections = subprocess.check_output(
                ["readelf", "--wide", "--section-headers", debug_file],
                text=True,
                stderr=subprocess.DEVNULL,
            )
            self.assertIn(".debug_info", debug_sections)
            self.assertIn(".symtab", debug_sections)

    def test_binary_without_build_id_fails_before_creating_package(self) -> None:
        compile_binary(
            self.build_dir / "ternweb",
            "int main(void) { return 0; }\n",
            build_id="none",
        )
        with self.assertRaisesRegex(ValueError, "no GNU build ID.*ternweb"):
            self.build()
        self.assertEqual(list((self.root / "dist").iterdir()), [])

    def test_duplicate_build_id_fails_before_creating_package(self) -> None:
        shutil.copy2(self.build_dir / "ternshard", self.build_dir / "ternweb")
        with self.assertRaisesRegex(ValueError, "duplicate build ID.*ternweb"):
            self.build()
        self.assertEqual(list((self.root / "dist").iterdir()), [])

    def test_different_build_numbers_are_coinstallable(self) -> None:
        # Both builds contain the same binaries, as happens when a change does
        # not touch them. Only stamping keeps their build IDs apart.
        build_dirs = []
        for build_number in (42, 43):
            build_dir = self.root / f"build-{build_number}"
            shutil.copytree(self.build_dir, build_dir)
            for name in build_release_deb.RELEASE_BINARIES:
                stamp_build_id(build_dir / name, build_number)
            build_dirs.append(build_dir)
        first = self.build(42, "a" * 40, build_dirs[0])
        second = self.build(43, "a" * 40, build_dirs[1])

        for first_package, second_package in zip(first, second):
            self.assertNotEqual(first_package.name, second_package.name)
            self.assertNotEqual(
                subprocess.check_output(
                    ["dpkg-deb", "--field", first_package, "Package"]
                ),
                subprocess.check_output(
                    ["dpkg-deb", "--field", second_package, "Package"]
                ),
            )
            first_contents = self.contents(first_package)
            self.assertTrue(first_contents)
            self.assertEqual(first_contents & self.contents(second_package), set())

        self.assertIn(
            "./usr/lib/ternfs/releases/r42-gaaaaaaaaaaaa/bin/ternshard",
            self.contents(first.package),
        )
        build_id = gnu_build_id(build_dirs[1] / "ternshard")
        self.assertIn(
            f"./usr/lib/debug/.build-id/{build_id[:2]}/{build_id[2:]}.debug",
            self.contents(second.debug_package),
        )

    def test_unstamped_builds_share_debug_files(self) -> None:
        # This is why packaged builds need distinct `build.sh --build-number`s.
        first = self.build(42, "a" * 40)
        second = self.build(43, "b" * 40)
        self.assertEqual(
            self.contents(first.package) & self.contents(second.package),
            set(),
        )
        self.assertEqual(
            self.contents(first.debug_package),
            self.contents(second.debug_package),
        )

    def contents(self, package: Path) -> Set[str]:
        listing = subprocess.check_output(
            ["dpkg-deb", "--contents", package],
            text=True,
        )
        return {
            line.split()[-1]
            for line in listing.splitlines()
            if not line.startswith("d")
        }

    def test_packages_have_no_maintainer_scripts(self) -> None:
        for index, package in enumerate(self.build()):
            control_dir = self.root / f"control-{index}"
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
        for first_package, second_package in zip(first, second):
            self.assertEqual(first_package.read_bytes(), second_package.read_bytes())

    def test_missing_binary_fails_before_creating_package(self) -> None:
        (self.build_dir / "ternweb").unlink()
        with self.assertRaisesRegex(FileNotFoundError, "ternweb"):
            self.build()
        self.assertEqual(list((self.root / "dist").iterdir()), [])

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
