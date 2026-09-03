# Kessoku v3.0.8 release checklist

This checklist defines the gate for the source commit named by immutable tag
`v3.0.8`. `RELEASE_STATUS` is `PREVIEW_APPROVED`: only a GitHub prerelease and
the versioned/`preview` images are authorized. `latest` and v3.0.7 remain the
stable line. Detailed runtime evidence is retained by the candidate workflow
rather than asserted only in prose.

## Source and compatibility

- [x] Project-owned Go imports and the module declaration use `/v3`; no
  project-owned `/v2` import remains.
- [x] The main database remains schema 313 from v3.0.7. The SP1 registry uses
  independent `/data/server-control/registry-v1.sqlite` schema 1 and introduces
  no main-database migration.
- [x] `version`, `config validate`, `database status`, and `database migrate`
  have command-specific initialization boundaries, documented JSON schema 1,
  explicit exit codes, secret-redaction tests, and golden fixtures.
- [x] Administrator recovery and 2FA reset use service-layer transactions,
  exact username confirmation, one auth-version increase, full active-token
  revocation, scope/challenge cleanup, and success/failure security auditing.
- [x] Password files reject permission errors, symlinks, replacement races,
  and values outside 12–128 bytes; no password argument or environment input
  exists.
- [x] `docs/releases/v3.0.8/migration.yaml` records schema 313, the exact frozen
  Starry contract commit, registry schema 1, and the in-place-compatible
  v3.0.7 binary downgrade.
- [x] Presence Lease v2 authenticates start through Starry capability 2,
  returns high-entropy lease credentials, stores token hashes only, aggregates
  all valid leases, expires crashed clients by TTL, and keeps the existing
  heartbeat route compatible without increasing its database-write cadence.
- [x] Adaptive Relay Quality is capability-negotiated, preserves patch-v1.2
  inventory compatibility, validates only Starry's aggregate contract, drives
  configuration from Starry's schema/UI-schema, applies a medium-risk floor,
  and keeps simulation explicitly non-binding and free of client quality
  scores.
- [x] FastCompat/FastMedia accepts only exact known capability versions and
  schema 5, preserves older Starry inventory, rejects incompatible writes,
  keeps independent default-off toggles, and requires current fresh healthy
  matching UDP Relay telemetry before FastMedia enablement.
- [x] Every `/fast_mode` plan has at least medium risk; FastMedia enablement,
  UDP endpoint/datagram/bitrate, and firewall-related changes are high risk and
  require super-admin RBAC, exact second confirmation, actor/ETag/generation/
  digest-bound review, redacted before/after audit, and subsystem activation
  ACKs.
- [x] Relay Fast state is typed and aggregate-only. Upstream process,
  allocation, and session UUIDs are discarded; client addresses, credentials,
  grants, private keys, and media are neither stored nor returned.
- [x] SP1 Control Agent pairing is allowlist-only, secret-digest and CSR-bound,
  recovers an interrupted same-key claim, learns/locks the instance UUID,
  creates independent per-instance mTLS/JWT identities, hot-loads the provider,
  and forces first pairing/rotation to read-only.
- [x] Relay enrollment requires authenticated Agent prepare/complete authority;
  health-gated auto-activation is immutable high-risk preauthorization, while
  normal activation requires exact operation/generation/digest/health evidence.
- [x] Registry permissions, process-shared locking, monotonic generation,
  backup/restore, host-clone detection/adoption, managed identity rotation,
  v3.0.7-compatible static export, and exact double-confirmation purge have
  automated coverage.
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

## Local source verification

- [ ] `scripts/check_docs.py`, module verification, formatting, vet, all Go
  tests, race tests, generated API documentation, release-identity consistency,
  and vulnerability scanning pass on the release commit.
- [ ] Admin frontend install, audit, lint, test, reproducible build, and SBOM
  checks pass with the pinned Node/npm versions.
- [ ] Browser frontend install, audit, lint, test, reproducible build, licence,
  and SBOM checks pass with the pinned Node/npm versions.
- [ ] SQLite, MySQL 8.4.2, and PostgreSQL 16.4 migration fixtures pass.
- [ ] Secret scanning, dependency vulnerability checks, workflow policy tests,
  online resolution of immutable Action pins, local-state exclusions, and
  forbidden historical browser-asset checks pass.
- [x] Starry's frozen contract-only summary is byte-identical, binds 16 files
  and the inherited Relay Quality contract to commit
  `6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`, and passes cross-repository
  fixture validation.
- [x] Starry `1.1.16-patch-v1.3.1` is an immutable GitHub prerelease. Its
  source, release-summary, image-index, and linux/amd64 digests replace every
  runtime `UNPUBLISHED`/`UNAVAILABLE` value in `CONTRACT_VERSION`; Starry
  `latest` remains unchanged.

## Preview integration and publication evidence

- [x] The isolated working-tree candidates passed exact-state SP1 certificate
  rotation, provider reload, same-data Kessoku force-recreate, v3.0.7 static
  takeover with Starry v1.3.0/schema 4, byte-identical registry preservation,
  return to v3.0.8/v1.3.1/schema 5, and the outward Relay privacy scan. This is
  diagnostic evidence and does not approve publication.
- [x] A protocol-level harness using the current Akari FastMedia library in
  both roles and the exact HBBR candidate preserved reliable TCP, exercised
  UDP-timeout fallback, and automatically re-entered FastMedia in the same
  session. Full client/signalling/device gates below remain open.
- [ ] Isolated v3.0.8 staging verifies the local JSON commands and v3.0.7 ->
  v3.0.8 -> v3.0.7 -> v3.0.8 in-place binary round trip,
  login, device discovery/manual ID refresh,
  signed-in and signed-out device discovery, A-to-B-to-A Presence Lease v2
  switching and TTL expiry, inventory/address-book refresh,
  connection and file audit creation, branding defaults, Starry status/logs,
  adaptive/eager Relay Quality metrics, Fast state/counters, stale/no-candidate
  alerts, acknowledged Relay Quality and Fast configuration changes, rejected
  incompatible writes, non-binding allocation simulation, SP1/Relay enrollment,
  and a real WebClient session that creates the expected audit record.
- [ ] Hosted CI, code/security review, vulnerability and secret scans, SBOM,
  package/image signing, provenance, and attestations pass on the exact final
  commit.

## Stable-promotion evidence (not a preview blocker)

- [ ] Real Akari dual-role SP1 pairing succeeds end to end.
- [ ] Reliable Relay fallback and automatic FastMedia re-entry pass with real
  clients.
- [ ] The real-device/NAT/UDP-loss, timeout, blocked-port, AP-change, rebind,
  rate-limit, expiry, and sustained media/thermal matrix passes without
  interrupting reliable fallback.
- [ ] Production Kessoku SP1 response recovery, certificate rotation,
  container recreation, registry backup/restore, v3.0.7 static takeover, and
  stopped-source multi-host adoption are demonstrated with retained evidence.
- [ ] Preview observation finds no compatibility, migration, security, or
  availability regression. Any correction uses a new third-component version.

The final source commit must include only reviewed product, test, deployment,
and release files. `deploy-local-test/`, databases, certificates, credentials,
uploaded media, and runtime logs are never release inputs.

## Protected preparation and publication

The candidate and publication-readiness steps occur before the immutable tag is
created. They are retained as GitHub Actions evidence rather than written back
to the reviewed source:

- [ ] Non-publishing candidate workflow on the exact `master` commit succeeds
  with `release_tag=v3.0.8` while that tag does not yet exist.
- [ ] Linux amd64 binary, archive, DEB, container smoke, checksum, provenance,
  and SBOM gates pass in that candidate.
- [ ] Protected `mode=prepare` consumes that exact candidate, exercises
  signing/environment/GHCR/image/Release-note readiness, pushes the
  commit-addressed candidate image, verifies its registry digest, and only then
  creates `v3.0.8` at the candidate commit. An interrupted final tag call is
  recoverable only when the existing annotated tag resolves to that commit.
- [ ] Protected `mode=publish` on `master` consumes the same candidate run ID,
  verifies the remote annotated tag against that candidate source through the
  GitHub API, and promotes its exact image digest without rebuilding it. Release
  assets remain a draft until their complete inventory and checksums have been
  downloaded and re-verified.
- [ ] GitHub Release assets and `SHA256SUMS` verify.
- [ ] GHCR `v3.0.8` and `preview` resolve to the same image index digest;
  `latest` remains unchanged.
- [ ] The reviewed bilingual Wiki is published and its navigation is verified.
- [ ] The Release body is at most 12 lines and links to the full bilingual notes.

Supported release artifacts are Linux amd64 only. ARM remains best-effort
source compatibility, and Windows is outside the blocking release matrix.
