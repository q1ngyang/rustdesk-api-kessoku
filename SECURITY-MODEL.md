# Kessoku Security Model

## Scope and trust boundaries

Kessoku owns user accounts, API sessions, access-token signing, revocation,
administrator authorization, and orchestration of the Starry control plane.
Starry owns RustDesk connection enforcement, Relay selection, configuration
semantics, persistence, reload, and last-known-good rollback.

The main trust boundaries are:

1. RustDesk clients to the public Kessoku API.
2. Starry to Kessoku's dedicated internal JWKS/introspection listener.
3. Kessoku to each fixed Starry Control Agent instance.
4. Administrators to Kessoku's versioned control API.
5. Kessoku to an independently hosted external Web Client Provider.

Kessoku never mounts Starry configuration, accesses a Docker socket, executes
shell commands, or accepts an Agent URL from a browser request.

## Access tokens

When `auth.enabled` is true, Kessoku issues only Ed25519/EdDSA access tokens
with protected `typ=at+jwt`. Verification fixes the algorithm, issuer,
audience, token type, key ID, numeric `user_id`/canonical decimal `sub`
binding, scope, `iat`, `nbf`, `exp`, UUID JTI, and positive `auth_version`.
Tokens larger than 8 KiB are rejected. The configured token lifetime cannot
exceed `maximum-token-ttl`.

New database rows contain only a SHA-256 token hash and token metadata. They do
not contain a reusable bearer token. A single-device logout revokes its JTI.
Password changes, user disablement, and global logout atomically increase the
user's `auth_version` and revoke existing rows.

The signing private key is read from `current-key.private-key-file`. Previous
keys are public-key files used only during a bounded rotation overlap. Access
keys must be stored outside the image and database, mounted read-only, backed
up separately, and readable only by the service account.

## Internal JWKS and introspection

The internal API runs on its own TLS listener and is never registered on the
public Gin listener:

```text
GET  /api/internal/v1/auth/jwks
POST /api/internal/v1/auth/introspect
```

TLS 1.3 and a client certificate chained to the configured private CA are
mandatory. The leaf certificate must contain an exact allow-listed URI SAN or
DNS SAN. Proxy headers and source IP do not establish identity. The listener
also enforces global and per-certificate request limits, strict JSON, a body
limit of at most 1 MiB, and a short request deadline.

Inactive introspection responses deliberately use the same external reason for
invalid, expired, revoked, disabled, unknown-user, and rotated-key cases. This
prevents account enumeration. Bearer tokens must not appear in logs, errors,
traces, or metrics labels.

Starry connection authentication is deployed as `off`, then `audit`, then
`enforce`. Those modes are a Starry responsibility. On an introspection cache
miss, an unavailable authority must fail closed in enforce mode. Any bounded
positive cache means revocation can take effect no later than the documented
cache TTL; local negative caching must also be bounded.

Starry's configured remote JWKS and introspection clients require TLS 1.3,
trust only the explicitly configured CA, present their client identity, require
the URL host to equal the configured DNS server name, and reject redirects.
Introspection sends only the token field; an active response without the exact
locally verified subject fails closed.

## Starry Control Agent

Every Agent instance is fixed in deployment configuration and must use an
HTTPS origin without credentials, query, fragment, or path. Kessoku disables
ambient HTTP proxies, requires TLS 1.3, validates the configured TLS server
name and CA, and presents a client certificate.

Each request also receives a separately signed EdDSA service JWT with a maximum
five-minute lifetime (currently two minutes), a fixed service subject, exact
instance audience, authorized party, operation-specific scope, administrator
actor, and unique JTI. Control signing keys are checked at startup to prevent
reuse of an access-token signing key.

The checked-in client permits only the versioned Control API route/method/scope
matrix. It rejects redirects, oversized requests and responses, unexpected
content types, malformed JSON, multiple JSON values, arbitrary paths, arbitrary
headers, and arbitrary commands. Agent error details are mapped to stable
Kessoku errors before they reach a browser.

All control routes require both an authenticated backend session and
`AdminPrivilege()`. Write operations additionally fail with
`CONTROL_READ_ONLY` while `server-control.read-only` is true. Every control
read and mutation creates a redacted intent record before local/provider work
and records success or failure afterward. An unavailable audit store prevents
the Agent call. Simulation responses are accepted only when the Agent marks
them non-binding; apply/rollback also require ETag and idempotency guards.

## Legacy server commands

Legacy command routes are absent by default. If the explicit compatibility
flag registers `/api/admin/rustdesk/sendCmd`, the route still requires
administrator privilege and always returns `410 Gone`; no command or option is
parsed, stored, forwarded, or executed. The historical `server_cmds` table is
preserved only so database upgrades do not destroy operator data.

## External Web Client Provider

The external provider is disabled by default and hosted on an independent
origin. Kessoku does not serve provider assets and does not pass a token,
cookie, user identity, address book, or session. See
[WEB-CLIENT-PROVIDER.md](WEB-CLIENT-PROVIDER.md).

## Assumptions and residual risks

- Host, database, secret-file, CA, and reverse-proxy administration are trusted
  deployment responsibilities.
- A compromised administrator can request all operations exposed by the typed
  API, but cannot turn it into a raw command or arbitrary-URL proxy.
- Audit rows are operational evidence, not an immutable external audit ledger.
  Export them to append-only storage where compliance requires it.
- The digest-locked local candidate has completed native/Secure TCP/WSS
  enforcement tests and real HBBS/Control-Agent-to-Kessoku configuration
  apply/rollback E2E. A published immutable Starry candidate and supported
  real-client staging matrix remain release gates.
- The separately maintained admin frontend is a release blocker until its fork,
  dependency audit, tests, and fixed reviewed commit are available.
