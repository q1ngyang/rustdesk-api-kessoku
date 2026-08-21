# Docker image usage

**English** | [简体中文](ZH-CN-Docker-Image-Usage.md)

The complete versioned package-page guide is
[`CONTAINER.md`](../../CONTAINER.md). It covers image contents, immutable tags,
Compose, secrets, ports, acceptance, and rollback.

Quick links:

- [GHCR package](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [Docker deployment](Docker-Deployment.md)
- [Compose example](../../docker-compose.yaml)
- [Environment example](../../examples/compose.env.example)
- [Caddy HTTPS example](../../examples/Caddyfile.example)
- [v2.8.3 release notes](../../RELEASE-NOTES-v2.8.3.md)

The supported image platform is `linux/amd64`. The release publishes immutable
`v2.8.3` and moving `latest` tags for the same image. Production deployments
should inspect and pin the versioned tag's resolved digest; use `latest` only
when intentionally following the newest stable release with rollback ready.
