# 更新日志

Kessoku 的重要变化记录于此；版本专用运维细节继续保存在
[`docs/releases/`](docs/releases/) 中。

[English](CHANGELOG.md)

## v3.0.7 — 2026-08-31

- 新增版本、配置校验、数据库状态与显式迁移的稳定人工/JSON 本地接口。
- 新增 SQLite/MySQL/PostgreSQL 串行迁移锁与未来 schema 拒绝保护。
- 新增带安全审计的事务化管理员恢复，以及按用户幂等重置 2FA；同时提升认证代际并撤销会话。
- 新增 Presence Lease v2：多 Profile 快速切换、任一有效 lease 聚合在线、崩溃 TTL 回收，
  并保持旧 heartbeat 客户端兼容。
- 数据库从 schema 312 增量升级到 313，新增只保存 token 哈希的租约、不可变网络身份绑定
  和重复设备 ID 预检。
- HTTP/配置兼容，历史密码重置命令继续可用。

## v3.0.6 — 2026-08-29

- 完成原生/网络设备安全发现及完整资料刷新。
- 新增 schema 312 设备身份与会话元数据。

完整历史范围见 [v3.0.6 发布说明](docs/releases/v3.0.6/RELEASE-NOTES-v3.0.6.zh-CN.md)。
