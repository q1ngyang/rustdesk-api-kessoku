# Kessoku v3.0.7 migration and rollback

v3.0.7 advances database schema `312` to `313`. The additive migration adds
Presence Lease v2 storage and makes device IDs unique. It does not rewrite
account ownership or merge device identities. The machine-readable contract is
[`migration.yaml`](migration.yaml).

## Compatibility prerequisites

- Keep `POST /api/heartbeat`; it remains available to legacy clients.
- Presence Lease v2 clients require Starry
  `1.1.16-patch-v1.3.0` and `peer_registry` capability 2 so `start` can verify
  the exact activation route after normal Kessoku authentication.
- Each client Profile must use a distinct `network_identity_uuid`. The client
  must not send its local `profile_id`; Kessoku rejects that field.
- Do not run v3.0.6 and v3.0.7 writers against the same database.

## Optional Adaptive Relay Quality control

This integration has no Kessoku database migration. Starry patch-v1.2 and
instances without `relay_quality` remain supported; the management page shows
the new fields as **Unsupported** instead of failing the legacy inventory.

Do not enable this optional control path from a moving branch or dirty Starry
checkout. First publish the reviewed patch-v1.3 Control v1 contract under an
immutable tag and pin its OpenAPI digest in
`internal/starrycontrol/CONTRACT_VERSION`. Then:

1. Verify that `/control/v1/capabilities` reports `relay_quality`,
   `relay_active_probe`, `relay_probe_protocol`, and `relay_load_protocol` at
   version 1.
2. In read-only mode, verify the aggregate quality snapshot, every Relay's
   reported protocols, telemetry freshness, and `quality_candidate` state.
3. Confirm that the schema form comes from Starry's config v4 JSON Schema and
   UI Schema and survives a YAML/form round trip without dropping unknown
   fields.
4. In staging, run `validate -> plan`; patch-v1.3 must classify every
   `/relay_quality` change at least `medium`. Kessoku retains the same defensive
   floor if an older or non-conforming agent returns `low`, without changing its
   plan ID or candidate digest.
5. Apply the exact reviewed plan and require a successful activation ACK whose
   generation and source/effective digests match the active runtime before
   closing the change window.

Alert on contract-defined state: stale Relay telemetry and an enabled policy
with zero quality candidates. Treat timeout, invalid/late-report, and fallback
reason counters as cumulative; monitor deltas/rates using a deployment baseline
instead of inventing absolute thresholds. The management surface must remain
aggregate-only.

## Preflight

1. Stop every Kessoku writer using the database.
2. Back up and restore-test the database, configuration, TOTP encryption key,
   access-token signing keys, internal PKI, uploaded media, and current image
   digest as one recovery set.
3. Validate the exact mounted configuration without side effects:

   ```sh
   kessoku-api config validate --config /app/conf/config.yaml --json
   ```

4. Inspect the database without migration. A v3.0.6 database must report
   `installed_schema: 312`, `target_schema: 313`, `state: upgrade_required`,
   and `migration_required: true`:

   ```sh
   kessoku-api database status --config /app/conf/config.yaml --json
   ```

5. Confirm that the device-ID uniqueness preflight returns no rows:

   ```sql
   SELECT id, COUNT(*)
   FROM peers
   GROUP BY id
   HAVING COUNT(*) > 1;
   ```

If duplicates exist, Kessoku refuses before creating schema-313 objects or
writing the version marker. An owner must determine the correct UUID/account
mapping. Do not delete, merge, or rebind identities merely to satisfy the
index.

## Upgrade

While all other writers remain stopped, run exactly one migration command:

```sh
kessoku-api database migrate --config /app/conf/config.yaml --json
```

Kessoku acquires `flock` for SQLite, `GET_LOCK` for MySQL, or a PostgreSQL
advisory lock. It runs the duplicate-ID preflight, creates
`peer_presence_leases`, adds peer activation/aggregate columns and the unique
device-ID index, records schema 313 last, and verifies the result before
success. Repeating the command at schema 313 is idempotent.

Start v3.0.7, repeat `database status`, and require
`installed_schema: 313`, `target_schema: 313`, and `state: current`. Then verify:

- legacy heartbeat clients can still become online;
- a Presence v2 client can switch A to B to A without a delayed old `end`
  changing the current activation;
- a killed client becomes offline after its 45-second lease TTL;
- two Profiles with different network identity UUIDs remain separate devices
  with unchanged account ownership;
- `GET /api/admin/presence/v2/metrics` works for a super administrator and
  exposes no lease token or high-cardinality identity labels.

The complete wire, retention, metric-scope, and alert contract is in the
[Presence Lease v2 operations guide](../../operations/PRESENCE-LEASE-V2.md).

## Failure handling

The target version is not recorded until migration operations succeed. A
failed initialization can leave retryable additive objects with the old
version marker; correct the cause and rerun the same locked command. Never edit
the marker manually, drop lease objects, or run arbitrary repair SQL.

`newer_than_binary` is a hard refusal for status, migrate, maintenance, and
default startup. Restore a matching application/database recovery set instead
of forcing Kessoku to read an unknown schema. Logs, migration output, backups,
metrics, traces, and support bundles must never contain raw lease tokens.

## Restore-only downgrade to v3.0.6

v3.0.6 understands schema 312, not schema 313. An in-place image rollback is
not supported after migration.

1. Stop every v3.0.7 writer and retain a diagnostic snapshot of the failed
   schema-313 database.
2. Restore the complete matching pre-upgrade schema-312 recovery set, including
   database, TOTP key, media, signing keys, configuration, and PKI.
3. Start exactly one v3.0.6 process and verify the schema marker, login,
   administrator role, address books, and real client connectivity before
   adding replicas.

[简体中文](MIGRATION-v3.0.7.zh-CN.md)
