# Kessoku v3.0.7

v3.0.7 为 S6 管理的 `starrydesk` 大镜像提供稳定本地维护接口，并新增用于多 Profile
快速、安全切换的 Presence Lease v2。救援接口仍只在本地提供；现有 heartbeat API 和旧
客户端保持兼容。

## 稳定本地接口

- `kessoku-api version [--json]` 不依赖配置、数据库或密钥，输出组件版本、数据库 schema、
  源码 revision、构建时间与 Go 版本。
- `kessoku-api config validate --config PATH [--json]` 使用正式配置解析规则，只读检查 URL、
  端口、路径、证书、私钥权限与密钥隔离，不连接数据库也不写本地状态。
- `kessoku-api database status --config PATH [--json]` 只读检查 schema；`database migrate`
  获取数据库专用排他锁，复用服务启动的同一迁移实现，并在返回前验证最终版本标记。
- `maintenance recover-admin` 把一个经精确二次确认的账户恢复为启用的 `super_admin`；
  `maintenance reset-2fa` 只清除目标账户的 TOTP 状态。两者都会撤销会话、只提升一次
  `auth_version` 并写专用安全审计。

所有 JSON 使用 snake_case 字段和 `schema_version: 1`。固定退出码区分参数、配置、连接、
schema 与维护失败。完整约定见[本地维护 CLI 参考](../../operations/LOCAL-MAINTENANCE-CLI.zh-CN.md)。

## Presence Lease v2

- `start` 先验证完全一致的设备 ID、Profile 独立 `network_identity_uuid`、activation 和
  当前 Starry 路由，再返回随机 lease ID、256 位 bearer token 与 45 秒到期时间；客户端
  activation ID 原样回传，本地 `profile_id` 不接收也不保存。
- `renew` 和 `end` 只选择 token 绑定的一条租约；新客户端同时发送 lease ID 与 token。
  数据库只保存 token 的 SHA-256 哈希；日志和指标都不含 token。
- 当前 activation 中任一 lease 有效即在线。旧 activation 的延迟请求不能结束新
  activation；客户端崩溃后由 TTL 自动离线。
- 不同网络身份 UUID 不能覆盖彼此的设备 ID、账户归属、资料或在线状态。迁移遇到重复
  设备 ID 时会拒绝，而不是自动合并。
- 超级管理员指标端点只暴露有界低基数计数器和数据库级 gauge，并有明确告警阈值且不含
  凭据。详见 [Presence Lease v2 运维说明](../../operations/PRESENCE-LEASE-V2.zh-CN.md)。

## 安全与兼容性

数据库从 schema `312` 增量升级到 `313`。迁移会创建 presence lease 存储和 peer
activation/聚合字段，并在只读重复预检通过后强制设备 ID 唯一。默认服务启动及所有数据库/
维护命令都会拒绝高于当前二进制的 schema。

恢复密码只能来自 owner-only 普通文件；读取时拒绝符号链接并检查文件替换竞态。必须且只能
使用一个用户标识选择目标，再用数据库中完全一致的用户名二次绑定。角色、状态、scope 清理、
可选密码/TOTP 修改、挑战清理、Token 撤销与成功审计完成都在同一数据库事务中。审计元数据
不包含密码、哈希、Token 或 TOTP 数据。

SQLite、MySQL 8.4.2 与 PostgreSQL 16.4 集成用例覆盖 schema 检查、迁移串行化、管理员
恢复、幂等 2FA 重置、会话撤销、审计完成及 schema-313 租约结构。Presence start 要求
Starry `1.1.16-patch-v1.3.0` 和 peer-registry capability 2。
仓库内逐字节一致的 [Starry 发布摘要](STARRY-RELEASE-SUMMARY.json)固定了其源码提交、
镜像 index/平台 digest、Control OpenAPI、配置/UI schema、冻结的 Relay Quality 协议及
遥测 schema。

## 升级与回退

部署前阅读[迁移与回退指南](MIGRATION-v3.0.7.zh-CN.md)。v3.0.6 不理解 schema 313，
因此只能恢复回退：停止全部写入者并恢复匹配的完整升级前备份，之后才能启动 v3.0.6。

[English](RELEASE-NOTES-v3.0.7.md)
