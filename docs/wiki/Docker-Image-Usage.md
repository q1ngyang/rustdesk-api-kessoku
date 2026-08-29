# Docker image usage

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Image-Usage)

The complete versioned package-page guide is
[`CONTAINER.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/deployment/CONTAINER.md). It covers image contents, immutable tags,
Compose, secrets, ports, acceptance, and rollback.

Quick links:

- [GHCR package](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [Docker deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Deployment)
- [Compose example](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docker-compose.yaml)
- [Environment example](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/compose.env.example)
- [Caddy HTTPS example](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/Caddyfile.example)
- [v3.0.4 release notes](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.4/RELEASE-NOTES-v3.0.4.md)

The supported image platform is `linux/amd64`. The release publishes immutable
`v3.0.4` and moving `latest` tags for the same image. Production deployments
should inspect and pin the versioned tag's resolved digest; use `latest` only
when intentionally following the newest stable release with rollback ready.
