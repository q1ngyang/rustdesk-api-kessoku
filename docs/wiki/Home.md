# rustdesk-api-kessoku documentation

**English** | [简体中文](ZH-CN-Home.md)

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
| Existing HBBS/HBBR; first Kessoku deployment | [Getting Started](Getting-Started.md) |
| Blank host; complete API + HBBS + HBBR deployment | [Complete Kessoku + Starry Deployment](Complete-Deployment.md) |
| Add a remote HBBR-only Relay node | [Relay-Only Deployment](Relay-Only-Deployment.md) |
| You arrived from GHCR | [Docker Image Usage](Docker-Image-Usage.md) |
| Recommended single-host API deployment | [Docker Deployment](Docker-Deployment.md) |
| You need Nginx, TLS, or port rules | [Reverse Proxy and Firewall](Reverse-Proxy-and-Firewall.md) |
| You need all configuration boundaries | [Configuration Reference](Configuration-Reference.md) |
| You are enabling connection authentication | [Connection Authentication](Connection-Authentication.md) |
| You need Relay visibility or configuration transactions | [Starry Control](Starry-Control.md) |
| You need the current browser-client boundary | [Web Client](Web-Client.md) |
| You need backups, health checks, or routine verification | [Operations and Verification](Operations-and-Verification.md) |
| You are hardening a public deployment | [Security Configuration](Security-Finding-Closure.md) |
| You are upgrading or preparing rollback | [Upgrade and Rollback](Upgrade-and-Rollback.md) |
| A deployment is failing | [Troubleshooting](Troubleshooting.md) |

## Safe defaults

- Use the immutable v3.0.1 image tag, then pin its resolved digest.
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

v3.0.1 is the stable release. Production deployments should verify release
checksums and pin the versioned GHCR digest. The project is MIT licensed and is
not affiliated with RustDesk. The repository-owned browser client is MIT;
third-party licences are included with release artifacts. Historical
WebClient2/V2 assets are not included.
