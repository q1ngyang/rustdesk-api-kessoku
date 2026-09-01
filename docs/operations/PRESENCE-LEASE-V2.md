# Presence Lease v2 operations

Presence Lease v2 lets a RustDesk client switch network profiles without a
late request from the previous activation changing the current profile's
online state. It supplements, and does not remove, `POST /api/heartbeat`.

[简体中文](PRESENCE-LEASE-V2.zh-CN.md)

## Identity and wire contract

The public endpoints are:

- `POST /api/presence/v2/start`
- `POST /api/presence/v2/renew`
- `POST /api/presence/v2/end`
- `POST /api/presence/v2/deactivate`, used to retire an authenticated Starry
  activation before switching profiles

`uuid` is the standard-base64 encoding of the profile-scoped
`network_identity_uuid`. Every profile must use a different value. The local
`profile_id` is not an identity, is absent from OpenAPI, and is rejected if a
caller includes it.

`start` accepts the normalized RustDesk `id`, network UUID, activation epoch,
random 16-byte activation ID, and one or more 32-byte Starry route leases. It
first verifies that exact active route through Starry `peer_registry`
capability 2. A successful response echoes the activation fields and returns:

- a random 16-byte base64url `lease_id`;
- a random 32-byte base64url bearer `lease_token`;
- absolute `expires_at`, relative `expires_in`, and aggregate `online_until`.

`renew` and `end` carry the same ID, UUID and activation fields plus the bearer
token. New clients should also send `lease_id`; Kessoku validates that the ID
and token select the same row. Token-only selection remains accepted for the
first Presence v2 client build. `end` is idempotent for the selected lease.
Responses containing credentials use `Cache-Control: no-store`.

The lease TTL is 45 seconds. Clients should renew no later than 15 seconds
before their local deadline and must treat an expired or rejected lease as a
new start. A crashed client needs no cleanup request: its lease stops
contributing to online state at `expires_at`.

## State and compatibility

A peer is v2-online while at least one unended lease for its current activation
has `expires_at` in the future. Ending one parallel lease cannot end another;
starting a higher activation retires lower-epoch leases; a delayed end remains
bound to its old token and cannot change the new activation.

The legacy heartbeat route and response body are unchanged. Kessoku retains
the existing 30-second `last_online_time` write throttle; v2 renewals do not
increase it. Immediately after v2 activity, the lease aggregate is
authoritative for 90 seconds so a recent inventory write cannot keep an ended
lease online. After that downgrade window, current legacy heartbeats determine
online state as before.

Device IDs are unique in schema 313. An established device ID cannot be rebound
to another non-empty network UUID, and an established account owner cannot be
replaced by another account. A conflict fails closed without changing device
metadata, ownership, or presence.

## Metrics

A super administrator can read `GET /api/admin/presence/v2/metrics` with the
normal `api-token` header. The response contains no token, device ID, UUID, IP
address, username, or other high-cardinality label.

`*_accepted_total`, `*_rejected_total`, and `*_errors_total` are process-local
counters and reset on restart. `active_leases`, `online_peers`, and
`expired_unended_leases` are database-wide gauges; use `max`, not `sum`, when
scraping multiple Kessoku replicas. The snapshot includes explicit
`counter_scope`, `gauge_scope`, `collected_at`, and `schema_version` fields.

Recommended initial alerts, to be tuned from production baselines:

- page when any `*_errors_total` increases for 5 minutes;
- warn when renew rejections exceed 5% of renew attempts for 10 minutes;
- warn when start rejections exceed 10% for 10 minutes, then inspect Starry
  capability/route health and client clock/activation state;
- warn when `expired_unended_leases` grows continuously for 30 minutes;
- page when `active_leases` falls to zero unexpectedly while registered
  clients are expected, or when the metrics endpoint cannot query the database.

Do not export lease tokens as labels, annotations, exemplars, traces, or alert
text. Lease IDs may be used only for short-lived diagnostic correlation and
must not become an unbounded metric label.

## Storage and migration

Schema 313 adds the `peer_presence_leases` table, peer activation/aggregate
columns, unique random lease/token-hash indexes, and a unique device-ID index.
Only SHA-256 token hashes are persisted. Expired lease history older than 24
hours is deleted by the existing six-hour retention worker; active and recent
rows are never deleted.

Before migration, this query must return no rows:

```sql
SELECT id, COUNT(*)
FROM peers
GROUP BY id
HAVING COUNT(*) > 1;
```

Kessoku performs the same duplicate-ID preflight and refuses schema changes or
the version-313 marker when a conflict exists. Resolve duplicates only after an
owner determines the correct UUID/account mapping. Never merge identities by
copying presence rows or lowering the schema marker.

Because v3.0.6 understands schema 312, rollback after a 313 migration is
restore-only: stop all writers and restore the matching pre-upgrade database
backup before starting v3.0.6.
