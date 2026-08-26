# Configuration reference

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Configuration-Reference)

The complete annotated template is
[`conf/config.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/conf/config.yaml). Configuration is read from that
YAML file and may be overridden with environment variables prefixed by
`RUSTDESK_API_`; dots and hyphens become underscores.

Example:

```text
auth.internal.request-timeout
RUSTDESK_API_AUTH_INTERNAL_REQUEST_TIMEOUT
```

## Primary sections

| Section | Purpose | Secure starting point |
| --- | --- | --- |
| `app` | Registration, login UI behavior, Swagger, token compatibility | Registration/Swagger off; legacy token read off after migration. |
| `gin` | Public API bind, mode, resources, trusted proxy | Release mode; exact proxy list or none. |
| `gorm`/database | SQLite, MySQL, or PostgreSQL selection and pooling | SQLite for simple single-node use; external MySQL/PostgreSQL must verify the certificate and hostname. |
| `rustdesk` | ID/Relay/API endpoints and public server key | Exact public endpoints and `id_ed25519.pub`; never the private key. |
| `auth` | EdDSA token profile and internal mTLS API | Enable only after keys/PKI are mounted and migration is rehearsed. |
| `server-control` | Fixed Starry Control Agent instances | Read-only, legacy commands off, no instance until credentials are ready. |
| `web-client` | Built-in browser client listener, origins, WSS map, public key, generation, grant TTL | `disabled`; when enabled, separate HTTPS origin and loopback listener. |
| `ldap`/OAuth | Optional identity providers | TLS verification and least privilege; no example password. |

## Authentication files

`auth.current-key.private-key-file` must reference an Ed25519 PKCS#8 private
key. Previous keys are public-key files. The internal listener separately
requires its server certificate/key, a Starry client CA, and at least one exact
allowed URI or DNS SAN.

Missing or invalid required material fails startup when the profile is enabled.

## Database transport

SQLite uses the local data file. MySQL and PostgreSQL are never allowed to
downgrade to plaintext or certificate-without-hostname verification.

```yaml
gorm:
  type: mysql
mysql:
  addr: "mysql.example.internal:3306"
  tls: "true"
  ca-file: "/run/secrets/mysql-ca.pem" # optional additional CA bundle
```

MySQL always verifies the hostname from `addr` against the server certificate.
It starts with the operating-system trust pool and adds certificates from
`ca-file` when configured.

```yaml
gorm:
  type: postgresql
postgresql:
  host: "postgres.example.internal"
  port: "5432"
  sslmode: "verify-full"
  ssl-root-cert: "/run/secrets/postgres-ca.pem" # optional with public CA
```

MySQL values other than the literal `true`, and PostgreSQL modes other than
`verify-full`, fail configuration validation. CA read/parse failures and
certificate/SAN mismatches also fail startup. Mount CA files read-only and use
the certificate DNS name, not an unreviewed IP alias.

## Starry instances

Each enabled instance fixes:

- deployment ID and human-readable name;
- absolute HTTPS Agent origin;
- expected Agent instance UUID and TLS server name;
- CA and client certificate/key file paths;
- a separate service-JWT signing key and key ID;
- control issuer and certificate-bound authorized party.

The browser cannot override these values. `server-control.read-only: true` is
the normal profile outside an approved configuration window.

## Built-in Web Client

`web-client.mode` is `disabled` or `builtin`. Built-in mode requires
`auth.enabled`, an explicit listener, different exact HTTPS `public-origin`
and `api-origin`, exact `wss://.../ws/id` rendezvous, one or more exact Relay-
name to `wss://.../ws/relay` mappings, a base64 32-byte Ed25519 public key, and
a positive `profile-generation`. `connection-token-ttl` defaults to 15 minutes
and cannot exceed one hour or `auth.maximum-token-ttl`.

The public client profile exposes endpoints, server public key/fingerprint and
generation only. Change the generation when an approved endpoint or key
profile changes. See [Built-in Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client).

## Removed or rejected settings

- `app.web-client` must remain zero; use `web-client.mode` instead.
- The removed root `web-client-provider` block is rejected rather than
  silently ignored.
- Old HS256 JWT settings are not a supported authentication profile.
- `proxy.enable` is rejected for OAuth/OIDC because a proxy independently
  resolves provider targets and would bypass Kessoku's destination validation.
- `legacy-command-enabled` does not restore command execution; compatibility
  routes only report removal.
- Do not put private keys, bearer tokens, raw YAML secrets, or certificate
  contents in environment variables, logs, or admin metadata.
