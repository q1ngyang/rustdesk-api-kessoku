# Project metadata publication draft

**English** | [简体中文](PROJECT-METADATA.zh-CN.md)

This file records the external GitHub values proposed for v2.8.0. It is a
review draft and does not authorize publication.

## Repository About

Proposed description:

> Unofficial RustDesk account and administration API with EdDSA token
> lifecycle, typed rustdesk-server-starry control, and a built-in open-source
> Relay-only Web Client.

Proposed website:

```text
https://github.com/q1ngyang/rustdesk-api-kessoku/wiki
```

Proposed topics:

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

Proposed image description:

> Kessoku v2.8.0 RustDesk account and administration API with EdDSA token
> lifecycle, typed Starry control, and a repository-owned Relay-only Web
> Client; Docker Compose is recommended.

The release workflow sets OCI/index title, source, release URL, documentation,
version, revision, licence, and description annotations. The documentation URL
points to [`CONTAINER.md`](../CONTAINER.md), which provides visible links to:

- the recommended [Docker deployment](../docs/wiki/Docker-Deployment.md);
- the [Compose example](../docker-compose.yaml);
- the [environment example](../examples/compose.env.example); and
- the [Starry integration guide](../docs/wiki/Starry-Control.md).
- the [built-in Web Client guide](../docs/wiki/Web-Client.md).

The release publishes immutable `v2.8.0` and moving `latest` tags for the same
image. `latest` identifies the newest successfully published stable release;
production operators resolve and pin the version tag's digest.

## Wiki publication

Reviewed Wiki source is staged under [`docs/wiki/`](../docs/wiki). GitHub Wiki
is a separate Git repository; after explicit approval, copy these files to
`rustdesk-api-kessoku.wiki.git` and push them as a distinct publication action.
`_Sidebar.md` provides a bilingual index.

## Release content

The protected release workflow is prepared to:

- publish the exact successful, non-publishing v2.8.0 candidate;
- attach the Compose and environment examples plus bilingual container and
  release documents;
- build the GitHub Release body from the reviewed English release notes and
  link the Chinese notes;
- publish one linux/amd64 GHCR image under both `v2.8.0` and `latest`, with OCI
  provenance and SBOM; and
- preserve the fail-closed checks for tag, commit, candidate run, contract,
  checksums, frontend source, and release approval.

No repository About, Wiki, package, tag, image, or Release update should occur
until the documentation/new-feature wording and final release gate are approved.
