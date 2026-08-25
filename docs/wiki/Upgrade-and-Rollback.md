# Upgrade and rollback

**English** | [简体中文](ZH-CN-Upgrade-and-Rollback.md)

## Before the window

Kessoku does not publish one universal RTO or RPO. Before deploying this
release, the deployment owner must record the responsible people, maintenance
window, local RTO/RPO, backup retention, rollback authority, and go/no-go
decision. This is a deployment gate rather than a software-publication gate.

Record the current Kessoku/Starry image digests, source/contract versions,
database version, active Starry generation, key IDs, and client matrix. Back up
and restore-test:

- the database;
- Kessoku access-token current/previous keys;
- internal mTLS PKI and Control Agent credentials;
- Kessoku/Starry configuration and audit/provenance records; and
- the prior images/packages.

Before changing the image, configure external MySQL with `tls: "true"` (and
`ca-file` for private PKI), or PostgreSQL with `sslmode: "verify-full"` (and
`ssl-root-cert` when needed). Test the exact database DNS name against its
certificate SAN. Also run the duplicate/empty OAuth identity queries in
[`MIGRATION.md`](../../MIGRATION.md); Kessoku stops instead of guessing how to
merge a conflicting external identity.

## Upgrade sequence

1. Deploy Kessoku v3.0.0 with authentication disabled and control read-only.
2. Verify database version 302, every former administrator's `super_admin`
   role, at least one enabled super administrator, and the OAuth/token
   invariants from version 301.
3. Verify that an `admin` with no grants sees no enterprise resources; then
   test each user-group, user, public-address-book, and ID-device grant.
4. Confirm that role or scope changes revoke existing administrator sessions.
5. Enable EdDSA issuance with a bounded compatibility overlap if required.
6. Bring up internal mTLS JWKS/introspection.
7. Upgrade Starry with connection authentication `off`, then `audit`, and
   commission the Control Agent read-only.
8. Complete real-client audit and staging rollback tests.
9. Commission the Web Client on its separate HTTPS origin, validate the public
   profile, ready/grant/ack handoff, forced-Relay VP9 session, grant expiry and
   logout; keep `web-client.mode: disabled` if any check fails.
10. Canary `enforce`; open configuration writes only in separate approved
   windows.

## Rollback warning

Version 302 keeps `is_admin=true` for scoped and super administrators as a
database compatibility mirror. A v2 binary can therefore grant a scoped
administrator unrestricted access. Prefer restoring the complete pre-upgrade
backup. If an in-place v2 rollback is unavoidable, follow
[`MIGRATION-v3.0.0.md`](../../MIGRATION-v3.0.0.md) before starting v2, and never
run v2 and v3 writers together.

New v3.0.0 credentials leave the historical plaintext token column empty.
Older applications cannot reconstruct or authenticate them. Once v3.0.0 has
issued tokens, roll back the old application only with its matching verified
pre-upgrade database backup, and expect sessions created after that backup to
require re-login.

## Ordered rollback

1. Return Starry authentication from `enforce` to `audit` under change control.
2. Set Kessoku and the Control Agent read-only.
3. Set `web-client.mode: disabled`, revoke active connection grants, and verify
   the 21122 public origin no longer serves a client. This does not require
   restoring historical browser assets.
4. Restore and verify the last-known-good Starry generation.
5. Preserve redacted evidence and decide between forward remediation and a
   matched application/database restore.
6. Restore the approved database backup to an isolated target, validate it,
   then switch the older application.
7. Verify admin/API login, token invalidation, native/WSS audit behavior,
   database row counts, and the absence of generic command routes.

See [`MIGRATION-v3.0.0.md`](../../MIGRATION-v3.0.0.md),
[`MIGRATION.md`](../../MIGRATION.md), and
[`ROLLBACK-RUNBOOK.md`](../../ROLLBACK-RUNBOOK.md) for the detailed procedures.
