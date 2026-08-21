# 常见问题排查

[English](Troubleshooting.md) | **简体中文**

| 现象 | 检查证据 | 安全处理 |
| --- | --- | --- |
| 容器启动退出 | 配置校验、密钥/证书路径、文件权限、数据库连接 | 修复部署输入，不要关闭校验。 |
| 启动拒绝 MySQL/PostgreSQL 传输 | MySQL `tls`/`ca-file`、PostgreSQL `sslmode`/`ssl-root-cert`、数据库 DNS 名称与证书 SAN | 使用 MySQL `true` 或 PostgreSQL `verify-full`，只读挂载已审核 CA，并修复 DNS/证书身份；不能 skip-verify。 |
| 迁移报告 OAuth 身份重复 | 恢复后预检库中的 `(user_id,op)`、`(op,open_id)` 重复及身份空字段 | 保持旧服务停止、另存备份，由身份负责人明确合并/解绑；不能只为通过索引而删除记录。 |
| 管理页面不存在 | `resources/admin`、反向代理路径、CSP/安全响应头 | 验证精确 release 镜像与代理路径，不要替换外部编译资产。 |
| Web Client 页面不存在 | `resources/client/index.html`、21122 bind、独立客户端 hostname/proxy、`web-client.mode` | 验证精确 release 镜像与已审核 profile；不能恢复 `resources/web*` 或下载 WebClient2。 |
| 管理端 Connect popup 打开但空白 | `web_client_public_origin`、popup policy、admin COOP、client 未发送 COOP、精确 origin grant 交接、无 secret 的浏览器 console | 要求 admin 使用 `same-origin-allow-popups`、client 不发送 COOP（默认 `unsafe-none`）；检查 ready/grant/accepted 与超时尽力撤销。不能把 grant 放入 query string 或使用通配 `postMessage`。 |
| Web Client login/grant 被拒 | API/client origin、精确 CORS origin、EdDSA auth、token audience/scope/expiry、时钟 | 修复固定 origin/auth profile 并申请新短期 grant；不能启用 credentials CORS 或让客户端复用 admin token。 |
| 浏览器会话无法选择 Relay | Rendezvous WSS 响应与精确 `relay-wss-urls` 名称 map | 只增加已审核精确 Relay mapping 并递增 profile generation；不能推导或跟随任意 Relay URL。 |
| 浏览器已连接但没有视频/输入 | 强制 Relay WSS、签名 key exchange、VP9 WebCodecs 支持、frame 上限、peer 密码、支持的鼠标/基本键 | 使用支持浏览器/VP9 peer 并只收集脱敏状态；不支持 codec、P2P、剪贴板/音频等排除项没有 fallback。 |
| 不知道初始管理员密码 | 数据库是否存在，以及服务用户能否读取 mode `0600` 密码文件 | 使用 `kessoku-api reset-admin-pwd --password-file PATH`；日志不会保存可复用 bootstrap 密码，也不要重建数据库。 |
| RustDesk 登录成功但连接被拒 | Starry 模式、token audience/scope/kid、JWKS freshness、introspection | 退回 `audit`，分类原因并修复契约/部署。 |
| 注销没有影响连接 | token row/JTI、auth version、introspection cache 与调用证据 | 验证权威 introspection，不能用 fail-open 缩短路径。 |
| 内部 API TLS 错误 | CA chain、server name、client SAN、TLS 1.3、时钟 | 修复 PKI/DNS/时间，不能 skip-verify 或改走公共代理。 |
| Starry 实例不可用 | 固定 origin、Agent identity UUID、CA/client cert、超时 | 保持控制只读，修复私有管理路径。 |
| Apply 返回 ETag/plan 错误 | 当前 ETag、actor/instance 绑定、plan 到期、candidate digest | 重新读取、合并、校验并创建 plan，不能强制覆盖。 |
| Apply 成功但 UI 状态旧 | Starry 活动 generation/digest 与 operation/audit | 重新读取权威状态；HTTP 成功不充分。 |
| 旧程序无法认证新会话 | v2.8.2 是否已签发仅 hash token | 恢复匹配的升级前数据库备份，或向前修复。 |
| audit/sysinfo 来源有争议 | 记录是否来自 RustDesk 1.4.9 无认证兼容路由 | 只把它视为运维 telemetry；需要不可抵赖性时使用 append-only/不可变外部日志。 |

修改行为前保存 request/operation ID、镜像/contract digest、脱敏日志、数据库版本与 Starry
generation。支持包中不得收集原始 bearer token、私钥、完整证书或完整 YAML。
