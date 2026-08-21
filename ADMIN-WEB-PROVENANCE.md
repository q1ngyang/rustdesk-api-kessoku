# Kessoku admin web provenance

Kessoku v2.8.0 keeps the reviewed management frontend source in
[`admin-web/`](admin-web/). Backend and frontend are reviewed, tagged, built,
and attested from the same Kessoku commit. No separate frontend repository,
moving branch, network checkout, URL, or build argument is accepted.

## Source lineage and license

- Monorepo path: `admin-web/`
- Reviewed local preparation commit:
  `2a9d037fc271cf96b39fd4add4b97c4ff4477f12`
- MIT seed commit: `3998c2a9213fcd047252776d0f0db33e6717026c`
- Seed source: `lejianwen/rustdesk-api-web`
- Seed license: MIT (`vue-manage-system`, copyright 2016–2021)
- Fixed build runtime: Node.js `24.15.0`, npm `11.12.1`
- Locked install command: `npm ci`

The imported source removes generic ServerCmd UI/API calls and every embedded
WebClient/WebClient2 protocol, launch, and share implementation. Its management
page calls only `/api/admin/server-control/v1` DTOs. Configuration changes use
authoritative validation, ETag-bound plans, idempotency keys, operation
polling, rollback, runtime reload, Relay inventory/simulation, and redacted
audit views.

The exact imported source keeps its MIT license in `admin-web/LICENSE`. The
historical WebClient/WebClient2 trees are not part of this import and remain
excluded from all runtime-copy, package, and image paths.

## Reproducible build controls

The root CI and candidate workflow install from `admin-web/package-lock.json`
with `npm ci`, then run source policy, the current eight tests, the high-severity production
dependency audit, registry signature verification, two production builds with
identical file checksums, and CycloneDX generation. Only the verified `dist/`
tree is copied into runtime `resources/admin`.

`Dockerfile.dev` copies the lockfile before source and accepts no frontend
location argument. `scripts/verify-local-admin-candidate.sh` snapshots the whole
current Kessoku tree, including `admin-web/`, and records the same snapshot SHA
for backend and frontend in `BUILD-INPUTS.txt`. It performs no push, image
publication, npm publication, or release operation.

The 2026-08-19 preparation run passed `npm ci`, eight tests, a zero-finding
production audit, registry signatures for 110 packages, two equivalent builds,
and an SBOM with license metadata for all 62 production components. Browser QA
covered login and every typed Control page flow against the repository mock.
These are local evidence only; clean candidate CI and the remaining release
checklist gates are still required.
