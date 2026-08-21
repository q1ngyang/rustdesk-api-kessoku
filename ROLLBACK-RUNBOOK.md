# Kessoku Rollback Runbook

Rollback is an incident/change-control operation. Rehearse this sequence with
each deployed database engine and the pinned Starry release candidate before
enabling connection enforcement or configuration writes.

## Critical compatibility warning

Version 301 is additive, but newly issued Kessoku token rows deliberately leave
the legacy plaintext `token` column empty. An older rustdesk-api binary that
reads only that column cannot authenticate those sessions. Therefore:

- before any Kessoku token has been issued, an application rollback may be
  possible against the additive schema only after a rehearsal;
- after Kessoku has issued tokens, do not point an old binary at the upgraded
  live database;
- restore the verified pre-upgrade database backup that matches the old binary,
  or keep the Kessoku binary and remediate forward;
- expect sessions created after the backup to be lost and require re-login.

Never attempt to repopulate plaintext bearer tokens: Kessoku does not retain
the originals and hashes cannot be reversed.

## Ordered rollback

1. Declare the incident/change rollback, identify the actor, affected instance,
   current database/image versions, current Starry generation, and time.
2. Change Starry connection authentication from `enforce` to `audit`. Keep
   would-deny telemetry. Use `off` only under separately authorized emergency
   policy.
3. Set Kessoku `server-control.read-only: true` and restart it. Confirm write
   endpoints return `CONTROL_READ_ONLY`.
4. Through the Agent, restore the approved last-known-good Starry generation.
   Verify the active generation and actual process health; do not rely only on
   the rollback HTTP response.
5. Preserve Kessoku and Starry audit records, logs, operation IDs, request IDs,
   image digests, and configuration digests without copying secrets.
6. Decide whether to remediate forward or restore the previous Kessoku binary
   and database as a matched pair.
7. If restoring, stop Kessoku writes, take a forensic snapshot of the current
   database, restore the already verified pre-upgrade backup to an isolated
   target, validate it, and then switch the old binary to that restored target.
8. Keep JWT current/previous keys independent of application images. Do not
   accidentally replace them with keys baked into an old deployment artifact.
9. Validate administrator login, ordinary API login, logout, native/WSS audit
   behavior, database row counts, and absence of legacy command routes.
10. Communicate the re-login window and revoke sessions according to incident
    scope before returning to normal operation.

## Key-rotation rollback

If a new signing key is faulty but not compromised, restore the prior private
key as current only while its public key is still trusted and its custody is
known. Continue publishing any key that signed an unexpired token. If either key
is suspected compromised, do not restore it: replace it, revoke affected
sessions, and force re-login.

The access-token and Control Agent keyrings must remain separate throughout a
rollback. Restoring one configuration must not overwrite or alias the other.

## Validation before closure

- Starry is in the intended `audit`/`enforce` mode and never silently fail-open.
- Kessoku control writes are read-only outside an approved window.
- Live Starry generation matches the recorded target.
- Application binary and database backup are a tested compatible pair.
- Current/previous key ownership and expiry windows are documented.
- A revoked/disabled user's credential is rejected.
- Audit evidence contains no token, private key, certificate, or complete YAML.
- Root cause and forward-fix criteria are recorded before enforcement resumes.
