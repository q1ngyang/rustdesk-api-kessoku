# Upgrade and rollback

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback)

Back up data and keys before changing an image or schema. The blocked v3.0.8
candidate keeps main database version `313` and adds an independent SP1
registry under `data/server-control`. Its container persistence, v3.0.7 binary
round trip, host migration, rotation, and purge procedure is the
[v3.0.8 migration guide](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.8/MIGRATION-v3.0.8.md).

The published Kessoku v3.0.7 advanced database version `312` to `313` for
Presence Lease v2. Returning from v3.0.7 to v3.0.6 is restore-only and requires
the matching pre-upgrade recovery set.

## Before an upgrade

Record the current state:

```sh
cd /opt/rustdesk-stack
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml images
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 120 \
  hbbs hbbr kessoku-api
```

Confirm:

- the target image supports `linux/amd64` and is an explicit release tag;
- release notes do not require new configuration, certificates, or migrations;
- a current, encrypted backup exists for the database, TOTP key, media,
  signing keys, Starry identity, configuration, and certificates;
- free disk space can hold both the backup and new image;
- the previous image tags or digests are recorded;
- a maintenance window exists for a real client test and possible rollback.

Do not use `latest` for a production upgrade.

## Back up the combined deployment

At minimum preserve:

```text
data/kessoku/
secrets/kessoku/
data/starry/
.env
compose.yaml
kessoku-config.yaml
starry-config.yaml
/etc/nginx/sites-available/rustdesk-stack.conf
/etc/letsencrypt/
```

Obtain a consistent SQLite copy by stopping the service briefly or using a
SQLite-aware snapshot. Store the backup outside the deployment directory and
verify that its files can be listed and read before proceeding.

## Upgrade Starry for network discovery

Before Kessoku v3.0.7, upgrade the center HBBS and its Control Agent to Starry
`1.1.16-patch-v1.2.2` when clients that are not signed in must be discovered,
or `1.1.16-patch-v1.3.0` when Presence Lease v2 activation verification is
required.
Relay-only nodes need no change for this capability. Preserve the Starry data,
identity key, instance ID, certificates, and service-JWT trust, then verify ID
registration, native peer-to-peer, forced Relay, and WSS.

## Upgrade Kessoku

For v3.0.6 to v3.0.7, validate the mounted configuration, inspect schema 312,
and migrate it to schema 313 before changing the running service:

```sh
docker compose --env-file .env -f compose.yaml run --rm kessoku-api \
  ./kessoku-api config validate --config /app/conf/config.yaml --json
docker compose --env-file .env -f compose.yaml run --rm kessoku-api \
  ./kessoku-api database status --config /app/conf/config.yaml --json
docker compose --env-file .env -f compose.yaml run --rm kessoku-api \
  ./kessoku-api database migrate --config /app/conf/config.yaml --json
```

The pre-migration state must be `upgrade_required` with
`migration_required: true`; after migration it must be `current` at schema
313. See the [v3.0.7 migration guide](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.7/MIGRATION-v3.0.7.md).

Edit `KESSOKU_IMAGE` in `.env` to the new explicit tag, then run:

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull kessoku-api
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 180 kessoku-api
```

After starting Kessoku, first verify:

- no configuration or database migration error appears;
- administrator and ordinary-user login work;
- address books and device data are present;
- logout and re-login work;
- one native and one forced-Relay session work;
- the built-in browser client works when enabled.

## Upgrade other Starry nodes

After Kessoku is stable, update any remaining Starry nodes. Change
`STARRY_VERSION`, then update HBBS first and HBBR second:

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull hbbs hbbr
docker compose --env-file .env -f compose.yaml up -d hbbs
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbs
docker compose --env-file .env -f compose.yaml up -d hbbr
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbr
```

Verify that `data/starry/id_ed25519.pub` is unchanged. Test ID registration,
native peer-to-peer, forced Relay, and WSS. Keep connection authentication in
`off` or `audit` during a major integration change; re-enable `enforce` only
after the expected client matrix passes.

## When to roll back

Rollback is appropriate when the new version cannot start, rejects the current
configuration, loses a required client path, or has an unacceptable functional
regression that cannot be corrected safely within the maintenance window.

Do not try to fix these conditions by deleting persistent data, regenerating
`id_ed25519`, disabling TLS verification, or publishing private ports.

## Image-only rollback

An image-only rollback is safe only when the new version did not modify the
database or persistent configuration in an incompatible way:

v3.0.7 to v3.0.6 does not meet this condition because v3.0.7 uses schema 313.
Stop every writer and use the database-rollback procedure with the complete
pre-upgrade recovery set.

1. restore the previous image tag or digest in `.env`;
2. run `docker compose config --quiet`;
3. recreate only the affected service;
4. inspect logs and run real client checks.

```sh
docker compose --env-file .env -f compose.yaml up -d kessoku-api
```

## Database rollback

If Kessoku migrated the database, restore the database and its matching TOTP
key, media directory, signing keys, and configuration from the pre-upgrade
backup before starting the old image. Keep the failed database copy for
diagnosis rather than overwriting it.

Restoring only `rustdeskapi.db` can break TOTP secrets and uploaded-image
references. Restoring only signing keys without the matching database can also
produce confusing token behavior. Treat them as one versioned recovery set.

For a rollback from v3 database `313` to v3.0.6 or any older image,
restore the complete matching recovery set. Never delete new tables or lower
the database version to force an older process to start.

## After recovery

Confirm container state, database contents, administrator login, ordinary-user
login, address books, native sessions, forced Relay, WSS, and browser sessions
as applicable. Record the restored image versions and backup timestamp, then
investigate the failed upgrade without changing the recovered production data.
