# RustDesk API Kessoku

**English** | [简体中文](README.zh-CN.md)

Kessoku is an unofficial RustDesk account, administration, and policy plane.
It provides the client API and an embedded management UI, and integrates with
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
through a typed, versioned Control API.

> **v2.8.0 release draft.** The implementation and exact local Linux amd64
> candidate checks are complete, the official Starry contract is pinned, and the
> published Starry audit-to-enforce real-client matrix passes locally.
> Publication remains blocked until deployment recovery acceptance, protected
> candidate CI, and final release-owner approval are recorded. Do not deploy an
> untagged worktree build as a production release.

## Component boundary

| Component | Responsibility |
| --- | --- |
| Kessoku API | Login, users, address books, devices, token lifecycle, administration, and audit. |
| Embedded `admin-web/` | Reviewed management UI built from the same Kessoku commit with `npm ci`. |
| Starry HBBS | Connection authentication, Relay allocation, and signalling. |
| Starry Control Agent | Optional mTLS/scoped-JWT API for Relay visibility and safe configuration transactions. |
| Official HBBR | Remote-control data Relay; Kessoku does not replace it. |

Kessoku does not expose a shell, arbitrary command, arbitrary Agent URL,
Docker socket, or browser-supplied file path. It does not include, download,
proxy, or bypass licensing for WebClient2 assets.

## v2.8.0 highlights

- Strict Ed25519/EdDSA access tokens with issuer, audience, key ID, JTI,
  lifetime, scope, and authentication-version checks.
- Revocation-aware JWKS and introspection on a dedicated TLS 1.3 mTLS listener.
- Typed Starry operations for capabilities, status, Relays, allocation
  simulation, configuration validation, plan/apply, history, rollback, and
  runtime reload.
- Administrator-only control routes and durable redacted intent/result audit.
- Legacy generic ServerCmd execution removed from the runtime surface.
- Embedded, reproducibly built management frontend; no moving frontend branch.
- SQLite, MySQL, and PostgreSQL migration support; external MySQL/PostgreSQL
  connections require certificate- and hostname-verified TLS.
- Docker `linux/amd64`, Linux x86_64 archive/binary, and amd64 DEB as the
  v2.8.0 release scope. ARM remains best-effort and non-blocking.

## Recommended deployment

Docker Compose on Linux amd64 is the recommended deployment. After the
reviewed release is published, use the immutable version tag and then record
the resolved digest in your deployment:

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v2.8.0
cp examples/compose.env.example .env
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
```

The Compose default binds the application port to `127.0.0.1`; publish it
through the reviewed HTTPS reverse-proxy example in
[`examples/Caddyfile.example`](examples/Caddyfile.example). The release also
publishes `latest` for users who intentionally track the newest stable build,
while production rollback should pin the `v2.8.0` digest.

Do not enable Starry `enforce` or configuration writes during the first start.
Migrate authentication in `off`/`audit`, commission the Control Agent
read-only, and complete a real-client and rollback rehearsal first.

## Documentation

| Topic | Document |
| --- | --- |
| First deployment | [Getting started](docs/wiki/Getting-Started.md) |
| GHCR package users | [Container image guide](CONTAINER.md) |
| Recommended Compose deployment | [Docker deployment](docs/wiki/Docker-Deployment.md) |
| Configuration | [Configuration reference](docs/wiki/Configuration-Reference.md) |
| JWT/JWKS/introspection rollout | [Connection authentication](docs/wiki/Connection-Authentication.md) |
| Starry integration | [Starry control](docs/wiki/Starry-Control.md) |
| Browser client boundary | [Web Client](docs/wiki/Web-Client.md) |
| Security review and accepted residual | [Security finding closure](docs/wiki/Security-Finding-Closure.md) |
| Acceptance evidence | [Operations and verification](docs/wiki/Operations-and-Verification.md) |
| Upgrade and rollback | [Upgrade and rollback](docs/wiki/Upgrade-and-Rollback.md) |
| Failure diagnosis | [Troubleshooting](docs/wiki/Troubleshooting.md) |
| 中文文档 | [中文文档主页](docs/wiki/ZH-CN-Home.md) |

The reviewed Wiki source is stored in [`docs/wiki/`](docs/wiki/). Publishing
those pages to GitHub Wiki is a separate release-owner action.

## Release status

The authoritative gate is [`RELEASE_STATUS`](RELEASE_STATUS), with evidence
requirements in [`RELEASE-CHECKLIST.md`](RELEASE-CHECKLIST.md). The v2.8.0
feature and compatibility draft is in
[`RELEASE-NOTES-v2.8.0.md`](RELEASE-NOTES-v2.8.0.md).

Local development checks are not permission to publish. Tagging, pushing,
GHCR publication, GitHub Release creation, and Wiki publication require
separate explicit approval.

## Licence and acknowledgement

Kessoku is MIT licensed and is not affiliated with RustDesk. It continues the
work of the upstream `lejianwen/rustdesk-api` contributors. The embedded admin
frontend retains its own reviewed MIT provenance in
[`ADMIN-WEB-PROVENANCE.md`](ADMIN-WEB-PROVENANCE.md).
