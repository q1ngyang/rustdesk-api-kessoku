# rustdesk-api-kessoku container image

**English** | [简体中文](CONTAINER.zh-CN.md)

This page is the versioned entry point for users arriving from the GHCR
package page. Docker Compose on Linux amd64 is the recommended deployment.

> The `v3.0.2` image referenced here is a release target until the protected
> publication workflow completes. The published `latest` tag will identify the
> newest successful stable release; do not substitute a local worktree image.

Deployment links:

- [GHCR image page](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [Recommended Docker deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Deployment)
- [Compose example](../../docker-compose.yaml)
- [Environment example](../../examples/compose.env.example)
- [Builtin Web Client configuration](../../examples/config.docker-builtin.yaml)
- [Caddy HTTPS example](../../examples/Caddyfile.example)
- [Getting started](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Getting-Started)
- [Starry integration](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control)
- [Upgrade and rollback](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback)

## Image scope

The v3.0.2 image contains one unprivileged `kessoku-api` process, the reviewed
management and Web Client frontends built from the same source commit, API
documentation, and runtime configuration templates. The image:

- targets `linux/amd64`;
- runs as UID/GID `65534:65534`;
- persists application data under `/app/data`;
- listens on public API port `21114`;
- exposes the independent built-in Web Client listener on `21122`;
- can use a separate internal mTLS listener on `21121` when explicitly enabled;
- contains reviewed `resources/client`, but no historical WebClient2/V2,
  `resources/web`, or `resources/web2` assets; and
- contains no private key or deployment credential.

Kessoku is not HBBS or HBBR. Deploy the matching Starry HBBS and official HBBR
separately.

## Pull and inspect

After publication:

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.2
docker image inspect \
  ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.2 \
  --format '{{json .RepoDigests}}'
```

The workflow publishes both immutable `v3.0.2` and moving `latest` tags for the
same image. Use `latest` only when tracking the newest stable release is
intentional; resolve and pin the versioned tag's digest for production change
control and rollback.

## Compose quick start

From a v3.0.2 source checkout or downloaded deployment files:

```sh
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
sudo install -d -m 0700 -o 65534 -g 65534 data/kessoku secrets

# Create the one-use bootstrap secret without placing it in Compose or shell
# arguments. UID 65534 is the unprivileged user inside the image.
openssl rand -base64 24 | sudo tee secrets/bootstrap-admin-password >/dev/null
sudo openssl genpkey -algorithm ED25519 \
  -out secrets/kessoku-access-ed25519.pem
sudo chown 65534:65534 secrets/bootstrap-admin-password
sudo chown 65534:65534 secrets/kessoku-access-ed25519.pem
sudo chmod 0600 secrets/bootstrap-admin-password secrets/kessoku-access-ed25519.pem

# Edit every placeholder before continuing. Keep relay-wss-urls as an exact
# YAML map in config.yaml rather than trying to encode it in .env.
vi .env config.yaml

docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

The example drops Linux capabilities, enables `no-new-privileges`, uses a
read-only root filesystem, keeps runtime logs in a tmpfs, and persists only the
application data directory. The public RustDesk server key in `.env` is not a
private key.

## First login

On a new database Kessoku creates `admin` with an unreachable random bootstrap
credential and never prints a reusable password. The reset command above sets
the first usable password from a regular file whose group/other permission
bits are zero. Transfer that value directly to an approved password manager,
open `https://your-api.example/dash/` through the reviewed reverse proxy,
sign in, rotate the password, then delete the host secret file. Do not pass a
password as a command argument or environment variable.

The Compose default publishes ports `21114` and `21122` on `127.0.0.1` only. A
host-local Caddy example provides different API/admin and Web Client HTTPS
origins in [`examples/Caddyfile.example`](../../examples/Caddyfile.example). Compose
mounts `KESSOKU_CONFIG_FILE` read-only; the supplied builtin example explicitly
enables `web-client.mode: builtin`. Edit its exact Relay-name-to-WSS YAML map,
origins, public key, generation, and authentication key path before startup;
Viper environment decoding is not used for the Relay map. Never serve the
client below the API origin as a path.
Set `gin.trust-proxy` to the exact proxy address; never publish internal port
`21121` through this proxy.

Do not expose Swagger in production unless it is an intentional, authenticated
operational decision.

The built-in MVP supports forced Relay WSS, VP9 video, mouse and basic
keyboard. It excludes P2P/direct transport, incoming mode, file/clipboard/
audio, display switching, and non-VP9 codecs. Its connection-only token is
short-lived and delivered in memory to the exact client origin. See
[Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client).

## Secrets and advanced configuration

Basic API operation can use environment variables. Authentication and Starry
control require a reviewed configuration plus files mounted read-only under
`/run/secrets`, including separate:

- Kessoku access-token Ed25519 private key;
- internal-listener server certificate/key and Starry client CA;
- Kessoku Control Agent client certificate/key and CA; and
- Control Agent service-JWT signing key.

The access-token and Control Agent signing keys must never be the same key.
Do not put secret bytes in Compose YAML or Git.

The internal listener must not be routed through the public API reverse proxy.
When it is reachable across a container network, configure its container-side
listener deliberately and restrict the path to approved Starry identities.

## Deployment acceptance

Container start and HTTP reachability are partial evidence only. Before
enabling connection enforcement or Agent writes, verify:

1. database migration and backup restoration;
2. administrator and ordinary-user login/logout;
3. JWKS and introspection over the dedicated mTLS path;
4. Starry `audit` results for every supported client transport;
5. Relay inventory and side-effect-free allocation simulation;
6. read-only Control Agent behavior, then plan/apply/rollback in staging; and
7. the final native, Secure TCP, WSS, and Relay desktop-session matrix.
8. a browser forced-Relay VP9 session with mouse/keyboard, grant expiry/logout,
   and separate-origin/CSP verification.

See [Operations and verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification).

## Upgrade and rollback

Back up the database, authentication keys, internal PKI, configuration,
current image digest, and Starry generation before upgrading. Kessoku database
version 309 includes enterprise roles, encrypted TOTP state, media references,
branding, GeoIP policy, preferences, and WebClient audit metadata. A v2 binary
can interpret a scoped administrator as unrestricted; restore the complete
matching pre-upgrade database/key/media set before starting an older
application.

External MySQL/PostgreSQL connections must use certificate- and hostname-
verified TLS. Mount a private CA read-only when required; see the
[configuration reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference).

Never overwrite or move an exposed version tag. See
[Upgrade and rollback](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback).
