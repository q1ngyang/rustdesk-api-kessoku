# Operations and verification

**English** | [简体中文](ZH-CN-Operations-and-Verification.md)

Use this checklist after deployment and before or after every configuration or
image change. A healthy container does not prove that login, signalling, or
Relay traffic works.

## Routine checks

For the combined example:

```sh
cd /opt/rustdesk-stack
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs \
  --since 24h hbbs hbbr kessoku-api
```

Check daily or monitor automatically:

- all expected containers are running without a restart loop;
- `https://api.example.com/api/version` and the administration page respond;
- `https://client.example.com/config/v1.json` is public but contains no secret;
- disk space, inode use, certificate expiry, and backup status are healthy;
- logs do not contain recurring database, permission, TLS, WSS, or token errors.

Check weekly and after network changes:

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
ss -lntup | grep -E ':(80|443|21115|21116|21117|21118|21119|21120|21121|21122)\b'
sudo ufw status numbered
sudo nginx -t
```

Confirm ports 21114 and 21122 bind only to loopback. Ports 21118 and 21119 are
plain WSS backends; they must be reachable by same-host Nginx but not publicly.

## Functional checks

Use an ordinary user on two real RustDesk clients and verify:

1. both clients obtain an ID and can sign in;
2. address books and device information synchronize;
3. a normal remote-desktop session opens and accepts input;
4. a forced-Relay session opens through HBBR;
5. logout and sign-in work again;
6. when WSS is enabled, WSS-to-WSS and required mixed modes work.

For the built-in browser client, verify login, target connection, video, mouse,
basic keyboard input, and logout. The browser client is forced-Relay; it does
not test peer-to-peer connectivity. HTTP `101` verifies only the WebSocket
upgrade, not a complete remote session.

## Backups

Back up Kessoku and Starry data as one recoverable set:

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

The most important files are:

| File | Consequence if lost |
| --- | --- |
| `data/kessoku/rustdeskapi.db` | Accounts, address books, devices, and API state are lost |
| `secrets/kessoku/kessoku-access-ed25519.pem` | Existing Kessoku tokens can no longer be verified |
| `data/starry/id_ed25519` | RustDesk server identity changes and every client key must be updated |
| `data/starry/db_v2.sqlite3` | HBBS server data is lost |

Do not copy a live SQLite database while it is being written unless the backup
method provides a consistent snapshot. Stop the relevant service briefly or
use a SQLite-aware backup mechanism. Encrypt off-host backups and periodically
test restoration on an isolated host.

## Restore exercise

A backup is not complete until it has been restored. On a non-production host:

1. restore the configuration, database, keys, and certificates with original
   ownership and permissions;
2. keep public DNS and production ports disconnected;
3. run `docker compose config --quiet` before startup;
4. start the same pinned image versions;
5. verify database migration, administrator login, and client sign-in;
6. verify the restored HBBS public key matches the clients' configured key;
7. record the restore duration and the newest recovered timestamp.

## Safe key rotation

### Kessoku access-token key

Add the old public key to `auth.previous-keys`, install a new current private
key, restart Kessoku, and test both an existing token and a newly issued token.
Keep the previous public key until the maximum token lifetime has elapsed.

### Internal mTLS

Rotate server and client certificates with an overlap period. Test the internal
JWKS/introspection path using the new client certificate before removing the
old trust anchor. Never expose port 21121 to perform the rotation.

### Starry identity

Rotate `id_ed25519` only for a confirmed identity-key incident or a planned
server migration. Every desktop client, Kessoku browser profile, and Relay-only
node must receive the new public key.

## Configuration changes

Before restarting services:

```sh
docker compose --env-file .env -f compose.yaml config --quiet
sudo nginx -t
```

Change one layer at a time: Nginx, Kessoku, HBBS, then HBBR as applicable.
Inspect logs and run a real client session after each layer. For connection
authentication, stay in `audit` long enough to see native, WSS, mixed, and
Relay traffic before enabling `enforce`.

## Information to collect for support

Provide:

- Kessoku and Starry image tags, plus `docker compose version`;
- operating system and CPU architecture;
- the failing client version and whether native/WSS/Relay was used;
- redacted relevant YAML sections and Nginx server blocks;
- time-bounded logs covering one failed attempt;
- public DNS and certificate results.

Remove passwords, cookies, bearer tokens, private keys, database DSNs, LDAP
credentials, OAuth secrets, and full `.env` contents. An HBBS public key may be
shared; `id_ed25519` may not.
