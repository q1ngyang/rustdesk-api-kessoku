# Kessoku v3.0.8 release notes

> Publication status: **PREVIEW_APPROVED**. v3.0.8 is published as a GitHub
> prerelease plus immutable `v3.0.8` and moving `preview` images. It does not
> replace the stable v3.0.7 Release or `latest` image.

v3.0.8 extends the frozen v3.0.7 baseline with typed management and aggregate
observation for Starry's frozen v1.3.1 FastCompat/FastMedia and Pairing v1
contracts. The implementation is pinned to the immutable Starry
`1.1.16-patch-v1.3.1` preview and its byte-identical runtime release summary in
this directory; it never infers capabilities from a version string. The wire
contract itself remains frozen at commit
`6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`.

## Relay FastCompat and FastMedia

- Known capability versions are exact: `fast_relay_authorization=1`,
  `fast_media_relay_udp=1`, and `config_schema=5`. Missing capabilities render
  as **Unsupported**; an unknown known-capability version fails closed.
- Schema v5 is still rendered from Starry's returned JSON Schema/UI Schema.
  Kessoku does not duplicate the Relay configuration specification.
- `fast_compat_enabled` and `fast_media_v1_enabled` are independent and default
  off. FastMedia enablement is rejected unless current authenticated telemetry
  proves at least one matching, fresh, healthy UDP-capable Relay candidate.
- Every `/fast_mode` plan has at least medium risk. FastMedia enablement and
  UDP endpoint, datagram, bitrate, or related firewall changes are high risk,
  require exact confirmation, administrator RBAC, before/after audit bindings,
  schema/generation digests, and accepted subsystem activation ACKs.
- Relay views expose only typed, redacted aggregate state: capability and
  telemetry freshness, candidate eligibility, bind outcomes, cookie/grant and
  role/session/allocation rejection classes, rebind, forward/drop/rate-limit,
  expiry, and reliable fallback counts. A server event is never presented as
  proof that an individual client used FastMedia.

Starry patch-v1.2 and v1.3.0 instances continue to support their existing
Relay and Relay Quality pages. New configuration writes are capability- and
schema-gated, so unsupported targets cannot receive schema-v5 FastMedia fields.

## SP1 Broker and Relay enrollment

- The Control Agent Broker accepts only deployment-allowlisted HTTPS origins.
  A browser cannot submit an Agent URL or callback.
- Codes are short-lived and single-purpose. Only a SHA-256 secret digest is
  retained. Claims bind the exact purpose/action, configuration digest, CSR
  public key, and instance identity, with same-key recovery after response loss.
- An unclaimed Control Agent code can be revoked by enrollment ID in the UI or
  CLI without sending the raw code back to Kessoku, then safely recreated.
- Each managed instance receives independent mTLS client credentials and a
  service-JWT Ed25519 key. The learned Starry instance UUID is locked, and every
  new pairing or rotation returns to Kessoku read-only mode.
- Relay enrollment is prepared and completed by an authenticated Starry Agent.
  `activate_after_health` is immutable high-risk preauthorization at code
  creation; otherwise activation remains pending explicit approval and exact
  generation/health evidence.
- Kessoku never stores the raw code, Relay private key, telemetry secret,
  signed grant, client address, session/allocation UUID, or media content.
  Kessoku is not in the Relay data plane, and stopping it does not stop remote
  control sessions.

Pairing data is independently versioned at
`data/server-control/registry-v1.sqlite`; private files under
`data/server-control/instances` are owner-only. The main database remains
schema 313. A static `server-control.instances[]` export is refreshed for every
managed instance so v3.0.7 can take over after an explicit manual merge.
Pairing requires an explicit host identity file. Compose mounts the host's
`${KESSOKU_HOST_IDENTITY_FILE}` at `/run/kessoku-host-machine-id`, while
systemd uses `/etc/machine-id`; only its domain-separated SHA-256 fingerprint
is stored. A copied registry fails closed on a different host until an operator
confirms the exact installation ID with `registry adopt-host`.

## Upgrade and compatibility

The database schema does not change from v3.0.7. Preserve the complete
`KESSOKU_DATA_DIR`, especially `server-control/`, across image pulls,
force-recreate, and down/up. v3.0.7 ignores the managed registry and must not
modify it. Re-upgrading to v3.0.8 recovers registry generation and credentials.

See [the migration guide](MIGRATION-v3.0.8.md) for container, systemd,
backup/restore, host-adoption, rotation, rollback, and explicit purge steps.

## Exact-state diagnostic validation

The isolated candidate exercise on 2026-09-03 closed the local half of several
previously interlocked gates:

- full Go tests, race tests, vet, generated-API stability, documentation and
  release-identity checks passed; the administration frontend passed lint, all
  33 tests, zero-vulnerability/signature audit, two byte-identical production
  builds, and production-dependency SBOM licensing checks; the WebClient passed
  lint/typecheck, all 63 tests, the same audit/signature and reproducible-build
  gates, and a complete 62-component licensed SBOM;
- the migration fixture passed against SQLite plus the CI-pinned PostgreSQL
  16.4 and MySQL 8.4.2 images, including schema inspection, maintenance
  recovery, and dialect-specific migration locking;
- `golang.org/x/crypto` 0.56.0 passed module verification, vet, full tests and
  full race tests; current `govulncheck` reports zero reachable and zero
  imported-package vulnerabilities. One module-only notice remains for the
  unmaintained `openpgp` package, which Kessoku does not import;
- a missing rollback confirmation was rejected with HTTP 428 without changing
  generation, ETag, or history; the exact revision-bound confirmation then
  completed with accepted schema/generation digests and all subsystem ACKs;
- an SP1 Control certificate rotation, Agent restart, Kessoku force-recreate,
  and provider hot reload preserved the same independent registry at schema 1,
  generation 30, with owner-only credentials and read-only managed writes;
- the same persistent state ran Kessoku v3.0.7 with Starry v1.3.0/schema 4 via
  its generated static export, returned HTTP 200 for status/capabilities/Relay
  inventory, left the registry byte-identical, and returned to
  v3.0.8/v1.3.1/schema 5 with fresh healthy UDP telemetry;
- the outward v3.0.8 Relay inventory contained none of the forbidden process,
  allocation, or session UUID, address, token, nonce, grant, private-key, or
  media field names; and
- a protocol-level harness using the current Akari FastMedia library in both
  roles and the exact HBBR candidate retained reliable TCP, fell back after a
  UDP probe timeout, and re-entered FastMedia in the same session.

These results establish preview readiness when combined with the protected
clean-commit candidate and publication workflows. They do not replace a clean
immutable Akari build, full HBBS signalling/GUI clients, real devices and
networks, or production PKI/multi-host evidence required for stable promotion.

## Preview boundary and stable blockers

The Kessoku preview requires the immutable Starry preview/runtime provenance
and its own clean-commit hosted security, build, package, SBOM, provenance, and
attestation gates. It deliberately does not wait for an Akari release. The
following remain stable-promotion gates and may drive later patch revisions:

- a clean immutable Akari candidate and real two-client Akari dual-role
  HBBS/HBBR end-to-end run across Native, WSS, and mixed signalling;
- reliable fallback and automatic re-entry on those real clients, rather than
  only the passing protocol-level same-session harness;
- the real-device NAT/AP-change/UDP-block/loss/overload/rebind/restart and
  sustained media/thermal soak matrix;
- production-PKI response-loss recovery, certificate and enrolled-Relay
  rotation, stopped-source backup/restore and multi-host migration/clone
  drills; and
- sustained production observation of the preview packages and images.

[简体中文](RELEASE-NOTES-v3.0.8.zh-CN.md)
