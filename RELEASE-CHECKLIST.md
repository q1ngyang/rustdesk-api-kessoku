# Kessoku v3.0.1 release checklist

This checklist defines the release gate for the exact source commit named by
the immutable `v3.0.1` tag. Detailed runtime evidence is retained by the
candidate workflow rather than duplicated here.

## Source and compatibility

- [x] Project-owned Go imports and the module declaration use `/v3`; no project
  `/v2` import remains.
- [x] Database version 302 migrates legacy administrators to `super_admin` and
  adds scoped grants for user groups, users, public address books, and ID
  devices.
- [x] Scope filtering, batch-denial, cleanup, audit, final-super-admin, and
  session-revocation behavior have automated coverage.
- [x] The responsive admin UI and all brand assets are stored in this repository
  and preserve the reviewed frontend provenance.
- [x] Breaking changes and safe v2 rollback preparation are documented in both
  languages and linked from the Wiki upgrade guide.

## Pre-publication verification

- [x] `scripts/check_docs.py`, module verification, formatting, vet, all Go
  tests, race tests, and vulnerability scan pass on the release commit.
- [x] Admin frontend install, audit, lint, test, reproducible build, and SBOM
  checks pass with the pinned Node/npm versions.
- [x] Browser frontend install, audit, lint, test, reproducible build, licence,
  and SBOM checks pass with the pinned Node/npm versions.
- [x] SQLite, MySQL 8.4.2, and PostgreSQL 16.4 migration fixtures pass.
- [x] Secret scanning, dependency vulnerability checks, workflow policy tests,
  and forbidden historical browser-asset checks pass.

The release preparation passed PR CI run `32841549238` and master CI run
`32841809241`. The first `v3.0.0` candidate run `32842075289` then failed closed
on stale generated Swagger identifiers before final candidate assembly and
published no artifacts. The generated documentation and `/v3` build-info gates
are corrected here; the final candidate rebuilds and re-verifies everything
from the immutable `v3.0.1` tag.

## Protected publication

The following steps occur after the immutable tag is created and are visible in
GitHub Actions/Release evidence, not as modifications to the tagged source:

- [ ] Non-publishing candidate workflow on `v3.0.1` succeeds.
- [ ] Linux amd64 binary, archive, DEB, container smoke, checksum, provenance,
  and SBOM gates pass in that candidate.
- [ ] Protected release workflow consumes that exact candidate run ID.
- [ ] GitHub Release assets and `SHA256SUMS` verify.
- [ ] GHCR `v3.0.1` and `latest` resolve to the same image index digest.
- [ ] The reviewed bilingual Wiki is published and its navigation is verified.

Supported release artifacts are Linux amd64 only. ARM remains best-effort
source compatibility, and Windows is outside the blocking release matrix.
