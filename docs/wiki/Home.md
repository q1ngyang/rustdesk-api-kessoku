# rustdesk-api-kessoku documentation

**English** | [简体中文](ZH-CN-Home.md)

Kessoku is an unofficial RustDesk account, administration, and policy plane.
It supplies the client API and embedded management UI; Starry HBBS remains
authoritative for signalling, connection authorization, and Relay allocation.

## Understand the boundary first

| Component | Included? | Role |
| --- | --- | --- |
| Kessoku API and admin UI | Yes | Accounts, login, address books, devices, token lifecycle, audit, and typed administration. |
| Starry HBBS | No | Signalling, strict connection JWT enforcement, and Relay decisions. |
| Starry Control Agent | No | Optional least-privilege Relay/configuration API for one HBBS. |
| Official HBBR | No | Remote-control data Relay. |
| Built-in browser remote-desktop MVP | Yes | Repository-owned forced-Relay WSS, VP9, mouse, and basic keyboard client on a separate origin. |

## Choose a starting point

| Situation | Start here |
| --- | --- |
| First deployment | [Getting Started](Getting-Started.md) |
| You arrived from GHCR | [Docker Image Usage](Docker-Image-Usage.md) |
| Recommended single-host API deployment | [Docker Deployment](Docker-Deployment.md) |
| You need all configuration boundaries | [Configuration Reference](Configuration-Reference.md) |
| You are enabling connection authentication | [Connection Authentication](Connection-Authentication.md) |
| You need Relay visibility or configuration transactions | [Starry Control](Starry-Control.md) |
| You need the current browser-client boundary | [Web Client](Web-Client.md) |
| You are collecting release/staging evidence | [Operations and Verification](Operations-and-Verification.md) |
| You are upgrading or preparing rollback | [Upgrade and Rollback](Upgrade-and-Rollback.md) |
| A deployment is failing | [Troubleshooting](Troubleshooting.md) |

## Safe defaults

- Use the immutable v3.0.1 image tag, then pin its resolved digest.
- Keep registration, Swagger, built-in Web Client, and legacy token
  compatibility disabled until their deployment profiles are explicitly
  reviewed. Enable the client only on a separate HTTPS origin.
- Begin Starry connection authentication in `off` or `audit`, never directly
  in `enforce`.
- Commission the Control Agent read-only and keep its endpoint private.
- Keep access-token, internal-PKI, and Control Agent keys outside images and
  separate from each other.
- Treat container health, HTTP 200, and a successful login as partial evidence;
  finish with real native/Secure TCP/WSS/P2P/Relay client sessions.

## Release and legal status

v3.0.1 is the stable release. Production deployments should verify the release
checksums and pin the versioned GHCR digest. The reviewed source is MIT licensed
and is not affiliated with RustDesk. The
repository-owned Web Client is MIT; third-party dependency licences are
recorded in the release SBOM. Historical WebClient2/V2 assets are not included.
