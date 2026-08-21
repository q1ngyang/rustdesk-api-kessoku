# 运维与验证

[English](Operations-and-Verification.md) | **简体中文**

验证是分层的，低层证据不能替代高层验收。

## 1. 源码与制品身份

- 验证不可变 tag、source commit、checksum、SBOM、provenance 和解析出的镜像 digest。
- 确认内置管理前端属于同一提交。
- 确认精确 Starry contract 为 `PINNED` 且与部署 Agent 一致。
- 确认 release 不含 `resources/web`、`resources/web2`、WebClient2、私钥或构建凭据。

## 2. 部署静态检查

```sh
docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
```

检查镜像身份、bind、mount、最终环境变量、反向代理信任和 secret 引用。升级启动前验证备份。

## 3. 进程与数据库

- 容器以非特权用户运行，并能干净重启。
- 数据库版本为 301；迁移行数、token hash、OAuth 身份索引与最后管理员不变量正确。
- 已修改管理员初始密码。
- 日志不包含 token、私钥、证书或完整配置。
- 注册、Swagger、provider 和控制写入默认值符合策略。

## 4. API 与认证

- 管理员和普通用户授权边界明确。
- 登录、地址簿、注销、再次登录、密码重置、禁用与全局撤销符合 token 生命周期。
- current/previous Ed25519 轮换覆盖最长 token 生命周期。
- 内部 JWKS/introspection 只允许预期 mTLS 身份，并在普通运维负载下保持有界。

## 5. Starry 控制

- 实例身份、能力、状态、Relay 和模拟与部署 Agent/HBBS 一致。
- 普通用户由后端拒绝，不只是 UI 隐藏。
- 只读模式拒绝变更。
- staging 的 plan/apply/operation/history/rollback/reload 最终得到预期活动 generation，
  且两侧脱敏审计可关联。

## 6. 真实客户端验收

对每个支持 RustDesk 客户端版本记录登录及以下证据：

- native TCP 与 Secure TCP；
- 启用 WSS 时的 WSS-to-WSS 和混合 WSS/native；
- 直连 P2P 与强制/实际 Relay 会话；
- 注销、禁用、密码重置、密钥轮换和依赖故障；
- Starry `audit`，之后才是小范围 `enforce`。

本地流程不执行渗透、利用、公网目标、fuzz/mutation 或压力测试。任何单独批准的韧性测试
只能在隔离 staging/CI 环境运行。

v2.8.0 本机发布证据精确覆盖 RustDesk 1.4.9 强制 Relay 会话：`audit` native/native，
以及 `enforce` native/native、WSS/WSS、WSS/native、native/WSS；不表示已覆盖直接 P2P
或独立 Secure TCP case。详见[安全发现闭环](ZH-CN-Security-Finding-Closure.md)。

## 7. 恢复与 go/no-go

在 staging 恢复数据库、密钥、PKI、配置与旧镜像，记录 RTO/RPO、回滚 generation、客户端
重新登录、负责人和 go/no-go 结论。完成后发布负责人才能批准 `RELEASE_STATUS`。

日常详细清单见 [`OPERATOR-RUNBOOK.md`](../../OPERATOR-RUNBOOK.md)。
