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
| `media` | Persistent deployment-brand and avatar images | Keep under the mounted `/app/data` volume; raster images only. |
| `two-factor` | TOTP issuer, encrypted-secret key file, and login challenge lifetime | Enabled; key under `/app/data`, five-minute challenges. |
| `ldap`/OAuth | Optional identity providers | TLS verification and least privilege; no example password. |

## Authentication files

`auth.current-key.private-key-file` must reference an Ed25519 PKCS#8 private
key. Previous keys are public-key files. The internal listener separately
requires its server certificate/key, a Starry client CA, and at least one exact
allowed URI or DNS SAN.

Missing or invalid required material fails startup when the profile is enabled.

## Branding media and two-factor authentication

`media.directory` defaults to `./data/media`; uploads are renamed by the
server, restricted to verified PNG/JPEG/WebP files, capped by
`media.max-image-bytes`, and served under `/media/`. Keep this directory on the
persistent data volume.

`two-factor.enabled` defaults to `true`. On first startup Kessoku creates an
exactly 32-byte mode-0600 key at `two-factor.key-file` and uses AES-GCM to
encrypt TOTP secrets in the database. Back up the key together with the
database: losing it makes enabled TOTP registrations unverifiable. The
`issuer` appears in authenticator apps; `challenge-ttl` must be one to ten
minutes. Enabling or disabling TOTP revokes all existing sessions.

## Application logging

`logger.path` enables the Kessoku file log and is on by default in every
example configuration. Rotation is bounded by `max-size-mb: 20`,
`max-backups: 5`, and `max-age-days: 14`; archived files are compressed and
use local timestamps by default. Container examples also cap the Docker
`json-file` stream at five 20 MiB files. Keep `level: info` for normal
operation and enable debug logging only for a limited troubleshooting window.

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

`server-control.registry-directory` defaults to `./data/server-control`
(`/app/data/server-control` in the container and
`/var/lib/kessoku-api/data/server-control` under the packaged systemd working
directory). It is an independent schema-v1 SQLite registry; it never changes
the main database schema. Keep its directories at `0700`, files at `0600`, and
persist the complete tree.

`server-control.host-identity-file` is required when SP1 pairing is enabled.
For systemd use `/etc/machine-id`. A container must use the separately mounted
host file (the official Compose example exposes it as
`/run/kessoku-host-machine-id`); the machine ID baked into an image cannot
detect a registry copied to another host. The value is hashed and is never
stored or returned verbatim.

SP1 is off by default. Enabling `server-control.pairing.enabled` requires an
exact public `broker-origin`, lowercase `broker-spki-sha256`, and one or more
deployment-owned `agent-origins`. Each allowlist entry has an `id`, display
`name`, exact HTTPS `origin`, and certificate `tls-server-name`. `code-ttl`
defaults to ten minutes and is capped at one hour; `recovery-ttl` defaults to
ten minutes and cannot exceed ten minutes. These fields are deployment-only;
the browser selects an ID and cannot submit an origin or callback.

Kessoku automatically exposes its own `logger.path` when it resolves to a
regular file. The remaining diagnostics viewer is a deployment-owned
allowlist. Set one absolute `log-directory`, then add `log-sources` entries containing a unique
`id`, display `label`, component (`kessoku`, `starry`, `relay`, or
`control-agent`), optional configured `instance-id`, and a simple `file` name.
Starry, Relay, and the Control Agent normally log to container stdout. Kessoku
does not receive the Docker socket: use the deployment log collector to write
the selected streams as regular text files and mount that directory into
Kessoku. If this directory differs from the directory containing
`logger.path`, add the Kessoku file explicitly as well. Paths, traversal, and
browser-provided filenames are rejected. Kessoku reads
only a bounded newest window and redacts common authorization, access/lease/
connection/control tokens, route leases, passwords, nonces, allocation/session
identifiers, session cookies, client secrets, private keys, and complete IPv4/IPv6
values before display or export. Keep older logs and retention policy in the
deployment logging system. Kessoku's process log level may be changed while control writes are
enabled; Starry patch v1.2 still requires changing deployment `RUST_LOG` and a
maintenance restart.

## Operator-managed system settings

Super administrators manage announcements, IP information, login lifetimes,
and record-retention periods under **System management** in `/dash/`. Web and
native-client lifetimes may be shortened independently but cannot exceed the
deployment-owned `auth.maximum-token-ttl`. Token cleanup never removes an
active session; its retention clock starts after expiry or revocation. GeoIP sources must be public
HTTPS MMDB URLs; downloads are limited to 128 MiB, validated before atomic
replacement, and refreshed every 1–2160 hours. The default City and ASN files
come from the P3TERX GeoLite mirror. These values are stored in the database,
not in deployment branding or YAML. Persist and back up `/app/data`, which also
contains the downloaded `geoip` directory.

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
