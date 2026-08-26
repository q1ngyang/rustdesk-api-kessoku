# Connection authentication

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication)

Kessoku issues and revokes access tokens; Starry verifies them on RustDesk
signalling transports. Neither service can provide the complete property alone.

## Token profile

Tokens use Ed25519/EdDSA, `typ=at+jwt`, and an explicit key ID. Kessoku binds
issuer, API and connection audiences, decimal subject and numeric user ID,
`connect:initiate` scope, authentication version, UUID JTI, and bounded
`iat`/`nbf`/`exp` claims. The maximum encoded token is 8 KiB.

RustDesk 1.4.9 retains one standard login token for both API calls and native
signalling and has no refresh/exchange step. That compatibility token therefore
has the two configured audiences `kessoku-api` and `rustdesk-connect`; this is
not the browser privilege model. The built-in Web Client never receives the
standard bearer: direct client login and admin grant exchange return only a
short-lived token with audience `rustdesk-connect` and scope
`connect:initiate`.

New token rows store a hash/JTI rather than a reusable token. Logout revokes one
JTI; password reset, disable, and global logout advance the user's
authentication version.

## JWKS and introspection trust boundary

The dedicated internal listener provides:

```text
GET  /api/internal/v1/auth/jwks
POST /api/internal/v1/auth/introspect
```

It is independent of the public listener and requires TLS 1.3, a verified
client certificate, exact allowed SAN identity, body/time limits, and global
plus per-certificate rate limits. Introspection accepts only a token and
returns a minimal non-enumerating result. Do not expose it through the public
reverse proxy.

## Rollout order

1. Back up and migrate the Kessoku database.
2. Enable EdDSA issuance with current/previous key overlap.
3. Confirm supported clients receive and retain the new login token.
4. Enable the internal mTLS listener and verify Starry identity.
5. Configure Starry schema v3 in `off`, then `audit` for a complete business
   cycle.
6. Reconcile every would-deny reason and exercise logout, disable, password
   reset, key rotation, and introspection outage.
7. Canary `enforce`; expand only after native, Secure TCP, WSS, P2P, and Relay
   sessions pass with no fail-open behavior.

## Failure policy and rollback

Unknown/stale keys and configured introspection failure cannot silently allow a
cache miss in enforce mode. Roll back enforcement to `audit` under change
control; do not bypass TLS verification or publish introspection publicly.

See [`MIGRATION.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/MIGRATION.md) and
[`ROLLBACK-RUNBOOK.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/operations/ROLLBACK-RUNBOOK.md).
