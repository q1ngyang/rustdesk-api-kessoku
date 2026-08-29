#!/usr/bin/env python3
"""Verify that every immutable GitHub Actions pin resolves to a real commit."""

from __future__ import annotations

import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
WORKFLOWS = ROOT / ".github" / "workflows"
ACTION_PIN = re.compile(
    r"^\s*(?:-\s*)?uses:\s+"
    r"([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)"
    r"(?:/[^@\s]+)?@([0-9a-f]{40})(?:\s+#.*)?$",
    re.MULTILINE,
)


def request_commit(repository: str, revision: str, token: str) -> str:
    url = f"https://api.github.com/repos/{repository}/git/commits/{revision}"
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "rustdesk-api-kessoku-release-gate",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    for attempt in range(3):
        request = urllib.request.Request(url, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                payload = json.load(response)
            return str(payload.get("sha", ""))
        except urllib.error.HTTPError as error:
            if error.code < 500 or attempt == 2:
                raise RuntimeError(
                    f"{repository}@{revision} does not resolve ({error.code})"
                ) from error
        except (TimeoutError, urllib.error.URLError) as error:
            if attempt == 2:
                raise RuntimeError(
                    f"could not verify {repository}@{revision}: {error}"
                ) from error
        time.sleep(attempt + 1)
    raise AssertionError("unreachable")


def main() -> int:
    pins: set[tuple[str, str]] = set()
    for workflow in sorted(WORKFLOWS.glob("*.yml")):
        pins.update(ACTION_PIN.findall(workflow.read_text(encoding="utf-8")))
    if not pins:
        raise RuntimeError("no immutable GitHub Actions pins found")

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN", "")
    for repository, revision in sorted(pins):
        resolved = request_commit(repository, revision, token)
        if resolved != revision:
            raise RuntimeError(
                f"{repository}@{revision} resolved to unexpected commit {resolved}"
            )
    print(f"GitHub Actions pins OK: {len(pins)} immutable commits")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as error:
        print(f"action pin check failed: {error}", file=sys.stderr)
        raise SystemExit(1) from error
