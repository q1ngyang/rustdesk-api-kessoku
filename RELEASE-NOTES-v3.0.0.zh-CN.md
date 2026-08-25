# Kessoku v3.0.0 发布说明

v3.0.0 在延续现有 Kessoku/Starry 技术架构的基础上，重构内置管理界面，并加入适合企业
使用的范围管理员权限。

## 主要更新

- 全新的响应式明暗主题管理界面，改善桌面、平板和手机体验，并使用新的
  Kessoku/StarryLinks 品牌资源。
- 新增 `admin` 层级，可限定管理指定用户组、用户、公共地址簿和 ID 设备；原无限制
  管理员调整为 `super_admin`。
- 范围变更、越权拒绝、角色变化和会话撤销均保留审计记录。
- 管理前端和浏览器前端继续与 Go 后端同仓库、同提交、可复现构建。
- 正式制品仍为 Linux amd64 Docker、archive/binary 与 DEB，并附 checksum、SBOM 和
  provenance attestation。

## 破坏性变更（Breaking Changes）

- Go module 路径变更为 `github.com/q1ngyang/rustdesk-api-kessoku/v3`；下游 Go
  import 和 module requirement 必须从 `/v2` 迁移到 `/v3`。
- 数据库版本升级为 302，新增 `users.role` 和 `admin_resource_scopes`。原
  `is_admin=true` 账号自动迁移为无限制 `super_admin`，v3 授权以 `role` 为准。
- 管理 API 以 `role`（`user`、`admin`、`super_admin`）作为权威权限字段；旧式
  `is_admin=true` 写入仍表示无限制 `super_admin`，RustDesk 客户端响应只对
  `super_admin` 返回 `is_admin=true`。
- 未按回退文档预处理时，不得让 v2 直接读取迁移后的数据库，否则范围管理员可能被
  v2 识别为全局管理员。
- 管理员角色或范围发生变化后，该账号的现有会话会被撤销。

生产升级前必须备份数据库并演练。详细步骤见
[升级与回滚 Wiki](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)和
[v3 数据库迁移说明](MIGRATION-v3.0.0.zh-CN.md)。

[English release notes](RELEASE-NOTES-v3.0.0.md)
