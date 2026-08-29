#!/usr/bin/env python3
"""Validate the bilingual Kessoku documentation set without network access."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[1]
WIKI = ROOT / "docs" / "wiki"
REPOSITORY = "q1ngyang/rustdesk-api-kessoku"
LINK = re.compile(r"!?\[[^\]\n]*\]\(([^)\n]+)\)")
FENCE = re.compile(r"^\s{0,3}(`{3,}|~{3,})")


def markdown_links(body: str) -> list[tuple[int, str]]:
    """Inspect rendered links, not commands or Markdown examples in fences."""
    links = []
    fence = ""
    for number, line in enumerate(body.splitlines(), 1):
        marker = FENCE.match(line)
        if marker:
            value = marker.group(1)
            if not fence:
                fence = value
            elif value[0] == fence[0] and len(value) >= len(fence):
                fence = ""
            continue
        if not fence:
            links.extend((number, match.group(1)) for match in LINK.finditer(line))
    return links


def link_error(source: Path, raw: str, root: Path = ROOT) -> str | None:
    target = raw.strip().removeprefix("<").removesuffix(">")
    if not target or target.startswith("#"):
        return None
    url = urlsplit(target)
    path = unquote(url.path)
    wiki_prefix = f"/{REPOSITORY}/wiki/"
    if url.netloc == "raw.githubusercontent.com" and path.startswith(f"/wiki/{REPOSITORY}/"):
        return "Wiki navigation must use the rendered github.com Wiki URL, not raw content"
    if url.netloc == "github.com" and path.startswith(wiki_prefix):
        page = path[len(wiki_prefix):]
        if page.endswith((".md", ".markdown")):
            return "Wiki page URLs must not include a Markdown extension"
        if not (root / "docs" / "wiki" / f"{page}.md").is_file():
            return f"unknown Wiki page: {page}"
        return None
    for kind in ("blob", "tree"):
        prefix = f"/{REPOSITORY}/{kind}/master/"
        if url.netloc == "github.com" and path.startswith(prefix):
            local = root / path[len(prefix):]
            valid = local.is_file() if kind == "blob" else local.is_dir()
            return None if valid else f"missing repository {kind} target: {path[len(prefix):]}"
    if url.scheme or url.netloc:
        return None
    if source.parent == root / "docs" / "wiki":
        return "Wiki links must be absolute rendered Wiki or repository URLs"
    local = (source.parent / path).resolve()
    if not local.exists() and (local.suffix or not local.with_suffix(".md").exists()):
        return f"broken local link: {raw}"
    return None


def paired_documents() -> list[tuple[Path, Path]]:
    pairs = [
        (ROOT / "README.md", ROOT / "README.zh-CN.md"),
        (ROOT / "docs/README.md", ROOT / "docs/README.zh-CN.md"),
        (ROOT / "docs/deployment/CONTAINER.md", ROOT / "docs/deployment/CONTAINER.zh-CN.md"),
        (
            ROOT / "docs/releases/v3.0.4/RELEASE-NOTES-v3.0.4.md",
            ROOT / "docs/releases/v3.0.4/RELEASE-NOTES-v3.0.4.zh-CN.md",
        ),
        (
            ROOT / "docs/releases/v3.0.4/MIGRATION-v3.0.4.md",
            ROOT / "docs/releases/v3.0.4/MIGRATION-v3.0.4.zh-CN.md",
        ),
        (
            ROOT / "docs/releases/v2.8.3/RELEASE-NOTES-v2.8.3.md",
            ROOT / "docs/releases/v2.8.3/RELEASE-NOTES-v2.8.3.zh-CN.md",
        ),
        (
            ROOT / ".github" / "PROJECT-METADATA.md",
            ROOT / ".github" / "PROJECT-METADATA.zh-CN.md",
        ),
        (ROOT / "docs/deployment/WEB-CLIENT.md", ROOT / "docs/deployment/WEB-CLIENT.zh-CN.md"),
    ]
    for english in sorted(WIKI.glob("*.md")):
        if english.name == "_Sidebar.md" or english.name.startswith("ZH-CN-"):
            continue
        pairs.append((english, english.with_name(f"ZH-CN-{english.name}")))
    return pairs


def main() -> int:
    errors: list[str] = []
    pairs = paired_documents()
    paths = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.md"],
        cwd=ROOT, text=True,
    ).split("\0")
    documents = {ROOT / path for path in paths if path and (ROOT / path).is_file()}

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
        for number, target in markdown_links(body):
            error = link_error(document, target)
            if error:
                errors.append(
                    f"{document.relative_to(ROOT)}:{number}: {error}: {target}"
                )

    for required in (
        ROOT / "README.md",
        ROOT / "README.zh-CN.md",
        ROOT / "docs/deployment/CONTAINER.md",
        ROOT / "docs/deployment/CONTAINER.zh-CN.md",
        ROOT / "examples" / "compose.env.example",
    ):
        if "v3.0.4" not in required.read_text(encoding="utf-8"):
            errors.append(f"v3.0.4 is missing from {required.relative_to(ROOT)}")

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
