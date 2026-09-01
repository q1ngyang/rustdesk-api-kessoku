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

Before publishing a Kessoku release with adaptive support, publish the exact
Starry contract under an immutable tag, change
`internal/starrycontrol/CONTRACT_VERSION` from `LOCAL_CANDIDATE_VALIDATED` to
`PINNED`, record the matching release digest, and rerun the cross-repository and
real-instance tests. A moving or dirty Starry checkout is not release evidence.

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
