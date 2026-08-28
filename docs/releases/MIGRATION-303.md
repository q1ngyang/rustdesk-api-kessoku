# Database version 303 migration

Version 303 is an additive migration for centralized branding, encrypted TOTP
two-factor authentication, and login challenges. It creates
`branding_settings`, `user_two_factors`, and
`two_factor_login_challenges`; existing users, roles, tokens, devices, and
audit data are preserved.

Before upgrading, stop writes and make a consistent database backup. Back up
the whole Kessoku data volume. With `two-factor.enabled: true`, first startup
creates `/app/data/totp.key` (or the configured `two-factor.key-file`) with
mode `0600`. Future backups must always contain both the database and this
file. Never copy the key into YAML, logs, an image, or the database.

After startup, verify:

- the latest database version row is `303`;
- the three new tables exist;
- `/app/data/totp.key` is a regular 32-byte mode-0600 file;
- `/app/data/media` is writable only by the service account;
- existing native-client login, system-information reporting, admin login,
  and Web Client grant handoff still work;
- one test account can enable TOTP, is logged out everywhere, and can complete
  the official two-stage RustDesk login flow.

Rollback after version 303 requires stopping every new instance and restoring
the matching pre-upgrade database backup. Do not delete the new tables in
place. Preserve the TOTP key with the version-303 backup even if rollback is
temporary.

## 中文说明

数据库版本 303 是增量迁移，新增集中品牌配置、加密保存的 TOTP 双重验证和登录挑战表，
不会删除现有用户、角色、令牌、设备或审计数据。升级前停止写入并一致性备份数据库；启用
双重验证后，备份必须同时包含数据库和 `/app/data/totp.key`。该文件为 32 字节、权限
`0600`，不得写入 YAML、日志、镜像或数据库。

升级后确认最新版本行为 `303`、三个新表存在、数据目录可写，并依次验证原生客户端登录、
系统信息上报、后台登录、WebClient 授权，以及测试账户启用 TOTP 后的全会话撤销和官方
两阶段登录。回退时必须停止新版实例并恢复升级前的完整数据库备份，不要原地删除新表。
