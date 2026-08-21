# Project metadata

**English** | [简体中文](PROJECT-METADATA.zh-CN.md)

This file records the release-owner-approved external GitHub values for
v2.8.3. Remote changes remain restricted to the protected release process.

## Repository About

Description:

> Unofficial RustDesk account and administration API with EdDSA token
> lifecycle, typed rustdesk-server-starry control, and a built-in open-source
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

> Kessoku v2.8.3 RustDesk account and administration API with EdDSA token
> lifecycle, typed Starry control, and a repository-owned Relay-only Web
> Client; Docker Compose is recommended.

The release workflow sets OCI/index title, source, release URL, documentation,
version, revision, licence, and description annotations. The documentation URL
points to [`CONTAINER.md`](../CONTAINER.md), which provides visible links to:

- the recommended [Docker deployment](../docs/wiki/Docker-Deployment.md);
- the [Compose example](../docker-compose.yaml);
- the [environment example](../examples/compose.env.example); and
- the [Starry integration guide](../docs/wiki/Starry-Control.md); and
- the [built-in Web Client guide](../docs/wiki/Web-Client.md).

The release publishes immutable `v2.8.3` and moving `latest` tags for the same
image. `latest` identifies the newest successfully published stable release;
production operators resolve and pin the version tag's digest.

## Wiki publication

Reviewed Wiki source is staged under [`docs/wiki/`](../docs/wiki). GitHub Wiki
is a separate Git repository; after explicit approval, copy these files to
`rustdesk-api-kessoku.wiki.git` and push them as a distinct publication action.
`_Sidebar.md` provides a bilingual index.

## Release content

The protected release workflow is prepared to:

- publish the exact successful, non-publishing v2.8.3 candidate;
- attach the Compose and environment examples plus bilingual container and
  release documents;
- build the GitHub Release body from the reviewed English release notes and
  link the Chinese notes;
- publish one linux/amd64 GHCR image under both `v2.8.3` and `latest`, with OCI
  provenance and SBOM; and
- preserve the fail-closed checks for tag, commit, candidate run, contract,
  checksums, frontend source, and release approval.

The documentation/new-feature wording and release gate are approved. Repository
About, Wiki, package, tag, image, and Release changes still follow the audited
sequence in [`RELEASE-PROCESS.md`](../RELEASE-PROCESS.md).
