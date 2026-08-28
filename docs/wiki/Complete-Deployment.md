# Complete deployment: Kessoku + Starry HBBS/HBBR

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)

This tutorial deploys the complete single-host stack on a blank Linux server:

- Kessoku API and administration UI;
- Kessoku's built-in browser client;
- Starry HBBS;
- the unmodified HBBR bundled in the same pinned Starry image;
- Nginx, HTTPS, and WSS.

Kessoku also works with compatible official HBBS/HBBR services. Starry is the
recommended new deployment because it is maintained by the same developer and
adds Secure TCP, WSS signalling, Geo Relay selection, connection-token
verification, and an optional control agent.

The examples use Debian/Ubuntu `linux/amd64` and three DNS names:

| Name | Service |
| --- | --- |
| `rustdesk.example.com` | Native HBBS/HBBR address and WSS entry point |
| `api.example.com` | Kessoku API and administration UI |
| `client.example.com` | Dedicated browser-client site |

## Final network layout

```text
RustDesk desktop clients
  ├─ 21115/TCP        HBBS NAT test
  ├─ 21116/TCP+UDP    registration, signalling, NAT traversal, Secure TCP
  ├─ 21117/TCP        native HBBR Relay
  └─ 443/TCP          Nginx
       ├─ rustdesk.example.com/ws/id    -> 127.0.0.1:21118
       ├─ rustdesk.example.com/ws/relay -> 127.0.0.1:21119
       ├─ api.example.com/*             -> 127.0.0.1:21114
       └─ client.example.com/*          -> 127.0.0.1:21122
```

Ports 21114, 21118, 21119, and 21122 are proxy backends and must not be public.
Optional control/authentication ports 21120 and 21121 are private as well.

## 1. Install Docker and configure DNS

Install Docker Engine and the Compose plugin from the
[official Docker instructions](https://docs.docker.com/engine/install/), then
check them:

```sh
docker version
docker compose version
```

Create `A` records for all three names. Add `AAAA` records only when the host
has working public IPv6:

```sh
getent ahosts rustdesk.example.com
getent ahosts api.example.com
getent ahosts client.example.com
```

## 2. Download the deployment files

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" /opt/rustdesk-stack
cd /opt/rustdesk-stack

base_url=https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/combined
curl -fsSLo compose.yaml "$base_url/compose.yaml"
curl -fsSLo .env "$base_url/.env.example"
curl -fsSLo kessoku-config.yaml "$base_url/kessoku-config.yaml"
curl -fsSLo starry-config.yaml "$base_url/starry-config.yaml"
curl -fsSLo nginx-bootstrap.conf "$base_url/nginx-bootstrap.conf.example"
curl -fsSLo nginx-rustdesk.conf "$base_url/nginx.conf.example"

chmod 0600 .env
chmod 0644 compose.yaml kessoku-config.yaml starry-config.yaml \
  nginx-bootstrap.conf nginx-rustdesk.conf
```

## 3. Prepare persistent storage and Kessoku secrets

```sh
sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/rustdesk-stack/data/kessoku \
  /opt/rustdesk-stack/secrets/kessoku
sudo install -d -m 0700 -o root -g root \
  /opt/rustdesk-stack/data/starry

sudo openssl genpkey -algorithm ED25519 \
  -out /opt/rustdesk-stack/secrets/kessoku/kessoku-access-ed25519.pem
openssl rand -base64 24 | sudo tee \
  /opt/rustdesk-stack/secrets/kessoku/bootstrap-admin-password >/dev/null
sudo chown 65534:65534 /opt/rustdesk-stack/secrets/kessoku/*
sudo chmod 0600 /opt/rustdesk-stack/secrets/kessoku/*
```

Kessoku runs as UID/GID `65534:65534`; both the data and secret directories
must be traversable by that identity. Do not solve permission failures with
root containers or mode 0777.

Back up these paths:

```text
/opt/rustdesk-stack/data/kessoku/
/opt/rustdesk-stack/secrets/kessoku/
/opt/rustdesk-stack/data/starry/
/opt/rustdesk-stack/.env
/opt/rustdesk-stack/compose.yaml
/opt/rustdesk-stack/kessoku-config.yaml
/opt/rustdesk-stack/starry-config.yaml
/etc/nginx/sites-available/rustdesk-stack.conf
/etc/letsencrypt/
```

## 4. Edit `.env` and both YAML files

Set the exact DNS names and absolute host paths in `.env`:

```dotenv
RUSTDESK_DOMAIN=rustdesk.example.com
KESSOKU_PUBLIC_URL=https://api.example.com

KESSOKU_CONFIG_FILE=/opt/rustdesk-stack/kessoku-config.yaml
KESSOKU_DATA_DIR=/opt/rustdesk-stack/data/kessoku
KESSOKU_SECRETS_DIR=/opt/rustdesk-stack/secrets/kessoku
STARRY_CONFIG_FILE=/opt/rustdesk-stack/starry-config.yaml
STARRY_DATA_DIR=/opt/rustdesk-stack/data/starry
```

Keep explicit release versions:

```dotenv
KESSOKU_IMAGE=ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.2
STARRY_IMAGE=ghcr.io/q1ngyang/rustdesk-server-starry
STARRY_VERSION=1.1.16-patch-v1.2.0
```

Leave `RUSTDESK_SERVER_PUBLIC_KEY=REPLACE_AFTER_FIRST_HBBS_START` until HBBS
has generated its identity.

In `starry-config.yaml`, replace the RustDesk and client hostnames. The safe
initial settings are:

```yaml
secure_tcp:
  mode: auto
websocket_signal:
  enabled: true
  allowed_origins:
    - https://client.example.com
connection_auth:
  mode: off
geo:
  enabled: false
```

In `kessoku-config.yaml`, replace all three names. Keep authentication enabled,
the browser client in `builtin` mode, SQLite selected, Swagger disabled, and
server control read-only with no instances. The API and browser origins must
be different. The key in `relay-wss-urls` must exactly equal the corresponding
`relay_servers` entry, including `:21117`.

## 5. Configure the firewall

Allow the real SSH port before enabling a remote firewall:

```sh
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 21115/tcp
sudo ufw allow 21116/tcp
sudo ufw allow 21116/udp
sudo ufw allow 21117/tcp
sudo ufw deny 21114/tcp
sudo ufw deny 21118/tcp
sudo ufw deny 21119/tcp
sudo ufw deny 21120/tcp
sudo ufw deny 21121/tcp
sudo ufw deny 21122/tcp
sudo ufw enable
sudo ufw status numbered
```

Apply the same public allow-list to the cloud security group. Kessoku's two
published ports remain bound to `127.0.0.1` even when a firewall is present.

## 6. Install Nginx and certificates

```sh
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

sudo cp nginx-bootstrap.conf /etc/nginx/sites-available/rustdesk-stack.conf
sudo editor /etc/nginx/sites-available/rustdesk-stack.conf
sudo ln -sfn /etc/nginx/sites-available/rustdesk-stack.conf \
  /etc/nginx/sites-enabled/rustdesk-stack.conf
sudo nginx -t
sudo systemctl reload nginx

sudo certbot certonly --nginx -d rustdesk.example.com
sudo certbot certonly --nginx -d api.example.com
sudo certbot certonly --nginx -d client.example.com
```

Install the final example, replace every name and certificate path, and test it
before reloading:

```sh
sudo cp nginx-rustdesk.conf /etc/nginx/sites-available/rustdesk-stack.conf
sudo editor /etc/nginx/sites-available/rustdesk-stack.conf
sudo nginx -t
sudo systemctl reload nginx
```

Never proxy ports 21120 or 21121 through the public site.

## 7. Start HBBS/HBBR and obtain the public key

```sh
cd /opt/rustdesk-stack
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull hbbs hbbr
docker compose --env-file .env -f compose.yaml up -d hbbs hbbr
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbs hbbr

sudo test -s data/starry/id_ed25519
sudo test -s data/starry/id_ed25519.pub
sudo cat data/starry/id_ed25519.pub
```

Only display the `.pub` file. Paste its complete single-line value into:

1. `.env` as `RUSTDESK_SERVER_PUBLIC_KEY`;
2. `kessoku-config.yaml` as `web-client.server-public-key`.

Then confirm no placeholders remain:

```sh
grep -RniE 'example\.com|REPLACE|replace-with' \
  .env kessoku-config.yaml starry-config.yaml \
  /etc/nginx/sites-available/rustdesk-stack.conf
```

## 8. Start Kessoku and set the administrator password

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml logs --tail 150 kessoku-api

docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

Sign in to `https://api.example.com/dash/` as `admin`, rotate the password in
the UI, store it in a password manager, then remove the one-use password file.
Create a normal user in the administration UI.

## 9. Configure and verify native clients

Configure two RustDesk clients:

| Field | Value |
| --- | --- |
| ID Server | `rustdesk.example.com` |
| API Server | `https://api.example.com` |
| Key | complete `id_ed25519.pub` value |
| Relay Server | empty, so Starry can select it |
| WebSocket | off for the first native test |

Verify ID registration, Kessoku login/address-book synchronization, a real P2P
desktop session, and a forced Relay session. Compare client and server logs for
the same attempt. API login does not prove HBBS/HBBR connectivity.

## 10. Verify WSS and the browser client

Probe both upgrade paths:

```sh
for path in ws/id ws/relay; do
  curl --http1.1 --include --no-buffer --max-time 5 \
    -H 'Connection: Upgrade' \
    -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' \
    -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
    "https://rustdesk.example.com/$path"
done

curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
```

Open `https://client.example.com/`, sign in as the normal user, and complete a
VP9 Relay session with video, mouse, basic keyboard, and logout. Then enable
WebSocket on the desktop clients and test WSS/WSS plus both mixed directions.
An HTTP 101 alone is not an end-to-end RustDesk test.

## Optional features

- Place lawfully obtained MMDB files under `data/starry/mmdb`, add a catch-all
  rule, and only then enable `geo`.
- Keep connection authentication `off` until the private mTLS listener is
  deployed; spend a full observation period in `audit` before `enforce`.
- Deploy the Starry control agent read-only first; enable writes only during a
  separately tested maintenance window.

See [Configuration Reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference),
[Relay-Only Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment),
[Connection Authentication](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Connection-Authentication),
[Starry Control](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control), and
[Operations and Verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification).
