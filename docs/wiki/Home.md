# rustdesk-api-kessoku documentation

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Home)

Kessoku is an unofficial RustDesk account, administration, and policy plane.
It supplies the client API and embedded management UI. It can use compatible
official HBBS/HBBR services; the recommended pairing is the same developer's
Starry HBBS, which remains authoritative for signalling, optional connection
authorization, and Relay allocation.

## Understand the boundary first

| Component | Included? | Role |
| --- | --- | --- |
| Kessoku API and admin UI | Yes | Accounts, login, address books, devices, token lifecycle, audit, and typed administration. |
| Starry HBBS | No | Signalling, strict connection JWT enforcement, and Relay decisions. |
| Starry Control Agent | No | Optional least-privilege Relay/configuration API for one HBBS. |
| HBBR | No | Remote-control data Relay; the Starry image includes an unmodified HBBR from its pinned upstream version. |
| Built-in browser remote-desktop MVP | Yes | Repository-owned forced-Relay WSS, VP9, mouse, and basic keyboard client on a separate origin. |

## Choose a starting point

| Situation | Start here |
| --- | --- |
| Existing HBBS/HBBR; first Kessoku deployment | [Getting Started](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Getting-Started) |
| Blank host; complete API + HBBS + HBBR deployment | [Complete Kessoku + Starry Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Complete-Deployment) |
| Add a remote HBBR-only Relay node | [Relay-Only Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment) |
| You arrived from GHCR | [Docker Image Usage](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Image-Usage) |
| Recommended single-host API deployment | [Docker Deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Deployment) |
| You need Nginx, TLS, or port rules | [Reverse Proxy and Firewall](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Reverse-Proxy-and-Firewall) |
| You need all configuration boundaries | [Configuration Reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference) |
| You are enabling connection authentication | [Connection Authentication](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Connection-Authentication) |
| You need Relay visibility or configuration transactions | [Starry Control](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control) |
| You need the current browser-client boundary | [Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client) |
| You need backups, health checks, or routine verification | [Operations and Verification](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Operations-and-Verification) |
| You are hardening a public deployment | [Security Configuration](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Security-Finding-Closure) |
| You are upgrading or preparing rollback | [Upgrade and Rollback](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback) |
| A deployment is failing | [Troubleshooting](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Troubleshooting) |

## Safe defaults

- Use the immutable v3.0.6 image tag, then pin its resolved digest.
- Keep public registration, Swagger, and legacy-token compatibility disabled
  unless they are intentionally required. Publish the browser client only on
  its own HTTPS origin with working Starry WSS paths.
- Begin Starry connection authentication in `off` or `audit`, never directly
  in `enforce`.
- Start the Control Agent read-only and keep its endpoint private.
- Keep access-token, internal-PKI, and Control Agent keys outside images and
  separate from each other.
- Container health, HTTP 200, and a successful login do not test remote-control
  transport; finish with real native, WSS, peer-to-peer, and Relay sessions.

## Release and legal status

v3.0.6 is the stable release. Production deployments should verify release
checksums and pin the versioned GHCR digest. The project is MIT licensed and is
not affiliated with RustDesk. The repository-owned browser client is MIT;
third-party licences are included with release artifacts. Historical
WebClient2/V2 assets are not included.
