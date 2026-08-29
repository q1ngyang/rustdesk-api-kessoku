# Kessoku release process

Kessoku releases are fail-closed: a read-only candidate workflow verifies the
exact immutable tag, and only the protected publication workflow may consume
that successful candidate to create public artifacts.

`RELEASE_STATUS` records the release owner's approval of `v3.0.4`. Approval
does not by itself prove that a tag, image, package, Release, or Wiki exists.

## Pre-tag approval

Before tagging, the reviewed commit must pass the checks in
[`RELEASE-CHECKLIST.md`](RELEASE-CHECKLIST.md), including:

- exact Go 1.26.6 and Node 24.15.0/npm 11.12.1 builds;
- Go formatting, vet, tests, race tests, migration fixtures, and generated API
  documentation consistency;
- reproducible builds, audits, tests, and licence/SBOM checks for both embedded
  frontends;
- database migrations through version 309 and backup/rollback guidance for the
  database, TOTP key, media, configuration, and signing keys;
- the pinned Starry contract, supported native-client matrix, separate-origin
  WebClient flow, and connection-audit evidence;
- concise bilingual release notes with explicit breaking changes; and
- absence of credentials, local test state, historical WebClient2/V2 assets,
  or unrelated development files in tracked source.

## Protected sequence

1. Merge the reviewed source commit to `master`.
2. Create immutable tag `v3.0.4` at that commit; never move or reuse it.
3. Dispatch `.github/workflows/build.yml` on the tag. This workflow is
   read-only and produces `kessoku-release-candidate-<commit>`.
4. Confirm the entire candidate workflow succeeds and record its run ID.
5. Dispatch `.github/workflows/release.yml` on the same tag with that run ID.
6. After the protected environment gate, verify the GitHub Release, asset
   checksums and attestations, GHCR `v3.0.4`, and `latest`; both image tags must
   resolve to the same approved image index digest.
7. Publish the reviewed `docs/wiki/` tree to the separate GitHub Wiki repository
   and verify bilingual navigation and upgrade links.
8. Keep the v3.0.1, v3.0.2, and v3.0.3 Git tags immutable as historical and
   audit records.

Follow the [documentation maintenance guide](../development/DOCUMENTATION.md)
for Wiki URL rules and post-publication link checks. Wiki sources already use
rendered page URLs; do not convert them to relative `.md` or raw-file links.

A failed or partial publication must be inventoried before retrying. Never
overwrite an exposed version tag; use a new reviewed patch version if identity
cannot be proven.

## Deployment and rollback

Deploy by immutable image digest or verified package checksum. Keep the
pre-upgrade database backup, TOTP encryption key, media, prior image digest,
configuration, signing keys, and PKI for the observation window. Database
version 309 is not safe for older writers; follow
[`MIGRATION-v3.0.4.md`](v3.0.4/MIGRATION-v3.0.4.md) and restore the complete
matching recovery set for rollback.

## Historical release records

The v3.0.1 GitHub Release was withdrawn after significant administration and
WebClient integration defects were confirmed. Its tag remains immutable and
must not be reused or moved. Candidate run `32844495798` and publication run
`32845110360` remain the historical evidence for that withdrawn artifact.

The immutable v3.0.2 tag published no Release asset or container image.
Candidate run `33149296963` succeeded; publication run `33149666586` was
rejected by the environment tag allowlist, and run `33149731867` then failed
closed before publication because its tagged workflow lacked the
`artifact-metadata: write` permission required for the signed attestation
storage record. v3.0.3 adds only that minimum publication permission and new
release identity around the already reviewed product candidate.

Historical unpublished tombstones `v2.8.0`, `v2.8.1`, `v2.8.2`, and `v3.0.0`
also remain immutable. The `v3.0.0` candidate run `32842075289` failed generated
API documentation consistency and published no Release, image, package, or
release asset. `v2.8.3` is the preceding supported published release.
