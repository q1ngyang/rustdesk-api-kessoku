# Starry 控制

[English](Starry-Control.md) | **简体中文**

Kessoku 通过 Control API v1 与可选 Starry Control Agent 集成。浏览器只访问 Kessoku，
绝不直接访问 Agent。

## 授权

- Kessoku 的所有 server-control 路由都要求管理员权限。
- Agent 同时要求已批准的 mTLS 客户端身份，以及携带操作精确 scope 的短期 EdDSA
  service JWT。
- Agent origin、TLS 名称、身份和凭据路径由部署配置固定。
- 用户 access-token 与 Control Agent 签名 keyring 相互独立。

## 支持操作

- 能力和状态；
- 已配置/可选 Relay 列表与健康；
- 输入两个 IP 和 transport 的无副作用分配模拟；
- 配置与 JSON/UI schema 读取；
- 权威校验与短期 plan；
- 受 ETag/idempotency 保护的 apply 和 operation poll；
- 历史、回滚与有确认的 runtime reload。

不存在 raw command、shell、任意 URL、Docker Socket、通用文件 API 或强制覆盖操作。

## 上线顺序

1. 在 `internal/starrycontrol/CONTRACT_VERSION` 固定精确已发布 Starry Control/Auth contract。
2. 把 Agent 部署在私有管理路径，保持写入关闭。
3. 从 Kessoku 验证实例身份、能力、状态、Relay 和模拟。
4. 在 staging 校验并 plan 一个无害配置变更。
5. 打开受控写窗口，apply 后验证真实活动 generation，并演练 rollback。
6. 变更窗口外恢复 Kessoku 和 Agent 只读。

HTTP 成功不表示候选一定激活。必须重新读取 generation/digest，并关联 Kessoku 意图/结果
审计与 Agent 持久审计。
