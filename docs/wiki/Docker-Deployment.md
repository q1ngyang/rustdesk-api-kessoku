# Docker deployment

**English** | [简体中文](ZH-CN-Docker-Deployment.md)

Docker Compose on Linux amd64 is the recommended Kessoku deployment. The
repository example runs only Kessoku; Starry HBBS and official HBBR remain
separate services with independent data and ports.

## Architecture

```text
RustDesk client ── HTTPS 443 ── reverse proxy ── Kessoku 21114
                                                │
Starry HBBS ── private TLS 1.3/mTLS ────────────┤ 21121 JWKS/introspection
                                                │
Admin browser ─ HTTPS /_admin/ ─────────────────┘

Kessoku ── private mTLS + scoped JWT ── Starry Control Agent
```

Never route port `21121` or the Control Agent through the public API path.

## Files

- [`docker-compose.yaml`](../../docker-compose.yaml)
- [`examples/compose.env.example`](../../examples/compose.env.example)
- [`examples/Caddyfile.example`](../../examples/Caddyfile.example)
- [`conf/config.yaml`](../../conf/config.yaml)
- [`CONTAINER.md`](../../CONTAINER.md)

## Prepare

```sh
install -d -m 0700 /opt/kessoku/data/kessoku /opt/kessoku/secrets
cd /opt/kessoku
cp /path/to/repository/docker-compose.yaml .
cp /path/to/repository/examples/compose.env.example .env
cp /path/to/repository/examples/Caddyfile.example .
vi .env

umask 077
openssl rand -base64 24 > secrets/bootstrap-admin-password
chown 65534:65534 secrets/bootstrap-admin-password
chmod 0600 secrets/bootstrap-admin-password
```

For advanced authentication/control settings, copy `conf/config.yaml`, edit it
under change control, and add a read-only Compose mount to
`/app/conf/config.yaml`. Place key and certificate files in `secrets/` with
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
only `/app/data` persists, and neither `/app/resources/web` nor
`/app/resources/web2` exists. Move the bootstrap password directly into an
approved password manager, sign in, rotate it, and delete the host secret.
Kessoku never prints a reusable bootstrap password.

## Reverse proxy and ports

The Compose default publishes Kessoku `21114` on host loopback only. Expose
public HTTPS through a reviewed reverse proxy; the supplied Caddy example is
for a proxy on the same host.
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

## Verify

Verify administrator login, ordinary API login/logout, address-book access,
database version, and logs before introducing Starry authentication. A complete
deployment then follows the staged acceptance in
[Operations and Verification](Operations-and-Verification.md).
