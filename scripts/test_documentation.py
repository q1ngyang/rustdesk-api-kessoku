"""Regression tests for rendered Wiki links and categorized release documents."""

from pathlib import Path
import tempfile
import unittest

from check_docs import REPOSITORY, link_error, markdown_links
from render_release_notes import render


class DocumentationLinksTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="kessoku-doc-links-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.wiki = self.root / "docs/wiki/Home.md"
        self.source = self.root / "docs/releases/v3.0.3/RELEASE-NOTES-v3.0.3.md"
        for path in (
            "docs/wiki/Home.md", "docs/wiki/ZH-CN-Home.md",
            "docs/releases/v3.0.3/RELEASE-NOTES-v3.0.3.md",
            "docs/releases/v3.0.3/MIGRATION-v3.0.3.md",
            "docs/deployment/CONTAINER.md", "examples/relay/compose.yaml",
        ):
            file = self.root / path
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text("# Fixture\n", encoding="utf-8")
        self.base = f"https://github.com/{REPOSITORY}"

    def check(self, url, source=None):
        return link_error(source or self.wiki, url, self.root)

    def test_rendered_wiki_link_with_anchor(self):
        self.assertIsNone(self.check(f"{self.base}/wiki/ZH-CN-Home#quick-start"))

    def test_rejects_markdown_extension(self):
        self.assertIn("extension", self.check(f"{self.base}/wiki/Home.md"))

    def test_rejects_raw_wiki_navigation(self):
        self.assertIn("raw content", self.check(f"https://raw.githubusercontent.com/wiki/{REPOSITORY}/Home.md"))

    def test_rejects_relative_wiki_page_and_repository_links(self):
        for target in ("Home.md", "../deployment/CONTAINER.md", "../../examples/relay/compose.yaml"):
            with self.subTest(target=target):
                self.assertIn("absolute", self.check(target))

    def test_rejects_unknown_wiki_page(self):
        self.assertIn("unknown", self.check(f"{self.base}/wiki/Missing"))

    def test_repository_files_and_directories(self):
        self.assertIsNone(self.check(f"{self.base}/blob/master/docs/deployment/CONTAINER.md"))
        self.assertIsNone(self.check(f"{self.base}/tree/master/examples/relay"))

    def test_rejects_stale_moved_path_and_wrong_url_kind(self):
        self.assertIn("missing", self.check(f"{self.base}/blob/master/CONTAINER.md"))
        self.assertIn("missing", self.check(f"{self.base}/blob/master/examples/relay"))

    def test_non_wiki_relative_links_still_work(self):
        self.assertIsNone(self.check("../../deployment/CONTAINER.md", self.source))
        self.assertIn("broken", self.check("../../../CONTAINER.md", self.source))

    def test_anchors_and_external_links(self):
        self.assertIsNone(self.check("#section"))
        self.assertIsNone(self.check("https://docs.github.com/"))

    def test_code_fences_are_not_navigation(self):
        body = "# Test\n```sh\n[x](Home.md)\n```\n~~~md\n[y](Home.md)\n~~~\n[page](#section)\n"
        self.assertEqual(markdown_links(body), [(8, "#section")])

    def test_renderer_rebases_nested_release_link_and_preserves_label(self):
        body = "[MIGRATION-v3.0.3.md](MIGRATION-v3.0.3.md#upgrade)\n"
        rendered = render(body, self.source, REPOSITORY, "v3.0.3", self.root)
        self.assertEqual(rendered, f"[MIGRATION-v3.0.3.md]({self.base}/blob/v3.0.3/docs/releases/v3.0.3/MIGRATION-v3.0.3.md#upgrade)\n")

    def test_renderer_uses_wiki_and_tree_urls(self):
        body = "[Wiki](../../wiki/Home.md) [files](../../../examples/relay/)\n"
        rendered = render(body, self.source, REPOSITORY, "v3.0.3", self.root)
        self.assertIn(f"]({self.base}/wiki/Home)", rendered)
        self.assertIn(f"]({self.base}/tree/v3.0.3/examples/relay)", rendered)

    def test_renderer_preserves_external_anchor_and_code(self):
        body = "[external](https://example.com/) [here](#section)\n```md\n[x](missing.md)\n```\n"
        self.assertEqual(render(body, self.source, REPOSITORY, "v3.0.3", self.root), body)

    def test_renderer_rejects_missing_target(self):
        with self.assertRaises(ValueError):
            render("[x](missing.md)", self.source, REPOSITORY, "v3.0.3", self.root)


if __name__ == "__main__":
    unittest.main()
