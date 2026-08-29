#!/usr/bin/env python3
"""Fail closed when release identity drifts across source or workflows."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

from check_docs import ROOT, current_release_tag


def main() -> int:
    errors: list[str] = []
    try:
        tag = current_release_tag()
    except ValueError as error:
        print(f"release identity error: {error}", file=sys.stderr)
        return 1

    version = tag.removeprefix("v")
    exact_files = {
        ROOT / "resources/version": f"{tag}\n",
    }
    for path, expected in exact_files.items():
        if path.read_text(encoding="utf-8") != expected:
            errors.append(f"{path.relative_to(ROOT)} must equal {expected.strip()}")

    package = json.loads((ROOT / "admin-web/package.json").read_text(encoding="utf-8"))
    package_lock = json.loads(
        (ROOT / "admin-web/package-lock.json").read_text(encoding="utf-8")
    )
    for label, actual in (
        ("admin-web/package.json", package.get("version")),
        ("admin-web/package-lock.json", package_lock.get("version")),
        (
            "admin-web/package-lock.json root package",
            package_lock.get("packages", {}).get("", {}).get("version"),
        ),
    ):
        if actual != version:
            errors.append(f"{label} version is {actual!r}, expected {version!r}")

    required_mentions = (
        ROOT / "README.md",
        ROOT / "README.zh-CN.md",
        ROOT / "docker-compose.yaml",
        ROOT / "examples/compose.env.example",
        ROOT / "examples/combined/compose.yaml",
        ROOT / ".github/PROJECT-METADATA.md",
        ROOT / ".github/PROJECT-METADATA.zh-CN.md",
    )
    for path in required_mentions:
        if tag not in path.read_text(encoding="utf-8"):
            errors.append(f"{path.relative_to(ROOT)} does not name {tag}")

    release_dir = ROOT / "docs/releases" / tag
    for name in (
        f"RELEASE-NOTES-{tag}.md",
        f"RELEASE-NOTES-{tag}.zh-CN.md",
        f"MIGRATION-{tag}.md",
        f"MIGRATION-{tag}.zh-CN.md",
    ):
        if not (release_dir / name).is_file():
            errors.append(f"missing release document: docs/releases/{tag}/{name}")

    first_changelog_line = (ROOT / "debian/changelog").read_text(
        encoding="utf-8"
    ).splitlines()[0]
    if not first_changelog_line.startswith(f"rustdesk-api-kessoku ({version}-1) "):
        errors.append(f"debian/changelog does not begin with {version}-1")

    for relative in (".github/workflows/build.yml", ".github/workflows/release.yml"):
        workflow = (ROOT / relative).read_text(encoding="utf-8")
        if re.search(r'test "\$release_tag" = v\d+\.\d+\.\d+', workflow):
            errors.append(f"{relative} hard-codes a release tag")
        if re.search(r"docs/releases/v\d+\.\d+\.\d+/RELEASE-NOTES-v", workflow):
            errors.append(f"{relative} hard-codes a release-document directory")

    if errors:
        for error in errors:
            print(f"release identity error: {error}", file=sys.stderr)
        return 1
    print(f"release identity OK: {tag}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
