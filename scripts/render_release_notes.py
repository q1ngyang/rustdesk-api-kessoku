#!/usr/bin/env python3
"""Render release-note links from their source directory, without publishing."""

from __future__ import annotations

import argparse
from pathlib import Path
import sys
from urllib.parse import quote, unquote, urlsplit

from check_docs import LINK, ROOT, markdown_links


def render(body: str, source: Path, repository: str, ref: str, root: Path = ROOT) -> str:
    linked_lines = {number for number, _ in markdown_links(body)}

    def replace(match):
        raw = match.group(1)
        url = urlsplit(raw)
        if url.scheme or url.netloc or raw.startswith("#"):
            return match.group(0)
        local = (source.parent / unquote(url.path)).resolve()
        relative = local.relative_to(root).as_posix()
        if not local.exists():
            raise ValueError(f"missing release-note link target: {raw}")
        base = f"https://github.com/{repository}"
        if local.parent == root / "docs" / "wiki" and local.suffix == ".md":
            target = f"{base}/wiki/{quote(local.stem)}"
        else:
            kind = "tree" if local.is_dir() else "blob"
            target = f"{base}/{kind}/{quote(ref, safe='')}/{quote(relative)}"
        if url.query:
            target += "?" + url.query
        if url.fragment:
            target += "#" + url.fragment
        start, end = match.start(1) - match.start(), match.end(1) - match.start()
        return match.group(0)[:start] + target + match.group(0)[end:]

    return "".join(
        LINK.sub(replace, line) if number in linked_lines else line
        for number, line in enumerate(body.splitlines(keepends=True), 1)
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--ref", required=True)
    args = parser.parse_args()
    source = args.source.resolve()
    try:
        source.relative_to(ROOT)
        sys.stdout.write(render(source.read_text(encoding="utf-8"), source, args.repository, args.ref))
    except (OSError, ValueError) as error:
        print(f"release-note error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
