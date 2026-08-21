# Kessoku Release Process

This procedure is fail-closed. The candidate workflow never publishes, and the
publication workflow accepts only a successful candidate from the exact tag
commit. The reviewed source candidate sets `RELEASE_STATUS` to `APPROVED` and
names `release_tag: v2.8.0`; this permits the protected sequence below but does
not claim that its tag or any public artifact already exists.

## Approval prerequisites

Before changing the gate, the release owner must verify every pre-publication
item in `RELEASE-CHECKLIST.md` and record the supporting local evidence, artifact
digests, compatibility results, and approvers in the release review. The
deployment owner separately records topology-specific recovery objectives and
the rollout go/no-go decision. In particular:

- `internal/starrycontrol/CONTRACT_VERSION` must name a published immutable
  Starry tag and verified contract digests with `status: PINNED`;
- the embedded `admin-web/` source, provenance, license, and lockfile must be
  reviewed in the same commit and pass `npm ci`, tests, build, registry
  signatures, and the production dependency audit;
- SQLite, MySQL, PostgreSQL, supported RustDesk client, mTLS/JWT rotation, and
  rollback evidence must belong to the exact source commit. One universal
  deployment RTO/RPO is not a software-publication requirement; and
- the release owner must approve the final repository/image name, concise
  bilingual README, bilingual release/container notes, paired Wiki source,
  Docker examples, GHCR description, deployment links, and v2.8.0 feature and
  compatibility wording; and
- the GitHub `kessoku-release` environment must require independent reviewers
  and allow only the intended protected tag.

The final reviewed source change sets:

```text
status: APPROVED
release_tag: v2.8.0
```

Do not approve a floating branch, unpublished Starry contract, externally
substituted frontend, unsupported platform artifact, or partially passing
candidate. Tag creation, protected candidate CI, and publication are sequenced
post-approval checks and are recorded only after they occur.

## Build the non-publishing candidate

Before approval, `scripts/verify-local-admin-candidate.sh` may be run with a new
absolute `KESSOKU_LOCAL_EVIDENCE_DIR` outside the repository. It clones the
clean HEAD, verifies Go/frontend tests and scans, and retains checksummed local
archive/DEB/SBOM/build evidence. This is review evidence only and cannot replace
the protected candidate workflow below.

1. Create the intended immutable tag at the reviewed commit. Do not move it.
2. Dispatch `.github/workflows/build.yml` on that tag.
3. Confirm all database migrations, Go tests/race checks, source and runtime
   scans, both embedded-frontend checks, Web Client grant/origin/browser
   acceptance, Docker linux/amd64 smoke, Linux x86_64
   archive, and real amd64 DEB installation passed. ARM and Windows are not
   v2.8.0 publication gates.
4. Download `kessoku-release-candidate-<commit>` and independently verify
   `SHA256SUMS`, `BUILD-INPUTS.txt`, `RELEASE_STATUS`, and `CONTRACT_VERSION`.
5. Inspect the archive, DEB, Docker context, all three SPDX SBOMs, and separate
   frontend CycloneDX/licence evidence. They must contain the reviewed admin UI
   and `resources/client`, and must not contain `resources/web`,
   `resources/web2`, WebClient2/V2 assets, private keys, or build credentials.
6. Record the successful candidate workflow run ID. A rerun is a distinct
   candidate and requires a fresh review.

Candidate artifacts are evidence, not a release. They may be deleted without
affecting users and must never be copied into an unrelated publication run.

## Publish the exact candidate

1. Dispatch `.github/workflows/release.yml` from the same immutable tag and
   enter the reviewed candidate run ID.
2. The workflow verifies the tag, approved gate, source SHA, workflow path,
   event type, successful conclusion, candidate identity, and checksums before
   requesting protected-environment approval.
3. After approval it creates GitHub/Sigstore build-provenance and SBOM
   attestations, pushes the immutable version tag and moving `latest` tag for
   the same approved image, and creates the GitHub release. The image metadata
   links the versioned container guide, which in turn links the recommended
   Docker deployment and examples.
4. Verify the published asset checksums and attestations from a clean machine;
   verify the version and `latest` tags resolve to the same image index digest;
   then record the versioned digest in deployment manifests before rollout.
5. As a separately approved documentation action, publish the reviewed files
   under `docs/wiki/` to `rustdesk-api-kessoku.wiki.git`, then verify the live
   bilingual navigation and every Docker/package-page link.

If publication fails after any external object was created, stop. Inventory the
GitHub release, attestations, and GHCR tag/digest before retrying. Never move or
overwrite an exposed version tag; use a reviewed replacement version if the
partial state cannot be proven identical.

## Deployment and rollback

Deploy by immutable image digest or verified package checksum, following
`OPERATOR-RUNBOOK.md`. Keep the database backup, prior image digest, prior
configuration, access-key material, and Control Agent key material available
for the entire observation window. Trigger `ROLLBACK-RUNBOOK.md` on migration,
authentication, control-plane, client-compatibility, or audit regressions.

正式发布同样遵循上述顺序：先完成清单与独立审批，再从目标 tag 运行不发布的候选流程，最后让受保护的发布流程按 run ID 消费同一提交的候选。不可变版本 tag 不得移动；`latest` 只能由成功的稳定版发布流程更新，并必须与该版本 tag 指向相同 digest。不得跳过两个前端的审计、可复现与许可证门禁；只允许仓库自有 `resources/client`，不能加入历史 WebClient2/V2 资产。
