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

1. 部署 Kessoku v2.8.1，认证关闭且控制只读。
2. 验证数据库版本 301、OAuth 身份索引、最后管理员不变量与旧 token 迁移。
3. 开启 EdDSA 签发；需要时使用有界兼容重叠期。
4. 上线内部 mTLS JWKS/introspection。
5. 升级 Starry，连接认证先 `off`，再 `audit`。
6. Control Agent 先只读上线。
7. 完成真实客户端 audit 和 staging 回滚测试。
8. 在独立 HTTPS origin 上上线 Web Client，验证公共 profile、ready/grant/ack 交接、
   forced-Relay VP9 会话、grant 到期与 logout；任一失败都保持
   `web-client.mode: disabled`。
9. 小范围开启 `enforce`；配置写入只能在单独批准窗口开启。

## 回滚警告

v2.8.1 新凭据不会填写历史明文 token 列。旧应用无法重建或认证它们。v2.8.1 一旦签发
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

详细流程见 [`MIGRATION.md`](../../MIGRATION.md)和
[`ROLLBACK-RUNBOOK.md`](../../ROLLBACK-RUNBOOK.md)。
