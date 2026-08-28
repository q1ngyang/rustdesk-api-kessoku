# Documentation index

**English** | [简体中文](README.zh-CN.md)

Start with the [online Wiki](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Home)
for deployment and daily use. This directory also keeps release history,
operations procedures, security design, and developer references organized by
purpose.

## Deployment guides

- [Kessoku with existing HBBS/HBBR](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Getting-Started)
- [Complete Kessoku + Starry stack](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Complete-Deployment)
- [Relay-only HBBR node](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment)
- [Nginx and firewall](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Reverse-Proxy-and-Firewall)
- [Container guide](deployment/CONTAINER.md)
- [Browser-client configuration and limitations](deployment/WEB-CLIENT.md)

## Directory map

| Directory | Contents |
| --- | --- |
| [wiki/](wiki/) | Bilingual source pages published to GitHub Wiki |
| [deployment/](deployment/) | Container and browser-client deployment references |
| [operations/](operations/) | Operator and rollback runbooks |
| [security/](security/) | Security model and trust boundaries |
| [releases/](releases/) | Release procedure, checklist, and migration history |
| [releases/v3.0.2/](releases/v3.0.2/) | Current v3.0.2 release notes and migration guide |
| [releases/v3.0.1/](releases/v3.0.1/) | Withdrawn v3.0.1 historical release documents |
| [releases/v2.8.3/](releases/v2.8.3/) | Historical v2.8.3 release notes |
| [development/](development/) | UI design, protocol details, provenance, and documentation maintenance |
| [api/](api/) and [admin/](admin/) | Generated API documentation; keep these paths stable for Go imports |

Runnable deployment templates remain in [examples/](../examples/). Component
READMEs and licence/provenance notices stay with their source code. Root READMEs
remain project entry points; other guides belong under this directory.

See [documentation maintenance](development/DOCUMENTATION.md) before moving
pages or publishing the Wiki.
