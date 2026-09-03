# Project metadata

**English** | [简体中文](PROJECT-METADATA.zh-CN.md)

This file records external GitHub values for the v3.0.8 preview. Remote changes
remain restricted to the protected release process and exact approved
candidate.

## Repository About

Description:

> Unofficial RustDesk account and enterprise administration API with a
> responsive UI, scoped administrators, Starry control, and a built-in
> Relay-only Web Client.

Website:

```text
https://github.com/q1ngyang/rustdesk-api-kessoku/wiki
```

Topics:

```text
rustdesk
rustdesk-api
self-hosted
remote-desktop
docker
golang
authentication
oidc
ldap
```

## GHCR package page

Image description:

> Kessoku RustDesk administration API with Starry control and a Relay-only
> WebClient.

The release workflow sets OCI/index title, source, release URL, documentation,
version, revision, licence, and description annotations. The documentation URL
points to [`CONTAINER.md`](../docs/deployment/CONTAINER.md), which provides visible links to:

- the recommended [Docker deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Deployment);
- the [Compose example](../docker-compose.yaml);
- the [environment example](../examples/compose.env.example); and
- the [Starry integration guide](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Starry-Control); and
- the [built-in Web Client guide](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client).

The preview publishes immutable `v3.0.8` and moving `preview` tags for the same
image. `latest` and v3.0.7 remain the published stable line; production
operators should resolve and pin an immutable digest.

## Wiki publication

Reviewed Wiki source is staged under [`docs/wiki/`](../docs/wiki). GitHub Wiki
is a separate Git repository; after explicit approval, copy these files to
`rustdesk-api-kessoku.wiki.git` and push them as a distinct publication action.
`_Sidebar.md` provides a bilingual index.

## Release content

The protected release workflow is prepared to:

- build the exact v3.0.8 candidate on `master`, exercise publication readiness,
  and only then create its immutable tag;
- include the stable local maintenance CLI, Presence Lease v2 contract, and
  machine-readable schema-313 migration contract for S6 integration;
- attach the Compose and environment examples plus bilingual container and
  release documents;
- publish a short GitHub Release summary with Read more links to the reviewed
  English and Chinese notes;
- publish one linux/amd64 GHCR image under both `v3.0.8` and `preview`, with OCI
  provenance and SBOM; and
- preserve the fail-closed checks for pre-tag commit, candidate run, protected
  environment, signing, registry authentication, contract, checksums, frontend
  source, and release approval.

Repository About and Wiki publication remain separate owner actions. Package,
tag, image, and prerelease publication must follow the audited sequence in
[`RELEASE-PROCESS.md`](../docs/releases/RELEASE-PROCESS.md).
