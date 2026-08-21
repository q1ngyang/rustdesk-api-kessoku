# Source policy

This subtree is an independent clean implementation. Contributions must not copy, inspect for derivation, import, or bundle:

- historical `resources/web` or `resources/web2` content;
- RustDesk WebClient2/V2, Flutter Web, or upstream web-client source;
- `web_deps` archives or hosted client assets;
- externally built JavaScript or WebAssembly binaries.

Allowed implementation inputs are the repository wire profile, the minimal protobuf file, documented Kessoku API contracts, browser platform APIs, and audited source dependencies pinned by the lockfile. New runtime dependencies require a provenance and license update, inclusion of all required third-party license/notice text in the production artifact, and a test that reconciles the lockfile license expression with the notice. The Kessoku project's MIT license must never be attributed to a runtime dependency unless that dependency's pinned package metadata says so. Secret values must remain in memory and must never enter URLs, persistent browser storage, logs, telemetry, or generated diagnostics.
