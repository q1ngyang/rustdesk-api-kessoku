# 更新日志

Kessoku 的重要变化记录于此；版本专用运维细节继续保存在
[`docs/releases/`](docs/releases/) 中。

[English](CHANGELOG.md)

## v3.0.8 预览版 — 2026-09-04

- 新增精确 Starry v1.3.1 schema-v5 FastCompat/FastMedia capability、Relay UDP
  强类型聚合、依赖门禁写入、风险下限、计划审计绑定、generation/schema 摘要和 activation
  ACK 校验。
- 保留 patch-v1.2/v1.3.0 Relay 兼容和 schema 驱动表单；capability 缺失显示“不支持”，
  已知 capability 的未知版本关闭失败。
- 新增 allowlist 限定的 SP1 Control Agent 和 Agent 授权 Relay enrollment Broker、CLI 与
  管理界面，并支持按 enrollment ID 撤销和重建未领取的 Control Agent code。
- 新增独立 schema-v1 SQLite registry、每实例属主专用 mTLS/JWT 凭据、CSR/幂等恢复绑定、
  UUID 锁定、Provider 热加载、v3.0.7 静态导出、主机克隆检测、轮换和显式确认 purge；
  主业务数据库仍为 schema 313。
- 所有 rollback 现在都要求管理员 RBAC 和精确绑定 revision 的二次确认，包括会重新启用
  FastMedia 的历史 revision；HTTP 服务收到 SIGTERM 时会正常 drain 并以 0 退出。
- 将 `golang.org/x/crypto` 从 0.55.0 更新到 0.56.0，移除两项未导入 SSH DoS module
  advisory；`govulncheck` 为 0 可达、0 imported-package 漏洞。
- 隔离精确状态验证已证明 Control 证书轮换、force-recreate、schema-v4 静态导出接管及返回
  schema v5 时会保留 registry generation 与凭据；这只是诊断证据，不代表发布批准。
- 固定 Starry 不可变运行时并通过 Hosted 候选检查后，已批准预览发布。真实
  Akari/客户端/NAT/fallback、生产迁移、PKI 与 soak 证据仍是稳定版晋级门禁，详见
  [发布说明](docs/releases/v3.0.8/RELEASE-NOTES-v3.0.8.zh-CN.md)。

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
