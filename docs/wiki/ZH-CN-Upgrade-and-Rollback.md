# 升级与回滚

[English](Upgrade-and-Rollback.md) | **简体中文**

## 维护窗口前

Kessoku 不承诺统一的 RTO 或 RPO。部署本版本前，部署负责人必须记录责任人、维护窗口、
本地 RTO/RPO、备份保留期、回滚授权人与 go/no-go 决策。它是部署门禁，而不是软件发布
门禁。

记录当前 Kessoku/Starry 镜像 digest、source/contract 版本、数据库版本、活动 Starry
generation、key ID 和客户端矩阵。备份并实际恢复验证：

- 数据库；
- Kessoku access-token current/previous key；
- 内部 mTLS PKI 与 Control Agent 凭据；
- Kessoku/Starry 配置及审计/provenance；
- 旧镜像/包。

切换镜像前，外部 MySQL 配置 `tls: "true"`（私有 PKI 再配置 `ca-file`），或 PostgreSQL
配置 `sslmode: "verify-full"`（需要时配置 `ssl-root-cert`）。用精确数据库 DNS 名称检查证书
SAN，并运行 [`MIGRATION.md`](../../MIGRATION.md)中的 OAuth 身份重复/空值查询。遇到外部
身份冲突时 Kessoku 会停止，不会猜测合并方式。

## 升级顺序

1. 部署 Kessoku v3.0.0，认证关闭且控制只读。
2. 验证数据库版本 302、原管理员均为 `super_admin`、至少保留一个启用超管，以及版本
   301 的 OAuth/token 不变量。
3. 验证空范围 `admin` 看不到企业资源，再依次测试用户组、用户、公共地址簿与 ID 设备授权。
4. 确认角色或范围变化会撤销管理员现有会话。
5. 开启 EdDSA 签发；需要时使用有界兼容重叠期。
6. 上线内部 mTLS JWKS/introspection。
7. 升级 Starry，连接认证先 `off`，再 `audit`，Control Agent 先只读上线。
8. 完成真实客户端 audit 和 staging 回滚测试。
9. 在独立 HTTPS origin 上上线 Web Client，验证公共 profile、ready/grant/ack 交接、
   forced-Relay VP9 会话、grant 到期与 logout；任一失败都保持
   `web-client.mode: disabled`。
10. 小范围开启 `enforce`；配置写入只能在单独批准窗口开启。

## 回滚警告

版本 302 为兼容数据库保留范围管理员和超管的 `is_admin=true` 镜像，因此 v2 可能把范围
管理员提升为无限制管理员。首选恢复完整的升级前备份；如必须原地回退 v2，应先按
[`MIGRATION-v3.0.0.zh-CN.md`](../../MIGRATION-v3.0.0.zh-CN.md)预处理，且禁止 v2/v3
同时写入。

v3.0.0 新凭据不会填写历史明文 token 列。旧应用无法重建或认证它们。v3.0.0 一旦签发
token，回滚旧应用必须使用匹配且经过验证的升级前数据库备份；备份之后创建的会话需要
重新登录。

## 有序回滚

1. 在变更控制下将 Starry 从 `enforce` 退回 `audit`。
2. 把 Kessoku 与 Control Agent 设为只读。
3. 设置 `web-client.mode: disabled`，撤销活动 connection grant，并确认 21122 公共 origin
   不再提供客户端；无需恢复任何历史浏览器资产。
4. 恢复并验证 Starry last-known-good generation。
5. 保留脱敏证据，并决定向前修复还是匹配恢复程序/数据库。
6. 把批准的数据库备份恢复到隔离目标，验证后再切换旧应用。
7. 验证管理/API 登录、token 失效、native/WSS audit、数据库行数以及不存在通用命令路由。

详细流程见 [`MIGRATION-v3.0.0.zh-CN.md`](../../MIGRATION-v3.0.0.zh-CN.md)、
[`MIGRATION.md`](../../MIGRATION.md)和
[`ROLLBACK-RUNBOOK.md`](../../ROLLBACK-RUNBOOK.md)。
