# Operations and verification

**English** | [简体中文](ZH-CN-Operations-and-Verification.md)

Verification is layered. A lower layer cannot substitute for a higher one.

## 1. Source and artifact identity

- Verify the immutable tag, source commit, checksums, SBOM, provenance, and
  resolved image digest.
- Confirm both embedded frontend sources belong to the same commit, and verify
  their separate dist checksums, CycloneDX SBOMs, and licence evidence.
- Confirm the exact Starry contract is `PINNED` and matches the deployed Agent.
- Confirm release artifacts contain reviewed `resources/client/index.html`
  and `resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt`, but
  no `resources/web`, `resources/web2`, WebClient2/V2, private key, or build
  credential.

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
- Registration, Swagger, built-in Web Client, and control-write defaults match
  policy.

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

For the built-in browser MVP, separately verify the dedicated HTTPS origin and
21122 listener, public profile without secrets, exact CORS, ready/grant/ack
`postMessage` handoff, token expiry/logout, and one forced-Relay WSS VP9
session with mouse and basic keyboard. This browser evidence does not claim
P2P, incoming mode, file/clipboard/audio, display switching, or another codec.

The 2026-08-21 local fixture passed this browser matrix from a clean start
against published Starry image digest
`sha256:3685543aee6e60c27bed5db1df2fa32af83e61a58e9bc4c0ea3464664863811b`:
direct login and admin popup grant, 1280x800 VP9/WebCodecs output, remote mouse
position 320x240, basic `K` and `Ctrl+S` input, logout, and no persistent
browser storage. It was an ordinary functional test, not an offensive scan.
The repeatable local orchestration entry point is
`scripts/verify-official-starry-web-client.sh`; it requires the documented
local RustDesk 1.4.9 QA target image by its exact content digest and never
publishes an artifact.

The v2.8.3 local release evidence specifically covers RustDesk 1.4.9
forced-Relay sessions: `audit` native/native and `enforce` native/native,
WSS/WSS, WSS/native, and native/WSS. It does not claim direct P2P or a separate
Secure TCP case. See [Security finding closure](Security-Finding-Closure.md).

## 7. Recovery and go/no-go

Restore the database, keys, PKI, configuration, and prior image in staging.
Record RTO/RPO, rollback generation, client re-login behavior, owners, and the
go/no-go decision. Only then may the release owner approve `RELEASE_STATUS`.

The detailed normal-operation checklist is
[`OPERATOR-RUNBOOK.md`](../../OPERATOR-RUNBOOK.md).
