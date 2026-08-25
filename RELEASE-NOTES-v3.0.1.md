# Kessoku v3.0.1

v3.0.1 modernizes the embedded administration experience and adds scoped
enterprise administration while keeping the existing Kessoku/Starry runtime
architecture.

The unpublished `v3.0.0` tag remains an immutable failed-candidate record;
v3.0.1 is the first public v3 release and includes corrected generated API
documentation.

## Highlights

- Responsive light/dark administration UI for desktop, tablet, and phone,
  with the new Kessoku/StarryLinks brand assets.
- New `admin` tier scoped to selected user groups, users, public address books,
  and ID devices; the former unrestricted administrator is now `super_admin`.
- Scope edits, denials, role changes, and session revocation remain auditable.
- Repository-owned admin and browser frontends are built reproducibly from the
  same commit as the Go backend.
- Published scope remains Linux amd64 Docker, archive/binary, and DEB, with
  checksums, SBOMs, and provenance attestations.

## Breaking changes

- The Go module path is now `github.com/q1ngyang/rustdesk-api-kessoku/v3`.
  Downstream Go imports and module requirements must move from `/v2` to `/v3`.
- Database version 302 adds `users.role` and `admin_resource_scopes`. Existing
  `is_admin=true` accounts migrate to unrestricted `super_admin`; v3
  authorization uses `role`.
- The admin API now treats `role` (`user`, `admin`, `super_admin`) as the
  authoritative privilege field. A legacy `is_admin=true` write still means
  unrestricted `super_admin`; the RustDesk client payload reports
  `is_admin=true` only for `super_admin`.
- Do not run v2 against a migrated database without the documented rollback
  preparation: v2 can interpret scoped administrators as unrestricted.
- Changing an administrator's role or scope revokes that account's sessions.

Back up and rehearse the database upgrade before production. See the
[upgrade and rollback guide](docs/wiki/Upgrade-and-Rollback.md) and the
[v3 migration details](MIGRATION-v3.0.1.md).

[中文发布说明](RELEASE-NOTES-v3.0.1.zh-CN.md)
