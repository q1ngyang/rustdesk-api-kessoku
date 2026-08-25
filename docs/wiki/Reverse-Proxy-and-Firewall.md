# Reverse proxy and firewall

**English** | [简体中文](ZH-CN-Reverse-Proxy-and-Firewall.md)

This page is the network reference for standalone Kessoku and combined
Kessoku/Starry deployments.

## Port matrix

| Port | Service | Public? | Purpose |
| --- | --- | --- | --- |
| `21114/TCP` | Kessoku | No | Plain API/admin proxy backend |
| `21115/TCP` | HBBS | Yes | NAT test |
| `21116/TCP+UDP` | HBBS | Yes | Registration, signalling, traversal, Secure TCP |
| `21117/TCP` | HBBR | Yes | Native Relay |
| `21118/TCP` | HBBS | No | Plain `/ws/id` backend |
| `21119/TCP` | HBBR | No | Plain `/ws/relay` backend |
| `21120/TCP` | Starry control agent | No | Loopback/private management only |
| `21121/TCP` | Kessoku internal auth | No | Private mTLS only |
| `21122/TCP` | Kessoku browser client | No | Plain dedicated-site backend |
| `80/TCP` | Nginx | Yes | ACME and HTTPS redirect |
| `443/TCP` | Nginx | Yes | API, browser client, and WSS |

A standalone Kessoku host needs only its SSH port plus 80/443. Apply the native
HBBS/HBBR rules on the host that actually runs those services.

## Recommended names

```text
api.example.com       Kessoku API and /_admin/
client.example.com    dedicated Kessoku browser client
rustdesk.example.com  native HBBS/HBBR address and /ws/id, /ws/relay
```

The API and browser client must be different HTTPS origins, not paths under one
host. A normal HTTP CDN does not proxy native 21115-21117 traffic; use DNS-only
records unless the provider offers an explicitly configured layer-4 product.

## Repository examples

| File | Use |
| --- | --- |
| [`examples/nginx/kessoku-bootstrap.conf.example`](../../examples/nginx/kessoku-bootstrap.conf.example) | Two-name certificate bootstrap |
| [`examples/nginx/kessoku.example.conf`](../../examples/nginx/kessoku.example.conf) | Standalone Kessoku |
| [`examples/combined/nginx-bootstrap.conf.example`](../../examples/combined/nginx-bootstrap.conf.example) | Three-name certificate bootstrap |
| [`examples/combined/nginx.conf.example`](../../examples/combined/nginx.conf.example) | Complete Kessoku + Starry proxy |
| [`examples/relay/nginx-bootstrap.conf.example`](../../examples/relay/nginx-bootstrap.conf.example) | Relay-only certificate bootstrap |
| [`examples/relay/nginx.conf.example`](../../examples/relay/nginx.conf.example) | Relay-only `/ws/relay` site |

Replace every example name and certificate path, then always run:

```sh
sudo nginx -t
sudo systemctl reload nginx
```

## Kessoku API

```nginx
server {
    listen 443 ssl;
    server_name api.example.com;
    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    client_max_body_size 16m;

    location / {
        proxy_pass http://127.0.0.1:21114;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
```

The administration UI at `/_admin/` uses the same upstream. Preserve the
application's security response headers.

## Dedicated browser client

```nginx
server {
    listen 443 ssl;
    server_name client.example.com;
    ssl_certificate /etc/letsencrypt/live/client.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/client.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://127.0.0.1:21122;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

`web-client.public-origin` and `api-origin` must exactly match the two browser
addresses, using canonical lowercase HTTPS origins without paths or explicit
default ports.

## Starry WSS paths

```nginx
location = /ws/id {
    proxy_pass http://127.0.0.1:21118;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_socket_keepalive on;
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;
}

location = /ws/relay {
    proxy_pass http://127.0.0.1:21119;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_socket_keepalive on;
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;
}
```

Do not swap or rewrite these paths. Every Starry Relay entry needs one
certificate-valid `/ws/relay` health endpoint.

## Trusted proxies

Kessoku trusts no forwarded address by default. Set `gin.trust-proxy` or
`KESSOKU_TRUST_PROXY` only to the exact source address by which Nginx reaches
the container. Inspect the Compose-network gateway if needed:

```sh
docker inspect rustdesk-api-kessoku \
  --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{end}}'
```

Never trust `0.0.0.0/0` or `::/0`. Starry's
`websocket_signal.trusted_proxies` follows the same rule; same-host Nginx can
use `127.0.0.1/32` and `::1/128`.

## Firewall example

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

Replace port 22 with the actual SSH port before enabling UFW. Docker port
publishing can bypass some forwarding rules, so Kessoku also binds backends to
host loopback. Apply the same public allow-list to the cloud security group.

## Validation

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json

curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  https://rustdesk.example.com/ws/id
```

Repeat for `/ws/relay`. Do not use `curl -k`; certificate failures will also
break real clients. HTTP and upgrade probes are partial checks—finish with
real native, Relay, and WSS desktop sessions.
