# Database version 305 migration

Version 305 creates the singleton `system_settings` record used for workspace
announcements and GeoIP source/update policy. When upgrading from version 304,
Kessoku copies a non-empty legacy branding announcement once, without changing
brand assets. The legacy column remains in place for rollback compatibility,
but new edits are made only in **System settings**.

LinuxDo is no longer a supported OAuth provider. The same migration deletes
all `oauths` and `user_thirds` rows whose provider type or provider code is
`linuxdo`. It does not delete local users, devices, user sessions, TOTP secrets,
other OAuth/OIDC providers, or Starry configuration.

Before upgrading, stop all Kessoku writers and make a consistent database
backup. Start one upgraded instance, then verify:

1. the latest `versions.version` is `305`;
2. `system_settings` contains the singleton row with ID `1`;
3. the announcement appears under **System settings** and on Overview;
4. no `linuxdo` rows remain in `oauths` or `user_thirds`; and
5. the configured MMDB download succeeds before relying on IP hover details.

Rollback requires the matching pre-upgrade database backup if LinuxDo identity
bindings must be recovered. Do not reinsert those rows into a running version
305 deployment.

## 中文说明

数据库版本 305 新增单例 `system_settings` 记录，用于工作区公告和 GeoIP 数据源、更新
策略。由 304 升级时，Kessoku 只会将品牌表内非空的旧公告迁移一次，不修改品牌图片；旧
字段为回退兼容而保留，后续公告只在“系统设置”中编辑。

LinuxDo 已不再是受支持的 OAuth 登录方式。本次迁移会删除 `oauths` 和 `user_thirds` 中
提供商类型或代码为 `linuxdo` 的记录，但不会删除本地用户、设备、用户会话、TOTP 密钥、
其他 OAuth/OIDC 提供商或 Starry 配置。

升级前应停止所有 Kessoku 写入实例并创建一致性数据库备份。启动一个新版实例后，确认
最新数据库版本为 `305`、`system_settings` 中存在 ID 为 `1` 的记录、公告已显示在系统
设置和概览页、数据库中不再存在 LinuxDo 记录，并在依赖 IP 悬浮信息前验证 MMDB 下载。
如需恢复 LinuxDo 绑定，必须使用升级前的匹配数据库备份回退，不能向运行中的 305
数据库重新插入这些记录。
