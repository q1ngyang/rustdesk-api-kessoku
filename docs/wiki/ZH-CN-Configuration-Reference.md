# 配置参数参考

[English](Configuration-Reference.md) | **简体中文**

完整注释模板为 [`conf/config.yaml`](../../conf/config.yaml)。程序读取该 YAML，也可用
`RUSTDESK_API_` 前缀的环境变量覆盖；点和连字符会转换为下划线。

示例：

```text
auth.internal.request-timeout
RUSTDESK_API_AUTH_INTERNAL_REQUEST_TIMEOUT
```

## 主要配置段

| 配置段 | 用途 | 安全起点 |
| --- | --- | --- |
| `app` | 注册、登录 UI、Swagger、token 兼容 | 注册/Swagger 关闭；迁移后关闭旧 token 读取。 |
| `gin` | 公共 API 监听、模式、资源、可信代理 | release 模式；只设精确代理地址或不设。 |
| `gorm`/数据库 | SQLite、MySQL、PostgreSQL 与连接池 | 简单单机可用 SQLite；生产外部 DB 按需验证 TLS。 |
| `rustdesk` | ID/Relay/API 地址与服务端公钥 | 精确公网地址和 `id_ed25519.pub`，不能使用私钥。 |
| `auth` | EdDSA token profile 与内部 mTLS API | 挂载密钥/PKI 且演练迁移后再开启。 |
| `server-control` | 固定 Starry Control Agent 实例 | 只读、旧命令关闭；凭据未就绪前不配置实例。 |
| `web-client-provider` | 独立浏览器客户端元数据 | `disabled`。 |
| `ldap`/OAuth | 可选身份源 | 验证 TLS、最小权限、删除示例密码。 |

## 认证文件

`auth.current-key.private-key-file` 必须引用 Ed25519 PKCS#8 私钥；previous key 使用公钥
文件。内部 listener 另需服务端证书/私钥、Starry 客户端 CA，以及至少一个精确允许的 URI
或 DNS SAN。

profile 启用后，所需材料缺失或无效会使启动失败。

## Starry 实例

每个启用实例固定：

- 部署 ID 与显示名称；
- 绝对 HTTPS Agent origin；
- 预期 Agent instance UUID 与 TLS server name；
- CA 和客户端证书/私钥文件路径；
- 独立 service-JWT 签名密钥与 key ID；
- control issuer 与证书绑定 authorized party。

浏览器无法覆盖这些值。除经过批准的配置窗口外，应保持
`server-control.read-only: true`。

## 已删除或拒绝的设置

- `app.web-client` 必须为零；浏览器客户端只能使用 external provider 治理模型。
- 旧 HS256 JWT 设置不是受支持认证 profile。
- `legacy-command-enabled` 不会恢复命令执行，兼容路由只会报告功能已删除。
- 不要把私钥、bearer token、完整 YAML secret 或证书内容写入环境变量、日志或管理审计。
