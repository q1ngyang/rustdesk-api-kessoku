# Starry control

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control)

Kessoku integrates with the optional Starry Control Agent through Control API
v1. The browser talks only to Kessoku; it never talks directly to the Agent.

## Authorization

- Every Kessoku server-control route requires administrator privilege.
- The Agent requires both an approved mTLS client identity and a short-lived
  EdDSA service JWT with the exact scope for the operation.
- Agent origins, TLS names, identities, and credential paths are fixed in
  deployment configuration.
- User access-token and Control Agent signing keyrings are separate.

## Supported operations

- capabilities and status;
- configured/eligible Relay inventory and health;
- side-effect-free allocation simulation for two IPs and a transport;
- configuration and JSON/UI schema reads;
- authoritative validation and short-lived plan creation;
- ETag/idempotency-protected apply and operation polling;
- history, rollback, and acknowledged runtime reload.

There is no raw command, shell, arbitrary URL, Docker socket, generic file API,
or force-overwrite operation.

## Adaptive Relay Quality

Kessoku negotiates Relay Quality only from Starry's Control v1 capabilities.
The current adaptive contract is identified by `relay_quality: 1`; the Relay
inventory repeats the frozen wire version as `quality.protocol_version: 1`.
Kessoku does not infer support from a Starry or HBBR version string.

| Starry response | Kessoku behavior |
| --- | --- |
| patch-v1.2 capabilities without `relay_quality` | Keeps the legacy Relay inventory working and shows quality fields as **Unsupported**. |
| `relay_quality: 1` with protocol/candidate/freshness fields | Validates and displays aggregate adaptive/eager state. |
| Advertised quality capability with missing or out-of-range required state | Rejects the response as an invalid upstream contract instead of showing partial data. |
| Unknown additive JSON fields | Ignores them; they are not reflected back through Kessoku's typed response. |
| Unsupported known capability version | Fails closed until Kessoku understands that version. |

The management page shows the current strategy and protocol, primary accepted
and expansion triggered ratios, P2P cancellations, estimated probe attempts
saved, timeout/invalid/late counters, fixed fallback-reason aggregates, and each
Relay's explicit probe/load protocol, telemetry freshness, and quality-candidate
state. It never renders individual client reports, nonces, allocation/session
UUIDs, full client IPs, or connection tokens.

Configuration remains schema driven. Kessoku renders the exact JSON Schema and
UI Schema returned by `/control/v1/config/schema`; it does not keep another
Relay Quality field specification. Localized help explains adaptive/eager,
primary samples, accept score, the loss gate, and P2P grace, while Starry remains
the authoritative validator. Every change follows `validate -> plan -> apply ->
generation ACK`. The frozen patch-v1.3 classifier returns at least `medium` for
every `/relay_quality` change. Kessoku also preserves a defensive `medium` floor
if an older or non-conforming agent returns `low`, without changing its plan ID
or candidate digest.

The page raises state-based alerts when Starry explicitly marks Relay telemetry
stale or Relay Quality is enabled with no current quality candidate. Aggregate
timeouts, invalid/late reports, and fallback reasons are cumulative snapshots;
monitor their deltas or rates against a deployment baseline rather than using
an invented absolute threshold. Alerts never contain per-client reports or
high-cardinality allocation data.

Allocation simulation is explicitly non-binding. It may show the matched GEO
rule, GEO primary, candidate order, transport eligibility, and a possible
adaptive/eager flow. The simulation has no real client probe data and must not
be interpreted as a final score, a selected Relay, or predicted client RTT.

## FastCompat, FastMedia, and schema v5

The v3.0.8 source candidate pins the frozen contract-only Starry commit
`6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`. It accepts only
`fast_relay_authorization: 1`, `fast_media_relay_udp: 1`, and
`config_schema: 5`. Completely absent capabilities render as **Unsupported**;
an unknown version of any known capability rejects the upstream response.
The Starry runtime release remains blocked, so this pin is not an image or tag
claim.

Schema v5 adds the Starry-owned `fast_mode.relay` fields
`fast_compat_enabled`, `fast_media_v1_enabled`,
`authorization_ttl_seconds`, `max_bitrate_kbps`, and
`relay_max_datagram`. Kessoku renders the returned schema and UI schema and
adds only localized explanations. The two toggles are independent and default
off. Enabling FastMedia is not shown as successful unless current authenticated
Relay telemetry proves a capability-v1, telemetry-schema-v2, matching-port,
fresh, healthy candidate.

Every `/fast_mode` plan is at least medium risk. FastMedia enablement and UDP
endpoint, datagram, bitrate, or related firewall changes are high risk. The
administrator must confirm the exact plan, and Kessoku binds the review to the
actor, ETag, base generation, candidate digest, schema digest, risk, change
count, and expiry. Success additionally requires the returned Starry generation,
source/effective digest, schema version, and accepted subsystem activation ACKs.

The dashboard distinguishes Unsupported, Configuration disabled, Dependency
unmet, Enabled without a healthy candidate, Authorized server event, UDP
activity unknown, and Reliable fallback. It displays only server aggregates:
bind success/rejection, cookie/grant and role/session/allocation rejection
classes, rebind, forwarded/dropped/rate-limited packets, expiry, listener
failure, and reliable fallback. It discards upstream process, allocation, and
session UUIDs and never receives or renders client addresses, tokens, signed
grants, private keys, or media content. Counters prove server events only; they
do not prove that a particular client entered FastMedia.

## SP1 pairing and Relay enrollment

When pairing is enabled, the deployment owns an exact HTTPS Broker origin,
its TLS SPKI SHA-256 pin, and a list of pre-approved Agent origins/TLS names.
The UI and CLI select only an allowlist ID; neither accepts an arbitrary Agent
URL or callback. Public claims are size-limited, strict JSON, `no-store`, and
bind the frozen SP1 purpose/action, enrollment/configuration digests, one-time
secret digest, CSR public key, and instance identity. Repeating the exact claim
with the same public key recovers a response lost within the recovery window;
purpose exchange, CSR change, expiry, and replay after that window fail closed.
An unclaimed code can be revoked by its enrollment ID in the UI or with
`server-control pair revoke`; the raw code is never submitted back to Kessoku.

Each instance has an independent client CA/certificate/key and service-JWT
Ed25519 key below `data/server-control/instances`. A new `pair` learns and then
locks the Agent instance UUID. `adopt` and `rotate` pre-bind the existing UUID.
First pairing and every rotation are read-only; the provider is hot-loaded only
after the registry transaction and static export succeed.

Relay enrollment is different from Control Agent pairing: Kessoku first asks
the already-authenticated Agent to authorize the exact endpoint, pool, profile,
capacity, certificate plan, and expiry. `activate_after_health` is immutable
high-risk preauthorization made when the code is created; otherwise the Relay
stays pending approval. Explicit activation sends the exact successful
operation, configuration generation/digest, and Agent health snapshot. Kessoku
does not retain the raw code, Relay key, telemetry secret, or returned bundle.

The independent registry is `/app/data/server-control/registry-v1.sqlite` in
the container and `/var/lib/kessoku-api/data/server-control/registry-v1.sqlite`
under systemd. It does not change the main schema. Every managed instance also
gets a v3.0.7-compatible `server-control.instances[]` static export. Preserve
the whole directory across container recreation and binary downgrade; see the
[v3.0.8 migration guide](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.8/MIGRATION-v3.0.8.md).
Pairing also requires an explicit `host-identity-file`: Compose mounts the
host file outside the registry at `/run/kessoku-host-machine-id`, and systemd
uses `/etc/machine-id`. Kessoku stores only a domain-separated SHA-256
fingerprint and rejects a copied registry on another host until the exact
installation is explicitly adopted.

Before publication, Starry must provide an immutable runtime release and image
provenance for the frozen contract, and Kessoku must pass real-instance,
fallback/re-entry, Akari dual-role, NAT/UDP, rotation/migration, hosted CI,
security, SBOM, signing, and attestation gates. A moving/dirty Starry checkout
or a contract-only summary is not runtime release evidence.

## Initial setup

1. Pin the exact released Starry Control/Auth contract in
   `internal/starrycontrol/CONTRACT_VERSION`.
2. Deploy the Agent on a private management path with writes disabled.
3. Verify instance identity, capabilities, status, Relay inventory, and
   simulation from Kessoku.
4. In staging, validate and plan a harmless configuration change.
5. Open a controlled write window, apply, verify the actual active generation,
   and rehearse rollback.
6. Restore Kessoku and the Agent to read-only outside change windows.

An HTTP success is not proof that the new configuration became active. Always re-read the
generation/digests and correlate Kessoku intent/result audit with the Agent's
durable audit.
