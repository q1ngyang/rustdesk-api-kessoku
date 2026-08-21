# Operations and verification

**English** | [简体中文](ZH-CN-Operations-and-Verification.md)

Verification is layered. A lower layer cannot substitute for a higher one.

## 1. Source and artifact identity

- Verify the immutable tag, source commit, checksums, SBOM, provenance, and
  resolved image digest.
- Confirm the embedded admin frontend belongs to the same commit.
- Confirm the exact Starry contract is `PINNED` and matches the deployed Agent.
- Confirm release artifacts contain no `resources/web`, `resources/web2`,
  WebClient2, private key, or build credential.

## 2. Static deployment validation

```sh
docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
```

Review image identity, binds, mounts, effective environment, reverse-proxy
trust, and secret references. Verify backups before starting an upgrade.

## 3. Process and database

- Container runs unprivileged and restarts cleanly.
- Database version is 301; migrated row counts, token hashes, OAuth identity
  indexes, and the final-admin invariant are correct.
- Initial administrator password is changed.
- Logs contain no token, private key, certificate, or full configuration.
- Registration, Swagger, provider, and control-write defaults match policy.

## 4. API and authentication

- Administrator and ordinary-user authorization are distinct.
- Login, address-book operations, logout, re-login, password reset, disable,
  and global revocation match the token lifecycle.
- Current/previous Ed25519 rotation behaves across the maximum token lifetime.
- Internal JWKS/introspection accepts only the expected mTLS identities and
  remains bounded under ordinary operational load.

## 5. Starry control

- Instance identity, capabilities, status, Relays, and simulation agree with
  the deployed Agent/HBBS.
- Non-administrators are denied by the backend, not only hidden by the UI.
- Read-only mode rejects mutations.
- Staging plan/apply/operation/history/rollback/reload ends with the expected
  live generation and correlated redacted audits.

## 6. Real client acceptance

For every supported RustDesk client version, collect evidence for login and:

- native TCP and Secure TCP;
- WSS-to-WSS and mixed WSS/native paths when enabled;
- direct P2P and forced/observed Relay sessions;
- logout, disable, password reset, key rotation, and dependency outage;
- Starry `audit` before canary `enforce`.

Do not perform penetration, exploit, public-target, fuzz/mutation, or stress
testing as part of this local workflow. Any separately approved resilience
testing belongs in an isolated staging/CI environment.

The v2.8.0 local release evidence specifically covers RustDesk 1.4.9
forced-Relay sessions: `audit` native/native and `enforce` native/native,
WSS/WSS, WSS/native, and native/WSS. It does not claim direct P2P or a separate
Secure TCP case. See [Security finding closure](Security-Finding-Closure.md).

## 7. Recovery and go/no-go

Restore the database, keys, PKI, configuration, and prior image in staging.
Record RTO/RPO, rollback generation, client re-login behavior, owners, and the
go/no-go decision. Only then may the release owner approve `RELEASE_STATUS`.

The detailed normal-operation checklist is
[`OPERATOR-RUNBOOK.md`](../../OPERATOR-RUNBOOK.md).
