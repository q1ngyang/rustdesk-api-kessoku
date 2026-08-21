import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const projectRoot = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath: string): string {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

describe("runtime dependency licensing", () => {
  it("matches the pinned lockfile expression", () => {
    const lock = JSON.parse(read("package-lock.json")) as {
      packages: Record<string, { version?: string; license?: string }>;
    };
    expect(projectRoot).toContain("web-client");
    expect(lock.packages["node_modules/@bufbuild/protobuf"]).toMatchObject({
      version: "2.9.0",
      license: "(Apache-2.0 AND BSD-3-Clause)",
    });
  });

  it("never attributes the project's MIT license to the runtime", () => {
    const claims = [read("NOTICE.md"), read("PROVENANCE.md"), read("SOURCE_POLICY.md")].join("\n");
    expect(claims).not.toMatch(/MIT[-\s\w]*@bufbuild\/protobuf|@bufbuild\/protobuf[^.\n]*MIT/i);
    expect(claims).toContain("(Apache-2.0 AND BSD-3-Clause)");
    expect(read("SOURCE_POLICY.md")).toContain("must never be attributed to a runtime dependency");
  });

  it("ships complete Apache and BSD terms through Vite public assets", () => {
    const notice = read("public/third-party-licenses/@bufbuild-protobuf-2.9.0.txt");
    expect(notice).toContain("Apache License");
    expect(notice).toContain("Version 2.0, January 2004");
    expect(notice).toContain("END OF TERMS AND CONDITIONS");
    expect(notice).toContain("Copyright 2008 Google Inc.  All rights reserved.");
    expect(notice).toContain("Neither the name of Google Inc. nor the names of its contributors");
    expect(notice).toContain("THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS \"AS IS\"");
  });
});
