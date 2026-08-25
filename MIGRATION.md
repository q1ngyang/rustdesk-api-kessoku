# Database and Authentication Migration Guide

This guide documents the version 301 authentication foundation. Kessoku v3.0.0
then upgrades it to database version 302 for enterprise roles and scoped
administration. Follow [`MIGRATION-v3.0.0.md`](MIGRATION-v3.0.0.md) after the
version 301 preflight, and rehearse every step against a restored production
backup before a maintenance window.

## Supported databases and tested fixtures

The migration fixture is exercised against:

- SQLite;
- MySQL 8.4.2;
- PostgreSQL 16.4.

CI creates an empty database whose name starts with `kessoku_test_`, constructs
the legacy schema and rows, runs the real migration, and verifies token-hash
backfill, auth versions, version recording, and preservation of the historical
`server_cmds` table.

## What version 301 changes

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

`user_thirds` receives unique `(user_id, op)` and `(op, open_id)` identity
indexes. Very old pre-245 provider fields are normalized before duplicate
validation and index creation. Kessoku never chooses, merges, or deletes a
duplicate identity automatically.

`security_invariant_locks` contains a non-secret shared row that serializes the
final-enabled-administrator check across Kessoku replicas.

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
6. For MySQL or PostgreSQL, configure and test the new certificate/hostname-
   verified database transport before changing the application image. Mount
   any private CA read-only and ensure the configured database hostname is in
   the certificate SAN.
7. Run the OAuth identity checks below. Resolve conflicts only after an owner
   reviews which local account and external identity must remain bound.
8. Leave Starry connection authentication in `off` or `audit`; never begin the
   schema upgrade by enabling `enforce`.
9. Keep `server-control.read-only: true` until the Agent rollback exercise is
   complete.
10. Remove every legacy root `web-client-provider` block and keep
    `app.web-client: 0`. Decide whether the new repository-owned client remains
    `web-client.mode: disabled` during migration or is commissioned later with
    its own HTTPS origin, listener, exact WSS map, public key, and positive
    profile generation. Do not restore old `resources/web`/`resources/web2` or
    migrate a hosted WebClient2 token/SSO mechanism.

### Database TLS preflight

MySQL uses the operating-system trust pool and can add a private CA bundle:

```yaml
gorm:
  type: mysql
mysql:
  addr: "mysql.example.internal:3306"
  tls: "true"
  ca-file: "/run/secrets/mysql-ca.pem" # optional additional CA
```

PostgreSQL requires full certificate and hostname verification:

```yaml
gorm:
  type: postgresql
postgresql:
  host: "postgres.example.internal"
  sslmode: "verify-full"
  ssl-root-cert: "/run/secrets/postgres-ca.pem" # optional with public CA
```

The old MySQL `false`/`skip-verify` values and PostgreSQL
`disable`/`require`/`verify-ca` values now fail startup. This is an intentional
security-breaking configuration change: correct CA/DNS/SAN deployment rather
than weakening verification.

### OAuth identity preflight

On version 245 or newer, the following read-only queries must return no rows:

```sql
SELECT user_id, op, COUNT(*) AS duplicate_count
FROM user_thirds
GROUP BY user_id, op
HAVING COUNT(*) > 1;

SELECT op, open_id, COUNT(*) AS duplicate_count
FROM user_thirds
GROUP BY op, open_id
HAVING COUNT(*) > 1;

SELECT id
FROM user_thirds
WHERE op IS NULL OR TRIM(op) = ''
   OR open_id IS NULL OR TRIM(open_id) = '';
```

For a pre-245 database, first rehearse the upgrade on a restored copy. Kessoku
adds `op`/`oauth_type`, copies the historical `third_type`, and then runs the
same duplicate checks before creating either index.

If startup reports `OAuth identity migration preflight`, keep the old service
stopped, preserve another backup, inspect the affected local accounts and
provider subjects, and explicitly merge or unbind under an approved identity-
recovery procedure. Do not delete a row merely to make the index build pass.

## Recommended rollout

### Browser-client transition

The former external-provider descriptor is removed and now fails startup if
present. The built-in client is not enabled by copying static files alone:
release packages already contain `resources/client`, while runtime exposure
requires a valid `web-client.mode: builtin` profile and enabled EdDSA auth.

Commission API/admin and client as two distinct HTTPS origins. Grant endpoints
issue short-lived `rustdesk-connect`/`connect:initiate` tokens; they do not
reuse a query-string token, provider cookie, or admin storage. Validate the
public profile, strict CORS, exact-origin admin handoff, forced-Relay WSS/VP9
session, logout, and rollback with `mode: disabled` before declaring the client
available. See [`WEB-CLIENT.md`](WEB-CLIENT.md).

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

- the application reports database version 301;
- every active legacy token row has a 64-character `token_hash`;
- every user and token row has a non-zero `auth_version`;
- row counts match the preflight record;
- legacy command records still exist but no command route is reachable.
- both `idx_user_thirds_user_op` and `idx_user_thirds_op_open_id` exist and the
  `enabled-admin` invariant row exists.

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
- MySQL/PostgreSQL connect only with the reviewed CA and expected hostname;
  both OAuth identity unique indexes and the final-admin invariant are present.

See [ROLLBACK-RUNBOOK.md](ROLLBACK-RUNBOOK.md) before approving the maintenance
window.
