# Kessoku v3.0.8 upgrade and rollback

> v3.0.8 is an opt-in **preview**. Use the exact Starry
> `1.1.16-patch-v1.3.1` preview and pin both images by immutable digest. The
> stable v3.0.7/`latest` line remains available for rollback.

## Compatibility boundary

Kessoku's main database remains at schema `313`; no application-database
migration is needed from v3.0.7. Starry support is negotiated only by exact
Control API capabilities:

| Starry contract | Read behavior | Write behavior |
| --- | --- | --- |
| patch-v1.2, no Relay Quality/Fast capability | Existing Relay inventory works; new state is **Unsupported** | No schema-v5 or Fast writes |
| v1.3.0 Relay Quality + FastCompat | Existing Relay Quality and legacy FastCompat aggregates work; FastMedia is **Unsupported** | Schema-v4 Quality/FastCompat writes follow the existing capabilities; schema-v5/FastMedia writes are blocked |
| `1.1.16-patch-v1.3.1` preview (contract commit `6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`) | Typed Fast and Pairing state is validated | Fast writes require capability versions 1 and schema 5 |
| Missing known capability | Explicitly **Unsupported**, not Disabled | Incompatible writes blocked |
| Unknown version of a known capability | Response fails closed | All dependent writes blocked |

The v1.3.1 contract and preview runtime are immutable and pinned by the bundled
release summary. Preview evidence is not stable production approval.

## New independent state

SP1 state does not use or migrate the Kessoku business database. It lives at:

- container: `/app/data/server-control/registry-v1.sqlite`;
- Debian/systemd: `/var/lib/kessoku-api/data/server-control/registry-v1.sqlite`;
- private credentials: adjacent `instances/` directories (`0700` directories,
  `0600` files);
- v3.0.7-compatible exports: adjacent
  `exports/<managed-id>.static-instance.yaml`.

The configured root and its contents must belong to the service UID and must
not be symlinks. The CLI and service use the same lock and monotonically
increasing registry generation.
Pairing also requires an explicit `server-control.host-identity-file`.
Systemd uses `/etc/machine-id`; containers must mount the host file separately
at `/run/kessoku-host-machine-id`. It must not come from the container image or
live inside the copied registry tree.

## Pre-upgrade

1. Stop every Kessoku process that can write the same data directory.
2. Record the exact host path used as `KESSOKU_DATA_DIR`; resolve it before
   changing Compose directories or environment files.
3. Back up the entire data directory, configuration, and secrets as one
   recovery set. For external SQL, also take the database vendor's consistent
   backup even though schema stays at 313.
4. Verify the Starry contract checkout is clean at the exact full commit and
   verify the bundled summary:

   ```sh
   sha256sum docs/releases/v3.0.8/STARRY-RELEASE-SUMMARY.json
   # fedeb47ff77bdbc594ddd3ba5b54238a469b02416cfb3410dbd535eff9c7e0ef
   ```

5. Validate the Kessoku configuration locally and start with both Kessoku and
   Starry writes disabled.

## Docker upgrade and persistence preflight

Keep the exact same bind source mounted at `/app/data`:

```sh
test -n "${KESSOKU_DATA_DIR:?set the existing absolute data directory}"
test -d "$KESSOKU_DATA_DIR"
test -s "${KESSOKU_HOST_IDENTITY_FILE:-/etc/machine-id}"
docker compose config
docker compose pull kessoku-api
docker compose up -d --force-recreate kessoku-api
docker compose exec kessoku-api ./kessoku-api server-control registry status \
  --config /app/conf/config.yaml --json
```

Normal `pull`, `up --force-recreate`, and `down`/`up` preserve pairing only
when they resolve to the same host directory. A changed relative path may
silently mount an empty directory, but Kessoku startup and `registry status`
fail with `REGISTRY_NOT_INITIALIZED` instead of creating another installation.
Compare the recorded absolute path and `installation_id` before pairing or
writing.

Do not use `docker compose down -v` as an upgrade instruction. The repository
default is a bind mount, which Docker does not delete with `-v`, but deployment
overrides may use named or anonymous volumes that `-v` removes. Inspect
`docker compose config`, identify the actual volume type, and have a tested
backup before any volume deletion.

The container runs as UID/GID `65534:65534`. A permission mismatch, symlinked
root, file mode broader than `0600`, or directory mode other than `0700` fails
registry preflight instead of generating a replacement identity.

## Debian/systemd upgrade

The default root is `/var/lib/kessoku-api/data/server-control`. Package upgrade,
downgrade, and normal removal preserve `/var/lib/kessoku-api`; maintainer
scripts do not delete managed identities. Confirm ownership by the
`kessoku-api` service user, then run:

```sh
sudo -u kessoku-api kessoku-api server-control registry status \
  --config /var/lib/kessoku-api/conf/config.yaml --json
```

The service and CLI must not run concurrent lifecycle operations during host
migration or purge. Ordinary status/pairing mutations share the registry lock.

## SP1 pairing and rotation

Configure `server-control.pairing.agent-origins` in the deployment file. Only an
allowlist ID may be selected by the CLI/UI; an arbitrary URL or callback is not
accepted. The broker origin and TLS SPKI SHA-256 pin must match the public
broker certificate.

Create a short-lived code without placing it in shell arguments, environment,
logs, or tickets:

```sh
kessoku-api server-control pair create --config /path/to/config.yaml \
  --id starry-main --name "Starry main" --agent-origin primary \
  --confirm confirm:pair:starry-main:primary
```

If the code is not claimed, revoke it by enrollment ID before creating a
replacement. The raw code is neither required nor accepted:

```sh
kessoku-api server-control pair revoke --config /path/to/config.yaml \
  --enrollment-id 019b0000-0000-7000-8000-000000000001 \
  --confirm confirm:revoke-pairing:019b0000-0000-7000-8000-000000000001
```

On a genuinely new deployment, this exact confirmed `pair` action is also the
only operation allowed to initialize the missing registry. Service startup,
status, `rotate`, `adopt`, and Relay commands never initialize one. Therefore,
after `down -v` or a path change, restore or verify the intended data root
before confirming a new pair; that confirmation deliberately creates a new
installation identity.

The Agent learns a new instance UUID during `pair`. `adopt` and `rotate` require
the already-known exact instance UUID. Rotation creates a new independent
credential generation, retains the prior files for controlled rollback, updates
the static export atomically, hot-loads the provider, and forces it back to
read-only. Prove the new certificate and JWT key work before archiving the old
generation; production rotation remains a stable-promotion gate.

Relay code creation first calls the authenticated Agent's prepare endpoint.
Relay endpoint, pool, certificate, secret, and configuration authority stay in
Starry. `--activate-after-health` is accepted only with the exact high-risk
confirmation at code creation. Otherwise use explicit activation with the
successful operation ID, generation, configuration digest, and health snapshot.

## Backup, restore, and host migration

Stop the service before copying `server-control/` so the SQLite database, WAL,
credentials, and exports form one generation-consistent set. Restore the whole
directory at its original absolute path and permissions. Never restore only the
SQLite file or only `instances/`.

The registry binds to a non-secret host fingerprint. A restored copy on a new
host fails with an identity-clone error. After proving the source host is
stopped and cannot write the same identity, run the two-part adoption:

```sh
kessoku-api server-control registry adopt-host --config /path/to/config.yaml \
  --installation-id <recorded-uuid> --old-host-stopped \
  --confirm confirm:adopt-host:<recorded-uuid>
```

Do not run both hosts with one registry identity. A cloned live identity is a
security incident, not a load-balancing configuration.

## v3.0.8 to v3.0.7 and back

1. Stop v3.0.8 and back up the full data/configuration set.
2. For every managed instance, manually merge the generated
   `exports/<id>.static-instance.yaml` entry into a v3.0.7-compatible
   `server-control.instances[]` configuration. Keep Kessoku and Agent writes
   disabled until verified.
3. Start v3.0.7 with the same data mount. It continues using the static mTLS/JWT
   files and ignores the managed registry; it must not modify or delete
   `data/server-control`.
4. Verify capabilities/status and a real reliable Relay session. FastMedia
   schema-v5 controls and SP1 management are unavailable in v3.0.7.
5. To return to v3.0.8, stop v3.0.7, restore/use the same unmodified directory
   and configuration, then start v3.0.8. Confirm the original installation ID,
   registry generation, instance UUID, and credential digests before enabling
   writes.

No business-database restore is required for this binary round trip because
both versions use schema 313. Restore from backup if any data or registry path
was changed outside this procedure.

## Explicit identity purge

Uninstalling a package, deleting a container, or disabling pairing does not
delete identities. Permanent deletion is available only after stopping every
process and supplying both confirmations:

```sh
kessoku-api server-control registry purge --config /path/to/config.yaml \
  --installation-id <recorded-uuid> --service-stopped \
  --data-loss-understood --confirm confirm:purge:<recorded-uuid>
```

This irreversibly removes the independent registry, managed credentials,
recovery records, and exports after verifying the exact installation identity.
It does not delete the main Kessoku database. Restore from a tested backup to
recover a purged identity.

## Acceptance before writes

The 2026-09-03 isolated exact-state exercise passed schema-v4 static-export
takeover and return to schema v5 without changing the registry SQLite digest,
plus certificate rotation and same-data force-recreate. Repeat the procedure
with production PKI, a stopped source host, tested backup restoration, and the
actual immutable release artifacts before treating it as production approval.

- Confirm schema v4/v5 and old/new capability behavior.
- Confirm the exact Starry generation, source/effective digest, schema digest,
  and every required subsystem ACK after apply and rollback.
- Every rollback requires administrator RBAC plus the exact
  `confirm:rollback:<instance-id>:<revision-id>` second-confirmation binding.
  This also protects a rollback that re-enables a previously active high-risk
  FastMedia configuration.
- Confirm the UI contains only aggregate Relay state and redacted endpoints.
- Run real reliable fallback, automatic FastMedia re-entry, dual-role Akari,
  NAT/UDP failure, rotation, backup/restore, and multi-host migration tests.
- Keep preview deployments opt-in and default-off. Hosted CI, security review,
  SBOM, signatures, provenance, and attestations gate preview publication;
  the real Akari/network/PKI matrix gates later stable promotion.

[简体中文](MIGRATION-v3.0.8.zh-CN.md)
