# 连接认证

[English](Connection-Authentication.md) | **简体中文**

Kessoku 签发和撤销 access token；Starry 在 RustDesk 信令传输上验证令牌。任一服务都不能
单独提供完整连接认证属性。

## Token profile

令牌使用 Ed25519/EdDSA、`typ=at+jwt` 和明确 kid。Kessoku 固定 issuer、API/连接
audience、十进制 subject 与数值 user ID、`connect:initiate` scope、认证版本、UUID JTI
及有界 `iat`/`nbf`/`exp`，编码后最大 8 KiB。

RustDesk 1.4.9 会为 API 调用与原生信令保留同一个标准登录 token，且没有 refresh/exchange
步骤，因此该兼容 token 具有配置的 `kessoku-api` 与 `rustdesk-connect` 双 audience；这不
是浏览器权限模型。内置 Web Client 永远不会收到标准 bearer：客户端直接登录与 admin
grant exchange 都只返回短期、audience 为 `rustdesk-connect` 且 scope 为
`connect:initiate` 的 token。

新 token 行只保存 hash/JTI，不保存可复用 token。注销撤销一个 JTI；密码重置、禁用与
全局注销会增加用户认证版本。

## JWKS 与 introspection 信任边界

独立内部 listener 提供：

```text
GET  /api/internal/v1/auth/jwks
POST /api/internal/v1/auth/introspect
```

它与公共 listener 独立，要求 TLS 1.3、已验证客户端证书、精确 SAN 身份、请求体/超时上限，
以及全局和单证书限流。introspection 只接受 token，并返回不会枚举用户的最小结果。不得通过
公共反向代理暴露。

## 灰度顺序

1. 备份并迁移 Kessoku 数据库。
2. 使用 current/previous key 重叠开启 EdDSA 签发。
3. 确认支持客户端收到并保留新登录 token。
4. 开启内部 mTLS listener 并验证 Starry 身份。
5. Starry schema v3 先用 `off`，再在完整业务周期使用 `audit`。
6. 解释所有 would-deny，并验证注销、禁用、密码重置、密钥轮换和 introspection 故障。
7. 小范围开启 `enforce`；native、Secure TCP、WSS、P2P 与 Relay 会话全部通过且不
   fail-open 后才扩大范围。

## 故障策略与回滚

enforce 模式的 cache miss 遇到未知/过期 key 或已配置 introspection 故障时不得静默放行。
应在变更控制下退回 `audit`，不能绕过 TLS 验证或公开 introspection。

详见 [`MIGRATION.md`](../../MIGRATION.md)和
[`ROLLBACK-RUNBOOK.md`](../../ROLLBACK-RUNBOOK.md)。
