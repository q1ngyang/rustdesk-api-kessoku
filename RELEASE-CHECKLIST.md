# Release Checklist

This worktree is a local v2.8.0 release candidate. Publication remains blocked:
an unchecked item below must be completed by the release owner or protected CI
before images, packages, tags, or releases are published.

## Local candidate evidence (not release approval)

The 2026-08-21 candidate passed the following repeatable local checks with
locked inputs:

- Go `1.26.6`: `gofmt`, `go mod verify/tidy`, `go vet ./...`, full
  `go test ./...`, and full `go test -race ./...`;
- the real version-301 legacy migration fixture on SQLite, MySQL `8.4.2`, and
  PostgreSQL `16.4`;
- `govulncheck v1.7.0` with zero reachable and zero imported-package
  vulnerabilities after dependency upgrades. One module-only advisory remains
  for the unimported `golang.org/x/crypto/openpgp` package; it has no upstream
  fixed version and must be reassessed at candidate review;
- actionlint `1.7.12`, Gitleaks `8.25.1` full-history scanning, Syft `1.50.0`
  source SBOM generation, packaging policy tests, and the digest-pinned
  `builder-backend` Docker target;
- clean-snapshot compilation of the module package `./cmd`, with the exact
  source revision and `vcs.modified=false` verified from persisted
  `GO-BUILD-INFO.txt`. Two independent exact-image builds were byte-identical;
  candidate CI repeats and compares the build, while candidate and publication
  workflows independently enforce the same binary identity;
- worktree scanning found only eight previously reviewed candidates under the
  historical `resources/web` tree. That tree is untouched and excluded from
  every runtime-copy and Docker build path.

The local Starry candidate now also passes 75 ordinary library tests, while the
separate deterministic mutation-corpus unit was intentionally not rerun under
the agreed no-fuzz/no-active-security-test constraint. All integration targets
compile, and the candidate passes a real HBBS/Control-Agent-to-Kessoku Provider
E2E, its native/Secure TCP/WSS transport matrix, 1,000 registered idle WSS
sessions plus 100 reconnect
replacements, and reproducible amd64/arm64 Debian-package installation/runtime
checks. RustSec reports no vulnerability, unsound, or yanked package; the one
remaining `sodiumoxide 0.2.7` unmaintained warning is disclosed for release risk
review. The local reviewed admin-web candidate also passes eight tests, a
zero-finding production audit, reproducible build comparison, SBOM/license
inspection, registry signature verification, secret scanning, and browser QA.
The exact local v2.8.0 candidate verifier additionally passed reproducible Go,
admin-web, tar, and amd64 DEB builds; package installation; a non-root
linux/amd64 image smoke; and live local HTTP security-header checks. It now
clones the clean reviewed HEAD, verifies that persisted Go build information
names that exact commit with `vcs.modified=false`, and can retain a checksummed
non-publishing evidence bundle with source/runtime/candidate SBOMs, scan
summaries, build inputs, archive, and DEB outside the repository.

Codex Security completed sealed, static-only repository reviews without
starting a service or sending probe traffic. Kessoku's frozen snapshot records
23 findings across 27 review surfaces under snapshot
`codex-security-snapshot/v1:sha256:d504807864f052238881f7e0e18548763d8e1b0134567f95ee0d08b497bef68d`.
Starry's frozen snapshot records 22 findings (6 medium, 16 low) across 40
surfaces, with no high or critical result, under snapshot
`codex-security-snapshot/v1:sha256:4b5ffa3ce6bc819a9a72e9f6e9ec7fd9dc63c0aee4c74645b1d67472d5b6aaac`.
The plugin scanner was not callable in this final Kessoku session, so no new
exact-tree plugin result is claimed; closure is based on the sealed snapshot,
post-remediation static review, ordinary tests, and the local candidate verifier.
The published Starry `1.1.16-patch-v1.2.0` assets, contract, source commit,
GHCR tag/digest identity, amd64 command smoke, and official-binary Kessoku
Provider E2E were independently rechecked on 2026-08-21. The exact local
Kessoku candidate and its post-remediation static review now pass. A protected
candidate run for the final approved tag and final owner approval remain
outstanding; topology-specific recovery acceptance gates deployment instead.

The exact published Starry image then passed a RustDesk 1.4.9 forced-Relay
desktop matrix: `audit` native/native, followed by `enforce` native/native,
WSS/WSS, WSS/native, and native/WSS. Each case verified the Remote Desktop
window, screenshot, and established HBBR connection. This evidence does not
claim direct P2P or a separate Secure TCP case.

## Source and contracts

- [x] The backend and embedded frontend source is reviewed together from the
      intended v2.7 upstream baseline.
- [ ] After final approval, the exact reviewed commit is tagged as Kessoku
      `v2.8.0`; the immutable tag has not been created or pushed yet.
- [x] `internal/starrycontrol/CONTRACT_VERSION` names a published, immutable
      Starry tag, contract version, and verified SHA-256, with `status: PINNED`.
- [x] Generated-compatible client DTOs match that published contract and pass
      forward/backward compatibility fixtures.
- [x] Cross-repository native, Secure TCP, WSS, Relay simulation, apply, failed
      apply, and rollback E2E pass against the exact Starry release candidate.

## Admin frontend

- Local evidence: reviewed candidate
  `2a9d037fc271cf96b39fd4add4b97c4ff4477f12`; `npm ci`, eight tests,
  zero-finding production audit, registry signature verification, two identical
  builds, CycloneDX/license check, Gitleaks, and full local browser QA passed on
  2026-08-20.
- The exact-SHA local verifier also passed duplicate frontend/Go/tar/DEB
  builds, non-root image smoke, real static security headers, disabled
  directory listing, and the removed server-config route's `404`. It is local
  evidence only and cannot set release approval.
- [x] `admin-web/` provenance, MIT license, reviewed import lineage, and
      lockfile are present in the exact Kessoku commit.
- [x] Backend and frontend build provenance use the same Kessoku commit SHA;
      no external frontend repository, URL, tag, or branch is an input.
- [x] Node/npm are fixed; installation uses `npm ci`.
- [x] Frontend lint, tests, build, and high-severity production dependency audit
      pass without weakening the gate.

## Security and migrations

- [x] Legacy server commands are absent by default, administrator-only in the
      compatibility route, and always `410 Gone` without parsing input.
- [x] All versioned management-control routes require administrator privilege.
- [x] Access and Control Agent Ed25519 keys are distinct, external read-only
      secrets with tested rotation and restoration.
- [x] Internal JWKS/introspection mTLS, SAN allow list, TLS 1.3, limits, timeout,
      and fail-closed behavior pass in deployment.
- [x] Version-301 migration fixtures pass on SQLite, MySQL 8.4.2, and PostgreSQL
      16.4; the SQLite fixture also passes file backup and restore verification.
- [x] Audit-to-enforce rollout, revocation-cache bound, token mass invalidation,
      and client re-login are rehearsed.
- [x] Control Agent read-only, ETag conflict, apply verification, automatic
      rollback, and manual rollback are rehearsed.

## Reproducible build and artifacts

- [x] `gofmt`, `go vet`, full tests, race tests, migration services, and contract
      tests pass from locked dependencies.
- [x] Every GitHub Action, tool, base image, service image, frontend source, and
      cross compiler is fixed to an immutable reviewed input with checksum or
      digest verification.
- [x] Secret scan, vulnerability scan, three SPDX SBOMs, frontend CycloneDX,
      artifact checksums, and build provenance pass and are attached to the
      retained local non-publishing candidate evidence bundle. Protected CI
      regenerates them for publication.
- [x] Codex Security produced sealed, auditable static results for both frozen
      worktrees without active probing, fuzzing, mutation, stress, or exploit
      traffic; snapshot digests and finding counts are recorded above.
- [x] The exact post-remediation local candidate receives static review against
      those findings, with every accepted residual risk and closure recorded.
- [x] The local candidate binary was built as module package `./cmd`; its
      persisted Go build info names the exact reviewed commit and
      `vcs.modified=false`. Protected CI repeats this against the approved tag.
- [x] Images, archives, and Debian packages contain no `resources/web`,
      `resources/web2`, browser-client download code, private keys, or build
      credentials.
- [x] Artifact names, service units, image names, module paths, titles, and
      documentation consistently use Kessoku branding.
- [ ] Published GHCR `v2.8.0` and `latest` tags resolve to the same approved
      image index digest; production instructions pin the versioned digest.

## v2.8.0 platform scope

- [x] The Docker `linux/amd64` image builds from the candidate context, runs as
      the unprivileged user, serves the embedded admin UI with security headers,
      and passes configuration/bootstrap smoke tests.
- [x] The Linux x86_64 binary/tar and amd64 Debian package are reproducible;
      the package installs and its service/runtime permissions pass in the
      pinned Debian environment.
- ARM remains best-effort source compatibility for v2.8.0. Existing arm64/QEMU
  evidence may be retained, but an ARM binary, DEB, image, or runtime smoke is
  not a release blocker or a promised v2.8.0 artifact.
- Windows is outside the supported v2.8.0 release matrix and is non-blocking.

## Documentation and approval

- [x] The release owner confirms that the public project/repository/image name
      remains `rustdesk-api-kessoku` and that no requested rename to
      `rustdesk-server-kessoku` is pending.
- [ ] The concise English/Chinese README, bilingual v2.8.0 release notes,
      bilingual container guide, all paired Wiki pages, and project/package
      metadata wording are reviewed and approved, including the requested
      current Web Client implementation explanation.
- [x] The GHCR description and OCI documentation URL identify the v2.8.0
      changes, recommend Docker Compose, and link the Docker guide, Compose
      file, environment example, and Starry integration guide.
- [x] `scripts/check_docs.py`, the release Compose render, and every local
      documentation link pass on the exact release candidate.
- [x] [SECURITY-MODEL.md](SECURITY-MODEL.md), [MIGRATION.md](MIGRATION.md),
      [OPERATOR-RUNBOOK.md](OPERATOR-RUNBOOK.md), and
      [ROLLBACK-RUNBOOK.md](ROLLBACK-RUNBOOK.md), and
      [RELEASE-PROCESS.md](RELEASE-PROCESS.md) were reviewed against the exact
      release candidate.
- [x] The known limitations and required recovery evidence are documented.
- [x] Documentation states that Kessoku does not publish one universal RTO/RPO;
      each deployment owner must record its recovery owners, maintenance window,
      RTO/RPO, and go/no-go decision before rollout. This is a deployment gate,
      not a software-publication gate.
- [ ] The release owner explicitly approves the final new-feature,
      compatibility, platform-scope, and migration wording before any tag,
      Wiki, GHCR image, or GitHub Release is published, including the final TLS,
      OAuth/OIDC, accepted-residual, Starry matrix, `latest`, and Web Client
      wording.
- [ ] A release owner confirms every checkbox and removes the WIP block only in
      the reviewed release change.

Remaining publication actions are final release-owner wording approval, the
immutable approved tag, protected candidate workflow, and the unpublished
image/Release/Wiki operations. Do not substitute moving upstream branches or
disable audits to make a release workflow pass.
