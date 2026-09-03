# RustDesk API Kessoku

**English** | [简体中文](README.zh-CN.md)

Kessoku is an unofficial RustDesk account, administration, and policy plane.
It provides the client API, an embedded management UI, and a built-in
open-source browser remote-desktop MVP, and integrates with
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
through a typed, versioned Control API.

> **v3.0.8 is the opt-in preview; v3.0.7 remains the stable release.** v3.0.8
> adds Starry v1.3.1 FastCompat/FastMedia management and SP1 pairing. The new
> controls remain default-off, the GitHub Release is a prerelease, and the
> moving image tag is `preview`; `latest` continues to identify v3.0.7. Real
> Akari/device/network and production-PKI validation remains required before a
> later stable promotion. v3.0.3 was the first
> supported v3 release. The
> earlier v3.0.1 Release was withdrawn after significant integration defects.
> The v3.0.2 tag records an unpublished release attempt and has no supported
> Release assets or container image; v3.0.4 likewise remains an unpublished
> failed-candidate record.
> The implementation and Linux
> amd64 checks are complete, the official Starry contract is pinned, and both
> the published Starry native-client matrix and built-in browser forced-Relay
> fixture pass. The complete candidate and publication-readiness checks finish
> before the protected workflow creates the immutable tag. Verify the GitHub Release checksums and pin the
> versioned GHCR digest for production.

## Component boundary

| Component | Responsibility |
| --- | --- |
| Kessoku API | Login, users, address books, devices, token lifecycle, administration, and audit. |
| Embedded `admin-web/` | Reviewed management UI built from the same Kessoku commit with `npm ci`. |
| Embedded `web-client/` | MIT browser MVP: forced Relay WSS, VP9 video, mouse, and basic keyboard. |
| Starry HBBS | Connection authentication, Relay allocation, and signalling. |
| Starry Control Agent | Optional mTLS/scoped-JWT API for Relay visibility and safe configuration transactions. |
| Official HBBR | Remote-control data Relay; Kessoku does not replace it. |

Kessoku does not expose a shell, arbitrary command, arbitrary Agent URL,
Docker socket, or browser-supplied file path. Its browser client is repository-
owned source; historical WebClient2/V2 and `resources/web*` assets remain
excluded.

## v3.0.8 preview highlights

- Exact, fail-closed negotiation of Starry
  `fast_relay_authorization=1`, `fast_media_relay_udp=1`, and schema v5,
  while patch-v1.2/v1.3.0 Relay pages remain compatible.
- Independent FastCompat/FastMedia controls with current healthy UDP Relay
  dependency checks, medium/high risk floors, exact confirmation, audited
  generation/schema digests, and subsystem activation ACKs.
- Aggregate-only Fast Relay telemetry and non-binding allocation simulation;
  no client report, address, session/allocation UUID, grant, token, or media is
  accepted for display.
- Allowlist-only SP1 Control Agent and Agent-authorized Relay enrollment, with
  one-time codes, CSR binding, response recovery, per-instance mTLS/JWT keys,
  UUID locking, read-only first pairing, and hot-loaded providers.
- An independent schema-v1 registry under `data/server-control`, secure static
  exports for v3.0.7 takeover, clone detection, backup/restore, host adoption,
  credential rotation, and explicitly confirmed purge. The main database stays
  at schema 313.

See the [v3.0.8 preview notes](docs/releases/v3.0.8/RELEASE-NOTES-v3.0.8.md)
and [upgrade/rollback guide](docs/releases/v3.0.8/MIGRATION-v3.0.8.md).

## v3.0.7 stable highlights

- Stable local S6/operator CLI for side-effect-free `version --json` and
  `config validate`, read-only `database status`, and explicit locked
  `database migrate`.
- Transactional, local-only `recover-admin` and `reset-2fa` operations with
  exact username confirmation, security audit, one authentication-generation
  rotation, and complete active-session revocation.
- Database schema advances from 312 to 313. SQLite, MySQL, and PostgreSQL
  serialize migrations, preflight duplicate device IDs, and reject databases
  newer than the binary before application services or key generation start.
- Presence Lease v2 keeps fast multi-profile switching race-safe: each
  profile uses its own network identity UUID, online state is the aggregate of
  valid 45-second leases, and delayed old activation requests cannot end a new
  activation. The legacy heartbeat API remains available unchanged.
- Secure automatic device discovery: active native sessions claim signed-in
  clients, Starry patch-v1.2.2 privately verifies the exact ID/UUID of network
  clients that are not signed in, and patch-v1.3.0 capability 2 additionally
  verifies Presence activation routes. Complete inventory refreshes on
  client startup/change and through a 24-hour heartbeat fallback, including
  every matching address-book reference.

- Responsive light/dark administration UI redesigned for desktop, tablet, and
  phone, centralized theme-aware branding, avatars, TOTP two-factor
  authentication, Japanese localization, announcements, and GeoIP details.
- Three-tier enterprise roles: `user`, scoped `admin`, and unrestricted
  `super_admin`. A scoped administrator can manage assigned user groups,
  users, public address books, and ID devices.
- Strict Ed25519/EdDSA access tokens with issuer, audience, key ID, JTI,
  lifetime, scope, and authentication-version checks.
- Revocation-aware JWKS and introspection on a dedicated TLS 1.3 mTLS listener.
- Typed Starry operations for capabilities, status, Relays, allocation
  simulation, configuration validation, plan/apply, history, rollback, and
  runtime reload.
- Administrator-only control routes and durable redacted intent/result audit.
- Legacy generic ServerCmd execution removed from the runtime surface.
- Embedded, reproducibly built management frontend; no moving frontend branch.
- Embedded, reproducibly built Web Client on a separate origin/listener, using
  short-lived connection-only grants delivered in memory. The MVP supports
  persistent sign-in, forced Relay WSS, VP9, mouse, basic keyboard, connection
  audit, and assistance chat; it excludes P2P, incoming mode, file transfer,
  clipboard, audio, display switching, and non-VP9 codecs.
- SQLite, MySQL, and PostgreSQL migration support; external MySQL/PostgreSQL
  connections require certificate- and hostname-verified TLS.
- Docker `linux/amd64`, Linux x86_64 archive/binary, and amd64 DEB as the
  v3.0.7 release scope. ARM remains best-effort and non-blocking.

> **Upgrade notice:** v3 changes the Go module path to `/v3` and database role
> semantics and upgrades the database to schema version 313. Read the
> [compatibility and upgrade notes](docs/releases/v3.0.7/RELEASE-NOTES-v3.0.7.md#safety-and-compatibility)
> before upgrading.

## Recommended deployment

Docker Compose on Linux amd64 is the recommended deployment. Use the immutable
version tag and then record the resolved digest in your deployment:

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.8
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
# Edit .env/config.yaml and provision the referenced signing key first.
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
```

The Compose default binds API port 21114 and Web Client port 21122 to
`127.0.0.1`. Publish them through two distinct reviewed HTTPS origins; see
[`examples/Caddyfile.example`](examples/Caddyfile.example). The preview also
publishes the moving `preview` tag. It does not replace `latest`, which remains
the v3.0.7 stable line. Production rollback should retain the previously
verified v3.0.7 digest.
The exact `relay-wss-urls` map lives in mounted YAML, not an environment
variable; follow the detailed Docker guide before startup.

Do not enable Starry `enforce` or configuration writes during the first start.
Migrate authentication in `off`/`audit`, commission the Control Agent
read-only, and complete a real-client and rollback rehearsal first.

## Documentation

Browse the [categorized documentation index](docs/README.md) for deployment,
operations, security, release history, and developer references.

| Topic | Document |
| --- | --- |
| First deployment | [Getting started](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Getting-Started) |
| Kessoku + Starry full stack | [Complete deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Complete-Deployment) |
| Remote HBBR-only node | [Relay-only deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment) |
| GHCR package users | [Container image guide](docs/deployment/CONTAINER.md) |
| Recommended Compose deployment | [Docker deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Deployment) |
| Nginx and firewall rules | [Reverse proxy and firewall](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Reverse-Proxy-and-Firewall) |
| Configuration | [Configuration reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference) |
| JWT/JWKS/introspection rollout | [Connection authentication](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Connection-Authentication) |
| Starry integration | [Starry control](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control) |
| Browser client deployment and exclusions | [Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client) |
| Deployment hardening | [Security configuration](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Security-Finding-Closure) |
| Backups and routine checks | [Operations and verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification) |
| Local S6/maintenance CLI | [Local maintenance CLI](docs/operations/LOCAL-MAINTENANCE-CLI.md) |
| Multi-profile presence leases | [Presence Lease v2 operations](docs/operations/PRESENCE-LEASE-V2.md) |
| Upgrade and rollback | [Upgrade and rollback](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback) |
| Failure diagnosis | [Troubleshooting](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Troubleshooting) |
| 中文文档 | [中文文档主页](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Home) |

The reviewed Wiki source is stored in [`docs/wiki/`](docs/wiki/). Publishing
those pages to GitHub Wiki is a separate release-owner action.

## Release status

The authoritative gate is [`RELEASE_STATUS`](RELEASE_STATUS), with evidence
requirements in [`RELEASE-CHECKLIST.md`](docs/releases/RELEASE-CHECKLIST.md). The
v3.0.8 preview feature and compatibility notes are in
[`RELEASE-NOTES-v3.0.8.md`](docs/releases/v3.0.8/RELEASE-NOTES-v3.0.8.md).
Version-by-version summaries are in the [English changelog](CHANGELOG.md).

Local development checks are not permission to publish. Tagging, pushing,
GHCR publication, GitHub Release creation, and Wiki publication require
separate explicit approval.

## Licence and acknowledgement

Kessoku is MIT licensed and is not affiliated with RustDesk. It continues the
work of the upstream `lejianwen/rustdesk-api` contributors. The embedded admin
frontend retains its own reviewed MIT provenance in
[`ADMIN-WEB-PROVENANCE.md`](docs/development/ADMIN-WEB-PROVENANCE.md). The repository-owned Web
Client is MIT licensed; dependency licences are recorded in its release SBOM.
