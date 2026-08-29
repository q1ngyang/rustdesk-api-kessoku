# Docker deployment

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment)

Docker Compose on Linux amd64 is the recommended Kessoku deployment. The
repository example runs only Kessoku; Starry HBBS and official HBBR remain
separate services with independent data and ports.

For a complete centre use [Complete Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Complete-Deployment). To
add a remote HBBR without another API or HBBS, use
[Relay-Only Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment).

## Architecture

```text
RustDesk client ── HTTPS 443 ── reverse proxy ── Kessoku 21114
                                                │
Starry HBBS ── private TLS 1.3/mTLS ────────────┤ 21121 JWKS/introspection
                                                │
Admin browser ─ HTTPS /dash/ ─────────────────┘

Browser remote client ── separate HTTPS origin ── Kessoku 21122

Kessoku ── private mTLS + scoped JWT ── Starry Control Agent
```

Never route port `21121` or the Control Agent through the public API path.

## Files

- [`docker-compose.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docker-compose.yaml)
- [`examples/compose.env.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/compose.env.example)
- [`examples/config.docker-builtin.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/config.docker-builtin.yaml)
- [`examples/Caddyfile.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/Caddyfile.example)
- [`examples/nginx/kessoku.example.conf`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/nginx/kessoku.example.conf)
- [`examples/relay/compose.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/relay/compose.yaml)
- [`examples/relay/.env.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/relay/.env.example)
- [`conf/config.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/conf/config.yaml)
- [`CONTAINER.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/deployment/CONTAINER.md)

## Prepare

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" /opt/kessoku
cd /opt/kessoku
cp /path/to/repository/docker-compose.yaml .
cp /path/to/repository/examples/compose.env.example .env
cp /path/to/repository/examples/config.docker-builtin.yaml config.yaml
cp /path/to/repository/examples/Caddyfile.example .
vi .env config.yaml

sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/kessoku/data/kessoku /opt/kessoku/secrets
openssl rand -base64 24 | sudo tee secrets/bootstrap-admin-password >/dev/null
sudo openssl genpkey -algorithm ED25519 \
  -out secrets/kessoku-access-ed25519.pem
sudo chown 65534:65534 secrets/bootstrap-admin-password
sudo chown 65534:65534 secrets/kessoku-access-ed25519.pem
sudo chmod 0600 secrets/bootstrap-admin-password secrets/kessoku-access-ed25519.pem
```

Compose mounts `KESSOKU_CONFIG_FILE` read-only at `/app/conf/config.yaml`.
The supplied builtin example enables the client and keeps
`relay-wss-urls` as an exact YAML map; edit that map, both origins, all WSS
endpoints, public key, generation, and authentication key path under change
control. Do not assume Viper can safely decode the Relay map from an
environment variable. Place key and certificate files in `secrets/` with
service-account-only permissions.

## Validate and start

```sh
docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml pull
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml ps
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

Confirm the container runs as `65534:65534`, the root filesystem is read-only,
only `/app/data` persists, `/app/resources/client/index.html` exists, and
neither `/app/resources/web` nor `/app/resources/web2` exists. Move the
bootstrap password directly into an
approved password manager, sign in, rotate it, and delete the host secret.
Kessoku never prints a reusable bootstrap password.

## Reverse proxy and ports

The Compose default publishes Kessoku `21114` and Web Client `21122` on host
loopback only. Expose them as two distinct public HTTPS origins through a
reviewed reverse proxy; the supplied Caddy example is for a proxy on the same
host. Enable `web-client.mode: builtin` only after its exact public/API origins,
WSS map, server public key and positive generation are configured.
Configure `gin.trust-proxy` only for exact proxy addresses. Preserve the
security headers produced by Kessoku and set an explicit request-body limit and
timeout at the proxy.

The internal `21121` listener is optional and disabled by default. When
enabled, bind it only to a private interface/container network, require TLS
1.3 and verified client certificates, and firewall it to Starry.

## Persistent data and backup

SQLite deployments store the database at `/app/data/rustdeskapi.db`. Back up
the entire data directory consistently. MySQL/PostgreSQL deployments require
vendor-consistent database backups in addition to Kessoku keys, PKI,
configuration, image digest, and release provenance.

External databases must be configured before the first v3.0.6 start. MySQL
requires `tls: "true"`; PostgreSQL requires `sslmode: "verify-full"`. For
private PKI, place the CA in `secrets/`, mount it read-only under
`/run/secrets`, and set `mysql.ca-file` or `postgresql.ssl-root-cert` to that
container path. The database address/host must match a certificate SAN.
Kessoku intentionally exits on an insecure mode, unreadable CA, unknown CA, or
hostname mismatch. See [Configuration reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference).

## Verify

Verify administrator login, ordinary API login/logout, address-book access,
database version 301, OAuth identity index/invariant presence, and logs before
introducing Starry authentication. A complete
deployment then follows the staged acceptance in
[Operations and Verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification).
Also verify the browser client public profile contains no secret, grant handoff
uses the exact origin, and one forced-Relay VP9 mouse/keyboard session completes
and logs out. See [Built-in Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client).
