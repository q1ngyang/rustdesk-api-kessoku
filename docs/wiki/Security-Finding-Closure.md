# Security configuration

**English** | [简体中文](ZH-CN-Security-Finding-Closure.md)

Use this page to harden a public Kessoku deployment. The filename is retained
so existing Wiki links continue to work.

## Network boundary

Expose only the ports required by the selected deployment:

| Port | Public use |
| --- | --- |
| `80/TCP`, `443/TCP` | Nginx, ACME, Kessoku API, browser client, and WSS |
| `21115/TCP` | HBBS NAT test when HBBS is on this host |
| `21116/TCP+UDP` | HBBS registration, signalling, and traversal |
| `21117/TCP` | Native HBBR Relay |

Keep `21114`, `21118`, `21119`, `21120`, `21121`, and `21122` off the public
network. Bind Kessoku's proxy backends to `127.0.0.1`; protect Starry's
host-network listeners with both the host firewall and cloud security group.
See [Reverse Proxy and Firewall](Reverse-Proxy-and-Firewall.md) for exact rules.

Configure `gin.trust-proxy` only with the exact proxy source address. Never
trust `0.0.0.0/0` or `::/0` merely to obtain a client address in logs.

## Files, ownership, and secrets

Kessoku runs as UID/GID `65534:65534`. Its data and secret directories should
be owned by that identity with mode `0700`; private keys and password files
should use `0600`.

Keep these secret classes separate:

- the Kessoku Ed25519 access-token signing key;
- internal mTLS CA, server key, and client certificates;
- Starry Control Agent service-token key;
- database, LDAP, and OAuth client credentials;
- Starry's `id_ed25519` server identity key.

Mount secrets read-only from files outside the image. The RustDesk server value
distributed to clients is `id_ed25519.pub`; never expose `id_ed25519`. Do not
store real `.env`, `config.yaml`, certificates, or backups in Git.

## Accounts and public features

Recommended initial settings are:

```yaml
app:
  register: false
  captcha-threshold: 3
  ban-threshold: 10
  show-swagger: 0

auth:
  enabled: true
  legacy-token-read-enabled: false
```

- replace the initial administrator password immediately and delete its
  one-time password file after a confirmed login;
- use an ordinary account for daily RustDesk access;
- leave public registration disabled, or require administrator approval when
  self-registration is necessary;
- revoke sessions after a password reset, account disable, or suspected leak;
- keep system clocks synchronized because JWT validation depends on time.

## Database transport

SQLite is suitable for a single host when `/app/data` is persistent and
backed up. For an external database:

- MySQL must use certificate-verified TLS (`tls: true` with `ca-file`);
- PostgreSQL should use `sslmode: verify-full` with `ssl-root-cert`;
- the database hostname must match the certificate identity;
- restrict the database account and network ACL to Kessoku only.

Do not disable certificate verification to make an invalid hostname or CA
configuration appear to work.

## LDAP and OAuth/OIDC

For LDAP, use verified TLS and a least-privilege bind account. Restrict allowed
groups and confirm how disabled or removed directory accounts are handled.

For OAuth/OIDC, use exact HTTPS issuer and redirect addresses. Keep the client
secret in a mounted file where supported, and verify provider subject binding
with a test account before enabling the provider for all users.

## Built-in browser client

Publish the browser client on a dedicated HTTPS origin, separate from the API:

```yaml
web-client:
  mode: "builtin"
  public-origin: "https://client.example.com"
  api-origin: "https://api.example.com"
```

Use exact Starry `/ws/id` and `/ws/relay` WSS addresses, an exact Relay-name
map, and the HBBS public key. Starry `allowed_origins` should contain only the
browser-client origin, not `*`. The public `config/v1.json` response must never
contain credentials, private keys, or access tokens.

## Starry authentication and control

Connection authentication and server control use different mTLS identities and
keys. Keep ports `21120` and `21121` private.

- move connection authentication from `off` to `audit`, observe normal native
  and WSS traffic, then use `enforce` only after all required clients pass;
- start the Control Agent and Kessoku provider in read-only mode;
- enable configuration writes only after backup and rollback have been tested;
- do not configure fail-open behavior for authoritative token introspection.

## Logs and audit records

Do not log bearer tokens, cookies, password files, private keys, full OAuth
responses, or complete configuration documents. Rotate container and Nginx
logs and restrict their readers.

Some RustDesk client audit and system-information uploads do not carry an
authorization header. Kessoku bounds and associates these compatibility
records, but a device UUID is not a secret. Treat them as operational records,
not cryptographic proof; export important audit data to append-only storage.

## If a credential may be exposed

1. restrict the affected endpoint or account immediately;
2. preserve redacted logs and timestamps;
3. rotate the relevant credential without replacing unrelated identities;
4. revoke affected user sessions and verify old credentials are rejected;
5. run real login, peer-to-peer, Relay, and WSS sessions after recovery;
6. verify backups before removing temporary containment rules.

Rotating `id_ed25519` changes the server identity for every client and should
not be used as a general troubleshooting step.
