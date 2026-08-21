import { readdir, readFile } from "node:fs/promises";
import { extname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const inputs = [join(root, "index.html"), join(root, "src")];
const extensions = new Set([".html", ".ts", ".css"]);
const forbidden = [
  [/(?:local|session)Storage\b/, "persistent browser storage"],
  [/\bindexedDB\b|\bcaches\.open\b|serviceWorker\.register/, "offline or persistent browser state"],
  [/document\.cookie\b/, "browser cookies"],
  [/\b(?:eval|Function)\s*\(/, "dynamic code execution"],
  [/\.(?:innerHTML|outerHTML)\s*=|insertAdjacentHTML\s*\(/, "unsafe HTML insertion"],
  [/\bconsole\.(?:log|info|debug|warn|error)\s*\(/, "console output"],
  [/[?&#](?:token|grant)=/i, "credential in URL"],
  [/postMessage\s*\([^,]+,\s*["']\*["']\s*\)/s, "wildcard postMessage target"],
  [/WebClient2|web_deps|resources\/web2?\b/i, "prohibited historical or closed client asset"],
  [/<script\b[^>]*\bsrc=["']https?:\/\//i, "remote script"],
];

async function files(path) {
  const entries = await readdir(path, { withFileTypes: true });
  const output = [];
  for (const entry of entries) {
    const candidate = join(path, entry.name);
    if (entry.isDirectory()) output.push(...await files(candidate));
    else if (extensions.has(extname(entry.name)) && !entry.name.endsWith(".test.ts")) output.push(candidate);
  }
  return output;
}

const candidates = [inputs[0], ...await files(inputs[1])];
for (const path of candidates) {
  const source = await readFile(path, "utf8");
  for (const [pattern, label] of forbidden) {
    if (pattern.test(source)) throw new Error(`${relative(root, path)} violates source policy: ${label}`);
  }
}
process.stdout.write(`source_policy_files=${candidates.length} result=PASS\n`);
