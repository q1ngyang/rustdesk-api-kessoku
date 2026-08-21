# Kessoku admin web provenance

This source directory was imported into the Kessoku monorepo from the reviewed local preparation commit `2a9d037fc271cf96b39fd4add4b97c4ff4477f12`. That preparation tree started from the MIT-licensed `lejianwen/rustdesk-api-web` commit `3998c2a9213fcd047252776d0f0db33e6717026c`.

The Kessoku changes remove every generic ServerCmd UI/API call and every embedded WebClient/WebClient2 protocol, launch, and share implementation. The replacement page calls only the versioned `/api/admin/server-control/v1` DTO surface. Configuration mutations require authoritative validation, a reviewed plan, the current ETag, and a generated idempotency key.

The 2026-08-19 preparation verification used the immutable `node:24.15.0-bookworm@sha256:f22d6a1f082c02f292e86929b5b0442ac2e5eaf438a5dea9b1566601c3e05940` image with npm `11.12.1`. `npm ci`, source policy, eight tests, production dependency audit, two equivalent Vite builds, registry signature verification, and CycloneDX generation passed. The production SBOM described 62 components, all with license metadata.

From Kessoku v2.8.3 onward, this directory and the backend are one versioned source unit. Candidate provenance therefore records the Kessoku commit itself plus this seed/import lineage; no independent frontend repository, branch, tag, or network checkout is a build input. The root candidate workflow rebuilds from `admin-web/package-lock.json` with `npm ci`, and runtime packaging copies only the resulting `dist/` into `resources/admin`.
