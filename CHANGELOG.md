# Changelog

All notable Kessoku changes are recorded here. Release-specific operational
details remain in [`docs/releases/`](docs/releases/).

[简体中文](CHANGELOG.zh-CN.md)

## v3.0.8 preview — 2026-09-04

- Added exact Starry v1.3.1 schema-v5 FastCompat/FastMedia capabilities,
  typed Relay UDP aggregates, dependency-gated writes, risk floors, audited
  plan bindings, generation/schema digests, and activation ACK validation.
- Preserved patch-v1.2/v1.3.0 Relay compatibility and schema-driven forms;
  missing capabilities display as Unsupported and unknown known versions fail
  closed.
- Added the allowlist-only SP1 Control Agent and Agent-authorized Relay
  enrollment Broker, CLI, and administration UI, including revocation and
  recreation of unclaimed Control Agent codes by enrollment ID.
- Added an independent schema-v1 SQLite registry with owner-only per-instance
  mTLS/JWT identity files, CSR-bound idempotent recovery, UUID locking, provider
  hot reload, static v3.0.7 exports, host-clone detection, rotation, and
  explicitly confirmed purge without changing business database schema 313.
- Require administrator RBAC and an exact revision-bound second confirmation
  for every rollback, including rollback to a revision that re-enables
  FastMedia, and drain HTTP cleanly on SIGTERM.
- Update `golang.org/x/crypto` from 0.55.0 to 0.56.0, removing the two
  non-imported SSH denial-of-service module advisories; `govulncheck` reports
  zero reachable and zero imported-package vulnerabilities.
- Verified in an isolated exact-state exercise that Control certificate
  rotation, force-recreate, schema-v4 static-export takeover, and the return to
  schema v5 preserve registry generation and credentials. This is diagnostic
  evidence, not release approval.
- Preview publication is approved after immutable Starry runtime pinning and
  hosted candidate checks. Real Akari/client/NAT/fallback, production migration,
  PKI, and soak evidence remain stable-promotion gates listed in the
  [release notes](docs/releases/v3.0.8/RELEASE-NOTES-v3.0.8.md).

## v3.0.7 — 2026-08-31

- Added stable human and JSON local interfaces for version, configuration
  validation, database status, and explicit migration.
- Added serialized SQLite/MySQL/PostgreSQL migrations and future-schema guards.
- Added audited transactional administrator recovery and idempotent per-user
  2FA reset with authentication-generation rotation and session revocation.
- Added Presence Lease v2 for race-safe multi-profile switching, aggregate
  online state, TTL crash recovery, and legacy-heartbeat compatibility.
- Advanced database schema from 312 to 313 with durable hashed-token leases,
  immutable network identity binding, and a duplicate device-ID preflight.
- Retained HTTP/config compatibility and the historical password-reset
  commands.

## v3.0.6 — 2026-08-29

- Completed safe native/network device discovery and inventory refresh.
- Added schema 312 device identity and session metadata.

See the [v3.0.6 release notes](docs/releases/v3.0.6/RELEASE-NOTES-v3.0.6.md)
for the complete historical scope.
