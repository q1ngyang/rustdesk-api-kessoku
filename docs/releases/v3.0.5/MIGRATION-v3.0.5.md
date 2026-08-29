# RustDesk API Kessoku v3.0.5 migration and rollback

v3.0.5 keeps database version `309` and uses the normal startup AutoMigrate to
add new audit-identity and platform-setting columns. The operation is additive,
but production upgrades still require a complete recovery set.

The `v3.0.4` candidate was never published, so there is no supported v3.0.4
runtime upgrade path; existing supported installations upgrade from v3.0.3 or
an earlier documented release.

## Before upgrade

Back up and verify restoration of:

- the complete database;
- the TOTP encryption key and access-token signing keys;
- the media directory, configuration, environment file, and current image
  digest;
- reverse-proxy configuration, TLS certificates, and Starry PKI.

Stop all Kessoku writers before taking the final backup. Do not run v3.0.3 and
v3.0.5 against the same database during migration.

## Upgrade

1. Restore the backup into an isolated rehearsal environment.
2. Start one v3.0.5 instance and wait for schema migration to finish.
3. Confirm that existing users, devices, tokens, and audit records remain and
   that their database sequence IDs are unchanged.
4. Verify device reporting, connection/file audit creation, full remote paths,
   Server Control log sources, service configuration/QR, and branding defaults.
5. Review Platform Settings. Retention values of `0` disable automatic cleanup;
   enable cleanup only after the required compliance period is known.
6. Repeat the stopped-writer backup and upgrade sequence in production, then
   pin the verified v3.0.5 image digest.

## Rollback

Do not point v3.0.3 or an older process at a database already opened by v3.0.5.
Stop every writer and restore the matching pre-upgrade database, TOTP key,
media, configuration, signing keys, PKI, and previous image digest together.

[简体中文](MIGRATION-v3.0.5.zh-CN.md)
