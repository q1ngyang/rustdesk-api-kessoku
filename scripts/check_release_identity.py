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

    migration = release_dir / "migration.yaml"
    if not migration.is_file():
        errors.append(f"missing release metadata: docs/releases/{tag}/migration.yaml")
    else:
        body = migration.read_text(encoding="utf-8")
        required_metadata = (
            "schema: 1\n",
            "component: kessoku\n",
            f"version: {version}\n",
            "  from: [313]\n",
            "  to: 313\n",
            "  - 3.0.7\n",
            "  to: 3.0.7\n",
            "  mode: in-place-compatible\n",
            '  starry: ">=1.1.16-patch-v1.2.2"\n',
            "  starry_peer_registry_capability: 1\n",
            '  starry: ">=1.1.16-patch-v1.3.0"\n',
            "  starry_peer_registry_capability: 2\n",
            '    starry: "1.1.16-patch-v1.3.1"\n',
            "  starry_contract_commit: 6f5a31008ab7761d8557c8cf9fefcb5be11c49e6\n",
            "  starry_runtime_source_commit: 1b8080bf074e3236cf9a3c0dfae2bdf16832249e\n",
            "  starry_release_channel: preview\n",
            "  starry_release_summary_sha256: fedeb47ff77bdbc594ddd3ba5b54238a469b02416cfb3410dbd535eff9c7e0ef\n",
            "  config_schema: 5\n",
            "  registry_schema: 1\n",
        )
        for required in required_metadata:
            if required not in body:
                errors.append(
                    f"docs/releases/{tag}/migration.yaml is missing {required.strip()!r}"
                )
        capabilities = {
            line.removeprefix("  - ")
            for line in body.splitlines()
            if line.startswith("  - ")
        }
        expected_capabilities = {
            "version-json",
            "config-validate",
            "database-status",
            "database-migrate",
            "recover-admin",
            "reset-two-factor",
            "presence-lease-v2",
            "server-control-sp1-v1",
            "relay-fast-compat-v1",
            "relay-fast-media-v1-observation",
            "relay-enrollment-v1",
            "static-control-export-v1",
        }
        if not expected_capabilities.issubset(capabilities):
            errors.append(
                f"docs/releases/{tag}/migration.yaml capabilities are incomplete"
            )

    for path in (
        ROOT / "CHANGELOG.md",
        ROOT / "CHANGELOG.zh-CN.md",
        ROOT / "docs/operations/LOCAL-MAINTENANCE-CLI.md",
        ROOT / "docs/operations/LOCAL-MAINTENANCE-CLI.zh-CN.md",
    ):
        if not path.is_file():
            errors.append(f"missing release support document: {path.relative_to(ROOT)}")

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
