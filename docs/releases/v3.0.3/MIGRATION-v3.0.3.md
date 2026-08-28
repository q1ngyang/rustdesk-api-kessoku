# RustDesk API Kessoku v3.0.3 migration and rollback

v3.0.3 upgrades Kessoku databases through version `309`. The migrations are
additive, but they introduce persistent encryption keys, media, identity
cleanup, and fields that older applications do not understand. Rehearse the
upgrade against a restored production backup before the maintenance window.

## Upgrade sources

- A v3.0.1 database starts at version `302` and runs migrations 303–309.
- An older v3 candidate may first run the documented 301/302 role and token
  migrations before continuing through 309.
- A new database is created directly at version 309.

The individual migration records are
[`MIGRATION-303.md`](../MIGRATION-303.md),
[`MIGRATION-304.md`](../MIGRATION-304.md),
[`MIGRATION-305.md`](../MIGRATION-305.md),
[`MIGRATION-306.md`](../MIGRATION-306.md),
[`MIGRATION-307.md`](../MIGRATION-307.md),
[`MIGRATION-308.md`](../MIGRATION-308.md), and
[`MIGRATION-309.md`](../MIGRATION-309.md).

## Preflight

1. Stop every Kessoku writer and record the current image digest, database
   version, enabled administrators, external identities, and row counts.
2. Create and verify a consistent database backup. Also back up configuration,
   access-token signing keys, internal PKI, and the current image digest.
3. Back up the entire data volume, including `media.directory` and an existing
   `two-factor.key-file`. Treat the database, TOTP key, and media as one
   versioned recovery set.
4. If LinuxDo was used, record the affected accounts and establish another
   approved login/recovery path. Migration 305 removes LinuxDo provider and
   identity-binding rows.
5. Confirm exact, different HTTPS WebClient public/API origins and valid
   certificates. Keep `web-client.mode: disabled` until API/admin migration is
   verified if a browser-client outage would affect the maintenance window.
6. Keep Starry connection authentication in `off` or `audit` and Server
   Control read-only until the upgraded API and real clients pass acceptance.

## Upgrade

1. Restore the backup into an isolated environment and start one v3.0.3
   instance. Do not allow old and new versions to write concurrently.
2. Confirm the newest `versions.version` row is `309` and startup reports no
   failed migration.
3. Verify at least one enabled `super_admin`, ordinary-user and scoped-admin
   access, native-client login, device system-information reporting, address
   books, and session revocation.
4. Confirm `/app/data/totp.key` (or the configured path) is a regular 32-byte,
   mode-0600 file. Enroll and verify TOTP on a test account without exposing
   the secret in logs or configuration.
5. Verify media upload, avatar crop, all light/dark brand defaults and custom
   assets, announcement display, and one successful Country/City/ASN MMDB
   refresh.
6. Test WebClient login persistence, admin handoff, a forced-Relay session,
   remote-hostname display, assistance chat, logout, and connection-audit
   creation from the separate public origin.
7. Validate Starry capabilities/status and read-only logs. Enable Control Agent
   writes only after plan/apply/rollback is rehearsed in staging; change the
   Kessoku runtime log level only for a bounded diagnostic window.
8. Repeat the verified sequence during the production maintenance window.

Useful read-only checks:

```sql
SELECT version, created_at FROM versions ORDER BY id DESC LIMIT 10;
SELECT id, username, role, is_admin, status FROM users ORDER BY id;
SELECT admin_user_id, scope_type, scope_id
FROM admin_resource_scopes
ORDER BY admin_user_id, scope_type, scope_id;
```

## Rollback

Do not start v3.0.1, v3.0.0, or any v2 image against a database already
migrated to 309. Stop every v3.0.3 instance and restore the complete matching
pre-upgrade recovery set: database, TOTP key, media, configuration, signing
keys, and image digest. Keep the failed version-309 copy for diagnosis.

Restoring only the database can break TOTP and image references; restoring only
keys can invalidate sessions. Never delete new tables or lower the recorded
database version to force an older process to start.

[简体中文](MIGRATION-v3.0.3.zh-CN.md)
