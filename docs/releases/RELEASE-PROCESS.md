# Kessoku release process

Kessoku releases are fail-closed: a read-only candidate workflow verifies the
exact `master` commit before any version tag exists. The protected release
workflow then exercises publication readiness and creates the immutable tag as
its final prepare step. Only that same workflow may consume the verified
candidate source commit to create public artifacts. The remote GitHub tag
object, rather than checkout's local tag representation, is authoritative.

`RELEASE_STATUS` records the release owner's approval of `v3.0.7`. Approval
does not by itself prove that a tag, image, package, Release, or Wiki exists.

## Pre-tag approval

Before tagging, the reviewed commit must pass the checks in
[`RELEASE-CHECKLIST.md`](RELEASE-CHECKLIST.md), including:

- exact Go 1.26.6 and Node 24.15.0/npm 11.12.1 builds;
- Go formatting, vet, tests, race tests, migration fixtures, and generated API
  documentation consistency;
- reproducible builds, audits, tests, and licence/SBOM checks for both embedded
  frontends;
- workflow linting plus online resolution of every immutable GitHub Action
  commit pin;
- database migrations through version 312 and backup/rollback guidance for the
  database, TOTP key, media, configuration, and signing keys;
- stable local CLI JSON/golden contracts, deterministic build-information
  injection, and the machine-readable v3.0.7 migration metadata;
- the pinned Starry contract, supported native-client matrix, separate-origin
  WebClient flow, and connection-audit evidence;
- concise bilingual release notes with explicit breaking changes; and
- absence of credentials, local test state, historical WebClient2/V2 assets,
  or unrelated development files in tracked source.

## Protected sequence

1. Merge the reviewed source commit to `master` and confirm required CI passes.
2. Confirm tag `v3.0.7` does not exist. Dispatch `.github/workflows/build.yml`
   on `master` with `release_tag=v3.0.7`. The read-only workflow rejects any
   non-`master` ref or existing tag and produces
   `kessoku-release-candidate-<commit>` only after every source, frontend,
   database, generated-documentation, package, image, checksum, SBOM, and
   vulnerability gate succeeds.
3. Record the successful candidate run ID and its exact `head_sha`. Download
   that candidate into an isolated staging environment and exercise login,
   device reporting, manual ID normalization, connection/file audit creation,
   branding defaults, WebClient authentication/connection, and Starry Control
   status/log retrieval. Do not approve the prepare environment until this
   acceptance passes.
4. Dispatch `.github/workflows/release.yml` on `master` with `mode=prepare` and
   that run ID. After the protected environment approval, it re-verifies the
   candidate, exercises Sigstore signing and the short Release body, then
   builds and pushes a commit-addressed `candidate-<sha>` image with OCI
   provenance and SBOM. Only after GHCR accepts that exact image does its last
   prepare step create annotated tag `v3.0.7` at the candidate `head_sha`.
5. Confirm the new tag resolves to that exact commit. Never move or reuse it.
6. Dispatch `.github/workflows/release.yml` on protected branch `master` with
   `mode=publish`, `release_tag=v3.0.7`, and the same candidate run ID. Publish
   verifies through the GitHub API that the remote annotated tag still resolves
   to the candidate source commit, then promotes the already-built candidate
   image digest to `v3.0.7` and `latest`; it does not rebuild the image after
   tagging. Release assets are uploaded to a draft and downloaded again for
   name and checksum verification before the Release becomes public.
7. After the protected environment gate, verify the GitHub Release, asset
   checksums and attestations, GHCR `v3.0.7`, and `latest`; both image tags must
   resolve to the same approved image index digest.
8. Publish the reviewed `docs/wiki/` tree to the separate GitHub Wiki repository
   and verify bilingual navigation and upgrade links.
9. Keep all earlier Git tags immutable as historical and audit records.

The `kessoku-release` environment must allow branch `master` for both modes;
the tag pattern `v*` remains for compatible retries. Keep the required reviewer
gate. Do not add one deployment policy per version; that previously caused a
valid v3.0.2 publication to fail only after its tag had already been created.

The protected reviewer is the final product-acceptance gate, not a ceremonial
click. Automation can prevent stale generated files and publication-pipeline
mistakes, but it cannot prove every browser, network, proxy, and native-client
combination. Staging acceptance before `mode=prepare` is what prevents a real
integration defect like the withdrawn v3.0.1 release from consuming a public
version.

Follow the [documentation maintenance guide](../development/DOCUMENTATION.md)
for Wiki URL rules and post-publication link checks. Wiki sources already use
rendered page URLs; do not convert them to relative `.md` or raw-file links.

A failed candidate or pre-tag `prepare` gate may be fixed on `master`, rebuilt,
and retried under the same intended version. If the final tag API call succeeded
but the runner lost its result, `prepare` accepts only the same annotated tag at
the exact candidate commit and resumes idempotently. Publish reads the candidate
SHA from the successful candidate run and validates the remote annotated tag
through the GitHub API, avoiding checkout implementations that locally flatten
an annotated tag. If `master` advanced after tagging, recovery is allowed only
when the candidate is its ancestor and every intervening path is release
workflow, release-process documentation, or its policy test; any product change
fails closed. A partial `publish` keeps its Release as a draft; retrying the same
run inputs replaces only those draft assets, verifies the complete asset
inventory and checksums, and then publishes it. A retry after publication is
read-only for the Release and succeeds only when its identity, body, and assets
still match. Never create a release tag manually, move an exposed tag, or reuse
a tag whose identity cannot be proven.

## Deployment and rollback

Deploy by immutable image digest or verified package checksum. Keep the
pre-upgrade database backup, TOTP encryption key, media, prior image digest,
configuration, signing keys, and PKI for the observation window. v3.0.7
advances schema 312 to 313 for Presence Lease v2; v3.0.6 cannot safely read
the new table/index contract. A return to v3.0.6 is restore-only after all
writers stop and the complete matching pre-upgrade recovery set is restored. Follow
[`MIGRATION-v3.0.7.md`](v3.0.7/MIGRATION-v3.0.7.md).

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

The immutable v3.0.4 tag also published no Release asset or container image.
Candidate run `33234210055` failed generated API documentation consistency.
The missing generated files were corrected on `master`; v3.0.5 moves that gate
and the complete publication-readiness probe before tag creation so an ordinary
candidate failure can no longer consume a version number.

Historical unpublished tombstones `v2.8.0`, `v2.8.1`, `v2.8.2`, and `v3.0.0`
also remain immutable. The `v3.0.0` candidate run `32842075289` failed generated
API documentation consistency and published no Release, image, package, or
release asset. `v2.8.3` is the preceding supported published release.
