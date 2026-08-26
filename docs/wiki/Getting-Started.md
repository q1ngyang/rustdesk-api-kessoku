# Getting started

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)

This guide deploys Kessoku for an existing HBBS/HBBR installation. If no
HBBS/HBBR exists yet, use the
[complete Kessoku + Starry tutorial](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Complete-Deployment). It intentionally
keeps strict connection enforcement and Starry configuration writes disabled
until their separate acceptance steps are complete.

## Prerequisites

- Linux amd64 with a supported Docker Engine and Compose plugin;
- a running RustDesk ID Server and Relay Server;
- the public `id_ed25519.pub` value from that server;
- public HTTPS names for the Kessoku API and its separate browser-client site;
- certificate-valid `/ws/id` and `/ws/relay` endpoints when the built-in
  browser client is enabled; and
- a database backup when upgrading an existing API deployment.

## Prepare and validate

```sh
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
sudo install -d -m 0700 -o 65534 -g 65534 data/kessoku secrets
sudo openssl genpkey -algorithm ED25519 \
  -out secrets/kessoku-access-ed25519.pem
openssl rand -base64 24 | sudo tee secrets/bootstrap-admin-password >/dev/null
sudo chown 65534:65534 secrets/*
sudo chmod 0600 secrets/*
vi .env config.yaml

docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
```

Review the resolved image, bind address, public API URL, ID/Relay addresses,
public server key, and persistent paths. Do not continue with placeholder
values.

## Start the API

```sh
docker compose --env-file .env -f docker-compose.yaml pull
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml ps
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

Kessoku does not log a reusable initial credential. Move the value in the
bootstrap file directly to an approved password manager, open
`https://your-api.example/_admin/`, sign in, rotate the password, and delete the
host secret file. Configure one supported RustDesk client with the same API
Server, ID Server, and server public key, then verify login, address-book
access, logout, and login again.

## Enable optional integrations

1. Verify and back up database version 302.
2. Configure Ed25519 access-token keys and enable Kessoku authentication.
3. Start the internal JWKS/introspection listener on private mTLS.
4. Deploy the matching Starry release with authentication `off`, then `audit`.
5. Start the Control Agent read-only.
6. Complete supported-client and rollback acceptance before `enforce` or writes.

See [Connection Authentication](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Connection-Authentication),
[Starry Control](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control), and
[Operations and Verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification).
