# rustdesk-api-kessoku 具体开发计划

> 文档状态：待实施
>
> 制定日期：2026-08-18
>
> 项目：`q1ngyang/rustdesk-api-kessoku`
>
> 建议后端基线：`lejianwen/rustdesk-api` v2.7 的干净后端
>
> 配套项目：`q1ngyang/rustdesk-server-starry`
>
> 配套计划：[RUSTDESK-SERVER-STARRY-DEVELOPMENT-PLAN.md](./RUSTDESK-SERVER-STARRY-DEVELOPMENT-PLAN.md)

## 0. 结论和固定决策

本项目定位为 RustDesk 的账户、策略和运维控制平面。HBBS/HBBR 的连接认证、Relay
运行状态和 Starry 配置语义继续由 `rustdesk-server-starry` 负责。

以下决策应在开发前固定，避免实现阶段反复改变边界：

1. 以 v2.7 后端作为公开开发基线。v2.6.29 与 v2.7 的后端源码相同，差异主要是
   v2.7 删除了 `resources/web2`；不把旧 WebClient2 编译产物重新带入仓库。
2. Go module 改为 `github.com/q1ngyang/rustdesk-api-kessoku/v2`，完成项目名称、镜像、
   文档、默认标题和构建产物的去上游化。
3. 管理前端单独 fork 为建议名称 `q1ngyang/rustdesk-api-kessoku-web`；后端构建只使用
   固定 commit/tag，不再在 CI 中拉取未固定的 `master`。
4. 现有“任意文本命令 + loopback 21115/21117”Server Control 进入废弃流程。新功能只走
   `StarryControlProvider` 和版本化 Starry Control API，不暴露通用命令执行接口。
5. 第一版 MUST_LOGIN 的语义是“发起远控的一端必须登录”。被控设备允许无人值守，
   不要求绑定已登录用户；该模式兼容现有 RustDesk 客户端携带的 access token。
6. 新 token 使用 Ed25519/EdDSA。Kessoku 持有私钥，Starry 只获得公钥/JWKS，禁止把
   可签发 token 的共享 HS256 密钥部署到 HBBS。
7. 连接认证采用“本地 JWT 校验 + 有界缓存的 introspection”。这样可以兼容客户端当前
   的长生命周期 access token，同时让注销、封禁和修改密码在短时间内生效。
8. Kessoku 不直接挂载或修改 Starry YAML，不持有 Docker Socket，也不直接访问远程
   21115/21117。配置写入、备份、原子替换、reload 和回滚全部由 Starry Control Agent 执行。
9. Web 客户端只提供合法、中立的 Provider 接口。Kessoku 不打包、镜像、下载或代理
   v2.6.29 的 `resources/web2`，也不提供许可证检查规避功能。
10. 旧 `/api/admin/rustdesk/*` 是首要安全修复：当前路由只有登录校验、没有管理员权限门，
    而后端接受任意命令字符串。实施任何新功能前先默认关闭该路由并补 `AdminPrivilege()`。
11. Kessoku→Agent 的远程控制请求同时使用 mTLS 和最长 5 分钟的 scoped service JWT；
    control keyring 与用户连接 JWT keyring 完全分离。

## 1. 目标和非目标

### 1.1 必须交付

- 可签发、验证、轮换和撤销用于 API 与连接认证的 JWT。
- Starry 能通过受保护的内部接口查询 token 是否仍然有效。
- 后台显示 Starry 实例能力、Relay 列表、native/WSS 健康状态和最近错误。
- 后台输入两个 IP 和 `native|wss|mixed`，展示 Relay 分配结果及完整决策轨迹。
- 后台以表单和高级 YAML 两种模式编辑 Starry 配置。
- 配置修改支持 schema 校验、diff、plan、ETag 并发保护、apply、历史和 rollback。
- 所有读取和变更均经过管理员鉴权、超时控制、结构化错误处理和审计。
- Kessoku 与 Starry 可以独立发布、独立升级；不 import、link 或复制对方业务源码。
- 后端单元、集成、契约和端到端测试进入 CI，`go test ./...` 成为合并门禁。

### 1.2 明确不做

- 不在 Kessoku 内重新实现 Geo 规则求值、Relay 健康判断或 Starry YAML 最终校验。
- 不从 Kessoku 直接执行 shell、Docker 命令或任意 HBBS/HBBR 文本命令。
- 不让浏览器直接访问 Starry Control Agent。
- 不在第一版要求被控设备也登录，或修改 RustDesk 客户端协议。
- 不在本项目中复刻、反编译或重新分发旧 WebClient2。
- 不承诺在线热修改 Starry Control Agent 自身的监听地址、证书或信任根；这些仍由部署配置管理。

## 2. 当前基线与必须先处理的问题

当前 fork 仍保留上游 module 名称和多数上游品牌。JWT 与 Server Control 的现状如下：

- `lib/jwt/jwt.go` 只签发包含 `user_id` 和 `exp` 的 HS256 token；解析时没有显式固定
  可接受算法，也没有 issuer、audience、JTI、token type 和 key id。
- `service/user.go` 把 JWT 原文写入 `user_tokens`。注销会删除数据库记录，但离线验证 JWT
  的 HBBS 不知道记录已被删除。
- `http/middleware/rustauth.go` 同时检查 JWT、token 数据库记录和用户状态；API 与 HBBS
  的撤销语义不一致。
- JWT 模式下数据库 `expired_at` 会自动延长，但 JWT 自身的 `exp` 不会改变，产生两个
  不一致的过期时间。
- `service/serverCmd.go` 只连接 API 容器自己的 `127.0.0.1/::1`，分容器部署时无法控制 Starry。
- `http/controller/admin/rustdesk.go` 接受前端传入的任意命令字符串，并支持保存自定义命令；
  该模型不适合作为远程管理平面。
- `http/router/admin.go` 的 `RustdeskCmdBind(adg)` 位于 `BackendUserAuth()` 后，但没有
  `AdminPrivilege()`；因此风险不只是命令过宽，还包括普通已登录用户可达。
- 管理前端来源是独立的 Vue 3/Vite 项目，必须 fork 并固定构建输入，不能只修改 Go 仓库中的
  编译后静态资源。
- 现有 CI 应补充测试、`go.sum`、固定工具版本、前端 lockfile 和可复现构建。

## 3. 目标架构与责任边界

```text
RustDesk 客户端
  │
  ├─ 登录/API ───────────────────> Kessoku API
  │                                  ├─ 用户与 user_tokens
  │                                  ├─ Ed25519 JWT signer
  │                                  ├─ JWKS
  │                                  └─ token introspection
  │
  └─ PunchHoleRequest.token ─────> Starry HBBS
                                     ├─ 本地 JWT 验证
                                     └─ introspection 短缓存

Kessoku Admin UI
  └─ /api/admin/server-control/v1/*
       └─ StarryControlProvider（Go）
            └─ mTLS/私有网络
                 └─ Starry Control Agent
                      ├─ Relay/Geo runtime snapshot
                      ├─ allocation simulation
                      └─ config validate/apply/rollback
```

| 领域 | Kessoku 所有权 | Starry 所有权 |
| --- | --- | --- |
| 用户、登录、封禁、注销 | 是 | 否 |
| JWT 私钥与签发 | 是 | 否 |
| JWT 公钥验证 | 否 | 是 |
| token 是否仍活跃 | 权威数据源 | 调用并短缓存 |
| PunchHole/Relay 协议执行 | 否 | 是 |
| Geo 规则和 Relay 资格 | 展示结果 | 权威实现 |
| 配置 UI、审批和审计 | 是 | 否 |
| 配置语法、持久化和回滚 | 否 | 是 |
| Control API 规范 | 消费并固定版本 | 发布并维护 |

## 4. 跨仓库契约

### 4.1 Starry Control API

Starry 仓库发布以下制品：

- `contracts/control/v1/openapi.yaml`
- `contracts/config/v3/config.schema.json`
- `contracts/config/v3/config.ui-schema.json`
- `contracts/control/v1/examples/*.json`

Kessoku 在更新依赖时生成并提交 Go client 到
`internal/starrycontrol/clientgen/`，同时在
`internal/starrycontrol/CONTRACT_VERSION` 记录 Starry tag、contract version 和 SHA-256。
生产构建不得临时从网络下载 schema 或生成器。

Control Agent 最低接口：

| 方法与路径 | 用途 | Kessoku 权限 |
| --- | --- | --- |
| `GET /control/v1/capabilities` | 协商 contract、schema 和功能 | `server_control.view` |
| `GET /control/v1/status` | Agent/HBBS/config/auth 状态 | `server_control.view` |
| `GET /control/v1/relays` | Relay 配置和运行状态 | `server_control.view` |
| `POST /control/v1/allocations:simulate` | 两 IP 分配模拟 | `server_control.simulate` |
| `GET /control/v1/config` | YAML、结构化值和 ETag | `server_control.config.read` |
| `GET /control/v1/config/schema` | JSON Schema/UI hints | `server_control.config.read` |
| `POST /control/v1/config:validate` | 无副作用校验 | `server_control.config.write` |
| `POST /control/v1/config:plan` | 保存短期 plan、diff 与影响 | `server_control.config.write` |
| `POST /control/v1/config:apply` | 按 `plan_id` 应用 | `server_control.config.apply` |
| `GET /control/v1/config/history` | 历史版本 | `server_control.config.read` |
| `POST /control/v1/config:rollback` | 回滚指定 generation | `server_control.config.apply` |
| `GET /control/v1/operations/{id}` | 查询异步 apply/rollback | `server_control.view` |

所有写请求必须包含：

- `X-Request-ID`：Kessoku 生成的 UUID；
- `Idempotency-Key`：apply/rollback 必填；
- `If-Match`：当前配置 ETag；
- 明确的 `Content-Type: application/json`；
- 由 Kessoku mTLS 客户端证书建立的身份。

标准错误体：

```json
{
  "error": {
    "code": "CONFIG_ETAG_MISMATCH",
    "message": "configuration changed since it was loaded",
    "request_id": "0191...",
    "retryable": false,
    "details": {}
  }
}
```

Kessoku 只依赖稳定 `code`，不得依赖 Starry 的英文 message 做业务判断。

### 4.2 连接认证契约 v1

JWT header：

```json
{
  "alg": "EdDSA",
  "kid": "kessoku-2026-01",
  "typ": "at+jwt"
}
```

JWT claims：

```json
{
  "iss": "https://api.example.com",
  "aud": ["kessoku-api", "rustdesk-connect"],
  "sub": "42",
  "user_id": 42,
  "jti": "0191...",
  "token_use": "access",
  "scope": ["connect:initiate"],
  "auth_version": 3,
  "iat": 1787000000,
  "nbf": 1787000000,
  "exp": 1787604800
}
```

约束：

- 只接受 `EdDSA`，不得根据 token header 动态选择任意算法。
- `iss`、`aud`、`token_use`、`scope`、`kid`、`nbf`、`exp` 全部强制校验。
- `sub` 与 `user_id` 必须指向同一用户。
- token 最大 8 KiB，允许时钟偏差默认 30 秒。
- 私钥只从只读 secret file/HSM 接口加载，不写入普通 YAML、数据库、日志或管理 API。
- JWKS 同时发布 current 和 previous key；轮换重叠期至少覆盖最长 token TTL。

内部接口：

```text
GET  /api/internal/v1/auth/jwks
POST /api/internal/v1/auth/introspect
```

`introspect` 请求只接收 token，不接受前端传入的 user id：

```json
{ "token": "eyJ..." }
```

响应：

```json
{
  "active": true,
  "sub": "42",
  "jti": "0191...",
  "exp": 1787604800,
  "auth_version": 3,
  "reason": "active"
}
```

无效 token 对外统一返回 `active:false`，详细原因只进入脱敏服务端日志和指标。接口仅允许
Starry mTLS 客户端证书访问，独立限流，禁止从公网反向代理路径直接开放。

## 5. 建议代码布局

### 5.1 Go 后端

| 路径 | 动作 | 责任 |
| --- | --- | --- |
| `go.mod` | 修改 | module 改名，固定 Go/tool 版本，提交 `go.sum` |
| `config/auth.go` | 新增 | JWT issuer、key file、TTL、JWKS 与内部 mTLS 配置 |
| `config/starry_control.go` | 新增 | Starry 实例、超时、CA/cert/key file 和只读开关 |
| `conf/config.yaml` | 修改 | 新配置示例；Web client 默认关闭；不放真实 secret |
| `internal/auth/claims.go` | 新增 | v1 claims/header 常量和严格校验 |
| `internal/auth/signer.go` | 新增 | Ed25519 signer 与 `kid` 轮换 |
| `internal/auth/token_hash.go` | 新增 | token SHA-256 存储和常量时间比较辅助 |
| `internal/controlauth/signer.go` | 新增 | 独立 control keyring、短期 scoped service JWT |
| `internal/starrycontrol/provider.go` | 新增 | 通用 `ServerControlProvider` 接口 |
| `internal/starrycontrol/starry/provider.go` | 新增 | Starry Provider 实现 |
| `internal/starrycontrol/clientgen/` | 生成后提交 | 固定 contract v1 的 Go client/types |
| `model/userToken.go` | 修改 | `jti/kid/token_hash/auth_version/revoked_at` |
| `model/user.go` | 修改 | 用户级 `auth_version` |
| `model/adminAuditEvent.go` | 新增 | 配置和控制平面审计 |
| `service/user.go` | 修改 | 新 token 生命周期、注销和全局撤销 |
| `service/authIntrospection.go` | 新增 | active 判定和 JWKS 输出 |
| `service/starryControl.go` | 新增 | Provider 编排、超时、错误映射、审计 |
| `http/middleware/rustauth.go` | 修改 | 严格 Bearer 解析和统一 token verifier |
| `http/middleware/internalMTLS.go` | 新增 | Starry 内部接口证书身份校验 |
| `http/controller/internal/auth.go` | 新增 | JWKS/introspection controller |
| `http/controller/admin/starryControl.go` | 新增 | 管理控制台 API |
| `http/router/admin.go` | 修改 | `/server-control/v1` 管理路由 |
| `http/router/api.go` | 修改 | 内部认证路由与独立 middleware |
| `service/serverCmd.go` | 废弃后删除 | 旧 loopback 文本命令实现 |
| `http/controller/admin/rustdesk.go` | 缩减 | 旧 sendCmd 返回 deprecation，最终删除 |

`ServerControlProvider` 最小接口：

```go
type ServerControlProvider interface {
    Capabilities(ctx context.Context) (Capabilities, error)
    Health(ctx context.Context) (Health, error)
    Relays(ctx context.Context) ([]Relay, error)
    SimulateAllocation(ctx context.Context, in SimulationInput) (SimulationResult, error)
    GetConfig(ctx context.Context) (ConfigDocument, error)
    GetConfigSchema(ctx context.Context) (SchemaBundle, error)
    ValidateConfig(ctx context.Context, in ConfigCandidate) (ValidationResult, error)
    PlanConfig(ctx context.Context, in ConfigCandidate) (ConfigPlan, error)
    ApplyConfig(ctx context.Context, in ApplyRequest) (ApplyResult, error)
    ConfigHistory(ctx context.Context) ([]ConfigRevision, error)
    RollbackConfig(ctx context.Context, in RollbackRequest) (ApplyResult, error)
}
```

Provider 返回领域类型，不把 Agent HTTP status、原始错误文本或 transport client 泄漏到 controller。

第一版仍可沿用现有 `AdminPrivilege()` 作为粗粒度授权；表中的
`server_control.*` 是 Provider/Agent scope 和未来 RBAC 的稳定名称，当前全部映射为管理员专属，
不能只在前端隐藏按钮。

Control Provider 每次调用 Agent 时签发独立 service JWT：

```json
{
  "iss": "https://api.example.com",
  "sub": "service:rustdesk-api-kessoku",
  "aud": "urn:starry-control:0191f6a0-...",
  "azp": "kessoku-production",
  "scope": ["starry.relay.read"],
  "act": { "sub": "user:42" },
  "iat": 1787047200,
  "nbf": 1787047200,
  "exp": 1787047500,
  "jti": "0191..."
}
```

Agent 必须同时验证 service JWT 和 mTLS 客户端身份，且 `azp` 与证书映射身份一致。

### 5.2 管理前端

先 fork `lejianwen/rustdesk-api-web`，建议代码落点：

```text
src/
├─ api/starry_control.js
├─ views/server_control/
│  ├─ overview.vue
│  ├─ relays.vue
│  ├─ allocation_simulator.vue
│  ├─ config_editor.vue
│  └─ config_history.vue
├─ components/starry/
│  ├─ relay_status_table.vue
│  ├─ decision_trace.vue
│  ├─ schema_form.vue
│  ├─ yaml_editor.vue
│  └─ config_diff.vue
└─ router/index.js
```

前端约束：

- 只调用 Kessoku `/api/admin/server-control/v1/*`，绝不直接调用 Agent。
- Relay 页面区分 `configured`、`native`、`websocket` 和 transport eligibility，不能只显示
  一个含糊的“在线”状态。
- 模拟器校验 IPv4/IPv6；结果展示命中规则、方向、候选、排除原因和最终选择。
- 表单根据 Starry 返回的 JSON Schema 生成；前端校验只是交互辅助，Agent 校验才是权威。
- YAML 编辑器不得自动修复或静默删除未知字段。
- apply 前必须显示 diff、当前 ETag、目标 generation、警告和 `restart_required`。
- ETag 冲突时重新读取并要求人工合并，不提供“强制覆盖”按钮。
- 敏感字段使用只读 secret reference，API 响应中不出现密钥内容。

## 6. 数据迁移方案

### 6.1 `users`

新增：

```text
auth_version BIGINT NOT NULL DEFAULT 1
```

以下操作原子增加 `auth_version`：

- 管理员禁用用户；
- 修改/重置密码；
- 删除所有会话；
- 明确执行“注销该用户所有设备”。

单设备注销只撤销对应 JTI，不增加用户全局版本。

### 6.2 `user_tokens`

新增字段：

```text
jti             VARCHAR(...) UNIQUE
kid             VARCHAR(...)
token_hash      BINARY(32) / VARCHAR(64) UNIQUE
auth_version    BIGINT
issued_at       BIGINT
revoked_at      BIGINT NULL
revoked_reason  VARCHAR(...)
```

迁移步骤：

1. 新增可空字段和索引。
2. 对现有 `token` 计算 SHA-256 并回填 `token_hash`。
3. 发布兼容读取：优先 hash，旧行回退原 token。
4. 启用新签发器；新 token 只存 hash/JTI，不存可复用的 JWT 原文。
5. 首个强制认证版本上线时主动清理旧 HS256/随机 token，要求客户端重新登录。
6. 观察一个发布周期后删除明文 token 回退路径；物理删列另开迁移版本。

该迁移必须在 SQLite、MySQL、PostgreSQL 三种数据库各执行一次升级和回滚演练。

### 6.3 管理审计

新增 `admin_audit_events`：

```text
id, actor_user_id, action, target_type, target_id,
request_id, result, error_code, metadata_json, created_at
```

必须记录：Relay/health 查看可只做指标；simulate、validate、plan、apply、rollback、
Provider 配置失败、JWT key 轮换和用户全局撤销必须审计。审计中不保存完整 token、私钥、
客户端证书、完整 YAML 或未脱敏公网 IP。

## 7. 分阶段实施

工期按一名熟悉 Go、Vue 3 和部署安全的工程师估算；不包含 Starry 侧工期。

### K0：基线、来源和可复现构建（3～5 人日）

任务：

- 从 v2.7 干净后端创建 `kessoku-base-v2.7` annotated tag。
- 立即给 `RustdeskCmdBind` 增加 `AdminPrivilege()`；新增
  `server-control.legacy-command-enabled=false`。默认不注册旧 sendCmd/自定义命令路由；
  兼容开启时也必须仅管理员可达并在响应中标记 deprecated。
- 确认仓库中不存在 `resources/web2` 及其历史恢复脚本。
- 修改 module/import、项目名、镜像名、README 和默认 UI 标题。
- fork 管理前端；记录起始 commit 和许可证。
- 提交 `go.sum` 和前端 lockfile。
- CI 增加格式、vet、`go test ./...`、前端 lint/test/build、SBOM 和 secret scan。
- 固定 `swag`、Node、npm、cross compiler 和前端 commit；构建不得使用 `@latest`。

验收：

- 全新环境能从固定输入构建相同后端和前端制品。
- 普通登录用户调用任一旧/新 Server Control 路由均为 403；默认安装的旧 sendCmd 路由不可达。
- `go test ./...` 实际运行，不再只有编译检查。
- Release provenance 能指出后端 commit、前端 commit、依赖 lock 和镜像 digest。

### K1：认证内核和数据库迁移（8～12 人日）

任务：

- 实现严格 EdDSA claims、signer、verifier、JWKS 和 key rotation。
- 实现 token hash/JTI 存储和三数据库迁移。
- 修复 JWT `exp` 与数据库 `expired_at` 双时钟问题：JWT 模式下 `exp` 为唯一到期上限，
  禁止只延长数据库记录却不返回新 JWT。
- 登录响应继续返回 RustDesk 客户端可识别的单一 access token。
- 实现单 token 注销、用户全局撤销、封禁和密码重置语义。
- 与 Starry 统一采用 `off|audit|enforce` 连接认证模式；Kessoku 只负责签发和内部查询，
  Starry 决定实际放行。

验收：

- 算法替换、错误 issuer/audience/type、过期、未来 nbf、未知 kid 全部拒绝。
- 注销单 token 后 introspection 在一个请求内返回 inactive。
- 用户禁用或 auth_version 增加后，该用户所有旧 token inactive。
- current/previous 两把 key 在重叠期均可验证，移除 previous 后旧 token 被拒绝。
- 日志、错误响应和 tracing 中均找不到完整 token。

### K2：内部 JWKS/introspection（4～6 人日）

任务：

- 建立独立 internal router 和 mTLS middleware。
- 校验客户端证书链、有效期和允许的 SAN；不只判断请求来自内网 IP。
- 对 introspection 增加每证书和全局限流、1 MiB 以下 body limit、短超时和指标。
- 输出 `active`、最小必要 claims 和稳定 reason code。
- 提供 Starry 用 contract fixtures，包括 active、expired、revoked、disabled、rotated key。

验收：

- 无证书、错误 CA、错误 SAN、过期证书和公网普通用户 token 均不能访问内部接口。
- introspection 不泄露“用户是否存在”等可枚举细节。
- 在目标 QPS 下延迟和数据库连接数有明确基线。

### K3：Starry Provider 与只读控制台（7～10 人日）

任务：

- 增加静态部署配置的 Starry 实例列表；连接凭据只允许 file reference。
- 生成并提交 contract v1 client，设置 connect/request/total timeout 和 response size limit。
- 实现 capabilities、health、relays、allocation simulation。
- 将 Agent 错误映射为 Kessoku 稳定错误码，不把内部 URL 和证书路径返回浏览器。
- 完成 Relay 表格、健康摘要和双 IP 模拟器。
- 增加 mock Agent 契约测试，前端加入 loading、partial failure、stale data 标识。

验收：

- Kessoku 与 Starry 分容器、不同 loopback 时正常工作。
- Agent 不可达不会拖垮 API 线程池；页面在规定超时内显示明确降级状态。
- `native|wss|mixed` 三种模拟结果与 Starry CLI/真实选择结果一致。
- UI 不包含 raw command 输入框。

### K4：配置读取、校验、计划和安全应用（10～15 人日）

任务：

- 实现 config/schema、get、validate、plan、apply、operation、history、rollback Provider 方法。
- 引入 schema form、YAML editor 和结构化 diff。
- apply/rollback 使用 `If-Match`、request id、idempotency key。
- 将 apply 设计成显式两步：先 plan，再由管理员确认后按 `plan_id` apply；plan 绑定 actor、
  instance、base ETag 和 candidate digest，默认 10 分钟失效。
- 对高风险变更显示额外警告：开启/关闭连接认证、修改全部 Relay、修改 trusted proxies、
  修改 JWKS/introspection 来源。
- 完成管理审计查询页面。

验收：

- 无效 YAML、未知字段、跨字段错误不会生成可 apply 的 plan。
- 两个浏览器同时编辑时，后提交者得到 412/ETag mismatch，不能覆盖新版本。
- Agent reload 失败时 UI 显示已自动回滚及回滚 generation。
- 刷新页面后可验证活动配置确实等于目标 generation，而不只依赖 apply HTTP 200。

### K5：旧 Server Control 迁移和删除（3～5 人日）

任务：

- 第一个兼容版本把 `/api/admin/rustdesk/sendCmd` 标记 deprecated，默认关闭高级命令 UI。
- 提供只读迁移说明，把系统命令对应到新 Provider 页面。
- 下一个 minor release 删除自定义命令创建/执行；数据库旧记录只保留导出工具。
- 最终删除 `service/serverCmd.go`、相关 controller/model/路由和前端页面。

验收：

- 默认安装没有任意命令执行入口。
- 搜索前后端源码不存在 `sendCmd` 可达路由和 raw command 输入。
- 升级旧数据库不因遗留 `server_cmds` 表失败。

### K6：合法 Web Client Provider（4～7 人日，不含浏览器客户端开发）

任务：

- 增加 `disabled|external` Provider 模式；默认 `disabled`。
- Provider manifest 只包含 id、名称、launch URL、允许 origin、许可证、源码 URL、版本和 digest。
- Kessoku 不下载或反向代理 Provider 静态文件；使用独立 origin 打开。
- 第一版不向外部 origin 注入 access token。后续 SSO 只采用短期授权码 + PKCE + 精确 redirect URI，
  不使用 query string 传 bearer token，也不共享 localStorage。
- 管理员配置 Provider 时必须填写来源和授权说明；这只是治理记录，不代替真实授权。

验收：

- Kessoku release、镜像和安装脚本不包含旧 WebClient2 文件、下载 URL 或绕过检查代码。
- 非允许 origin 无法完成授权码交换。
- 禁用 Provider 时所有 launch/session 路由均不可用。

### K7：集成、灰度和发布（7～10 人日）

任务：

- 与 Starry 固定 release candidate 做跨仓库 E2E。
- 先启用 `audit`：记录缺失/无效 token 和 would-deny 指标但不拒绝连接。
- 确认目标客户端版本都会发送 token 后，再小范围启用 `enforce`。
- 先把 Control Agent 设为 read-only；配置 apply 通过回滚演练后单独开启。
- 执行 SQLite/MySQL/PostgreSQL 升级、备份恢复、token 全失效和重新登录演练。
- 发布 migration guide、security model、operator runbook 和 rollback runbook。

验收：

- 登录用户的 native TCP、Secure TCP、WSS 连接成功；当前 Starry 不支持的 UDP
  `PunchHoleRequest` 保持拒绝且不触发分配；未登录、注销和禁用用户被拒绝。
- Relay 页面与双 IP 模拟在真实多节点环境吻合。
- 配置 apply、失败自动回滚、人工 rollback 都有审计和真实进程验证。
- 关闭 Kessoku、Agent 或某个 Relay 时，失败策略符合文档，没有静默放行。

## 8. Kessoku 管理 API

浏览器只访问下列 Kessoku 路由：

```text
GET  /api/admin/server-control/v1/instances
GET  /api/admin/server-control/v1/instances/:id/capabilities
GET  /api/admin/server-control/v1/instances/:id/health
GET  /api/admin/server-control/v1/instances/:id/relays
POST /api/admin/server-control/v1/instances/:id/allocation-simulations
GET  /api/admin/server-control/v1/instances/:id/config
GET  /api/admin/server-control/v1/instances/:id/config/schema
POST /api/admin/server-control/v1/instances/:id/config/validate
POST /api/admin/server-control/v1/instances/:id/config/plan
POST /api/admin/server-control/v1/instances/:id/config/apply
GET  /api/admin/server-control/v1/instances/:id/operations/:operation_id
GET  /api/admin/server-control/v1/instances/:id/config/history
POST /api/admin/server-control/v1/instances/:id/config/rollback
GET  /api/admin/server-control/v1/audit-events
```

实例在部署配置中定义，第一版不允许管理员从 UI 输入任意 Agent URL，避免把 Provider 变成 SSRF
代理。每个实例必须配置固定 base URL、TLS server name、CA、client cert/key file 和 enabled 状态。

## 9. 测试矩阵

### 9.1 后端单元测试

- JWT：正常、算法替换、错误 kid/iss/aud/type、nbf、exp、时钟偏差、超大 token。
- Token lifecycle：单点注销、全局注销、禁用、密码修改、删除用户、并发刷新。
- Provider：超时、TLS 错误、异常 JSON、过大响应、未知 capability、contract 版本不兼容。
- Config orchestration：ETag mismatch、重复 idempotency key、plan 过期、apply 部分失败。
- 审计：成功与失败均记录，secret/token/YAML 不进入 metadata。

### 9.2 集成测试

- SQLite、MySQL、PostgreSQL migration fixtures。
- 使用真实 mTLS 的 mock Starry Agent。
- Kessoku 重启后 key、token、实例和审计状态一致。
- Agent 返回 v1 增量字段时客户端忽略未知字段；删除必填字段时契约测试失败。

### 9.3 前端测试

- Relay 状态组合与 stale/partial 状态。
- IPv4、IPv6、无效地址和 transport 切换。
- JSON Schema 条件字段、数组排序和 Geo 嵌套表达式。
- YAML 与表单双向切换不丢字段。
- ETag 冲突、apply 回滚和 Agent 不可达交互。
- 无 admin 权限时菜单、路由和 API 都不可达；不能只隐藏按钮。

### 9.4 跨仓库 E2E

| 场景 | 预期 |
| --- | --- |
| 已登录 native 发起连接 | 允许 |
| 已登录 WSS 发起连接 | 允许 |
| 未登录 native/WSS | 拒绝，稳定错误分类 |
| token 签名错误/过期 | 拒绝 |
| API 注销后再次连接 | 超过正缓存 TTL 后拒绝 |
| 用户被禁用/修改密码 | 旧 token 拒绝 |
| introspection 临时不可用 | cache miss fail closed；已缓存项按文档到期 |
| JWKS current→previous 轮换 | 重叠期不断连，新旧 token 行为符合 TTL |
| 两 IP native/wss/mixed 模拟 | 与真实 Starry 选择一致 |
| 配置 apply 失败 | 自动恢复 last-known-good |

## 10. CI、分支和提交策略

建议分支：

```text
master                         稳定分支
develop                        可选集成分支
feature/auth-v1
feature/starry-control-readonly
feature/starry-config-editor
feature/webclient-provider
```

每个 PR 必须满足：

- 一个主要关注点，数据库迁移与使用代码在同一 PR 或有明确 feature flag；
- 更新测试、配置示例、Swagger/OpenAPI 和中英文运维文档；
- `gofmt`、`go vet`、`go test ./...`、race test（适用包）、前端 test/build 通过；
- 不出现新 secret、编译 WebClient2 产物或未固定的网络下载；
- contract 变化必须先在 Starry 发布向后兼容版本，再更新 Kessoku 的固定 client。

建议 Issue/PR 顺序：

```text
K-001  fork provenance、module/branding、v2.7 clean baseline
K-002  reproducible CI、go.sum、frontend pinning
K-010  auth contract v1、EdDSA signer/verifier
K-011  token schema migration and lifecycle
K-012  JWKS/introspection mTLS endpoints
K-020  StarryControlProvider and generated client
K-021  relay/health/simulation backend
K-022  relay/health/simulation frontend
K-030  config read/schema/validate/plan
K-031  config plan/apply/operation/history/rollback
K-032  schema form/YAML/diff frontend
K-040  legacy server command deprecation/removal
K-050  lawful external Web Client Provider
K-060  cross-repo E2E, migration, rollout and release
```

## 11. 发布门禁和回滚

### 11.1 发布阻断项

- 任一数据库迁移失败或无法从备份恢复。
- 未登录连接在 enforce 模式下仍可建立。
- introspection/JWKS 错误导致 Starry 静默 fail-open。
- Kessoku 能向 Agent 发送任意 URL、任意命令或未经 schema 的 YAML。
- apply 成功响应后 Starry 活动 generation 与目标不一致。
- Release 中出现来源/许可证不清晰的 Web 客户端文件。
- Go/前端测试未运行，或构建依赖未固定。

### 11.2 回滚顺序

1. Starry 从 `enforce` 切回 `audit`，保留认证指标；只有明确授权的维护窗口才能临时关闭。
2. Kessoku 把 Control Provider 切为 `read_only`。
3. 通过 Agent 回滚 last-known-good Starry 配置并验证 generation。
4. 回滚 Kessoku 应用镜像；数据库只使用已演练的向后兼容迁移或备份恢复。
5. JWT key 不能随应用镜像回滚而丢失；current/previous key 集合独立持久化。

## 12. 工作量和关键路径

| 工作流 | Kessoku 估算 | 关键依赖 |
| --- | ---: | --- |
| 基线和 CI | 3～5 人日 | 无 |
| JWT、迁移、JWKS/introspection | 12～18 人日 | auth contract v1 |
| Provider + Relay/模拟 UI | 7～10 人日 | Starry Control read-only API |
| 配置编辑和安全应用 | 10～15 人日 | Starry schema/apply/rollback |
| 旧命令迁移 | 3～5 人日 | Provider 功能完成 |
| Web Client Provider 壳 | 4～7 人日 | 法律/来源门禁；不含客户端 |
| 跨仓库验收和发布 | 7～10 人日 | 两边 release candidate |

单人串行约 10～14 周；Go/前端与 Starry 两条工作流并行时，日历时间可缩短，但认证契约、
Control contract 和 E2E 门禁不能并行跳过。

## 13. 完成定义

只有同时满足以下条件，才可宣布本计划完成：

- v2.7 干净基线、Kessoku module/品牌和可复现 CI 已落地。
- EdDSA JWT、撤销、轮换、JWKS/introspection 通过全部测试。
- Starry 与 Kessoku 分容器部署，后台可查看 Relay、模拟分配并安全管理配置。
- 旧任意命令入口已删除或在明确的兼容发布中默认不可用。
- audit→enforce 灰度完成，真实 native/WSS 客户端矩阵通过。
- 配置错误、并发冲突、Agent 故障和回滚均做过真实演练。
- Release 不含旧 WebClient2；外部 Web Client Provider 默认关闭且没有 bearer token 注入。
- 运维、升级、回滚、安全模型和 contract 版本文档齐全。
