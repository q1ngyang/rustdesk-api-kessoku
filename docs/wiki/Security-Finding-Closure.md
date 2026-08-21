# Security finding closure

**English** | [简体中文](ZH-CN-Security-Finding-Closure.md)

This page records the defensive, static-only security review used for the
Kessoku v2.8.1 release decision. It is not a penetration-test report and does
not contain exploit instructions.

## Evidence boundary

- The sealed Kessoku review snapshot is
  `codex-security-snapshot/v1:sha256:d504807864f052238881f7e0e18548763d8e1b0134567f95ee0d08b497bef68d`
  and records 23 findings across 27 surfaces.
- The sealed Starry snapshot is
  `codex-security-snapshot/v1:sha256:4b5ffa3ce6bc819a9a72e9f6e9ec7fd9dc63c0aee4c74645b1d67472d5b6aaac`
  and records 22 findings (6 medium and 16 low) across 40 surfaces, with no
  high or critical result.
- The Codex Security plugin installation was approved, but its scanner was not
  callable in the final Kessoku candidate session. The sealed snapshots are
  therefore historical evidence, not a claim that the exact final tree was
  freshly rescanned by the plugin.
- On 2026-08-21 the release owner accepted this evidence boundary and removed
  a fresh plugin run as a release prerequisite.
- The exact post-remediation tree is covered by source review and ordinary
  functional, race, migration, frontend, container, packaging, and real-client
  compatibility tests. No penetration, exploit, fuzz/mutation, stress, or
  public-target testing is part of this evidence.

## Kessoku finding disposition

| Finding | Disposition | Release evidence |
| --- | --- | --- |
| `KS-ADMIN-LOGOUT-REVOCATION` | Closed | Logout and administrative lifecycle changes revoke stored sessions and rotate authentication versions. |
| `KS-ANONYMOUS-AUDIT-MUTATION` | Accepted residual | RustDesk 1.4.9 audit/sysinfo uploads have no authorization header; the bounded compatibility route is described below. |
| `KS-ANONYMOUS-PEER-STORAGE` | Closed | Request size/cardinality and fields are bounded; persisted peer identity and ownership cannot be reassigned by the request. |
| `KS-BOOTSTRAP-PASSWORD-LOG` | Closed | Startup creates an unreachable random credential; the operator supplies a mode-`0600` password file and no reusable password is logged. |
| `KS-CAPTCHA-ALLOCATION` | Closed | CAPTCHA state is bounded and the client-facing bypass is removed. |
| `KS-CSV-FORMULA-INJECTION` | Closed | Spreadsheet-export cells that could be interpreted as formulas are neutralized. |
| `KS-DB-BOOTSTRAP-TLS` | Closed | External MySQL requires verified TLS and PostgreSQL requires `verify-full`; DSNs preserve encoded credentials. |
| `KS-LDAP-FILTER-INJECTION` | Closed | LDAP search values are escaped before use in filters. |
| `KS-LDAP-IDENTITY-COLLISION` | Closed | Provider/subject and provider/user identities are stored with uniqueness constraints. |
| `KS-LDAP-INSECURE-TRANSPORT` | Closed | LDAP requires certificate-verified TLS and rejects insecure transport profiles. |
| `KS-OAUTH-BIND-STATE` | Closed | Bind state is bound to the initiating authenticated user and the verified identity is persisted before redirect completion. |
| `KS-OAUTH-CACHE-AMPLIFICATION` | Closed | OAuth state count, lifetime, state/code length, and provider response bodies are bounded. |
| `KS-OIDC-ISSUER-SSRF` | Closed | Provider endpoints require public HTTPS destinations, reject local/private addresses and redirects, and use bounded clients. |
| `KS-OIDC-STATE-TRANSFER` | Closed | Callback state is atomically claimed once and login results remain bound to the initiating device ID/UUID. |
| `KS-OIDC-UNVERIFIED-EMAIL` | Closed | OIDC identity is keyed by the required ID-token subject and the exact matching UserInfo subject, not an unverified email address. |
| `KS-PEER-IDENTITY-HIJACK` | Closed | Address-book and peer metadata reads/writes are scoped to the authenticated owner; caller row/user/collection identifiers are discarded. |
| `KS-REGISTRATION-STORAGE` | Closed | Registration inputs and state are bounded and secure defaults keep public registration disabled. |
| `KS-REQUEST-CARDINALITY` | Closed | API body, peer, tag, batch, field, and serialized metadata limits are enforced before persistence. |
| `KS-STARRY-ASYNC-AUDIT` | Closed | Control intent is recorded before provider work and success/failure is finalized with correlated identifiers. |
| `KS-STARRY-OPERATION-BINDING` | Closed | Typed DTOs bind operations to deployment, actor, expected operation/plan identity, ETag, and idempotency data. |
| `KS-STARRY-RELOAD-DIGEST` | Closed | Reload/apply responses must report the non-zero expected generation/digest before success is accepted. |
| `KS-TRUSTED-PROXY-DEFAULT` | Closed | No proxy is trusted by default; deployments must configure exact proxy addresses. |
| `KS-USER-DIRECTORY-OVEREXPOSURE` | Closed | Ordinary user/group directory responses use a minimal DTO and omit administrative/authentication fields. |

## Accepted compatibility residual

RustDesk 1.4.9 does not send an authentication header on its audit connection,
file-transfer, and sysinfo upload calls. Removing those routes would break the
supported client behavior. Kessoku therefore retains a narrow compatibility
surface with all of these controls:

- a 64 KiB request limit and strict field limits;
- an exact match to an already persisted peer ID and UUID;
- no request-controlled owner reassignment; and
- separate administrator audit events when stored audit records are deleted.

A peer UUID is not a secret. Someone who already knows a valid peer ID and
UUID may still submit a spoofed compatible audit record. Treat these rows as
operational telemetry, not non-repudiation evidence, and export them to
append-only or immutable storage when stronger audit assurance is required.
This residual is accepted for v2.8.1 compatibility and must be reconsidered
when supported RustDesk clients provide authenticated audit uploads.

## Published Starry compatibility evidence

The local release matrix used the published
`ghcr.io/q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0` image with
repository digest
`sha256:3685543aee6e60c27bed5db1df2fa32af83e61a58e9bc4c0ea3464664863811b`
and source revision `5e73b3af1423acf5ee20ca32a2d747eef6df3494`.
Official HBBS, HBBR, and Control Agent binary hashes were checked before use.

With RustDesk 1.4.9, the matrix passed Starry `audit` for native-to-native,
then `enforce` for native-to-native, WSS-to-WSS, WSS-to-native, and
native-to-WSS controller and target combinations. Every case opened a Remote
Desktop session and observed the expected HBBR Relay connection. This is normal
compatibility validation; it does not claim an offensive security assessment.

## Release decision rule

The accepted residual above, the database/OAuth migration preflight, exact
artifact identity, and all ordinary candidate checks must be visible to the
release owner. The owner approved the source candidate through the reviewed
release process; publication remains fail-closed on the immutable tag and
protected candidate workflow described in `RELEASE-PROCESS.md`.
