# Kessoku Release Process

Kessoku releases are fail-closed: a read-only candidate workflow verifies the
exact immutable tag, and only the protected publication workflow may consume
that successful candidate to create public artifacts.

`RELEASE_STATUS` records the release owner's approval of `v3.0.1`. This does
not by itself prove that a tag, image, package, Release, or Wiki exists.

## Pre-tag approval

Before tagging, the reviewed commit must pass the checks in
[`RELEASE-CHECKLIST.md`](RELEASE-CHECKLIST.md), including:

- exact Go 1.26.6 and Node 24.15.0/npm 11.12.1 builds;
- Go formatting, vet, tests, race tests, migration fixtures, and generated API
  documentation consistency;
- reproducible builds, audits, tests, and licence/SBOM checks for both embedded
  frontends;
- the pinned Starry contract and supported client matrix;
- concise bilingual release notes, explicit breaking changes, database backup
  and rollback guidance; and
- absence of credentials, historical WebClient2/V2 assets, or unrelated local
  development notes in the tracked source.

## Protected sequence

1. Merge the reviewed source commit to `master`.
2. Create immutable tag `v3.0.1` at that commit; never move or reuse it.
3. Dispatch `.github/workflows/build.yml` on the tag. This workflow is
   read-only and produces `kessoku-release-candidate-<commit>`.
4. Confirm the entire candidate workflow succeeds and record its run ID.
5. Dispatch `.github/workflows/release.yml` on the same tag with that run ID.
6. After the protected environment gate, verify the GitHub Release, asset
   checksums and attestations, GHCR `v3.0.1`, and `latest`; both image tags must
   resolve to the same approved image index digest.
7. Publish the reviewed `docs/wiki/` tree to the separate GitHub Wiki repository
   and verify bilingual navigation and upgrade links.

A failed or partial publication must be inventoried before retrying. Never
overwrite an exposed version tag; use a new reviewed patch version if identity
cannot be proven.

## Deployment and rollback

Deploy by immutable image digest or verified package checksum. Keep the
pre-upgrade database backup, prior image digest, configuration, and keys for
the observation window. Database version 302 changes administrator semantics;
follow [`MIGRATION-v3.0.1.md`](MIGRATION-v3.0.1.md) before any v2 rollback.

Historical unpublished tombstones `v2.8.0`, `v2.8.1`, `v2.8.2`, and `v3.0.0`
remain immutable and must not be reused. The `v3.0.0` candidate run
`32842075289` failed generated API documentation consistency before assembling
the final candidate; it published no Release, image, package, or release asset.
`v2.8.3` is the preceding published release.
