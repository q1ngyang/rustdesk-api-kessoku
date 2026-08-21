# Database and Authentication Migration Guide

This guide upgrades an existing rustdesk-api database to Kessoku database
version 300. Rehearse every step against a restored production backup before a
maintenance window.

## Supported databases and tested fixtures

The migration fixture is exercised against:

- SQLite;
- MySQL 8.4.2;
- PostgreSQL 16.4.

CI creates an empty database whose name starts with `kessoku_test_`, constructs
the legacy schema and rows, runs the real migration, and verifies token-hash
backfill, auth versions, version recording, and preservation of the historical
`server_cmds` table.

## What version 300 changes

`users` gains `auth_version`, initialized to `1`.

`user_tokens` gains bounded JTI/key-ID columns, a unique SHA-256 `token_hash`,
`auth_version`, issue time, revocation time, and revocation reason. Existing
plaintext credentials are hashed during migration and the legacy `token` value
is cleared in the same row update. Newly issued rows also leave it empty. A
bounded compatibility period can still recognize migrated legacy credentials
by their authoritative database hash; it does not require retaining plaintext.

`admin_audit_events` is added for control-plane audit records. The old
`server_cmds` table remains untouched, but there is no CRUD or execution
service for it.

The migration is additive. There is intentionally no automatic down migration:
restoring an older application safely requires restoring its matching database
backup once Kessoku has issued new tokens.

## Preflight

1. Stop writes or schedule a maintenance window.
2. Record the current application image digest, configuration, database
   version, and row counts for `users` and `user_tokens`.
3. Back up the database with the database vendor's consistent/snapshot option.
4. Restore that backup into an isolated database and verify it before
   proceeding.
5. Back up existing authentication keys separately. Never put a private key in
   the database backup, image, Git repository, YAML, or log archive.
6. Leave Starry connection authentication in `off` or `audit`; never begin the
   schema upgrade by enabling `enforce`.
7. Keep `server-control.read-only: true` until the Agent rollback exercise is
   complete.

## Recommended rollout

### Phase 1: additive database upgrade

Start Kessoku with:

```yaml
auth:
  enabled: false
  legacy-token-read-enabled: false
server-control:
  read-only: true
```

Startup runs the schema migration and backfills token hashes. With the auth
manager disabled, migrated opaque or legacy JWT strings remain usable through
their database hash and expiry. New logins receive random opaque credentials
stored only as hashes.

Verify:

- the application reports database version 300;
- every active legacy token row has a 64-character `token_hash`;
- every user and token row has a non-zero `auth_version`;
- row counts match the preflight record;
- legacy command records still exist but no command route is reachable.

### Phase 2: enable EdDSA with a compatibility overlap

Generate an Ed25519 PKCS#8 private key using an approved key-management
process. Mount it read-only with service-account-only permissions, then set:

```yaml
auth:
  enabled: true
  issuer: "https://api.example.com"
  audiences: ["kessoku-api", "rustdesk-connect"]
  access-token-ttl: 168h
  maximum-token-ttl: 168h
  legacy-token-read-enabled: true
  current-key:
    id: "kessoku-2026-01"
    private-key-file: "/run/secrets/kessoku-access-ed25519.pem"
```

New logins now receive EdDSA JWTs. The compatibility flag permits an existing
database-backed legacy credential only after strict EdDSA verification fails;
it does not accept a credential absent from the token table. Monitor legacy
usage and require clients to log in again during this bounded overlap.

Enable the dedicated internal listener only after its server certificate,
private key, client CA, and exact Starry SAN allow list are ready. Do not expose
the listener through the public reverse proxy.

### Phase 3: Starry audit

Configure Starry with the Kessoku JWKS and introspection endpoints, then enable
Starry's `audit` mode. Confirm all supported native and WSS client versions send
the connection token and inspect would-deny metrics. Cache behavior must match
the documented revocation bound and must fail closed on cache misses in future
enforce mode.

### Phase 4: remove legacy authentication

After at least one maximum token lifetime, or after an explicit all-session
revocation and verified re-login campaign:

1. revoke remaining legacy sessions;
2. set `legacy-token-read-enabled: false`;
3. restart and verify that legacy HS256/opaque credentials are rejected;
4. verify the historical plaintext column is empty; and
5. schedule physical removal of that empty column as a separately reviewed
   future migration.

### Phase 5: enforce gradually

Enable Starry `enforce` for a small approved cohort, test native and WSS
connections, logout, user disablement, password reset, key overlap, and an
introspection outage. Expand only after there is no unexplained would-deny
traffic.

## Ed25519 key rotation

1. Create a new private key and unique key ID.
2. Export the old current key's public key to a read-only file.
3. Move the old key ID/public file into `previous-keys` and make the new key the
   current private key in one reviewed configuration change.
4. Restart Kessoku and confirm JWKS publishes both key IDs while new JWTs use
   only the new `kid`.
5. Retain the previous public key for at least the maximum issued token TTL plus
   clock skew.
6. Remove it only after confirming no valid token can still reference it.

Do not rotate the access and Control Agent keyrings to the same key. Startup
rejects that reuse, but deployment review must also keep their storage,
ownership, backup, and rotation procedures separate.

## Post-upgrade acceptance

- New `user_tokens.token` values are empty and their hashes/JTIs are populated.
- A single logout makes introspection inactive immediately.
- Password change, user disablement, and global logout invalidate all older
  auth versions.
- Current and previous key overlap behaves as documented.
- No bearer token or private key appears in application logs or audit rows.
- Database backup restoration and the rollback procedure have been timed and
  recorded for all deployed database engines.

See [ROLLBACK-RUNBOOK.md](ROLLBACK-RUNBOOK.md) before approving the maintenance
window.
