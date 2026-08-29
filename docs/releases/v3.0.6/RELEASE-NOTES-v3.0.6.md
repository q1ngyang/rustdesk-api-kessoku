# Kessoku v3.0.6

v3.0.6 fixes the device-discovery path end to end. Signed-in RustDesk clients
are claimed from a currently valid native session, while clients on the same
RustDesk network that are not signed in can be admitted only after Starry
confirms the exact ID and machine UUID in its private HBBS registry.

## Highlights

- Newly signed-in native clients appear automatically in **My Devices**.
  Network-registered clients that are not signed in appear in administrator
  **Device Management** when they report to the configured API server.
- On every client start the official client re-evaluates CPU, hostname, memory,
  operating system, username, UUID, and version and uploads when that inventory
  changed. Kessoku replaces all prior fields, including intentionally empty
  values, and a 24-hour heartbeat fallback requests a fresh full report.
- Every address-book record targeting the same RustDesk ID receives the latest
  verified hostname, username, and platform. Personal aliases, passwords,
  tags, RDP settings, and relay preferences remain untouched; an older client
  cache cannot overwrite newer verified metadata.
- RustDesk IDs are normalized before login, heartbeat, inventory, connection,
  file, and address-book processing, so pasted spacing does not split identity.
- Native token type is now stored with the session. Migration recovers only
  enabled users' unexpired, unrevoked native sessions with a matching
  authentication version; historical login logs alone grant no ownership.
- Starry verification uses mTLS and the dedicated `starry.peer.verify` service
  JWT scope. Starry returns only its instance ID and a boolean exact-match
  result—never peer metadata, keys, addresses, or a registry listing.

## Compatibility and upgrade notes

- Database version is `312`. The migration is additive and does not delete
  devices, sessions, address books, or audits.
- Upgrade center HBBS and its Control Agent to
  [Starry 1.1.16-patch-v1.2.2](https://github.com/q1ngyang/rustdesk-server-starry/releases/tag/1.1.16-patch-v1.2.2)
  before Kessoku when discovery of clients not signed in to the API is
  required. Relay-only nodes do not provide this capability.
- Existing signed-in clients remain discoverable through their active native
  sessions even while Starry is being upgraded.
- A client must have this Kessoku API server configured and must start or send
  a heartbeat before it can report inventory; the server does not enumerate or
  probe arbitrary desktops.
- Do not downgrade a migrated database in place. Restore the complete matching
  pre-upgrade backup instead.

See the [v3.0.6 migration and rollback guide](MIGRATION-v3.0.6.md).

[简体中文](RELEASE-NOTES-v3.0.6.zh-CN.md)
