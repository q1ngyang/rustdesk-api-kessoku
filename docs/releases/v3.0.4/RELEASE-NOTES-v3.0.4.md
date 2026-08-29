# Kessoku v3.0.4

v3.0.4 is a stability and operations release for the supported Kessoku v3
line. It focuses on device visibility, auditable activity records, service
configuration, bounded logging, and practical administration workflows.

## Highlights

- Fixed native-client discovery and report reconciliation so devices can be
  found automatically and manually added IDs can refresh host, user, UUID,
  operating-system, CPU, memory, and version data. RustDesk IDs are normalized
  before lookup, so pasted spaces no longer prevent a match.
- Added an all-user service-information card with the effective ID server, API
  server, public key, copyable client configuration, and QR code. Relay node
  details remain available without encouraging users to enter a native-only
  Relay endpoint manually.
- Corrected connection and file audit ingestion, ownership, endpoint identity,
  remote paths, UUIDs, and newest-first ordering. Token, login, connection, and
  file views now expose their durable database sequence IDs so deletion gaps
  remain visible instead of being renumbered per page.
- Added date filters to token, login, connection, file, and Server Control audit
  views. Platform settings now control web/native login lifetime and record
  retention; every retention value defaults to `0`, meaning no automatic
  deletion.
- Added Kessoku, Starry, and Relay log sources to Server Control, with download
  support and guarded runtime log-level changes when writable control mode is
  enabled. Example configurations enable rotated logging with bounded file size
  and count.
- Added a configurable current-management-instance name and improved status,
  protocol, authentication, capability, and control-mode guidance.
- Restored built-in branding defaults for databases upgraded from older v3
  builds and improved theme-aware assets, uploads, service About information,
  responsive layouts, table density, usernames, dates, and localized help.
- Removed the obsolete Share Records administration surface. Address-book
  collections and sharing rules remain supported and are not removed.

## Compatibility and upgrade notes

- The database version remains `309`. Startup AutoMigrate adds the new audit
  identity and platform-setting columns without renumbering existing records.
- Back up the database, TOTP key, media directory, configuration, signing keys,
  and PKI before upgrading. Start only one v3.0.4 writer during the migration.
- Existing record-retention settings are initialized to `0`; administrators
  must opt in before Kessoku deletes historical records automatically.
- The supported artifact platform remains `linux/amd64`. The Go module path is
  unchanged at `github.com/q1ngyang/rustdesk-api-kessoku/v3`.
- Do not downgrade a migrated database in place. Stop all writers and restore
  the complete matching pre-upgrade recovery set instead.

See the [v3.0.4 migration and rollback guide](MIGRATION-v3.0.4.md) and the
[operations guide](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification).

[简体中文](RELEASE-NOTES-v3.0.4.zh-CN.md)
