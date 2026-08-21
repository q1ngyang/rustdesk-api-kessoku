# Configuration reference

**English** | [简体中文](ZH-CN-Configuration-Reference.md)

The complete annotated template is
[`conf/config.yaml`](../../conf/config.yaml). Configuration is read from that
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
| `gorm`/database | SQLite, MySQL, or PostgreSQL selection and pooling | SQLite for simple single-node use; TLS-verified external DB for production as required. |
| `rustdesk` | ID/Relay/API endpoints and public server key | Exact public endpoints and `id_ed25519.pub`; never the private key. |
| `auth` | EdDSA token profile and internal mTLS API | Enable only after keys/PKI are mounted and migration is rehearsed. |
| `server-control` | Fixed Starry Control Agent instances | Read-only, legacy commands off, no instance until credentials are ready. |
| `web-client-provider` | Independently hosted browser client metadata | `disabled`. |
| `ldap`/OAuth | Optional identity providers | TLS verification and least privilege; no example password. |

## Authentication files

`auth.current-key.private-key-file` must reference an Ed25519 PKCS#8 private
key. Previous keys are public-key files. The internal listener separately
requires its server certificate/key, a Starry client CA, and at least one exact
allowed URI or DNS SAN.

Missing or invalid required material fails startup when the profile is enabled.

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

## Removed or rejected settings

- `app.web-client` must remain zero; use the external-provider governance
  model instead.
- Old HS256 JWT settings are not a supported authentication profile.
- `legacy-command-enabled` does not restore command execution; compatibility
  routes only report removal.
- Do not put private keys, bearer tokens, raw YAML secrets, or certificate
  contents in environment variables, logs, or admin metadata.
