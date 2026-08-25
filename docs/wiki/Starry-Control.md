# Starry control

**English** | [简体中文](ZH-CN-Starry-Control.md)

Kessoku integrates with the optional Starry Control Agent through Control API
v1. The browser talks only to Kessoku; it never talks directly to the Agent.

## Authorization

- Every Kessoku server-control route requires administrator privilege.
- The Agent requires both an approved mTLS client identity and a short-lived
  EdDSA service JWT with the exact scope for the operation.
- Agent origins, TLS names, identities, and credential paths are fixed in
  deployment configuration.
- User access-token and Control Agent signing keyrings are separate.

## Supported operations

- capabilities and status;
- configured/eligible Relay inventory and health;
- side-effect-free allocation simulation for two IPs and a transport;
- configuration and JSON/UI schema reads;
- authoritative validation and short-lived plan creation;
- ETag/idempotency-protected apply and operation polling;
- history, rollback, and acknowledged runtime reload.

There is no raw command, shell, arbitrary URL, Docker socket, generic file API,
or force-overwrite operation.

## Initial setup

1. Pin the exact released Starry Control/Auth contract in
   `internal/starrycontrol/CONTRACT_VERSION`.
2. Deploy the Agent on a private management path with writes disabled.
3. Verify instance identity, capabilities, status, Relay inventory, and
   simulation from Kessoku.
4. In staging, validate and plan a harmless configuration change.
5. Open a controlled write window, apply, verify the actual active generation,
   and rehearse rollback.
6. Restore Kessoku and the Agent to read-only outside change windows.

An HTTP success is not proof that the new configuration became active. Always re-read the
generation/digests and correlate Kessoku intent/result audit with the Agent's
durable audit.
