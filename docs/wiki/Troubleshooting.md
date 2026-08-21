# Troubleshooting

**English** | [简体中文](ZH-CN-Troubleshooting.md)

| Symptom | Evidence to check | Safe action |
| --- | --- | --- |
| Container exits during startup | Configuration validation, key/certificate paths, file permissions, database connectivity | Correct the deployment input; do not disable validation. |
| Startup rejects MySQL/PostgreSQL transport | MySQL `tls`/`ca-file`, PostgreSQL `sslmode`/`ssl-root-cert`, database DNS name and certificate SAN | Use MySQL `true` or PostgreSQL `verify-full`, mount the reviewed CA read-only, and correct DNS/certificate identity; never use skip-verify. |
| Migration reports duplicate OAuth identity | Duplicate `(user_id,op)` or `(op,open_id)` rows and empty identity fields in the restored preflight database | Keep the old service stopped, preserve a backup, and have the identity owner explicitly merge/unbind; never delete a row only to satisfy the index. |
| Admin page is missing | `resources/admin`, reverse-proxy path, CSP/security headers | Verify the exact release image and proxy path; do not substitute external compiled assets. |
| Web Client page is missing | `resources/client/index.html`, port 21122 bind, dedicated client hostname/proxy, `web-client.mode` | Verify the exact release image and reviewed profile; never restore `resources/web*` or download WebClient2. |
| Admin Connect popup opens but remains blank | `web_client_public_origin`, popup policy, admin COOP, absent client COOP, strict-origin grant delivery, browser console without secrets | Require admin `same-origin-allow-popups` and no client COOP (`unsafe-none` default); verify ready/grant/accepted and best-effort revoke on timeout. Never use a query-string token or wildcard `postMessage`. |
| Web Client login/grant is rejected | API/client origins, exact CORS origin, EdDSA auth, token audience/scope/expiry, clock | Correct the fixed origins/auth profile and request a new short-lived grant; do not enable credentials CORS or reuse an admin token in the client. |
| Browser session cannot select Relay | Rendezvous WSS response and exact `relay-wss-urls` name map | Add only the reviewed exact Relay mapping and increment profile generation; never derive or follow an arbitrary Relay URL. |
| Browser connects but video/input is unavailable | Forced-Relay WSS, signed key exchange, VP9 WebCodecs support, frame limits, peer password, supported mouse/basic keys | Use a supported browser/VP9 peer and collect redacted state; unsupported codecs, P2P, clipboard/audio and other excluded features have no fallback. |
| Initial admin password is unknown | Whether the database exists and a mode-`0600` password file is available to the service user | Use `kessoku-api reset-admin-pwd --password-file PATH`; no reusable bootstrap password is logged, and the database must not be recreated. |
| RustDesk login succeeds but connection is denied | Starry mode, token audience/scope/key ID, JWKS freshness, introspection result | Return to `audit`, classify the reason, and fix the contract/deployment. |
| Logout does not affect a connection | Token row/JTI, auth version, introspection cache and call evidence | Verify authoritative introspection; never shorten the path by failing open. |
| Internal API returns TLS error | CA chain, server name, client SAN, TLS 1.3, clock | Repair PKI/DNS/time; never use skip-verify or the public proxy. |
| Starry instance is unavailable | Fixed origin, Agent identity UUID, CA/client certificate, timeout | Keep control read-only and repair the private management path. |
| Apply returns ETag/plan error | Current ETag, actor/instance binding, plan expiry, candidate digest | Re-read, merge, validate, and create a new plan; never force overwrite. |
| Apply returns success but UI is stale | Active Starry generation/digest and operation/audit records | Re-read authoritative state; HTTP success alone is insufficient. |
| Old binary cannot authenticate new sessions | Whether v2.8.2 issued hash-only tokens | Restore the matching pre-upgrade database backup or remediate forward. |
| Audit/sysinfo provenance is disputed | Whether the record came from the RustDesk 1.4.9 unauthenticated compatibility route | Treat it as operational telemetry and use append-only/immutable external logs for non-repudiation. |

Before changing behavior, preserve request/operation IDs, image and contract
digests, redacted logs, database version, and Starry generation. Never collect
raw bearer tokens, private keys, complete certificates, or complete YAML in a
support bundle.
