#!/usr/bin/env python3
"""Validate the bilingual Kessoku documentation set without network access."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[1]
WIKI = ROOT / "docs" / "wiki"
LINK = re.compile(r"(?<!!)\[[^\]]+\]\(([^)]+)\)")


def paired_documents() -> list[tuple[Path, Path]]:
    pairs = [
        (ROOT / "README.md", ROOT / "README.zh-CN.md"),
        (ROOT / "CONTAINER.md", ROOT / "CONTAINER.zh-CN.md"),
        (
            ROOT / "RELEASE-NOTES-v3.0.0.md",
            ROOT / "RELEASE-NOTES-v3.0.0.zh-CN.md",
        ),
        (
            ROOT / "MIGRATION-v3.0.0.md",
            ROOT / "MIGRATION-v3.0.0.zh-CN.md",
        ),
        (
            ROOT / ".github" / "PROJECT-METADATA.md",
            ROOT / ".github" / "PROJECT-METADATA.zh-CN.md",
        ),
        (ROOT / "WEB-CLIENT.md", ROOT / "WEB-CLIENT.zh-CN.md"),
    ]
    for english in sorted(WIKI.glob("*.md")):
        if english.name == "_Sidebar.md" or english.name.startswith("ZH-CN-"):
            continue
        pairs.append((english, english.with_name(f"ZH-CN-{english.name}")))
    return pairs


def local_target(source: Path, raw: str) -> Path | None:
    target = raw.strip().split("#", 1)[0]
    if not target or target.startswith(("http://", "https://", "mailto:")):
        return None
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1]
    return (source.parent / unquote(target)).resolve()


def main() -> int:
    errors: list[str] = []
    pairs = paired_documents()
    documents = {WIKI / "_Sidebar.md", ROOT / "README_EN.md"}

    for english, chinese in pairs:
        documents.update((english, chinese))
        if not english.is_file():
            errors.append(f"missing English document: {english.relative_to(ROOT)}")
        if not chinese.is_file():
            errors.append(f"missing Chinese document: {chinese.relative_to(ROOT)}")

    chinese_wiki = {
        page.name.removeprefix("ZH-CN-") for page in WIKI.glob("ZH-CN-*.md")
    }
    english_wiki = {
        page.name
        for page in WIKI.glob("*.md")
        if page.name != "_Sidebar.md" and not page.name.startswith("ZH-CN-")
    }
    for orphan in sorted(chinese_wiki - english_wiki):
        errors.append(f"Chinese Wiki page has no English peer: ZH-CN-{orphan}")

    for document in sorted(documents):
        if not document.is_file():
            continue
        body = document.read_text(encoding="utf-8")
        if not body.startswith("#"):
            errors.append(f"document has no leading heading: {document.relative_to(ROOT)}")
        if sum(line.startswith("```") for line in body.splitlines()) % 2:
            errors.append(f"unbalanced code fence: {document.relative_to(ROOT)}")
        for match in LINK.finditer(body):
            target = local_target(document, match.group(1))
            if target is None:
                continue
            candidates = [target]
            if not target.suffix:
                candidates.append(target.with_suffix(".md"))
            if not any(candidate.exists() for candidate in candidates):
                errors.append(
                    f"broken local link in {document.relative_to(ROOT)}: "
                    f"{match.group(1)}"
                )

    for required in (
        ROOT / "README.md",
        ROOT / "README.zh-CN.md",
        ROOT / "CONTAINER.md",
        ROOT / "CONTAINER.zh-CN.md",
        ROOT / "examples" / "compose.env.example",
    ):
        if "v3.0.0" not in required.read_text(encoding="utf-8"):
            errors.append(f"v3.0.0 is missing from {required.relative_to(ROOT)}")

    if errors:
        for error in errors:
            print(f"documentation error: {error}", file=sys.stderr)
        return 1

    print(
        f"documentation OK: {len(pairs)} bilingual pairs, "
        f"{len(documents)} Markdown files"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
