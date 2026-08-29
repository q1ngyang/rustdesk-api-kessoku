# RustDesk API Kessoku v3.0.6 migration and rollback

v3.0.6 advances the v3.0.5 database from version `311` to `312`. AutoMigrate
adds persistent native-client type and peer identity/inventory timestamps; the
explicit v312 migration then performs conservative backfill without deleting
records.

## Upgrade

1. Stop every Kessoku writer and take a database-consistent backup together
   with the TOTP key, media, YAML, signing keys, Control PKI, and current image
   digests. Verify the backup in an isolated restore.
2. For network discovery of clients not signed in to Kessoku, upgrade the
   center HBBS and its Control Agent to Starry `1.1.16-patch-v1.2.2`, keeping
   the same persistent data, identity key, local-control token, instance ID,
   certificates, and service-JWT trust. Keep that Agent as an enabled
   `server-control.instances` entry in Kessoku; an empty instance list supports
   signed-in discovery only. No Starry schema change is required.
3. Start exactly one Kessoku v3.0.6 writer and wait for migration completion.
   Do not run v3.0.5 and v3.0.6 concurrently against the same database.
4. Confirm database version `312`, then start the remaining replicas.
5. Restart representative signed-in and signed-out RustDesk clients. Confirm
   My Devices, Device Management, and matching address-book entries receive
   current hostname, username, OS, hardware, UUID, version, IP, and online time.

## Migration behavior

- Existing token rows recover `client` from their linked login audit when it
  still exists. Newly issued tokens persist it directly.
- Only active native/app tokens for enabled users, with matching auth version,
  exact normalized ID, and nonempty UUID can claim or fill a peer row. Expired,
  revoked, browser, disabled-user, and stale-auth-version sessions are ignored.
- Existing nonempty inventories inherit their last online timestamp as the
  initial inventory time. Empty or incomplete placeholders stay stale so the
  next heartbeat requests a full upload.
- Existing device aliases and address-book personal fields are preserved.
  Identity conflicts are logged and skipped for explicit administrator review.

## Rollback

Stop all Kessoku writers and restore the complete pre-v3.0.6 database and
matching application/configuration/key set. Do not point an older binary at a
database already opened by v3.0.6. Starry patch-v1.2.2 can remain deployed; its
new verification endpoint is read-only and does not change the HBBS database.

[简体中文](MIGRATION-v3.0.6.zh-CN.md)
