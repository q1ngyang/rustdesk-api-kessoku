# Kessoku v2.8.0 release notes (unreleased)

**English** | [简体中文](RELEASE-NOTES-v2.8.0.zh-CN.md)

v2.8.0 turns the former general RustDesk API service into a bounded account
and administration plane designed to pair with
`rustdesk-server-starry patch-v1.2.0`.

This document is a release-content draft for owner review. It does not claim
that a tag, image, Wiki, package, or GitHub Release has been published.

## Recommended Docker deployment

Docker Compose on Linux amd64 is the recommended deployment.

- [GHCR image page](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [Container image guide](CONTAINER.md)
- [Docker deployment guide](docs/wiki/Docker-Deployment.md)
- [Compose example](docker-compose.yaml)
- [Environment example](examples/compose.env.example)
- [Starry integration guide](docs/wiki/Starry-Control.md)

The release publishes
`ghcr.io/q1ngyang/rustdesk-api-kessoku:v2.8.0` and `:latest` for the same image.
The version tag is immutable; `latest` moves only after a successful stable
release. Resolve and pin the version tag's digest for production rollout.

## New in v2.8.0

### Authentication and token lifecycle

- Ed25519/EdDSA access tokens with fixed `at+jwt` type, key ID, issuer,
  audience, subject/user binding, JTI, scope, authentication version, and
  bounded time claims.
- Current/previous JWKS key overlap for controlled rotation.
- Token hashes replace reusable plaintext-token storage for newly issued
  credentials.
- Single-session logout, password change, user disable, and global session
  invalidation have explicit revocation semantics.
- A dedicated TLS 1.3 mTLS internal listener provides bounded JWKS and token
  introspection to exact approved Starry certificate identities.
- OIDC requires a non-empty ID-token subject and an exact UserInfo subject
  match. Callback state is claimed atomically once, provider bodies and codes
  are bounded, trailing JSON is rejected, and callback origins must be fixed
  public HTTPS origins. OAuth/OIDC use of the application proxy is rejected
  because a proxy cannot preserve destination-address validation.
- Provider/subject and user/provider bindings are unique. Upgrade preflight
  reports duplicate legacy bindings without silently deleting or merging them.

### Typed Starry management

- Fixed deployment-owned Starry instance origins and credential file
  references; browser requests cannot choose an Agent URL.
- Capabilities, status, Relay inventory, and side-effect-free two-address
  allocation simulation bound to an explicit non-zero configuration generation.
- Configuration schema/read/validate/plan/apply/operation/history/rollback and
  synchronous runtime reload through Control API v1.
- Per-request mTLS and short-lived least-scope service JWTs.
- ETag, plan identity, idempotency, response-size, timeout, error normalization,
  and redacted intent/result audit boundaries.
- Read-only control is the default.

### Administration and frontend supply chain

- Every control route is administrator-only.
- The generic legacy ServerCmd execution path is absent from the runtime API;
  the compatibility endpoint can only return `410 Gone`.
- Management frontend source lives under `admin-web/`, shares the backend
  commit/tag, and builds from its lockfile with `npm ci`.
- No moving frontend branch and no externally substituted compiled assets.
- User create/delete, role/status changes, session revocation, and audit-record
  deletion produce durable administrator audit events. Role or status changes
  revoke existing sessions, and a database-wide invariant prevents concurrent
  operations from removing the final enabled administrator.
- Address books, collections, tags, and peer metadata are owner-scoped. Client
  persistence IDs and nested ORM associations are not accepted, and address
  book/tag synchronization commits atomically with bounded string tags.

### Browser-client boundary

- Kessoku contains no WebClient2 assets or download/proxy path.
- The optional external Web Client Provider is disabled by default, requires
  provenance/licence/version/digest metadata, and receives no bearer token.
- The provider is only an independently hosted HTTPS launch/governance
  descriptor. v2.8.0 does not host a browser remote-desktop client, share
  cookies or storage, or implement SSO/token exchange. See
  [Web Client](docs/wiki/Web-Client.md).

### Packaging and automation

- Linux amd64 is the supported v2.8.0 platform: GHCR image, Linux x86_64
  archive/binary, and amd64 DEB.
- The image runs as an unprivileged user and excludes browser-client assets.
- Candidate workflows run Go tests and race checks, three database migration
  fixtures, embedded-frontend tests/audits/reproducibility checks, SBOM and
  secret/dependency scans, image smoke tests, and real DEB installation.
- Publication consumes one exact successful non-publishing candidate run and
  emits checksums plus Sigstore provenance/SBOM attestations.

## Compatibility and migration

- Database version 301 adds token hashes, JTI/key/auth-version fields,
  administrator audit events, OAuth identity uniqueness, and the shared final-
  administrator invariant. SQLite, MySQL 8.4, and PostgreSQL 16 fixtures are
  covered.
- External MySQL now requires `mysql.tls: "true"`; an optional `mysql.ca-file`
  adds a private CA to the operating-system trust pool. PostgreSQL requires
  `postgresql.sslmode: verify-full` and can use `ssl-root-cert`. Insecure or
  hostname-unverified database transport fails startup.
- The migration is additive, but older applications cannot use credentials
  issued without a plaintext token. After v2.8.0 issues tokens, application
  rollback requires the matching verified pre-upgrade database backup.
- Existing opaque credentials may use a bounded compatibility phase. Removed
  HS256 settings are not a supported connection-authentication profile.
- Start Starry authentication in `off`, then `audit`. Enable `enforce` only
  after the supported real-client matrix has no unexplained would-deny result.
- Commission the Control Agent read-only and rehearse rollback before enabling
  configuration writes.

See [Upgrade and rollback](docs/wiki/Upgrade-and-Rollback.md) and
[`MIGRATION.md`](MIGRATION.md).

## Known limitation accepted for v2.8.0

RustDesk 1.4.9 audit/sysinfo uploads do not carry an authorization header.
Kessoku retains a bounded compatibility route requiring an already registered
peer ID and exact UUID, but UUIDs are not secrets and a known pair can still be
used to submit spoofed operational telemetry. Export these records to
append-only or immutable storage when non-repudiation is required. The full
disposition and evidence boundary are in
[Security finding closure](docs/wiki/Security-Finding-Closure.md).

## Platform scope

- Supported: Docker `linux/amd64`, Linux x86_64 archive/binary, amd64 DEB.
- Best effort and non-blocking: ARM source/build compatibility.
- Outside the v2.8.0 release promise: Windows artifacts.

## Pre-release status

Local Go, race, frontend, reproducibility, package-install, non-root image,
security-header, contract, and cross-project process tests have passed. The
published Starry `1.1.16-patch-v1.2.0` Control API has been pinned by tag,
source commit, contract hash, schema hashes, and amd64 image digest. RustDesk
1.4.9 forced-Relay desktop sessions passed `audit` native-to-native and
`enforce` native-to-native, WSS-to-WSS, WSS-to-native, and native-to-WSS,
including Remote Desktop window/screenshot and established HBBR connection
checks. This matrix does not claim direct-P2P or a separate Secure TCP case.
The exact local candidate verifier and post-remediation static review now pass.
Publication remains blocked on release-owner and protected-workflow actions:

- record the target deployment's backup/restore, key recovery, failover,
  rollback, RTO/RPO, and go/no-go ownership;
- approve the final documentation and new-feature wording; and
- run protected non-publishing candidate CI for the approved commit before the
  immutable tag, GHCR `v2.8.0`/`latest`, GitHub Release, and Wiki are published.
