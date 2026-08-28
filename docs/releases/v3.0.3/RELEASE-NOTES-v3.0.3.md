# Kessoku v3.0.3

v3.0.3 is the first published and supported Kessoku v3 release. The earlier
v3.0.1 GitHub Release was withdrawn after significant administration-console,
client-report, and WebClient integration defects were confirmed. The v3.0.2
tag records a failed, unpublished release attempt and has no supported release
assets or container image. Do not deploy either version to a new environment;
existing v3.0.1 installations should back up their database, keys, media
directory, and configuration before upgrading to v3.0.3.

Kessoku remains an unofficial RustDesk account, administration, and policy
plane. It integrates with the pinned
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
Control API and ships its administration console and Relay-only browser client
from the same reviewed source commit.

## Highlights

- Restored complete workspace and system-management navigation, corrected
  responsive dialogs and tables, and improved desktop, tablet, mobile,
  light-mode, and dark-mode presentation.
- Fixed RustDesk client inventory/report ingestion so device identity, host,
  user, operating-system, CPU, memory, UUID, and version data populate the
  administration views. Login, connection, file, share, and WebClient audit
  ownership are handled consistently.
- Reworked WebClient authentication for a distinct HTTPS origin, persistent
  signed-in state, one-time administration handoff, and connection auditing.
  The client adds password visibility, connection-state-aware controls, remote
  hostname display, theme/language synchronization, and a compact assistance
  chat interface. The browser transport remains forced Relay WSS with VP9.
- Added centralized, theme-aware branding: one light/dark logo and icon set for
  sign-in, administration, About, and WebClient; separate light/dark sign-in
  and WebClient backgrounds; shared footer HTML; custom copy, HTML, CSS, URLs,
  uploads, previews, and bundled StarryDesk defaults. The Server Control
  StarryLinks identity remains fixed.
- Added encrypted TOTP two-factor authentication, account avatars with crop
  and scale handling, personal profile editing, session revocation, and saved
  language/theme preferences. Japanese joins the existing interface languages.
- Expanded typed Starry Server Control with clearer status/help indicators,
  capability-safe configuration plans, complete schema-driven YAML editing,
  audit history, Kessoku/Starry/Relay log views and export, and guarded Kessoku
  runtime log-level control in writable mode.
- Added administrator-managed announcements and GeoLite2 Country/City/ASN
  sources, scheduled MMDB refresh, explicit update feedback, and reusable IP
  detail popovers across activity views.
- Removed LinuxDo OAuth support and its stored provider bindings. GitHub,
  Google, LDAP, and standards-based OIDC remain available when configured.
- Publishes Linux amd64 container, archive/binary, and DEB artifacts with
  checksums, frontend and source SBOMs, and Sigstore provenance attestations.

## Compatibility and breaking changes

- The Go module path is `github.com/q1ngyang/rustdesk-api-kessoku/v3`.
  Downstream Go imports and requirements must use `/v3`.
- Database version `309` includes the v3 role model plus additive migrations
  for branding, TOTP, login challenges, announcements, GeoIP policy, user
  presentation preferences, themed assets, shared footer content, and
  WebClient audit ownership.
- Existing `is_admin=true` accounts first migrate to unrestricted
  `super_admin`; v3 authorization uses `role` (`user`, `admin`, or
  `super_admin`). Never let a v2 and v3 process write the same database.
- With two-factor authentication enabled, `/app/data/totp.key` (or the
  configured `two-factor.key-file`) is required to decrypt enrolled TOTP
  secrets. Back it up with the database and never regenerate it for an
  existing deployment.
- Uploaded branding and avatar images live under `media.directory`, normally
  `/app/data/media`; include that directory in backup and rollback sets.
- WebClient built-in mode requires exact, different HTTPS
  `web-client.public-origin` and `web-client.api-origin` values. Cookies are
  deliberately not shared between unrelated domains; Kessoku uses a scoped
  API-origin browser session plus authenticated preference synchronization.
- LinuxDo provider configuration and bindings are unsupported and removed
  during migration. Review affected identities before upgrading if that
  provider was previously in use.
- Do not downgrade in place from database version 309. Stop all writers and
  restore the matching pre-upgrade database, TOTP key, media, configuration,
  and signing keys instead.

Back up and rehearse the upgrade before production. See the
[upgrade and rollback guide](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback)
and [v3.0.3 migration details](MIGRATION-v3.0.3.md).

[简体中文](RELEASE-NOTES-v3.0.3.zh-CN.md)
