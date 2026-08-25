# Relay-only deployment: HBBR

**English** | [简体中文](ZH-CN-Relay-Only-Deployment.md)

This tutorial deploys one HBBR-only node on a separate Linux server. It runs no
Kessoku API or HBBS. Use it to move Relay traffic away from the centre, add
regional capacity, or provide a dedicated `wss://relay-1.example.com/ws/relay`
endpoint for the built-in browser client.

The node supports the official
[`rustdesk/rustdesk-server:1.1.16`](https://hub.docker.com/r/rustdesk/rustdesk-server/tags?name=1.1.16)
HBBR. The recommended default is the HBBR bundled in
[`q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0`](https://github.com/q1ngyang/rustdesk-server-starry).
That HBBR remains
unmodified upstream code, but using the same pinned upstream revision as the
Starry centre avoids independent HBBS/HBBR version drift. Both image choices
are present in the example YAML and environment file; the official choice is
commented out by default.

## What the node does

```text
Client A ── registration/signalling ──> centre HBBS <── Client B
   │                                         │
   └──── native 21117 or WSS ──> Relay-only HBBR <────┘
                                  relay-1.example.com
```

HBBS selects and returns a Relay. HBBR only forwards the remote-session data.
Kessoku login, successful registration, and an open TCP port do not prove that
a Relay session works.

## Requirements

- Debian/Ubuntu `linux/amd64` host with `sudo` access;
- Docker Engine and the Compose plugin;
- public IPv4 and `relay-1.example.com` DNS record;
- the centre HBBS `id_ed25519.pub` complete one-line public key;
- permission to update the centre HBBS Relay list.

Native Relay needs public `21117/TCP`. The built-in browser client requires a
certificate-valid WSS endpoint, so also prepare Nginx and public 80/443.

```sh
docker version
docker compose version
getent ahosts relay-1.example.com
```

Create an `AAAA` record only when public IPv6 actually works on the node.

## Download the examples

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" /opt/rustdesk-relay
cd /opt/rustdesk-relay

base_url=https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/relay
curl -fsSLo compose.yaml "$base_url/compose.yaml"
curl -fsSLo .env "$base_url/.env.example"
curl -fsSLo nginx-bootstrap.conf "$base_url/nginx-bootstrap.conf.example"
curl -fsSLo nginx-relay.conf "$base_url/nginx.conf.example"

chmod 0600 .env
chmod 0644 compose.yaml nginx-bootstrap.conf nginx-relay.conf
sudo install -d -m 0700 -o root -g root /opt/rustdesk-relay/data
```

Persist `.env`, `compose.yaml`, `data/`, the Nginx site, and certificates. Never
copy the centre private `id_ed25519` file to a Relay-only node. HBBR receives
only the public key through `KEY`.

## Choose the HBBR image

The active Compose line is the recommended image:

```yaml
image: ${STARRY_IMAGE:-ghcr.io/q1ngyang/rustdesk-server-starry}:${STARRY_VERSION:-1.1.16-patch-v1.2.0}
# image: ${OFFICIAL_HBBR_IMAGE:-rustdesk/rustdesk-server:1.1.16}
```

The matching `.env` entries are:

```dotenv
STARRY_IMAGE=ghcr.io/q1ngyang/rustdesk-server-starry
STARRY_VERSION=1.1.16-patch-v1.2.0
# OFFICIAL_HBBR_IMAGE=rustdesk/rustdesk-server:1.1.16
```

Keep the default when the centre uses the same Starry version. To use official
HBBR, comment the Starry image line and uncomment the official image line in
`compose.yaml`; then comment the two Starry variables and uncomment
`OFFICIAL_HBBR_IMAGE` in `.env`. Run `docker compose config` and confirm that
exactly one intended image remains. Do not use `latest`.

## Configure `.env`

```sh
editor /opt/rustdesk-relay/.env
```

Set the public key and an absolute data path:

```dotenv
RUSTDESK_PUBLIC_KEY=PASTE_THE_COMPLETE_CENTRE_ID_ED25519_PUB_VALUE
RELAY_DATA_DIR=/opt/rustdesk-relay/data
```

Only read the centre file ending in `.pub`. Do not display or copy
`id_ed25519`, and do not generate a separate key pair on each Relay.

The example also exposes upstream HBBR limits:

| Variable | Example | Meaning |
| --- | ---: | --- |
| `RELAY_SINGLE_BANDWIDTH` | `128` | Per-session limit in Mb/s |
| `RELAY_TOTAL_BANDWIDTH` | `1024` | Aggregate HBBR limit in Mb/s |
| `RELAY_LIMIT_SPEED` | `32` | Throttled-session limit in Mb/s |
| `RELAY_DOWNGRADE_START_CHECK` | `1800` | Seconds before downgrade checks |
| `RELAY_DOWNGRADE_THRESHOLD` | `0.66` | Average-use fraction of the per-session limit |

Tune these values for actual egress capacity and concurrency.

## Firewall

| Port | Public? | Purpose |
| --- | --- | --- |
| actual SSH port/TCP | trusted sources | Administration |
| `21117/TCP` | Yes | Native HBBR Relay |
| `80/TCP`, `443/TCP` | When WSS is enabled | ACME, redirect, and `/ws/relay` |
| `21119/TCP` | No | Plain local WebSocket backend |

The node does not need public 21114-21116 or 21118.

```sh
sudo ufw allow 22/tcp
sudo ufw allow 21117/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 21119/tcp
sudo ufw enable
sudo ufw status numbered
```

Replace port 22 before enabling UFW and apply the same allow-list to the cloud
security group. Because the example uses host networking, the host firewall is
the boundary for port 21119.

## Start HBBR

```sh
cd /opt/rustdesk-relay
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbr

docker inspect rustdesk-relay-hbbr --format '{{.Config.Image}}'
sudo ss -lntp | grep -E ':(21117|21119)\b'
```

From another public host, test `nc -vz relay-1.example.com 21117`. This proves
only reachability; complete a real forced-Relay session later.

## Add the Relay to Starry HBBS

Add the exact native address to the centre `starry-config.yaml`:

```yaml
relay_servers:
  - relay-1.example.com:21117
```

List every Relay when there are multiple nodes. If WSS is enabled,
`relay_health.endpoints` must cover the Relay list exactly:

```yaml
websocket_signal:
  enabled: true
  relay_health:
    endpoints:
      - relay: relay-1.example.com:21117
        url: wss://relay-1.example.com/ws/relay
```

Geo rules must reference the same exact address:

```yaml
geo:
  enabled: true
  rules:
    - name: Default Relay
      symmetric: true
      match:
        client_a: "*"
        client_b: "*"
      relays:
        - relay-1.example.com:21117
```

Restart HBBS, inspect its logs, and confirm the configuration was accepted.

An official HBBS can receive comma-separated Relay addresses with `-r`:

```yaml
command:
  - hbbs
  - -r
  - relay-1.example.com:21117,relay-2.example.com:21117
```

Official HBBS does not provide Starry Geo rules or Starry WSS health-based
selection.

## Publish `/ws/relay` for browser use

Skip this section for native-only desktop clients.

```sh
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

cd /opt/rustdesk-relay
sudo cp nginx-bootstrap.conf /etc/nginx/sites-available/rustdesk-relay.conf
sudo editor /etc/nginx/sites-available/rustdesk-relay.conf
sudo ln -sfn /etc/nginx/sites-available/rustdesk-relay.conf \
  /etc/nginx/sites-enabled/rustdesk-relay.conf
sudo nginx -t
sudo systemctl reload nginx
sudo certbot certonly --nginx -d relay-1.example.com

sudo cp nginx-relay.conf /etc/nginx/sites-available/rustdesk-relay.conf
sudo editor /etc/nginx/sites-available/rustdesk-relay.conf
sudo nginx -t
sudo systemctl reload nginx
```

The final site proxies only exact `/ws/relay` to `127.0.0.1:21119`; native
21117 does not pass through Nginx.

```sh
curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  https://relay-1.example.com/ws/relay
```

Expect HTTP 101. Never use `curl -k` to hide a certificate failure.

## Update the Kessoku browser profile

When HBBS can return this Relay to the built-in browser client, add the exact
map and increment the generation:

```yaml
web-client:
  relay-wss-urls:
    "relay-1.example.com:21117": "wss://relay-1.example.com/ws/relay"
  profile-generation: 2
```

Every Relay available to browser users needs a certificate-valid WSS mapping.
A native-only Relay may still serve desktop clients, but must not be selected
for the built-in browser client.

## Real-session verification

1. configure two desktop clients with the same centre and public key;
2. make the centre select `relay-1.example.com:21117` for the test;
3. force a Relay connection from one client;
4. verify video, mouse, keyboard, and a sustained session;
5. correlate the same time range in HBBR logs;
6. when WSS is enabled, test WSS-to-WSS and required mixed modes.

```sh
docker compose --env-file .env -f compose.yaml logs --since 10m hbbr
```

A peer-to-peer session does not use HBBR and is not a Relay-node test.

## Backup and update

Back up `.env`, `compose.yaml`, `data/`, the Nginx site, and certificates. Record
the current image before an update, use explicit tags, update one Relay at a
time, and complete a real session after each change:

```sh
docker inspect rustdesk-relay-hbbr --format '{{.Config.Image}}'
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbr
```

Rollback by restoring the previous image line and tag. Never rotate the centre
identity merely to troubleshoot a Relay node.

For the complete centre stack, see [Complete Deployment](Complete-Deployment.md).
For the full port matrix, see
[Reverse Proxy and Firewall](Reverse-Proxy-and-Firewall.md).
