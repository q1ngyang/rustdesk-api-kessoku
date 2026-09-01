# Kessoku v3.0.7 release checklist

This checklist defines the release gate for the exact source commit named by
the immutable `v3.0.7` tag. Detailed runtime evidence is retained by the
candidate workflow rather than duplicated here.

## Source and compatibility

- [x] Project-owned Go imports and the module declaration use `/v3`; no
  project-owned `/v2` import remains.
- [x] Database schema advances from 312 to 313. SQLite, MySQL, and PostgreSQL
  schema inspection, exclusive migration locks, duplicate device-ID
  preflight, idempotence, failure markers, and future-schema refusal have
  automated coverage.
- [x] `version`, `config validate`, `database status`, and `database migrate`
  have command-specific initialization boundaries, documented JSON schema 1,
  explicit exit codes, secret-redaction tests, and golden fixtures.
- [x] Administrator recovery and 2FA reset use service-layer transactions,
  exact username confirmation, one auth-version increase, full active-token
  revocation, scope/challenge cleanup, and success/failure security auditing.
- [x] Password files reject permission errors, symlinks, replacement races,
  and values outside 12–128 bytes; no password argument or environment input
  exists.
- [x] `docs/releases/v3.0.7/migration.yaml` records schema 313, the Starry
  contract, stable maintenance capabilities, Presence Lease v2, and the
  restore-only v3.0.6 downgrade mode.
- [x] Presence Lease v2 authenticates start through Starry capability 2,
  returns high-entropy lease credentials, stores token hashes only, aggregates
  all valid leases, expires crashed clients by TTL, and keeps the existing
  heartbeat route compatible without increasing its database-write cadence.
- [x] Adaptive Relay Quality is capability-negotiated, preserves patch-v1.2
  inventory compatibility, validates only Starry's aggregate contract, drives
  configuration from Starry's schema/UI-schema, applies a medium-risk floor,
  and keeps simulation explicitly non-binding and free of client quality
  scores.
- [x] A-to-B-to-A switching, delayed and out-of-order renew/end, crash expiry,
  parallel leases, legacy fallback, exact profile identity isolation, and
  concurrent renew/end have automated coverage.
- [x] Active native sessions claim signed-in clients, while the least-privilege
  Starry peer-registry check admits only an exact registered ID/UUID pair for
  clients that are not signed in; historical login records grant no ownership.
- [x] Full client inventory is refreshed on startup/change and after the
  24-hour stale heartbeat fallback, and verified metadata is propagated to
  matching address-book entries without overwriting personal fields.
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
  online resolution of immutable Action pins, local-state exclusions, and
  forbidden historical browser-asset checks pass.
- [x] Starry patch-v1.3.0 is approved, committed, published under an immutable
  tag, and its OpenAPI, config schema/UI-schema, image-index, and linux/amd64
  digests replace every `UNPUBLISHED`/`UNAVAILABLE` value in
  `internal/starrycontrol/CONTRACT_VERSION`.
- [x] Starry's release-only 1,000 registered idle WebSocket gate passes on the
  exact release commit; registration timeouts or missing `RegisterPkResponse`
  are blocking failures.
- [ ] Isolated v3.0.7 staging verifies the local JSON commands, schema-312 to
  schema-313 upgrade and restore-only downgrade, recovery from a disposable
  database copy,
  login, device discovery/manual ID refresh,
  signed-in and signed-out device discovery, A-to-B-to-A Presence Lease v2
  switching and TTL expiry, inventory/address-book refresh,
  connection and file audit creation, branding defaults, Starry status/logs,
  adaptive/eager Relay Quality metrics, stale/no-candidate alerts, an
  acknowledged Relay Quality configuration change, non-binding allocation
  simulation, and a real WebClient session that reaches the registered
  virtual-client hostname and creates the expected audit record.

The final source commit must include only reviewed product, test, deployment,
and release files. `deploy-local-test/`, databases, certificates, credentials,
uploaded media, and runtime logs are never release inputs.

## Protected preparation and publication

The candidate and publication-readiness steps occur before the immutable tag is
created. They are retained as GitHub Actions evidence rather than written back
to the reviewed source:

- [ ] Non-publishing candidate workflow on the exact `master` commit succeeds
  with `release_tag=v3.0.7` while that tag does not yet exist.
- [ ] Linux amd64 binary, archive, DEB, container smoke, checksum, provenance,
  and SBOM gates pass in that candidate.
- [ ] Protected `mode=prepare` consumes that exact candidate, exercises
  signing/environment/GHCR/image/Release-note readiness, pushes the
  commit-addressed candidate image, verifies its registry digest, and only then
  creates `v3.0.7` at the candidate commit. An interrupted final tag call is
  recoverable only when the existing annotated tag resolves to that commit.
- [ ] Protected `mode=publish` on `master` consumes the same candidate run ID,
  verifies the remote annotated tag against that candidate source through the
  GitHub API, and promotes its exact image digest without rebuilding it. Release
  assets remain a draft until their complete inventory and checksums have been
  downloaded and re-verified.
- [ ] GitHub Release assets and `SHA256SUMS` verify.
- [ ] GHCR `v3.0.7` and `latest` resolve to the same image index digest.
- [ ] The reviewed bilingual Wiki is published and its navigation is verified.
- [ ] The Release body is at most 12 lines and links to the full bilingual notes.

Supported release artifacts are Linux amd64 only. ARM remains best-effort
source compatibility, and Windows is outside the blocking release matrix.
