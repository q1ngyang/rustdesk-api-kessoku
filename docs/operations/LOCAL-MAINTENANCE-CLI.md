# Local maintenance CLI

Kessoku v3.0.7 exposes a bounded local command surface for S6 supervisors and
human recovery. These commands are process-local Cobra commands; no equivalent
recovery HTTP route, arbitrary SQL runner, shell, or arbitrary-file reader is
provided.

[简体中文](LOCAL-MAINTENANCE-CLI.zh-CN.md)

## Commands and initialization boundaries

| Command | Configuration | Database | Writes |
| --- | --- | --- | --- |
| `version` | Not loaded | Not opened | None |
| `config validate` | Parsed and references checked | Not opened | None |
| `database status` | Parsed | Read-only connection | None |
| `database migrate` | Parsed | Read/write, exclusive migration lock | Schema migration only |
| `maintenance recover-admin` | Parsed | Exact schema 313 required | One audited transaction |
| `maintenance reset-2fa` | Parsed | Exact schema 313 required | One audited transaction |

All commands accept `--json`; every JSON object contains `schema_version: 1`.
JSON is written only to stdout. Human diagnostics use stderr. Structured
outputs contain no password, password hash, token, TOTP secret, private key, or
complete DSN.

## Version, validation, and database

```sh
kessoku-api version --json
kessoku-api config validate --config /app/conf/config.yaml --json
kessoku-api database status --config /app/conf/config.yaml --json
kessoku-api database migrate --config /app/conf/config.yaml --json
```

`database status` reports `empty`, `current`, `upgrade_required`,
`newer_than_binary`, or `invalid`. Only `current` has `safe_to_start: true`.
`database migrate` is idempotent and never starts an HTTP listener. The
configured SQLite file remains `/app/data/rustdeskapi.db` when the process
working directory is `/app`.

## Administrator recovery

Select exactly one user identifier and independently confirm the exact stored
username:

```sh
install -m 0600 /dev/null /run/secrets/kessoku-recovery-password
# Write 12–128 bytes to the file without exposing the password in shell history.
kessoku-api maintenance recover-admin \
  --config /app/conf/config.yaml \
  --username alice \
  --confirm-username alice \
  --password-file /run/secrets/kessoku-recovery-password \
  --reset-2fa \
  --json
```

`--user-id ID` may replace `--username`, but they cannot be omitted or
combined. The password file must be a regular owner-only file. Symlinks,
group/other permissions, replacement races, and values outside 12–128 bytes
are rejected. Never pass a password in arguments, environment variables, YAML,
logs, or an image layer.

Successful recovery enables the account, sets `role=super_admin` and the
compatibility `is_admin` mirror, removes every stale administrator scope,
clears login challenges, optionally replaces the password and/or TOTP row,
increments `auth_version` exactly once, and revokes active tokens. The global
TOTP encryption key and every other account remain unchanged.

## Two-factor recovery

```sh
kessoku-api maintenance reset-2fa \
  --config /app/conf/config.yaml \
  --user-id 42 \
  --confirm-username alice \
  --json
```

The operation is idempotent when no factor exists, but each authorized
execution still advances `auth_version` and revokes current sessions. It
deletes only the target user's enabled/pending factor row and login challenges;
it never deletes or regenerates the global TOTP key.

## JSON errors and exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `2` | Command usage, selector, or password-file input error |
| `3` | Configuration parse or validation error |
| `4` | Database connection or migration execution error |
| `5` | Schema is incompatible with this binary |
| `6` | Maintenance target or transactional operation failed |
| `1` | Unexpected process/output error |

Representative failure:

```json
{"schema_version":1,"operation":"reset_2fa","success":false,"request_id":"0191f6a0-0000-7000-8000-000000000402","password_reset":false,"two_factor_reset":false,"two_factor_was_configured":false,"login_challenges_cleared":0,"scopes_cleared":0,"sessions_revoked":0,"error":{"code":"MAINTENANCE_CONFIRMATION_MISMATCH","message":"confirm-username does not exactly match the stored username"}}
```

Every attempted database-backed recovery creates an intent audit and completes
it as success or failure. A future-schema refusal happens before any audit or
account write. Restrict local command execution and configuration/database
mount access to the trusted S6 control plane; do not expose a Docker socket to
Kessoku.

The historical `reset-admin-pwd --password-file PATH` and
`reset-pwd --user-id ID --password-file PATH` commands remain available and
use the same secure password reader, authentication-generation rotation,
session revocation, and password-change audit path.
