# Changelog

All notable Kessoku changes are recorded here. Release-specific operational
details remain in [`docs/releases/`](docs/releases/).

[简体中文](CHANGELOG.zh-CN.md)

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
