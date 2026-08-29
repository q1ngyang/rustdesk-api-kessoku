# Kessoku v3.0.5 release checklist

This checklist defines the release gate for the exact source commit named by
the immutable `v3.0.5` tag. Detailed runtime evidence is retained by the
candidate workflow rather than duplicated here.

## Source and compatibility

- [x] Project-owned Go imports and the module declaration use `/v3`; no
  project-owned `/v2` import remains.
- [x] Database migrations reach version 309 across SQLite, MySQL, and
  PostgreSQL while preserving users, roles, devices, sessions, and audit data.
- [x] Branding, encrypted TOTP, login challenges, announcements, GeoIP policy,
  preferences, media, and WebClient audit ownership have migration and
  rollback documentation.
- [x] Scope filtering, batch denial, cleanup, audit, final-super-admin, and
  session-revocation behavior have automated coverage.
- [x] The administration and browser frontends, themed StarryDesk defaults,
  and fixed StarryLinks control assets are stored in this repository and built
  from the exact backend commit.
- [x] Separate-origin WebClient authentication, grant handoff, connection
  auditing, theme/language preferences, and forced-Relay protocol behavior
  have automated coverage.
- [x] v3.0.1 withdrawal, unpublished v3.0.2/v3.0.4 release attempts,
  v3.0.3 history, breaking changes, complete backups, LinuxDo removal, release
  sequencing, and safe rollback are documented in English and Simplified
  Chinese.

## Pre-publication verification

- [x] `scripts/check_docs.py`, module verification, formatting, vet, all Go
  tests, race tests, generated API documentation, release-identity consistency,
  and vulnerability scanning pass on the release commit.
- [x] Admin frontend install, audit, lint, test, reproducible build, and SBOM
  checks pass with the pinned Node/npm versions.
- [x] Browser frontend install, audit, lint, test, reproducible build, licence,
  and SBOM checks pass with the pinned Node/npm versions.
- [x] SQLite, MySQL 8.4.2, and PostgreSQL 16.4 migration fixtures pass.
- [x] Secret scanning, dependency vulnerability checks, workflow policy tests,
  local-state exclusions, and forbidden historical browser-asset checks pass.
- [x] Isolated staging verifies login, device discovery/manual ID refresh,
  connection and file audit creation, branding defaults, Starry status/logs,
  and a real WebClient session that reaches the registered virtual-client
  hostname and creates the expected audit record.

The final source commit must include only reviewed product, test, deployment,
and release files. `deploy-local-test/`, databases, certificates, credentials,
uploaded media, and runtime logs are never release inputs.

## Protected preparation and publication

The candidate and publication-readiness steps occur before the immutable tag is
created. They are retained as GitHub Actions evidence rather than written back
to the reviewed source:

- [ ] Non-publishing candidate workflow on the exact `master` commit succeeds
  with `release_tag=v3.0.5` while that tag does not yet exist.
- [ ] Linux amd64 binary, archive, DEB, container smoke, checksum, provenance,
  and SBOM gates pass in that candidate.
- [ ] Protected `mode=prepare` consumes that exact candidate, exercises
  signing/environment/GHCR/image/Release-note readiness, pushes the
  commit-addressed candidate image, verifies its registry digest, and only then
  creates `v3.0.5` at the candidate commit. An interrupted final tag call is
  recoverable only when the existing annotated tag resolves to that commit.
- [ ] Protected `mode=publish` on the immutable tag consumes the same candidate
  run ID and promotes that exact candidate image digest without rebuilding it.
  Release assets remain a draft until their complete inventory and checksums
  have been downloaded and re-verified.
- [ ] GitHub Release assets and `SHA256SUMS` verify.
- [ ] GHCR `v3.0.5` and `latest` resolve to the same image index digest.
- [ ] The reviewed bilingual Wiki is published and its navigation is verified.
- [ ] The Release body is at most 12 lines and links to the full bilingual notes.

Supported release artifacts are Linux amd64 only. ARM remains best-effort
source compatibility, and Windows is outside the blocking release matrix.
