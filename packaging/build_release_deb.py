#!/usr/bin/env python3

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""Build an immutable, co-installable TernFS userspace release package."""

import argparse
import datetime
import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import tempfile
from pathlib import Path
from typing import Dict, List, Optional, Sequence


RELEASE_BINARIES = (
    "ternblocks",
    "terncdc",
    "terncli",
    "terndbtools",
    "terngc",
    "ternktools",
    "ternregistry",
    "ternregistryproxy",
    "ternshard",
    "ternweb",
)

REVISION_RE = re.compile(r"^[0-9a-f]{7,40}$")
PROJECT_VERSION_RE = re.compile(r"^[0-9]+(?:\.[0-9]+){2}$")
ARCHITECTURE_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
BUILD_VARIANT_RE = re.compile(r"^[a-z0-9][a-z0-9.+-]*$")


def _run_git(repo_dir: Path, *args: str) -> str:
    return subprocess.check_output(
        ["git", "-C", str(repo_dir), *args],
        text=True,
    ).strip()


def git_revision(repo_dir: Path) -> str:
    revision = _run_git(repo_dir, "rev-parse", "HEAD")
    if not REVISION_RE.fullmatch(revision):
        raise ValueError(f"invalid git revision: {revision!r}")
    return revision


def require_clean_checkout(repo_dir: Path) -> None:
    status = _run_git(repo_dir, "status", "--porcelain")
    if status:
        raise RuntimeError("refusing to package a dirty checkout")


def project_version(repo_dir: Path) -> str:
    version = _run_git(repo_dir, "describe", "--tags", "--abbrev=0")
    if version.startswith("v"):
        version = version[1:]
    if not PROJECT_VERSION_RE.fullmatch(version):
        raise ValueError(f"git tag is not a supported release version: {version!r}")
    return version


def release_id(build_number: int, revision: str) -> str:
    if build_number < 1:
        raise ValueError("build number must be positive")
    if not REVISION_RE.fullmatch(revision):
        raise ValueError(f"invalid source revision: {revision!r}")
    return f"r{build_number}-g{revision[:12]}"


def package_version(version: str) -> str:
    if not PROJECT_VERSION_RE.fullmatch(version):
        raise ValueError(f"invalid project version: {version!r}")
    return f"{version}-1"


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _copy_binaries(
    build_dir: Path,
    destination: Path,
) -> List[Dict[str, object]]:
    result = []
    destination.mkdir(parents=True)
    for name in RELEASE_BINARIES:
        source = build_dir / name
        if not source.is_file():
            raise FileNotFoundError(f"release binary not found: {source}")
        if not os.access(source, os.X_OK):
            raise PermissionError(f"release binary is not executable: {source}")

        target = destination / name
        shutil.copy2(source, target)
        target.chmod(
            stat.S_IRUSR
            | stat.S_IWUSR
            | stat.S_IXUSR
            | stat.S_IRGRP
            | stat.S_IXGRP
            | stat.S_IROTH
            | stat.S_IXOTH
        )
        result.append(
            {
                "name": name,
                "sha256": _sha256(target),
                "size": target.stat().st_size,
            }
        )
    return result


def _installed_size_kib(package_root: Path) -> int:
    size = sum(path.stat().st_size for path in package_root.rglob("*") if path.is_file())
    return max(1, (size + 1023) // 1024)


def _normalize_mtimes(package_root: Path, created_epoch: int) -> None:
    for path in [package_root] + list(package_root.rglob("*")):
        os.utime(
            path,
            (created_epoch, created_epoch),
            follow_symlinks=False,
        )


def _write_control(
    package_root: Path,
    name: str,
    version: str,
    architecture: str,
) -> None:
    control_dir = package_root / "DEBIAN"
    control_dir.mkdir()
    control = (
        f"Package: {name}\n"
        f"Version: {version}\n"
        f"Architecture: {architecture}\n"
        "Section: admin\n"
        "Priority: optional\n"
        "Maintainer: TernFS Authors <ternfs@users.noreply.github.com>\n"
        f"Installed-Size: {_installed_size_kib(package_root)}\n"
        "Description: immutable TernFS userspace release\n"
        " Co-installable server binaries and administrative tools for one\n"
        " source revision. Installing this package does not activate services.\n"
    )
    (control_dir / "control").write_text(control, encoding="utf-8")


def build_release_deb(
    *,
    build_dir: Path,
    output_dir: Path,
    build_number: int,
    revision: str,
    version: str,
    architecture: str = "amd64",
    build_variant: str = "alpine",
    created_epoch: Optional[int] = None,
) -> Path:
    if not ARCHITECTURE_RE.fullmatch(architecture):
        raise ValueError(f"invalid Debian architecture: {architecture!r}")
    if not BUILD_VARIANT_RE.fullmatch(build_variant):
        raise ValueError(f"invalid build variant: {build_variant!r}")
    rid = release_id(build_number, revision)
    deb_version = package_version(version)
    package_name = f"ternfs-release-{rid}"
    output_dir.mkdir(parents=True, exist_ok=True)
    output = output_dir / f"{package_name}_{deb_version}_{architecture}.deb"
    if output.exists():
        raise FileExistsError(f"refusing to replace existing package: {output}")

    if created_epoch is None:
        created_epoch = int(os.environ.get("SOURCE_DATE_EPOCH", "0"))
    created = datetime.datetime.fromtimestamp(
        created_epoch,
        tz=datetime.timezone.utc,
    ).isoformat()

    with tempfile.TemporaryDirectory(prefix="ternfs-release-") as temporary:
        package_root = Path(temporary) / "root"
        release_root = package_root / "usr/lib/ternfs/releases" / rid
        binaries = _copy_binaries(build_dir, release_root / "bin")
        manifest = {
            "schema_version": 1,
            "release_id": rid,
            "source_revision": revision,
            "build_number": build_number,
            "build_variant": build_variant,
            "created": created,
            "package": {
                "name": package_name,
                "version": deb_version,
                "architecture": architecture,
            },
            "binaries": binaries,
        }
        (release_root / "release.json").write_text(
            json.dumps(manifest, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        _write_control(package_root, package_name, deb_version, architecture)
        _normalize_mtimes(package_root, created_epoch)
        environment = os.environ.copy()
        environment["SOURCE_DATE_EPOCH"] = str(created_epoch)
        subprocess.run(
            [
                "dpkg-deb",
                "--root-owner-group",
                "-Zgzip",
                "-z6",
                "--build",
                str(package_root),
                str(output),
            ],
            check=True,
            env=environment,
        )
    return output


def _parse_args(argv: Optional[Sequence[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Build an immutable TernFS userspace release .deb",
    )
    parser.add_argument("--build-number", required=True, type=int)
    parser.add_argument("--build-dir", type=Path, default=Path("build/alpine"))
    parser.add_argument("--output-dir", type=Path, default=Path("dist"))
    parser.add_argument("--architecture", default="amd64")
    parser.add_argument("--build-variant", default="alpine")
    return parser.parse_args(argv)


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = _parse_args(argv)
    repo_dir = Path(__file__).resolve().parent.parent
    require_clean_checkout(repo_dir)
    revision = git_revision(repo_dir)
    created_epoch = int(_run_git(repo_dir, "show", "-s", "--format=%ct", revision))
    output = build_release_deb(
        build_dir=args.build_dir.resolve(),
        output_dir=args.output_dir.resolve(),
        build_number=args.build_number,
        revision=revision,
        version=project_version(repo_dir),
        architecture=args.architecture,
        build_variant=args.build_variant,
        created_epoch=created_epoch,
    )
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
