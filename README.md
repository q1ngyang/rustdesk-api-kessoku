# RustDesk API Kessoku

**English** | [简体中文](README.zh-CN.md)

Kessoku is an unofficial RustDesk account, administration, and policy plane.
It provides the client API, an embedded management UI, and a built-in
open-source browser remote-desktop MVP, and integrates with
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
through a typed, versioned Control API.

> **v3.0.1 stable release.** The implementation and Linux
> amd64 checks are complete, the official Starry contract is pinned, and both
> the published Starry native-client matrix and built-in browser forced-Relay
> fixture pass. The immutable tag is published only through the protected
> candidate/release workflow. Verify the GitHub Release checksums and pin the
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

## v3.0.1 highlights

- Responsive light/dark administration UI redesigned for desktop, tablet, and
  phone, with repository-owned Kessoku/StarryLinks brand assets.
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
  forced Relay WSS, VP9, mouse, and basic keyboard; it excludes P2P, incoming
  mode, file transfer, clipboard, audio, display switching, and non-VP9 codecs.
- SQLite, MySQL, and PostgreSQL migration support; external MySQL/PostgreSQL
  connections require certificate- and hostname-verified TLS.
- Docker `linux/amd64`, Linux x86_64 archive/binary, and amd64 DEB as the
  v3.0.1 release scope. ARM remains best-effort and non-blocking.

> **Upgrade notice:** v3 changes the Go module path to `/v3` and database role
> semantics at schema version 302. Read the
> [breaking changes](docs/releases/v3.0.1/RELEASE-NOTES-v3.0.1.md#breaking-changes) before upgrading.

## Recommended deployment

Docker Compose on Linux amd64 is the recommended deployment. Use the immutable
version tag and then record the resolved digest in your deployment:

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.1
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
# Edit .env/config.yaml and provision the referenced signing key first.
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
```

The Compose default binds API port 21114 and Web Client port 21122 to
`127.0.0.1`. Publish them through two distinct reviewed HTTPS origins; see
[`examples/Caddyfile.example`](examples/Caddyfile.example). The release also
publishes `latest` for users who intentionally track the newest stable build,
while production rollback should pin the `v3.0.1` digest.
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
| Upgrade and rollback | [Upgrade and rollback](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback) |
| Failure diagnosis | [Troubleshooting](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Troubleshooting) |
| 中文文档 | [中文文档主页](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Home) |

The reviewed Wiki source is stored in [`docs/wiki/`](docs/wiki/). Publishing
those pages to GitHub Wiki is a separate release-owner action.

## Release status

The authoritative gate is [`RELEASE_STATUS`](RELEASE_STATUS), with evidence
requirements in [`RELEASE-CHECKLIST.md`](docs/releases/RELEASE-CHECKLIST.md). The v3.0.1
feature and compatibility notes are in
[`RELEASE-NOTES-v3.0.1.md`](docs/releases/v3.0.1/RELEASE-NOTES-v3.0.1.md).

Local development checks are not permission to publish. Tagging, pushing,
GHCR publication, GitHub Release creation, and Wiki publication require
separate explicit approval.

## Licence and acknowledgement

Kessoku is MIT licensed and is not affiliated with RustDesk. It continues the
work of the upstream `lejianwen/rustdesk-api` contributors. The embedded admin
frontend retains its own reviewed MIT provenance in
[`ADMIN-WEB-PROVENANCE.md`](docs/development/ADMIN-WEB-PROVENANCE.md). The repository-owned Web
Client is MIT licensed; dependency licences are recorded in its release SBOM.
