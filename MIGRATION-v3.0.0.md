# RustDesk API Kessoku v3.0.0 database migration and rollback

v3.0.0 moves the database from version 301 to 302. The migration is additive,
but the privilege model changes; rehearse the upgrade and rollback against a
restored production backup.

## Version 302

Version 302 adds `users.role` (`user`, `admin`, or `super_admin`) and
`admin_resource_scopes` for user-group, user, public-address-book, and ID-device
grants. Existing `is_admin=true` users become `super_admin`; other users become
`user`. The `is_admin` column remains a v2 compatibility mirror, while v3
authorization uses only `role`.

## Upgrade

1. Stop writes, record the current database version and administrators, and
   create a verified backup.
2. Start v3.0.0 against a restored copy and verify database version 302.
3. Confirm every former administrator is `super_admin` and at least one
   enabled super administrator remains.
4. Test an `admin` first with no grants, then with each of the four scope types;
   verify list filtering and rejection of out-of-scope batch operations.
5. Confirm role and scope changes revoke existing sessions.
6. Repeat the verified procedure in the production maintenance window.

Useful read-only checks:

```sql
SELECT id, username, role, is_admin, status FROM users ORDER BY id;
SELECT admin_user_id, scope_type, scope_id FROM admin_resource_scopes ORDER BY admin_user_id, scope_type, scope_id;
```

## Rollback warning

Restoring the full pre-upgrade backup is the preferred rollback. Do not start
v2.8.x directly against version 302: v2 recognizes only `is_admin`, whose
compatibility mirror is true for both scoped and super administrators. If a
temporary in-place rollback is unavoidable, stop every v3 instance, back up
the version 302 database, then run:

```sql
UPDATE users SET is_admin = FALSE WHERE role = 'admin';
```

Only then start v2.8.x. Never allow v2 and v3 to write the same database at the
same time. Back up again and verify every `role` before returning to v3.

[简体中文](MIGRATION-v3.0.0.zh-CN.md)
