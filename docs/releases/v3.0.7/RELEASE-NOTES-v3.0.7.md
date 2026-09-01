# Kessoku v3.0.7

v3.0.7 provides the stable local maintenance surface required by an S6-managed
`starrydesk` container and adds Presence Lease v2 for race-safe, fast
multi-profile switching. Recovery remains local-only; the existing heartbeat
API and legacy clients remain compatible.

## Stable local interfaces

- `kessoku-api version [--json]` works without configuration, a database, or
  keys and reports the component version, schema, source revision, build time,
  and Go version.
- `kessoku-api config validate --config PATH [--json]` parses the production
  configuration and checks URLs, ports, paths, certificates, private-key
  permissions, and key separation without connecting or writing local state.
- `kessoku-api database status --config PATH [--json]` performs a read-only
  schema inspection. `database migrate` takes a database-specific exclusive
  lock, runs the same migration implementation used at service startup, and
  verifies the final marker before returning.
- `maintenance recover-admin` restores one exactly confirmed account as an
  enabled `super_admin`. `maintenance reset-2fa` clears only the selected
  account's TOTP state. Both revoke sessions, advance `auth_version` once, and
  write dedicated security audit events.

Every JSON response uses snake_case fields and `schema_version: 1`. Stable exit
codes distinguish usage, configuration, connection, schema, and maintenance
failures. See the [local maintenance CLI reference](../../operations/LOCAL-MAINTENANCE-CLI.md).

## Presence Lease v2

- `start` verifies the exact ID, profile-scoped `network_identity_uuid`,
  activation, and current Starry route before returning a random lease ID,
  256-bit bearer token, and 45-second expiry. The client activation ID is
  echoed unchanged; local `profile_id` is neither accepted nor stored.
- `renew` and `end` select exactly one token-bound lease; new clients send both
  its ID and token. Tokens are stored only as SHA-256 hashes and are never
  included in metrics or logs.
- A device is online while any valid lease for its current activation remains.
  Delayed old activation requests cannot end a new activation, and a crashed
  client becomes offline automatically at TTL expiry.
- Separate network identity UUIDs cannot overwrite each other's device ID,
  account owner, metadata, or online state. Duplicate device IDs fail the
  schema migration preflight instead of being merged automatically.
- A bounded super-administrator metrics endpoint exposes low-cardinality
  counters and database-wide gauges, with documented alert thresholds and no
  lease credentials. See the
  [Presence Lease v2 operations guide](../../operations/PRESENCE-LEASE-V2.md).

## Safety and compatibility

Database schema advances from `312` to `313`. The additive migration creates
the presence lease store and peer activation/aggregate columns, then enforces
unique device IDs after a read-only duplicate preflight. The default server
startup and every database or maintenance command reject a schema newer than
the binary.

Recovery accepts a password only from an owner-only regular file opened with
no symlink following and replacement-race checks. The requested user must be
selected by exactly one identifier and bound again with the exact stored
username. Role, status, scope cleanup, optional password/TOTP changes,
challenge cleanup, token revocation, and success audit finalization share one
database transaction. Audit metadata never contains credentials or TOTP data.

Valid sub-hour access-token lifetimes are preserved at their exact configured
duration instead of being rounded down to zero by the hour-based management
setting. Operator-selected hour values remain capped by
`auth.maximum-token-ttl`.

SQLite, MySQL 8.4.2, and PostgreSQL 16.4 integration fixtures exercise schema
inspection, migration serialization, recovery, idempotent 2FA reset, session
revocation, audit completion, and schema-313 lease objects. Presence start
requires Starry `1.1.16-patch-v1.3.0` with peer-registry capability 2.
The byte-identical [Starry release summary](STARRY-RELEASE-SUMMARY.json) pins
its source commit, image index/platform digests, Control OpenAPI, config/UI
schemas, frozen Relay Quality protocol, and telemetry schema.

## Upgrade and downgrade

Read the [migration and rollback guide](MIGRATION-v3.0.7.md) before deployment.
A return to v3.0.6 is restore-only because it does not understand schema 313.
Stop all writers, restore the complete matching pre-upgrade recovery set, and
only then start v3.0.6.

[简体中文](RELEASE-NOTES-v3.0.7.zh-CN.md)
