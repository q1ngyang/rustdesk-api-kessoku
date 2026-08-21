# Upgrade and rollback

**English** | [简体中文](ZH-CN-Upgrade-and-Rollback.md)

## Before the window

Record the current Kessoku/Starry image digests, source/contract versions,
database version, active Starry generation, key IDs, and client matrix. Back up
and restore-test:

- the database;
- Kessoku access-token current/previous keys;
- internal mTLS PKI and Control Agent credentials;
- Kessoku/Starry configuration and audit/provenance records; and
- the prior images/packages.

## Upgrade sequence

1. Deploy Kessoku v2.8.0 with authentication disabled and control read-only.
2. Verify database version 300 and legacy-token migration.
3. Enable EdDSA issuance with a bounded compatibility overlap if required.
4. Bring up internal mTLS JWKS/introspection.
5. Upgrade Starry with connection authentication `off`, then `audit`.
6. Commission the Control Agent read-only.
7. Complete real-client audit and staging rollback tests.
8. Canary `enforce`; open configuration writes only in separate approved
   windows.

## Rollback warning

New v2.8.0 credentials leave the historical plaintext token column empty.
Older applications cannot reconstruct or authenticate them. Once v2.8.0 has
issued tokens, roll back the old application only with its matching verified
pre-upgrade database backup, and expect sessions created after that backup to
require re-login.

## Ordered rollback

1. Return Starry authentication from `enforce` to `audit` under change control.
2. Set Kessoku and the Control Agent read-only.
3. Restore and verify the last-known-good Starry generation.
4. Preserve redacted evidence and decide between forward remediation and a
   matched application/database restore.
5. Restore the approved database backup to an isolated target, validate it,
   then switch the older application.
6. Verify admin/API login, token invalidation, native/WSS audit behavior,
   database row counts, and the absence of generic command routes.

See [`MIGRATION.md`](../../MIGRATION.md) and
[`ROLLBACK-RUNBOOK.md`](../../ROLLBACK-RUNBOOK.md) for the detailed procedures.
