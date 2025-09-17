#!/usr/bin/env python3

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""
Utility for importing binaries into vm for tests
"""
import argparse
import subprocess
from pathlib import Path

REPO_DIR = Path(__file__).parent.parent


def build_and_upload(build_type: str) -> None:
    """Builds and uploads binaries to the vm."""
    subprocess.run([str(REPO_DIR / "build.sh"), build_type], check=True)
    test_binaries = [
        f"build/{build_type}/ternshard",
        f"build/{build_type}/terncdc",
        f"build/{build_type}/ternregistry",
        f"build/{build_type}/ternrun",
        f"build/{build_type}/ternblocks",
        f"build/{build_type}/ternfuse",
        f"build/{build_type}/terngc",
        f"build/{build_type}/terntests",
        f"build/{build_type}/terndbtools",
    ]

    subprocess.run(
        [
            "rsync",
            "-p",
            "--quiet",
            "-e",
            "ssh -p 2223 -i ../kmod/image-key -l fmazzol",
        ]
        + [str(REPO_DIR / f) for f in test_binaries]
        + ["localhost:tern/"],
        check=True,
    )


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--build-type", choices=["alpine", "alpinedebug"], default="alpine"
    )
    args = parser.parse_args()
    build_and_upload(args.build_type)
