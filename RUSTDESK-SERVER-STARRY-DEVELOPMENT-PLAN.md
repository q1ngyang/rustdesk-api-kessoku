# rustdesk-server-starry 具体开发计划

> 文档状态：待实施
>
> 制定日期：2026-08-18
>
> 项目：`q1ngyang/rustdesk-server-starry`
>
> 当前基线：Starry `c4ba896`、官方 rustdesk-server `1.1.16`
>
> 当前发布：`1.1.16-patch-v1.1.0`
>
> 建议下一发布：`1.1.16-patch-v1.2.0`，最终版本号在 contract freeze 后确认
>
> 配套计划：[RUSTDESK-API-KESSOKU-DEVELOPMENT-PLAN.md](./RUSTDESK-API-KESSOKU-DEVELOPMENT-PLAN.md)

## 0. 结论和固定决策

Starry 继续采用“官方 rustdesk-server 固定版本 + 可重复应用的小型 overlay”模式。本计划不会把
Kessoku 的账户、UI 或数据库代码合入 Starry，也不 fork HBBR 业务实现。

以下决策在开发前固定：

1. MUST_LOGIN 第一版只要求发起控制的一端登录；被控端允许无人值守，不要求用户登录。
2. 连接认证在 HBBS 执行，必须覆盖 `PunchHoleRequest` 和可直接进入 Relay 流程的
   `RequestRelay`。两类 protobuf 都携带 token，必须走同一个认证函数。
3. 验证顺序是：消息/尺寸校验 → JWT 本地验证 → Kessoku introspection → 目标查询和 Relay
   分配。未经认证的请求不能通过错误差异枚举目标 ID 是否存在。
4. 使用 Ed25519/EdDSA 和 JWKS。Starry 只持有公钥；禁止共享能够签发用户 token 的 HS256
   secret，禁止 JWT key 为空时接受任意非空 token。
5. 认证模式固定为 `off|audit|enforce`。`enforce` 在 verifier、JWKS 或强制 introspection
   不可用时 fail closed；无可用初始公钥时拒绝启动/配置激活。
   部署级 `--must-login`/`MUST_LOGIN=Y` 是不可被 YAML 或远程控制降低的 enforce 下限。
6. HBBR 第一阶段不改协议。HBBS 只有在发起请求认证成功后才分配/转发 Relay 会话。
7. Control Agent 是 Starry 仓库拥有并发布的独立、可选二进制。Kessoku 只访问 Agent，
   不访问裸 21115、不读写 `config.yaml`，也不取得 Docker Socket。
8. Agent 外部接口采用版本化 HTTPS API；远程部署要求 mTLS + 最长 5 分钟的 scoped service
   JWT。Agent 不提供 shell、任意文件路径、任意文本命令或通用进程控制。
9. Relay 分配模拟必须是纯函数，不能递增生产轮转计数、改变健康状态或修改任何运行时状态。
10. 配置编辑上线前，必须先把 reload 改为 last-known-good：空/无效 reload 保留当前活动配置，
    不能像当前实现一样把活动 Starry 状态清空。
11. 配置 schema 升级到 v3 以容纳连接认证；继续读取 v1/v2。配置 schema 版本与 Control API
    版本分离。
12. 所有新增代码仍按本仓库许可证公开；Kessoku 通过网络协议调用，不复制或 link Starry 代码。

## 1. 目标和非目标

### 1.1 必须实现

- native TCP、Secure TCP 和 `/ws/id` 的受支持发起连接路径认证行为一致。当前 overlay 明确
  不支持 UDP `PunchHoleRequest`；它必须保持拒绝且不能绕过认证或触发 Relay 分配。未来若支持，
  必须复用同一认证入口。
- JWT 严格校验 issuer、audience、token type、kid、签名、nbf、exp 和最大长度。
- 通过 Kessoku introspection 感知注销、用户禁用、密码修改和 token 撤销。
- JWKS current/previous key 轮换、缓存、最大陈旧时间和故障行为可配置、可观测。
- 输出结构化 Relay 列表：配置顺序、native 状态、WSS 状态、eligibility、规则引用和探测信息。
- 输入两个 IPv4/IPv6 地址和 transport，返回无副作用的分配结果与决策轨迹。
- 发布 Control API v1、OpenAPI、配置 JSON Schema、UI Schema 和示例 fixture。
- Agent 支持配置读取、validate、plan、apply、history、rollback 和异步 operation 查询。
- 配置写入具备 ETag、幂等、独占锁、原子替换、fsync、runtime ack、健康验证和失败回滚。
- overlay 对干净上游源码重复应用幂等，锚点变化明确失败。
- Linux amd64/arm64、Windows amd64、DEB、容器和现有 WSS/Secure TCP/Geo 功能不回归。

### 1.2 明确不实现

- 不要求被控端账户登录或设备证书，不修改 RustDesk 客户端。
- 不在第一版主动中断已经建立的远控会话；撤销影响后续新连接尝试。
- 不在 HBBR 中再次解析 Kessoku 用户 JWT。
- 不允许 Kessoku 或 Agent 自动控制 Docker、systemd、Windows Service 或宿主机防火墙。
- 不把 21115、Agent 管理端口或内部认证接口直接公开到互联网。
- 不用 `test-geo` 结果替代两台真实客户端的连接与 Relay 验收。
- 不复制 RustDesk Server Pro 的闭源账户、ACL 或管理实现。

## 2. 当前实现与关键缺口

### 2.1 已具备

- `overlay/src/starry_config.rs`：schema v1/v2、严格未知字段拒绝、范围和交叉引用校验。
- `overlay/src/geo_relay.rs` 与 `geo_relay/rules.rs`：Geo rule 和优先 Relay 选择。
- `overlay/src/websocket_signal.rs`：持久 `/ws/id`、跨传输路由和 Relay requirement。
- `overlay/src/websocket_signal/relay_health.rs`：WSS endpoint 探测和健康阈值。
- `scripts/apply_overlay.py` 注入：
  `reload-starry-config`、`reload-geo`、`relay-servers`、`websocket-status`、
  `test-geo <ip1> <ip2> [native|wss|mixed]`。
- 真实 HBBS/HBBR WSS、native 和 mixed 集成测试基础。

### 2.2 必须先修复

1. 当前 `test-geo` 直接调用生产 `get_relay_server`。没有规则命中且有多个候选时，会递增
   全局 `ROTATION_RELAY_SERVER`，所以现有命令不是无副作用模拟。
2. `starry_config::apply_loaded` 遇到空或无效文件会把 `STATE.config` 设为 `None`，导致无效
   reload 关闭 Starry 并回退上游行为，而不是保留 last-known-good。
3. relay 列表更新通过异步 channel 发送，现有 reload 文本响应不能证明 HBBS 已应用同一配置。
4. 现有 21115 管理路径是 1024 字节单次读取、纯文本、一次 `write` 返回，缺少 framing、版本、
   错误码、请求 ID、大小限制和 apply ack，不适合作为远程控制 API。
5. `websocket-status` 是面向人的文本；Kessoku 若解析文本会与实现细节耦合。
6. WSS health 目前保留状态和错误，但结构化清单还缺少探测 latency、绝对观测时间、规则引用
   和统一 native/WSS eligibility。
7. Starry 当前没有 MUST_LOGIN/JWT verifier、JWKS、introspection、撤销缓存或认证指标。
8. `RequestRelay` 必须与 `PunchHoleRequest` 同时纳入强制认证，不能只在常见入口检查 token。

## 3. 目标架构

```text
                            Kessoku API
                       ┌────────┴────────┐
                       │                 │
                 JWKS/introspection   Admin Provider
                       │                 │ mTLS + service JWT
                       ▼                 ▼
RustDesk Client ──> Starry HBBS      starry-control-agent
 token in request     │                  │
                      │                  ├─ config store/history
                      │                  └─ loopback local control v1
                      │                             │
                      ├─ connection_auth            │
                      ├─ relay/runtime snapshot <───┘
                      ├─ pure allocation trace
                      └─ official HBBS/HBBR routing
```

分层责任：

| 层 | 责任 | 禁止事项 |
| --- | --- | --- |
| `connection_auth` | JWT、JWKS、introspection、缓存、模式 | 不查询 Relay/Geo |
| `relay_observer` | runtime snapshot 和稳定 DTO | 不修改运行时 |
| `allocation_explain` | 同一快照上的纯分配解释 | 不递增 rotation |
| `starry_config` | parse、validate、schema、active generation | 不提供远程 HTTP |
| `local_control` | loopback、结构化 runtime RPC | 不做互联网鉴权 |
| `control_agent` | mTLS/API、config transaction、audit | 不含 HBBS 协议逻辑 |

## 4. 跨仓库契约和制品

Starry release 必须附带：

```text
contracts/
├─ control/v1/openapi.yaml
├─ control/v1/examples/
│  ├─ capabilities.json
│  ├─ relays.json
│  ├─ allocation-simulation.json
│  ├─ config.json
│  └─ operation.json
├─ config/v3/config.schema.json
├─ config/v3/config.ui-schema.json
└─ auth/v1/
   ├─ profile.md
   └─ fixtures/
      ├─ active.jwt.txt
      ├─ expired.jwt.txt
      ├─ wrong-audience.jwt.txt
      └─ jwks.json
```

版本规则：

- Agent URL 使用 `/control/v1`；破坏性修改才创建 `/control/v2`。
- v1 响应可以新增字段，不能删除、改名或改变已有字段语义。
- 请求默认 `additionalProperties:false`；新请求能力先通过 capabilities 协商。
- Kessoku 必须根据 capability 而不是 Starry 软件版本推测功能。
- 时间使用 RFC 3339 UTC，ID 使用 UUIDv7，时长字段显式使用 `_ms` 或 `_seconds`。
- 配置顶层 `version: 1|2|3` 与 Control API v1 没有对应关系。

### 4.1 Control API v1

| 方法与路径 | 用途 | Scope |
| --- | --- | --- |
| `GET /control/v1/capabilities` | 协议、实例、能力、限制 | `starry.control.read` |
| `GET /control/v1/status` | HBBS/config/auth 状态 | `starry.control.read` |
| `GET /control/v1/relays` | Relay 清单和资格 | `starry.relay.read` |
| `POST /control/v1/allocations:simulate` | 无副作用分配模拟 | `starry.relay.simulate` |
| `GET /control/v1/config/schema` | JSON/UI Schema | `starry.config.read` |
| `GET /control/v1/config` | 当前磁盘和 runtime 配置 | `starry.config.read` |
| `POST /control/v1/config:validate` | 语法/语义校验 | `starry.config.validate` |
| `POST /control/v1/config:plan` | diff、风险、重启影响 | `starry.config.plan` |
| `POST /control/v1/config:apply` | 异步配置事务 | `starry.config.apply` |
| `GET /control/v1/config/history` | 历史 revision | `starry.config.read` |
| `POST /control/v1/config:rollback` | 异步回滚 | `starry.config.rollback` |
| `GET /control/v1/operations/{id}` | 查询事务结果 | `starry.control.read` |
| `POST /control/v1/runtime:reload` | 类型化 reload | `starry.runtime.reload` |
| `GET /health/live` | 最小存活探针 | 无敏感内容 |
| `GET /health/ready` | 最小就绪探针 | 无敏感内容 |

不提供 `/commands`、`/exec`、任意 `path`、Docker 或 systemd endpoint。

### 4.2 Capabilities

最低响应字段：

```json
{
  "protocol": { "name": "starry-control", "version": "1.0.0", "major": 1 },
  "instance": {
    "id": "0191f6a0-0000-7000-8000-000000000001",
    "role": "hbbs",
    "starry_version": "1.1.16-patch-v1.2.0",
    "upstream_version": "1.1.16"
  },
  "capabilities": {
    "relay_inventory": 1,
    "allocation_simulation": 1,
    "config_transaction": 1,
    "config_rollback": 1,
    "connection_auth": 1
  },
  "config": {
    "supported_schema_versions": [1, 2, 3],
    "active_schema_version": 3,
    "schema_digest": "sha256:..."
  },
  "limits": {
    "max_config_bytes": 1048576,
    "max_plan_lifetime_seconds": 600,
    "operation_retention_seconds": 86400
  }
}
```

`instance.id` 持久化在数据目录。Kessoku 首次登记后固定预期 ID，避免 DNS、证书或网络配置
错误时对另一实例执行操作。

### 4.3 Relay 和模拟 DTO

Relay 必须明确区分：

```json
{
  "id": "relay-asia-1.example.com:21117",
  "configured_order": 0,
  "native": { "state": "online", "observed_at": "2026-08-18T10:00:00Z" },
  "websocket": {
    "configured": true,
    "url": "wss://relay-asia-1.example.com/ws/relay",
    "state": "healthy",
    "last_probe_at": "2026-08-18T09:59:30Z",
    "latency_ms": 83,
    "error_code": null
  },
  "eligible_for": ["native", "wss", "mixed"],
  "referenced_by_rules": ["Asia preference"]
}
```

native 在线来自官方 HBBS Relay 池；WSS 健康来自 TLS/WebSocket 探测。两者都不是完整远控会话
证明，响应中必须带此 warning。

模拟请求：

```json
{
  "client_a": { "ip": "192.0.2.10" },
  "client_b": { "ip": "198.51.100.20" },
  "transport": "mixed",
  "explain": true,
  "expected_config_generation": 42
}
```

响应至少包含：

- `config_generation` 和 `health_snapshot_id`；
- `matched_rule.name/index/direction`；
- 每个 candidate 的 priority、eligible 和排除 reason code；
- `selection.kind`、`relay_id`、`non_binding:true`；
- “没有注册客户端或建立 HBBR 会话”的 warning。

无规则命中的轮转场景只能返回当前候选和 predicted 值，不能承诺真实下一节点。模拟不得调用会
修改 `ROTATION_RELAY_SERVER` 的函数。

### 4.4 统一错误模型

使用 `application/problem+json`：

```json
{
  "type": "https://starry.invalid/problems/config-etag-mismatch",
  "title": "Configuration changed",
  "status": 412,
  "code": "CONFIG_ETAG_MISMATCH",
  "detail": "The configuration changed after the plan was created.",
  "request_id": "0191...",
  "retryable": false,
  "errors": []
}
```

固定错误码至少包括：

```text
REQUEST_INVALID, IP_INVALID, TRANSPORT_INVALID
AUTH_REQUIRED, TOKEN_INVALID, TOKEN_EXPIRED, AUTH_KEY_UNAVAILABLE
CLIENT_CERT_DENIED, SCOPE_DENIED
PLAN_EXPIRED, PLAN_STALE, RESTART_REQUIRED, OPERATION_IN_PROGRESS
IDEMPOTENCY_KEY_REUSED, CONFIG_ETAG_MISMATCH, PRECONDITION_REQUIRED
CONFIG_TOO_LARGE, CONFIG_INVALID, SCHEMA_UNSUPPORTED
LOCAL_CONTROL_PROTOCOL_ERROR, LOCAL_CONTROL_UNAVAILABLE, LOCAL_CONTROL_TIMEOUT
STARRY_NOT_READY, ROLLBACK_FAILED
```

## 5. 建议代码布局

```text
overlay/src/
├─ connection_auth.rs
├─ connection_auth/
│  ├─ claims.rs
│  ├─ jwks.rs
│  ├─ introspection.rs
│  ├─ cache.rs
│  └─ metrics.rs
├─ relay_observer.rs
├─ allocation_explain.rs
├─ local_control.rs
├─ local_control/
│  ├─ protocol.rs
│  ├─ methods.rs
│  └─ types.rs
├─ starry_config.rs
├─ starry_config/
│  ├─ types.rs
│  ├─ parse.rs
│  ├─ schema.rs
│  └─ state.rs
├─ bin/starry-control-agent.rs
└─ control_agent/
   ├─ api.rs
   ├─ auth.rs
   ├─ local_client.rs
   ├─ config_store.rs
   ├─ operations.rs
   └─ audit.rs

overlay/tests/
├─ connection_auth.rs
├─ connection_auth_transport.rs
├─ relay_snapshot.rs
├─ allocation_explain.rs
├─ local_control.rs
├─ control_agent.rs
└─ config_transaction.rs

contracts/
├─ control/v1/
├─ config/v3/
└─ auth/v1/
```

`scripts/apply_overlay.py` 必须：

- 复制所有新增文件和 contracts 构建输入；
- 注入 `mod connection_auth`、`mod relay_observer`、`mod allocation_explain`、
  `mod local_control`；
- 把认证放在 TCP/Secure TCP/WSS 共用业务入口，不复制三套逻辑；UDP unsupported 分支显式
  拒绝并断言不产生分配；
- 为 `RequestRelay` 加入同一认证调用；
- 注入 agent binary target 和需要的 Cargo dependencies；
- 对上游锚点数量做断言；第一次应用成功、第二次零变化；
- 上游关键函数漂移时明确失败，不能静默跳过认证注入。

## 6. 配置 schema v3

建议新增：

```yaml
version: 3

connection_auth:
  mode: audit                 # off | audit | enforce
  issuer: https://api.example.com
  audience: rustdesk-connect
  token_use: access
  required_scope: connect:initiate
  max_token_bytes: 8192
  clock_skew_seconds: 30

  jwks:
    file: auth/jwks.json
    url: https://api.example.com/api/internal/v1/auth/jwks
    refresh_interval_seconds: 300
    max_stale_seconds: 3600

  introspection:
    required: true
    url: https://api.example.com/api/internal/v1/auth/introspect
    timeout_ms: 1000
    positive_cache_seconds: 10
    negative_cache_seconds: 1
    max_cache_entries: 100000
    ca_file: auth/kessoku-ca.pem
    cert_file: auth/starry-client.pem
    key_file: auth/starry-client-key.pem
    server_name: api.example.com
```

规则：

- v1/v2 不允许 `connection_auth`，加载后等价 `mode:off`。
- CLI `--must-login` 或环境变量 `MUST_LOGIN=Y` 启用时，effective mode 恒为 enforce；YAML、
  Agent 和本机文本命令均不能将其降为 audit/off。计划不保留运行时文本 `must-login N` 降级命令。
- v3 的 `enforce` 必须有可加载 Ed25519 JWKS；`required:true` 时必须有完整 mTLS introspection
  配置。缺失时配置校验失败，不能激活。
- 只允许 HTTPS introspection/JWKS URL；测试构建可显式允许 loopback HTTP。
- 私钥/证书字段是文件引用，schema 标为敏感且 UI 只读；API 不读取或返回文件内容。
- positive cache 建议 5～10 秒，negative cache 不超过 1 秒；配置上限必须被校验。
- cache key 是 token SHA-256，不在内存 key、日志或指标 label 保存明文 token。
- JWKS current/previous key 按 `kid` 查找，不尝试所有算法或接受对称 key。
- `mode:enforce` 下 JWKS 超过 `max_stale_seconds` 后拒绝新连接并产生高优先级告警。

Agent 自身配置独立为 `starry-control-agent.yaml`，不允许通过 Agent API 修改：

```yaml
version: 1
instance_id_file: /data/starry/control-instance-id
listen: 0.0.0.0:21120
tls:
  ca_file: /run/secrets/control-ca.pem
  cert_file: /run/secrets/control-agent.pem
  key_file: /run/secrets/control-agent-key.pem
  allowed_client_uri_sans:
    - spiffe://example.com/kessoku/production
service_jwt:
  jwks_file: /run/secrets/control-jwks.json
  audience_prefix: urn:starry-control:
local_control:
  address: 127.0.0.1:21115
config:
  path: /data/starry/config.yaml
  backup_dir: /data/starry/config-history
  max_bytes: 1048576
```

控制面 service JWT 与用户连接 JWT 必须使用不同 issuer audience、keyring 和验证代码配置。

## 7. 本机结构化控制协议

Agent 与 HBBS 共用网络命名空间，通过 loopback 访问。保留现有人工文本命令，但新增有明确魔数
和长度 framing 的 local control v1：

```text
magic:      "STARRYCTL/1\n"
length:     u32 big-endian
payload:    UTF-8 JSON，最大 1 MiB
response:   同样的 magic + length + JSON
```

要求：

- 只有 loopback 连接可以进入 local control；非 loopback 继续走官方 NAT test/协议路径。
- 使用 `read_exact`/`write_all`、读写超时、frame 上限和 UTF-8/JSON 校验。
- 每个请求包含 `request_id`、`method`、`params`；method 是编译期 allowlist。
- 第一版方法只有 `capabilities`、`status`、`relays`、`allocation.simulate`、
  `config.runtime_state`、`runtime.reload`。
- 不在 local protocol 传任意 shell、文件路径或证书内容。
- `runtime.reload` 同步返回 accepted generation、source digest 和各子系统 ack；不能在异步 channel
  send 后立即声称成功。
- Agent 执行配置文件事务，HBBS 只负责解析/激活和返回 runtime ack。
- 文本 `test-geo` 改用纯函数 explain API；保留兼容输出但不再影响生产 rotation。

## 8. 连接认证设计

### 8.1 公共认证入口

新增单一入口：

```rust
async fn authorize_connection_attempt(
    &self,
    token: &str,
    kind: ConnectionAttemptKind,
    transport: SignalTransport,
    effective_ip: IpAddr,
) -> AuthDecision
```

`ConnectionAttemptKind` 至少包括 `PunchHole`、`RequestRelay`；`SignalTransport` 至少包括
`UnsupportedUdp`、`Tcp`、`SecureTcp`、`WebSocket`。`UnsupportedUdp` 只用于明确拒绝与审计，
不是承诺增加 UDP PunchHole 支持。

处理顺序：

1. 在 frame/protobuf 上限内解码；token 为空或超过上限立即分类。
2. 验证 header 只允许 EdDSA，查找明确 `kid`。
3. 验证签名、issuer、audience、token_use、required scope、sub/user_id、iat/nbf/exp 和时钟偏差。
4. 以 token hash 查询 introspection cache。
5. cache miss 调用 Kessoku mTLS introspection；限制 timeout、并发数和响应大小。
6. 只有 local + active 均成功才返回 allow。
7. `audit` 模式记录 would_allow/would_deny 后继续旧行为；`enforce` 模式 deny 立即返回通用失败。
8. 认证通过后才查询目标、记录 punch request、选择 Relay 或向目标转发。

禁止：

- token 非空即放行；
- verifier/key 缺失时自动降级；
- 在日志中打印 token/JTI 全值或把 user id 放进高基数 metrics label；
- local 校验失败后仍调用 introspection；
- 为同一请求在 TCP/Secure TCP/WSS 分别实现不同验证规则，或让 UDP unsupported 分支进入分配；
- 先返回 `ID_NOT_EXIST` 再做认证。

### 8.2 协议响应

现有 RustDesk protobuf 未必有专门的 LOGIN_REQUIRED 枚举。实施前必须对目标客户端版本做协议
trace，选用客户端能够稳定处理的失败值，并在 `other_failure` 使用非敏感、稳定文本。

服务端日志/指标使用内部 reason code：

```text
missing_token, malformed_token, unsupported_alg, unknown_kid,
bad_signature, wrong_issuer, wrong_audience, wrong_token_use,
not_yet_valid, expired, revoked, user_disabled,
introspection_timeout, introspection_unavailable, key_stale
```

客户端响应不能区分“用户不存在”和“token 已撤销”等内部状态。

### 8.3 JWKS、introspection 和缓存

- 启动先加载本地 JWKS cache file，再尝试 HTTPS refresh；`enforce` 没有任何有效 key 时失败。
- 成功 refresh 先解析、校验全体 key，再原子替换并安全写回 cache file。
- 只接受 OKP/Ed25519 public key；拒绝 private material、重复 kid 和未知 use/alg。
- introspection 最多读取固定大小响应；HTTP 200 `active:false` 是正常 deny，不重试。
- 网络/5xx 可在总 deadline 内有限重试一次；cache miss 最终 fail closed。
- 正缓存到期不超过 `min(config_ttl, token_exp-now)`。
- 负缓存很短，避免误伤恢复后的用户；cache 有最大条目和确定性淘汰策略。
- 已建立远控会话不持续 introspect；文档明确撤销只阻止后续新连接。

## 9. Relay 清单与无副作用分配解释

### 9.1 Runtime snapshot

新增不可变 snapshot，单次请求始终使用同一个 generation：

```rust
struct RelayRuntimeSnapshot {
    config_generation: u64,
    health_snapshot_id: Uuid,
    observed_at: SystemTime,
    configured: Vec<ConfiguredRelay>,
    native_online: HashSet<String>,
    websocket: HashMap<String, RelayHealthSnapshot>,
    rule_references: HashMap<String, Vec<String>>,
}
```

WSS health 增加：

- `last_probe_at`；
- `latency_ms`；
- 稳定 error code 与截断后的 message；
- consecutive successes/failures；
- snapshot generation。

状态读取不得持锁跨网络 await；先克隆快照再序列化。

### 9.2 Pure explain

从生产选择逻辑抽取共享纯函数：

```rust
fn explain_relay_selection(
    a: IpAddr,
    b: IpAddr,
    requirement: RelayRequirement,
    snapshot: &RelayRuntimeSnapshot,
) -> AllocationTrace
```

生产选择与模拟共用规则匹配、transport/health 过滤代码；区别只在最终 fallback：

- 确定性规则优先级：模拟返回同一 selected relay。
- rotation fallback：生产路径才 `fetch_add`；模拟返回 candidates 和 predicted index，并标记
  `non_binding:true`，绝不修改 counter。
- 没有候选：返回稳定原因，例如 `native_unavailable`、`wss_unhealthy`、`mixed_ineligible`。

测试应在调用模拟前后读取 rotation counter 和 health/config generations，证明全部未改变。

## 10. 配置生命周期和事务

### 10.1 `starry_config` 重构

将当前加载逻辑拆成无副作用 parse 与有状态 activate：

```rust
fn parse_document(raw: &[u8]) -> Result<ParsedConfig, Diagnostics>;
fn validate_config(config: ParsedConfig) -> Result<ValidatedConfig, Diagnostics>;
fn plan_activation(current: &ActiveConfig, next: &ValidatedConfig) -> ActivationPlan;
fn activate(next: ValidatedConfig) -> Result<ActivationAck, ActivationError>;
```

活动状态包含：

```text
generation, schema_version, source_digest, effective_digest,
activated_at, active_config, subsystem_acks
```

语义：

- 首次启动文件不存在/为空：允许明确进入 `disabled_no_config` 上游兼容状态。
- 已有活动配置后的 reload：空、不可读、YAML 错误、语义错误全部拒绝并保留 last-known-good。
- 若要关闭 Starry，必须提交一份有效配置，将各功能显式设为 off/disabled；不能靠清空文件。
- 新配置只有在 relay pool、Geo、WSS health、Secure TCP 和 connection auth 都完成原子准备后才
  交换 active snapshot。
- 任一子系统准备失败则不交换 generation。

### 10.2 ETag 与 digest

- ETag 基于原始配置 bytes 的 SHA-256，注释或排版变化也会改变。
- `effective_digest` 基于规范化配置，用来判断磁盘与 runtime 是否同步。
- `generation` 用于展示和排序，ETag 才是并发控制依据。
- 外部人工修改文件必须被检测并产生新 ETag/漂移状态。
- JSON Schema endpoint 有独立 ETag，不能复用配置 ETag。

### 10.3 validate、plan 和 apply

`validate` 使用同一个 `parse_document/validate_config`；正常配置错误返回结构化 diagnostics，包含
code、JSON Pointer、YAML line/column 和 severity。

`plan` 绑定：

```text
plan_id, caller, instance_id, base_etag, base_generation,
candidate_digest, schema_version, changes, impact, expires_at
```

plan 默认 10 分钟到期。风险分类至少覆盖 auth mode、JWKS/introspection、Relay 全量变化、trusted
proxy、WSS listener/health 变化。

apply 必须异步并按以下顺序执行：

1. 验证 mTLS/service JWT scope、plan、过期时间、instance、candidate digest 和 `If-Match`。
2. 获取跨进程独占配置锁；同一时间只允许一个 apply/rollback。
3. 持久化 operation/audit intent；失败则不做任何修改。
4. 备份原始文件、权限、owner、ETag 和 runtime digest。
5. 在同一文件系统创建临时文件，设置权限，`write_all`、file fsync、atomic rename、dir fsync。
6. 通过 local control 调用同步 reload。
7. 验证 HBBS accepted generation、source/effective digest 和所有 subsystem ack。
8. 成功后写入 revision manifest 和 operation result。
9. 任一步失败：恢复原始文件，fsync，再次 reload 并验证原 runtime digest。
10. 返回 `succeeded`、`rolled_back` 或 `manual_intervention_required`；不能把 rollback failure 隐藏成普通失败。

若 plan 判定 `restart_required:true`，v1 返回 `409 RESTART_REQUIRED`，不写配置，也不自动重启进程。

### 10.4 History 和 rollback

- 备份目录不允许由 API 参数指定；只使用 Agent 启动配置中的固定目录。
- revision manifest 包含 generation、before/after ETag、digest、actor、comment、时间和 apply result。
- 原始 YAML 可保存到权限受限备份；API 返回前必须按 schema 标记遮盖未来敏感值。
- history 有数量和空间保留上限；删除时不能删当前活动或 last-known-good revision。
- rollback 本质是对历史候选重新执行 validate→plan→apply，不直接覆盖文件。

## 11. Agent 认证、安全和审计

### 11.1 双重身份

跨主机请求必须同时满足：

1. mTLS：CA 信任、服务器证书校验、允许的 Kessoku URI SAN；
2. service JWT：独立 control keyring，`iss/sub/aud/azp/scope/act/iat/nbf/exp/jti`。

service JWT：

- `aud=urn:starry-control:<instance_id>`；
- 最长 5 分钟，时钟偏差不超过 30 秒；
- `azp` 必须与 mTLS 证书映射身份一致；
- `act.sub` 可携带最终管理员 ID 用于审计，但 Agent 不据此替代 scope 校验；
- 连接 JWT 与 control JWT 的 issuer/audience/key 不能互换。

同机部署可以用 Unix Socket/文件权限作为 transport，但仍建议使用短期 service JWT 传递 scope 和
actor。公网只用静态 Bearer token 的部署不受支持。

### 11.2 审计

Kessoku 和 Agent 双边记录，通过 `X-Request-ID`、`traceparent` 和 `audit_id` 关联。

Agent 变更审计至少保存：

```text
actor/service identity, certificate URI SAN, instance_id, action,
before/after ETag, generation, candidate digest, result/error code,
recovery result, idempotency-key hash, comment, timestamps
```

不保存 raw JWT、私钥、证书私钥、完整请求 header 或完整未遮盖配置。audit intent 不能持久化时，
配置变更 fail closed。

### 11.3 幂等和并发

- apply、rollback、runtime reload 要求 `Idempotency-Key`，至少保留 24 小时。
- 相同 key + 相同 request digest 返回同一 operation。
- 相同 key + 不同 request 返回 `409 IDEMPOTENCY_KEY_REUSED`。
- apply/rollback 缺少 `If-Match` 返回 428，不匹配返回 412。
- Agent 重启后 operation、idempotency、backup 和审计状态必须可恢复。

## 12. 分阶段实施

工期按一名熟悉 Rust、异步网络、RustDesk 协议和容器安全的工程师估算。S2/S3 控制面轨道与
S4 认证轨道在 S0/S1 后可以并行。

### S0：协议 trace、contract freeze 和测试夹具（4～6 人日）

任务：

- 对目标 RustDesk desktop/mobile/web 客户端记录 native TCP、Secure TCP、WSS 的
  `PunchHoleRequest` 和 `RequestRelay` 消息序列及 token 行为；同时记录 UDP 请求被拒绝且
  不产生分配的现状。
- 验证未登录、登录、注销后的 token 字段，并锁定 controller-only 语义。
- 固定 auth profile v1、Control OpenAPI v1、错误码、capabilities 和 fixture。
- 给当前 `test-geo` 副作用、无效 reload 清空状态、异步 relay ack 写失败复现测试。

验收：

- 每种 transport 都有可重复 packet/protobuf fixture，不依赖生产 token。
- 证明 `PunchHoleRequest` 与 `RequestRelay` 的所有实际入口已列出。
- 三个当前缺陷测试在修复前稳定失败，而不是只写文档描述。

### S1：配置 core、last-known-good 和 schema v3（7～10 人日）

任务：

- 拆分 parse/validate/plan/activate，增加 Serialize/JSON Schema 支持。
- 实现 active generation、source/effective digest 和 subsystem ack。
- 改变 reload：无效/空 reload 保留当前活动配置。
- 添加 connection_auth v3 schema 和校验，但默认 `mode:off`。
- 生成 config.schema.json、ui-schema 和 fixtures。

验收：

- 启动空配置仍可保持上游兼容；有活动配置后空/无效 reload 不改变 generation/digest。
- v1/v2 兼容测试和 v3 unknown/range/cross-field 测试通过。
- 配置 parser、Agent validate 和 HBBS runtime 使用同一实现。

### S2：纯 Relay snapshot、decision trace 和 local control v1（8～12 人日）

任务：

- 增加不可变 RelayRuntimeSnapshot、WSS latency 和规则引用。
- 抽取 `explain_relay_selection`，生产与模拟共用匹配/过滤逻辑。
- 生产 rotation 与模拟 predicted 分支分离。
- 实现 loopback magic/length-framed local control v1。
- 为 relay pool 更新增加同步 ack，runtime status 返回结构化 DTO。
- 让旧 `websocket-status`/`test-geo` 调用新 core，保留人类可读兼容输出。

验收：

- 模拟前后 rotation、health generation、config generation 完全相同。
- native/wss/mixed、IPv4/IPv6、对称/非对称规则和无候选测试全部通过。
- 1 MiB 边界、短读、半包、超长 frame、错误 JSON、读写超时均不会 panic/挂死。
- local control 只允许 loopback，非 loopback 不暴露任何状态。

### S3：Control Agent 只读 API（8～12 人日）

任务：

- 构建 `starry-control-agent` binary、配置解析、mTLS 和 service JWT verifier。
- 实现 capabilities、status、relays、simulate、config/schema/get 和 health endpoints。
- 输出 OpenAPI、example fixtures、contract lint 和向后兼容 diff CI。
- 增加 Docker sidecar 和 systemd 示例：Agent 与 HBBS 共享 network namespace 和配置 volume。
- 加入 response size、连接/请求 timeout、rate limit、request ID、trace 和审计基础。

验收：

- 无/错误证书、错误 SAN、错误 audience/scope、过期 service JWT 全部拒绝。
- Kessoku 与 Starry 分容器时只通过 Agent 获取结构化数据。
- Agent/HBBS 任一不可达时返回稳定 problem code，不泄露内部路径和证书信息。
- Read-only profile 没有任何可写 endpoint。

### S4：配置事务、operations 和 rollback（10～15 人日）

任务：

- 实现 validate、plan、apply、history、rollback、operation store 和 idempotency store。
- 实现文件锁、备份、原子写、双 fsync、同步 runtime ack 和自动回滚。
- 实现外部文件漂移检测和 ETag 412。
- 为写入、rename、reload、子系统激活、健康验证、恢复 reload 每一步注入故障。
- 加入保留策略、磁盘空间下限和 `manual_intervention_required` 告警。

验收：

- 无效配置绝不修改文件或 runtime。
- 两个并发管理员不能互相覆盖；重复 idempotency key 只生成一个 operation。
- 每个故障点都能恢复原始 bytes、权限、ETag 和 runtime digest。
- Agent 在 apply 中途重启后能恢复或明确进入人工干预状态，不能丢失 operation。
- `restart_required` 不写文件、不调用 Docker/systemd。

### S5：连接 JWT audit 模式（10～15 人日）

任务：

- 实现严格 EdDSA/JWKS verifier、mTLS introspection client、缓存和指标。
- 在共用 PunchHole 入口和 RequestRelay 入口调用 `authorize_connection_attempt`。
- 保持 `mode:audit`，记录 would_allow/would_deny，不改变连接结果。
- 覆盖 TCP/Secure TCP/WSS、UDP unsupported/no-allocation、错误顺序和 target enumeration 测试。
- 进行 Kessoku API 故障、JWKS 轮换、cache eviction 和高并发负载测试。

验收：

- audit 指标能解释所有真实测试连接，没有未分类的协议入口。
- local JWT 失败不请求 introspection；注销/禁用在约定缓存 TTL 内变成 would_deny。
- 日志、heap diagnostic 和 metrics 不包含完整 token。
- introspection 延迟/失败不会无限堆积 task 或耗尽 HBBS 连接资源。

### S6：enforce、真实客户端和安全加固（7～10 人日）

任务：

- 在灰度环境切换 `audit→enforce`。
- 验证已登录/未登录/注销/禁用/密码修改/过期/错误 audience/key rotation。
- 进行认证绕过 fuzz、protobuf/transport 顺序 fuzz、rate limit 和 DoS 测试。
- 明确 emergency rollback 只能通过本机配置和受控 reload，不能从 Kessoku 一键关闭认证。
- 完成 runbook、告警、dashboard 和生产容量基线。

验收：

- 未登录请求在 TCP/Secure TCP/WSS 和直接 RequestRelay 全部拒绝；UDP 请求无论 token
  均保持 unsupported 且不产生分配。
- 登录用户的 native P2P、native Relay、WSS/WSS、WSS/native 双向 mixed 全部通过。
- Kessoku 注销、封禁和密码修改后，新连接最迟在 positive cache TTL 后拒绝。
- JWKS/introspection 故障不会 fail-open；恢复后无需重启即可回到正常状态。

### S7：overlay/release 全矩阵（7～10 人日）

任务：

- 在官方 1.1.16 干净源码重复应用 overlay 两次，第二次无 diff。
- 运行 unit、integration、真实 HBBS/HBBR、Agent、contract 和故障注入测试。
- Linux amd64/arm64、Windows amd64、DEB、容器、SBOM、签名和 provenance。
- 1,000 空闲 WSS、认证并发、Relay 探测、配置 apply/reload 和重连风暴压力测试。
- 在七节点或目标生产拓扑执行故障切换、配置回滚、key rotation 和备份恢复演练。

验收：

- 所有发布阻断项通过并有证据；未测试项目必须标为未测试，不能写成通过。
- Release 附带 OpenAPI/Schema/auth fixture digest 和 Kessoku 兼容矩阵。
- 旧 v1.1.0 配置可升级，回滚到旧镜像前有明确 schema/配置恢复步骤。

## 13. 测试矩阵

### 13.1 认证

| 维度 | 必测值 |
| --- | --- |
| 消息 | PunchHoleRequest、RequestRelay、畸形/未知消息顺序 |
| Transport | UDP unsupported/no-allocation、TCP、Secure TCP、WSS ephemeral、WSS persistent 路由 |
| Token | missing、empty、malformed、oversize、bad signature、expired、future nbf |
| Claims | wrong iss/aud/type/sub-user mismatch、unknown/duplicate kid |
| 状态 | active、logout、revoked、disabled、password reset、deleted user |
| Key | current、previous、removed、JWKS refresh、stale keys |
| API | timeout、TLS failure、5xx、invalid JSON、oversize response、recovery |
| 模式 | off、audit、enforce、invalid enforce config |

每个 deny 测试必须证明目标没有收到 PunchHole/RequestRelay，Relay UUID/地址没有被分配，且响应
不会泄露目标是否存在。

### 13.2 Relay/模拟

- IPv4、IPv6、同 IP、不同地区、缺失 MMDB 数据。
- symmetric true/false、A/B 交换、首规则命中、无规则命中。
- native online/offline、WSS unknown/healthy/unhealthy、mixed eligibility。
- 所有 Relay 不可用、单节点、多个 rotation candidate。
- snapshot 一致性：请求期间 health/config 更新不混合两个 generation。
- simulation 不改变 rotation/health/config/metrics 中的生产选择计数。

### 13.3 配置事务

- YAML syntax、unknown field、range、duplicate、cross-reference、schema 1/2/3。
- ETag missing/mismatch、plan expired/stale、candidate digest mismatch。
- 并发 apply、重复/冲突 idempotency key、外部文件修改。
- 临时文件创建、write、file fsync、rename、dir fsync、reload、ack、rollback 各点失败。
- 磁盘满、权限变化、备份损坏、Agent 重启、HBBS 重启。
- rollback 后文件 bytes、mode、owner、runtime digest、generation 和实际连接功能。

### 13.4 安全

- 非 loopback local control、frame smuggling、partial read、JSON bomb、超大响应。
- Agent 无证书/错误 CA/SAN、service JWT confused deputy、错误 scope/audience/azp。
- path traversal、symlink/hardlink、备份目录逃逸、TOCTOU、任意 URL/命令测试。
- token/config/cert secret 日志扫描和 core dump/metrics 标签检查。
- OpenAPI fuzz 和 property-based config parser 测试。

## 14. CI、分支和 Issue 顺序

建议分支：

```text
main
feature/config-lkg-v3
feature/local-control-v1
feature/control-agent-v1
feature/connection-auth-v1
release/patch-v1.2.0
```

建议 Issue/PR 顺序：

```text
S-001  protocol traces and auth/control contracts
S-002  regression fixtures for test-geo side effect and invalid reload
S-010  starry_config parse/validate/activate refactor
S-011  last-known-good, generation, digests and synchronous ack
S-012  schema v3 and generated JSON/UI Schema
S-020  immutable RelayRuntimeSnapshot
S-021  pure allocation explain and rotation separation
S-022  loopback local control v1 framing
S-030  control-agent skeleton, mTLS and service JWT
S-031  capabilities/status/relays/simulate read-only API
S-032  OpenAPI, fixtures, packaging and deployment examples
S-040  config validate/plan and diagnostics
S-041  atomic apply/operation/idempotency/audit
S-042  history/rollback and fault-injection suite
S-050  EdDSA/JWKS verifier
S-051  introspection client/cache/metrics
S-052  PunchHoleRequest and RequestRelay shared authorization
S-053  all-transport audit mode integration
S-060  enforce mode, fuzz/load and real-client matrix
S-070  release artifacts, SBOM, provenance and rollback drill
```

每个 overlay PR 必须：

- 同时修改 overlay 源、`apply_overlay.py`、单元/集成测试和中英文文档；
- 对干净上游执行两次 apply，第二次无文件变化；
- `git diff --check`、Rust format、clippy、test、contract lint 通过；
- 不降低现有 WSS、Secure TCP、Geo、Relay health 的限制或测试覆盖；
- 认证注入锚点变化时 CI fail，不能生成缺少认证的“成功”制品。

## 15. 部署和灰度

建议发布顺序：

1. 发布 config LKG、结构化 snapshot 和 local control，不启动 Agent。
2. 部署只读 Agent，Kessoku 接入 Relay/模拟页面。
3. 开启 Agent 配置写能力，在测试环境完成 apply/rollback 故障演练。
4. Kessoku 先发布新 EdDSA token、JWKS 和 introspection，客户端重新登录。
5. Starry 部署 connection auth，但保持 `mode:audit` 至少一个完整业务周期。
6. 对比 missing/invalid/would-deny 指标与真实客户端版本，修复全部非预期拒绝。
7. 单实例/小用户组切 `enforce`，再逐步扩大。
8. 最后才在生产 Kessoku UI 开放 config apply；read-only 和 auth enforcement 不必同时切换。

Docker 建议：

- Agent sidecar 与 HBBS 使用同一 network namespace，使其可访问 loopback local control；
- Agent 与 HBBS 共享 Starry config volume，但只有 Agent 需要配置写权限；
- Agent 21120 只加入 Kessoku 私有网络，不映射公网；
- TLS/JWT 私钥使用 Docker/Kubernetes secret，以只读文件挂载；
- HBBR 容器和公网 21117/21119 保持现有职责。

## 16. 发布阻断项

出现任一情况不得发布：

- `enforce` 配置不完整时仍启动为放行状态。
- PunchHoleRequest 或 RequestRelay 任一路径能绕过认证。
- 未认证请求能够区分目标不存在和认证失败。
- introspection/JWKS 故障导致 fail-open。
- 模拟会改变 rotation 或任何生产状态。
- 无效 reload 清空 last-known-good。
- apply HTTP 成功但 runtime generation/digest 未确认。
- rollback failure 被隐藏，或 Agent 重启后丢失 operation/audit intent。
- Agent 暴露 shell、任意路径、Docker Socket、裸 21115 proxy 或仅静态 Bearer 鉴权。
- overlay 第二次应用有变化，或上游锚点漂移后 CI 仍成功。
- 真实 native/WSS/mixed 客户端矩阵、升级和回滚没有证据。

## 17. 回滚方案

### 17.1 认证

- 正常回退：通过本机受控配置把 `enforce` 改为 `audit` 并同步 reload。
- Kessoku/Agent 远程 UI 不提供“一键关闭认证”，避免控制面账户失陷后关闭门禁。
- 保留 current/previous JWKS 直到最长 token TTL 结束；不要随容器回滚删除 key cache。
- auth 配置失败时保留上一活动配置；不得自动降级到 off。

### 17.2 配置

- Agent 保存 last-known-good 原始 bytes、manifest 和 runtime digest。
- apply 失败自动 rollback；`manual_intervention_required` 时停止接受后续写请求。
- 人工恢复后先用 local control 验证 runtime digest，再恢复 Agent write readiness。
- 回滚旧 Starry 镜像前，把 config v3 转换为旧版本可读取的 v2 文件，或使用发布前备份。

### 17.3 Agent

- Agent 可以独立停止；HBBS/HBBR 数据面继续使用最后活动配置运行。
- Agent 不可达时 Kessoku 只失去管理能力，不应导致现有远控会话中断。
- Agent write profile 可切回 read-only，不重新开放旧任意文本命令的远程代理。

## 18. 工作量和关键路径

| 工作流 | Starry 估算 | 关键依赖 |
| --- | ---: | --- |
| Trace、契约、缺陷夹具 | 4～6 人日 | 目标客户端版本 |
| Config LKG/schema v3 | 7～10 人日 | 无 |
| Relay snapshot/纯模拟/local control | 8～12 人日 | Config generation |
| Read-only Agent/API/packaging | 8～12 人日 | local control v1 |
| Config transaction/rollback | 10～15 人日 | Agent + LKG |
| JWT/JWKS/introspection/audit | 10～15 人日 | Kessoku auth v1 |
| Enforce/fuzz/load/真实客户端 | 7～10 人日 | Kessoku issuer + audit 数据 |
| Release 全矩阵 | 7～10 人日 | 全部功能完成 |

单人串行约 12～16 周。Control 与 JWT 两条轨道在 S1 后可由两名工程师并行，但共享的
`rendezvous_server` 注入、config generation 和真实协议测试必须在同一 release branch 集成。

## 19. 完成定义

只有同时满足以下条件，才可宣布本计划完成：

- config v3、last-known-good、generation/digest 和同步 subsystem ack 已落地。
- Relay 清单结构化，双 IP 模拟被证明无副作用，并与生产选择逻辑共享核心。
- Control Agent v1、OpenAPI/Schema、mTLS/service JWT、审计和配置事务全部通过故障注入。
- JWT 对 PunchHoleRequest/RequestRelay 的 TCP/Secure TCP/WSS 全路径生效；UDP unsupported
  路径已证明不能绕过或触发分配。
- Kessoku 注销、禁用、密码修改和 key rotation 在约定 TTL 内正确反映。
- audit→enforce 灰度、真实客户端、混合 Relay、Agent 故障和配置回滚均有可重复证据。
- overlay 幂等、跨架构构建、容器/DEB/Windows、SBOM、签名和 provenance 通过。
- 运维、安全、升级、回滚和跨仓库兼容矩阵完整发布。
