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
5. Admin/API origin to the independent built-in Web Client origin, and that
   browser client to fixed HTTPS/WSS endpoints.
6. Kessoku to SQLite storage or a certificate-verified external database.

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

Role/status changes and deletion serialize through a shared database invariant
row before counting enabled administrators and writing the change. This keeps
at least one enabled administrator even when multiple Kessoku replicas receive
concurrent requests. A disabled administrator is not incorrectly treated as
the final enabled administrator.

The signing private key is read from `current-key.private-key-file`. Previous
keys are public-key files used only during a bounded rotation overlap. Access
keys must be stored outside the image and database, mounted read-only, backed
up separately, and readable only by the service account.

## OAuth and OIDC identities

OAuth state is bounded, expires, and is atomically claimed once before a
provider callback is used. Login completion remains bound to the initiating
device ID/UUID; account binding remains bound to the initiating authenticated
user. OIDC requires a verified non-empty ID-token `sub` and an exact UserInfo
`sub` match. Provider responses and authorization codes are bounded, and
trailing JSON is rejected.

Provider discovery/token/UserInfo endpoints and the public callback origin
must be absolute public HTTPS URLs. Direct connections resolve every address,
reject private/local/special-use targets, pin the selected public address, and
reject redirects. `proxy.enable` is rejected because an HTTP proxy performs an
independent target resolution that Kessoku cannot verify. OAuth identity rows
enforce unique provider/subject and user/provider bindings; migration stops
with an actionable error instead of choosing between duplicate identities.

## Database transport and invariants

SQLite is a local file trust boundary. External MySQL requires `tls: "true"`,
uses hostname verification and the operating-system trust pool, and can add a
private CA bundle with `ca-file`. PostgreSQL requires `sslmode: verify-full`
and can use `ssl-root-cert`. Insecure, require-only, or skip-verify profiles
fail configuration validation. Database credentials are encoded by driver/URL
builders and are not interpolated into a raw DSN.

The schema contains unique authentication/identity indexes and a shared
security-invariant lock used to serialize the final-enabled-administrator
check across replicas. Database backups are confidential and must be restored
and checked before an upgrade window.

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
them non-binding and both request and response carry the same explicit non-zero
configuration generation. Relay inventory also requires a non-zero generation.
Apply/rollback additionally require ETag and idempotency guards.

## Legacy server commands

Legacy command routes are absent by default. If the explicit compatibility
flag registers `/api/admin/rustdesk/sendCmd`, the route still requires
administrator privilege and always returns `410 Gone`; no command or option is
parsed, stored, forwarded, or executed. The historical `server_cmds` table is
preserved only so database upgrades do not destroy operator data.

## Built-in Web Client

The repository-owned Web Client is disabled unless `web-client.mode` is
`builtin`. Its static files are served only by the independent listener and
origin; the API/admin origin never serves `resources/client`. Historical
`resources/web`, `resources/web2`, WebClient2/V2, and downloaded client assets
are rejected by packaging policy.

The client accepts only fixed HTTPS/WSS endpoints from same-origin public
configuration. It requires forced Relay, an exact configured Relay-name map,
signed peer identity, encrypted application messages, VP9 WebCodecs, and
bounded input/frame/state sizes. It fails closed on identity, authentication,
codec, counter, decrypt, decode, origin, destination, or resource-limit errors.

Connection credentials are audience/scope-limited and short-lived (15 minutes
by default, one hour maximum). Admin launch obtains a connection-only grant
with the existing RustAuth bearer, but passes only that grant—not the API/admin
credential—to the exact deployment-configured client origin. Strict-origin
`postMessage` and client memory are the only handoff/storage boundary. Tokens,
passwords, keys, and session state are forbidden in URLs, cookies, persistent
browser storage, service workers, analytics, or logs. CORS permits only the
exact client public origin, three fixed POST/OPTIONS endpoints,
`Authorization`/`Content-Type`, and no credentials.

The API/admin origin sends COOP `same-origin-allow-popups` for the one-shot
popup handoff. The independent client listener sends no COOP header
(`unsafe-none` default), because copying `same-origin` or severing the opener
would break the exact-source handshake. The client validates the API/admin
origin and opener source, acknowledges once, removes its listener, then tries
to clear the opener. Admin timeout or failure closes the popup and revokes the
issued connection token on a best-effort basis; accepted handoff transfers
revocation responsibility to the client.

RustDesk 1.4.9's standard login token is the native compatibility exception:
the client retains one bearer for API access and signalling, so it carries
both configured `kessoku-api` and `rustdesk-connect` audiences. The Web Client
never receives that standard bearer. Its login and grant paths return only a
short-lived `rustdesk-connect`/`connect:initiate` connection token.

See [WEB-CLIENT.md](WEB-CLIENT.md).

## Assumptions and residual risks

- Host, database, secret-file, CA, and reverse-proxy administration are trusted
  deployment responsibilities.
- A compromised administrator can request all operations exposed by the typed
  API, but cannot turn it into a raw command or arbitrary-URL proxy.
- RustDesk 1.4.9 audit/sysinfo uploads have no authorization header. The
  compatibility surface is limited to 64 KiB, bounded fields, and an exact
  already-persisted peer ID/UUID; the request cannot change its owner. A UUID
  is not a secret, so someone who knows both values can still submit spoofed
  operational telemetry.
- Audit rows are operational evidence, not an immutable external audit ledger.
  Export them to append-only storage where non-repudiation is required.
- The published Starry `1.1.16-patch-v1.2.0` image and binaries are pinned by
  digest/hash. RustDesk 1.4.9 forced-Relay sessions passed audit native/native
  and enforce native/native, WSS/WSS, WSS/native, and native/WSS. Direct P2P
  and a separate Secure TCP case were not claimed by that matrix.
- Both `admin-web/` and the MIT `web-client/` live in the same Kessoku commit,
  use fixed lockfiles and independent SBOM/licence evidence, and are built with
  `npm ci`. The browser MVP deliberately excludes P2P/direct transport,
  incoming mode, file/clipboard/audio, display switching, and non-VP9 codecs.
  No historical WebClient2/V2 assets are included.

See [Security finding closure](docs/wiki/Security-Finding-Closure.md) for the
sealed-snapshot boundary, all 23 Kessoku dispositions, and the accepted
compatibility residual.
