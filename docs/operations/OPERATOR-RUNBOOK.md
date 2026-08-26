# Kessoku Operator Runbook

This runbook describes normal operation of the authentication and Starry
control boundaries. It does not replace database-, PKI-, or Starry-specific
procedures.

## Startup checks

Before each deployment, verify:

- the image/artifact digest and backend commit match the approved provenance;
- both pinned frontend and Starry contract gates are satisfied;
- access-token and Control Agent private keys are distinct, outside the image,
  read-only, and service-account-only;
- the internal server key, client CA, and exact Starry URI/DNS SAN allow list
  are present;
- every Starry instance uses a fixed HTTPS origin, expected instance ID, TLS
  server name, CA, client certificate/key, and separate control signing key;
- `legacy-command-enabled` is false;
- `read-only` is true unless a configuration-change window is approved;
- built-in Web Client mode is disabled unless its separate HTTPS origin,
  listener, exact WSS map, public key/generation, short grant TTL, and artifact
  evidence were reviewed.

When the Web Client is enabled, verify `resources/client/index.html`, host-
local 21122 exposure, public profile without secrets, exact CORS, and the
ready/grant/ack origin/source handshake. Alert on grant/login failures without
logging username, peer ID, token, password, public profile body, or key bytes.

Startup must fail when the EdDSA profile or internal TLS listener is enabled
but its required key/certificate material is invalid. A Starry instance with
invalid deployment material is listed as unavailable rather than silently
falling back to a raw transport.

## Authentication health

From a host holding an approved Starry client certificate, query the dedicated
internal listener with its CA and client keypair. Verify JWKS contains the
expected current and previous key IDs and that introspection returns only the
minimal documented claims. Run these checks directly against the internal
listener, not through the public API proxy.

Expected operational properties:

- TLS below 1.3, missing/untrusted certificates, and unmatched SANs fail;
- JWKS is cacheable for five minutes but contains public keys only;
- introspection is `no-store` and invalid credentials return
  `{"active":false,"reason":"inactive"}` without enumeration detail;
- rate-limit responses are explicit and must not cause Starry to fail open;
- a single logout is reflected on the next authoritative introspection call.

Alert on internal listener unavailability, authentication failure rate,
would-deny/enforce-deny rate, latency, database saturation, and rate limiting.
Do not label metrics with tokens, JTI values, usernames, certificate contents,
or full public IPs.

## Starry control operation

All browser calls go to `/api/admin/server-control/v1`; only administrators are
authorized. Check an instance in this order:

1. capabilities and instance identity;
2. health/status;
3. Relay inventory;
4. an allocation simulation using approved test IPs;
5. configuration and schema ETags.

An instance ID mismatch or contract-major mismatch is a deployment incident,
not a retriable UI error. An unreachable Agent should degrade within the
configured timeout and must not consume an unbounded response.

For a configuration change:

1. Confirm an approved change ticket, actor, target instance, and maintenance
   window.
2. Read the current configuration, generation, schema, and ETag.
3. Validate the candidate through the Agent. Browser-side validation is only a
   convenience.
4. Create a plan and review its diff, warnings, restart requirement, base ETag,
   target generation, and expiry.
5. Enable writes only for the window by setting `read-only: false` and
   restarting Kessoku under change control.
6. Apply the exact plan with its current ETag and a new idempotency key.
7. Poll the returned operation when asynchronous.
8. Re-read active configuration and verify the actual generation rather than
   trusting HTTP success alone.
9. Confirm the persisted audit intent/result and Starry's own audit record.
10. Restore `read-only: true` after the window.

Never resolve an ETag conflict by force-overwriting. Re-read, merge, validate,
and plan again. Never paste private keys or secret values into YAML; use the
secret-reference fields supported by the published Starry schema.

## Incident responses

### Suspected access-token key compromise

Isolate the key, preserve evidence, create a distinct replacement key, revoke
affected sessions/auth versions, publish the reviewed current/previous set,
and force re-login as required. Removing a previous key immediately invalidates
all still-valid JWTs signed by it. Coordinate Starry JWKS cache invalidation and
do not reuse the compromised key as a Control Agent key.

### Internal introspection outage

Keep connection enforcement fail-closed for cache misses. Do not expose the
listener publicly or disable certificate verification as a workaround. If
service restoration cannot meet the approved recovery objective, follow the
explicit rollback sequence; any temporary `audit` change requires incident
authorization and must be recorded.

### Starry Agent identity or TLS failure

Set/keep Kessoku read-only, block configuration changes, verify DNS/routing,
certificate chain, server name, expected instance ID, clock, and deployment
provenance. Do not enable proxy inheritance, skip verification, or edit the
browser request to another URL.

### Configuration apply failure

Poll the operation, inspect only redacted stable error codes, verify whether
Starry restored last-known-good, and compare the live generation. If automatic
recovery is incomplete, execute the approved Agent rollback and then the
application rollback sequence in [ROLLBACK-RUNBOOK.md](ROLLBACK-RUNBOOK.md).

## Backup and retention

Back up the database, current/previous access public keys, private signing keys,
internal PKI, deployment configuration, and provenance records under separate
access controls. Test restoration regularly. Export audit events to protected
append-only storage according to local retention policy; never enrich exports
with bearer tokens, private keys, certificates, complete YAML, or raw public
IP addresses.
