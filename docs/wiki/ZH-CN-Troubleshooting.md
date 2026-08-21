# 常见问题排查

[English](Troubleshooting.md) | **简体中文**

| 现象 | 检查证据 | 安全处理 |
| --- | --- | --- |
| 容器启动退出 | 配置校验、密钥/证书路径、文件权限、数据库连接 | 修复部署输入，不要关闭校验。 |
| 启动拒绝 MySQL/PostgreSQL 传输 | MySQL `tls`/`ca-file`、PostgreSQL `sslmode`/`ssl-root-cert`、数据库 DNS 名称与证书 SAN | 使用 MySQL `true` 或 PostgreSQL `verify-full`，只读挂载已审核 CA，并修复 DNS/证书身份；不能 skip-verify。 |
| 迁移报告 OAuth 身份重复 | 恢复后预检库中的 `(user_id,op)`、`(op,open_id)` 重复及身份空字段 | 保持旧服务停止、另存备份，由身份负责人明确合并/解绑；不能只为通过索引而删除记录。 |
| 管理页面不存在 | `resources/admin`、反向代理路径、CSP/安全响应头 | 验证精确 release 镜像与代理路径，不要替换外部编译资产。 |
| 不知道初始管理员密码 | 数据库是否存在，以及服务用户能否读取 mode `0600` 密码文件 | 使用 `kessoku-api reset-admin-pwd --password-file PATH`；日志不会保存可复用 bootstrap 密码，也不要重建数据库。 |
| RustDesk 登录成功但连接被拒 | Starry 模式、token audience/scope/kid、JWKS freshness、introspection | 退回 `audit`，分类原因并修复契约/部署。 |
| 注销没有影响连接 | token row/JTI、auth version、introspection cache 与调用证据 | 验证权威 introspection，不能用 fail-open 缩短路径。 |
| 内部 API TLS 错误 | CA chain、server name、client SAN、TLS 1.3、时钟 | 修复 PKI/DNS/时间，不能 skip-verify 或改走公共代理。 |
| Starry 实例不可用 | 固定 origin、Agent identity UUID、CA/client cert、超时 | 保持控制只读，修复私有管理路径。 |
| Apply 返回 ETag/plan 错误 | 当前 ETag、actor/instance 绑定、plan 到期、candidate digest | 重新读取、合并、校验并创建 plan，不能强制覆盖。 |
| Apply 成功但 UI 状态旧 | Starry 活动 generation/digest 与 operation/audit | 重新读取权威状态；HTTP 成功不充分。 |
| 旧程序无法认证新会话 | v2.8.0 是否已签发仅 hash token | 恢复匹配的升级前数据库备份，或向前修复。 |
| audit/sysinfo 来源有争议 | 记录是否来自 RustDesk 1.4.9 无认证兼容路由 | 只把它视为运维 telemetry；需要不可抵赖性时使用 append-only/不可变外部日志。 |

修改行为前保存 request/operation ID、镜像/contract digest、脱敏日志、数据库版本与 Starry
generation。支持包中不得收集原始 bearer token、私钥、完整证书或完整 YAML。
