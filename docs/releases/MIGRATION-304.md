# Database version 304 migration

Version 304 adds the deployment announcement to the existing centralized
branding record. The migration is additive and does not change users, devices,
tokens, TOTP secrets, or Starry configuration.

Before upgrading, stop all Kessoku writers and back up the database. Start one
3.0.x instance first and confirm that the latest `versions.version` value is
`304` and that `branding_settings.announcement` exists. The column defaults to
an empty string, so deployments continue using `admin.hello` until an
announcement is saved. Database version 305 subsequently moves announcement
editing to System settings while preserving this legacy column for upgrade
compatibility.

## 中文说明

数据库版本 304 新增公告字段。该迁移仅增加字段，不修改用户、设备、
令牌、TOTP 密钥或 Starry 配置。升级前请停止所有 Kessoku 写入实例并备份数据库；先启动
一个新实例，确认最新数据库版本为 `304` 且 `branding_settings.announcement` 已创建。
公告为空时仍会使用原有 `admin.hello` 配置。数据库版本 305 会把公告编辑入口迁移到
“系统设置”，同时保留该旧列以兼容升级。
