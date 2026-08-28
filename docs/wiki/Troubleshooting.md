# Troubleshooting

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Troubleshooting)

Identify the failing layer first: Compose, Kessoku API, account login, HBBS
signalling, HBBR Relay, WSS, browser client, or advanced authentication. Change
one item at a time; disabling the firewall, TLS, and authentication together
hides the original cause.

## Collect the basic state

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 200
docker inspect rustdesk-api-kessoku --format '{{.Config.Image}} {{.Config.User}}'
sudo nginx -t
sudo ss -lntup | grep -E ':(80|443|2111[4-9]|2112[0-2])\b'
```

Record the failure time and RustDesk client version. Redact tokens, passwords,
private keys, database credentials, and full configuration files.

## Compose or container startup

| Symptom | Likely cause | Action |
| --- | --- | --- |
| `set ...` or empty-variable error | Required `.env` value is missing | Edit `.env`, then rerun `docker compose config` |
| Config path became a directory | The host file did not exist when Docker mounted it | Stop the service and create a regular YAML file at that path |
| Database `permission denied` | Kessoku data is not owned by `65534:65534` | Set owner `65534:65534` and directory mode `0700` |
| Signing key cannot be read | Secret directory or file permissions are wrong | Owner `65534:65534`; directory `0700`, private file `0600` |
| `read-only file system` | A writable log/temp path points into the image | Keep `logger.path` under `./runtime` and the Compose tmpfs mount |
| `app.web-client is removed` | Obsolete nonzero `app.web-client` remains | Set it to `0`; use root-level `web-client.mode` |
| `web-client-provider is removed` | Removed legacy configuration remains | Delete that section/environment variable |
| Restart loop | Validation, database, certificate, or key failure | Read the first fatal log and correct its input |

Check for unedited examples before startup:

```sh
grep -RniE 'example\.com|REPLACE|replace-with' \
  .env config.yaml kessoku-config.yaml starry-config.yaml 2>/dev/null
```

In a combined first start, run HBBS/HBBR first. Copy the complete public value
from `data/starry/id_ed25519.pub` into `.env` and Kessoku's browser-client
configuration before starting Kessoku.

## Administration UI

| Symptom | Check |
| --- | --- |
| `/dash/` returns 404 | Use the trailing slash; confirm Nginx proxies the whole API site to 21114 |
| 502 Bad Gateway | Kessoku container, logs, and `127.0.0.1:21114` listener |
| Administrator password is unknown | Use `reset-admin-pwd --password-file`; do not rebuild the database |
| Password file is rejected | It must be a regular file readable by UID 65534 with no group/other permissions |
| Repeated CAPTCHA/ban | Wait for expiry and verify exact trusted-proxy configuration |
| Password login was disabled too early | Restore `app.disable-pwd-login: false`, then repair OAuth/OIDC |

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

## RustDesk login fails

Check in this order:

1. API Server is `https://api.example.com`, without `/api` or `/dash/`;
2. `/api/version` loads with a valid certificate;
3. the user is enabled and the credentials are correct;
4. client and server clocks are synchronized;
5. Nginx forwards `Host` and `X-Forwarded-Proto`;
6. LDAP/OAuth identities are not duplicated;
7. the client signs in again after password reset or session revocation.

An empty address book usually means the client is using another API address,
`rustdesk.personal` is disabled, or the user is outside the relevant sharing
rule.

## Login works but remote control fails

This is usually outside the Kessoku API path. Verify both clients use the same
ID Server and the same `id_ed25519.pub` value, HBBS is reachable on
`21116/TCP+UDP`, and the target remains registered. When Starry allocates a
Relay, leave the client Relay field empty.

If peer-to-peer works but forced Relay fails, inspect HBBR and `21117/TCP`.
Correlate both clients and HBBS/HBBR logs for the same timestamp. Successful
login does not test signalling or Relay transport.

## Browser client

| Symptom | Check |
| --- | --- |
| Client hostname returns 404/502 | `web-client.mode: builtin`, loopback port 21122, Nginx, and Kessoku logs |
| Public key is invalid | It must be the one-line HBBS Ed25519 public key, decoding to 32 bytes |
| Origins are reported equal | `public-origin` and `api-origin` must be different canonical HTTPS origins |
| Browser CORS failure | Both configured origins exactly match the address bar; Starry allows the client origin |
| Admin popup is blank | Popup blocking, client-site certificate/proxy, and returned client URL |
| Login expires immediately | Server clocks and configured token lifetimes |

```sh
curl -fsS https://client.example.com/config/v1.json
```

That response must not contain a password, private key, or access token.

## WSS or Relay selection

Probe both `/ws/id` and `/ws/relay`:

```sh
curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
  https://rustdesk.example.com/ws/id
```

Confirm Nginx sends the paths to ports 21118 and 21119 respectively, the TLS
chain is valid, Starry WSS is enabled, trusted proxies are exact, and the
browser origin is allowed. Each `relay_health.endpoints[].relay`, Starry
`relay_servers` entry, and Kessoku `relay-wss-urls` key must use the same exact
Relay name. Do not use `curl -k` to hide a certificate failure.

HTTP `101` confirms only the upgrade. If the browser still has no video, check
the HBBR session, target password, VP9, and browser WebCodecs support.

## External database

- MySQL `tls` must be the string `"true"` and a readable CA file must be
  mounted;
- PostgreSQL requires `sslmode: verify-full` and `ssl-root-cert`;
- connect through a DNS name present in the certificate SAN;
- resolve OAuth identity collisions on a backup copy rather than deleting rows
  in production.

Do not fall back to plaintext transport during a database outage.

## Connection authentication rejects valid users

Move Starry from `enforce` back to `audit`, then inspect issuer, the
`rustdesk-connect` audience, `connect:initiate` permission, key ID, JWKS cache,
mTLS CA and URI SAN, introspection reachability, account/session state, and
clocks. Do not expose port 21121 or disable certificate verification.

## Starry control is unavailable

Check the instance is enabled, the Agent address and TLS server name are exact,
the instance UUID matches, the Kessoku certificate has the required URI SAN,
and both sides start read-only. For an ETag conflict or expired plan, read the
current configuration again, merge the intended change, validate it, and
create a new plan rather than forcing an overwrite.

## Requesting help

Provide image tags, OS/architecture, relevant redacted Compose/YAML sections,
Nginx validation, time-bounded logs, client version/platform, WebSocket mode,
and database type. Never provide raw tokens, cookies, passwords,
`id_ed25519`, signing keys, client private keys, or a complete database.
