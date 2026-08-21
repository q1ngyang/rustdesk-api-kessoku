# Kessoku v2.8.2 × Starry patch-v1.2.0 联合开发状态与后续计划

> 状态日期：2026-08-21
> 状态：Starry 正式版已发布并固定；Kessoku 为本地 v2.8.2 发布候选，最终确认前不得推送或发布

## 1. 当前基线

| 项目 | 当前本地检查点 | 对齐目标 |
| --- | --- | --- |
| `rustdesk-api-kessoku` | 从本地恢复检查点 `de57744` 收敛的受审核候选；审计基线 `c5687e1` | Kessoku v2.8.2；Docker/Linux x86_64 优先 |
| `rustdesk-server-starry` | 正式 tag `1.1.16-patch-v1.2.0`；commit `5e73b3af1423acf5ee20ca32a2d747eef6df3494` | `patch-v1.2.0` HBBS overlay 与 Linux Control Agent |
| Control API | `control/v1` 正式契约 | OpenAPI SHA-256 `f42714264d61408c8d6c709efcf87d869b9422ca83fb5c88a9735cc5a02a5e68` |
| Starry 配置 | `config/v3` | JSON Schema SHA-256 `425c1bafe956a256caff5ad761731583d31b9eee067bad9341cc53211ea19df3` |

`internal/starrycontrol/CONTRACT_VERSION` 已记录 `status: PINNED`、正式 Starry tag/source
commit、Control/config schema 哈希，以及 GHCR image index/linux-amd64 digest。正式
Control OpenAPI 与此前本地候选 digest 完全一致。

## 2. 已完成的联合能力

| 领域 | Kessoku | Starry | 本地证据 |
| --- | --- | --- | --- |
| 管理权限边界 | 所有 Control 路由要求 `AdminPrivilege`；legacy ServerCmd 默认不注册，显式兼容模式也只返回 `410` | 不提供通用命令、任意路径或进程执行端点 | 路由枚举测试、固定 route/method/scope 客户端测试 |
| Access JWT 生命周期 | EdDSA、`typ=at+jwt`、独立 `kid`、数值 `user_id`/十进制 `sub`、UUID JTI/hash 存储、撤销、`auth_version`、注销/禁用/改密失效 | native TCP、Secure TCP、WSS 对两类连接请求按同一精确 wire contract 验证；UDP 明确 unsupported | Kessoku 实际签发 token 测试、固定跨仓 auth fixtures、Starry 真实进程 transport 矩阵 |
| 内部信任边界 | 独立 TLS 1.3 mTLS JWKS/introspection listener、SAN allow list、限流与 fail closed | JWKS 与 introspection 均强制 TLS 1.3，固定显式 CA/client cert/key/精确 DNS identity，禁用系统根与跳转；token-only introspection DTO | 双方单元/集成测试、服务端验证 client cert 的 JWKS refresh 功能测试与固定 auth fixtures |
| 管理信任边界 | 固定 HTTPS origin、CA、server name、client cert；每请求签发最小 scope 的短期 EdDSA JWT；`azp` 必须等于 client cert URI SAN | Agent 同时要求可信 client cert、允许的唯一 URI SAN、正确 `azp`/aud/iss/kid/scope | 真实 Agent↔Kessoku E2E |
| Relay 与模拟 | 类型化 inventory/simulation DTO，不接受任意 cmd/option/URL | 不可变 Relay/health snapshot 与无副作用选择 trace | 实际响应 fixture、真实 HBBS simulation |
| 配置读取与 Schema | 校验 exact YAML 的 SHA-256/ETag、runtime drift、Schema digest 与 capability digest 一致 | `GET /config` 返回精确 UTF-8 YAML、format、runtime state 和 strong ETag；Schema bundle 显式建模 | 跨仓 contract fixture 与真实 Agent 调用 |
| 配置事务 | typed validate/plan/apply/operation/history/rollback/reload；16–128 字节幂等键；审计不记录 secret | 乐观并发、plan 绑定、原子替换、fsync、HBBS ack、自动恢复、持久 operation/audit、人工 rollback | 成功 apply/rollback/reload 跨仓 E2E；失败 apply/outage/restart Starry 黑盒测试 |
| 管理前端 | 审核候选 `2a9d037fc271cf96b39fd4add4b97c4ff4477f12` 已内置为 `admin-web/`；仅类型化 Control DTO；DOMPurify；严格 CSP/禁止嵌入 | 浏览器不直连 Agent，所有权威状态与变更仍由 Agent 返回 | 与后端同 commit；`npm ci`、9 tests、0-vulnerability audit、110 package signatures、双构建一致、62-component CycloneDX/license、Gitleaks、完整浏览器交互 QA |
| WebClient 边界 | 仓库自有 MIT `web-client/`，强制 Relay WSS/VP9/鼠标/基本键盘；独立 origin 与短期内存 grant；历史 WebClient2/V2 永久排除 | 提供 patch-v1.2.0 WSS Rendezvous/Relay 兼容路径 | 46 tests、双前端 packaging/policy，以及正式 Starry 镜像上的双 origin 浏览器/forced-Relay VP9/真实远端输入验收通过 |

## 3. 本次联合验证结果

- Kessoku（精确 Go 1.26.6）：格式、`go mod verify`、`go vet ./...`、`go test ./...` 与
  `go test -race ./...` 全部通过；SQLite、MySQL 8.4.2、PostgreSQL 16.4 的真实 legacy
  migration 均通过；Swagger 已从当前 DTO/路由重新生成。
- Kessoku 管理端 logout 已移到 `BackendUserAuth` 之后注册；真实 token 的路由测试确认无 token
  返回 `401`、有效 token 返回 `200` 并持久化 `revoked_reason=logout`、同一 token 随后返回
  `401`。所有 Control 路由仍要求 `AdminPrivilege`，legacy ServerCmd 默认不注册且无法透传
  任意 `cmd`/`option`。
- Kessoku 漏洞与供应链：`govulncheck v1.7.0` 为 0 reachable、0 imported-package；仅有
  未导入的 `golang.org/x/crypto/openpgp` 模块级 `GO-2026-5932`，上游当前无修复版本。
  actionlint、shell 语法、Gitleaks 395-commit 历史扫描与 Syft 源码 SBOM 通过；历史
  `resources/web`/`resources/web2` 已删除，并被全部 runtime/制品路径永久拒绝。
- Kessoku 候选构建不再以单文件 `cmd/apimain.go` 产生 `command-line-arguments` 二进制；正式
  路径统一编译 `./cmd`。候选 workflow 会在创建未跟踪产物前验证干净 Git 树，并保存、复核
  `GO-BUILD-INFO.txt` 中的模块路径、精确 source SHA 与 `vcs.modified=false`；该机制已在临时
  干净 Git 快照及精确 Go 镜像中实测通过，同一提交的两次独立构建 SHA-256 完全一致；候选
  CI 也会执行第二次构建并逐字节比较。排除 `.git` 的 `Dockerfile.dev` 被明确标记为非发布
  源码构建，发布镜像只消费已验证候选二进制。
- Kessoku 管理前端本地审核候选提交为
  `2a9d037fc271cf96b39fd4add4b97c4ff4477f12`。旧任意 ServerCmd 与全部嵌入式
  旧 WebClient/WebClient2 源码已删除；版本化 Control 页面覆盖 JWT 状态、Relay/模拟、
  YAML/Schema、校验、plan/apply、operation、rollback/reload 与脱敏审计。精确 Node/npm 下
  `npm ci`、9 tests、生产 audit 0 漏洞、110 package signatures、双构建一致、
  62-component CycloneDX/license、Gitleaks 与本地浏览器 QA 均通过；欢迎 Markdown XSS
  payload 已验证被净化。该源码现位于 Kessoku `admin-web/`，候选 CI、开发 Dockerfile 和
  本地候选均只从同一 Kessoku commit 构建，不再依赖独立前端仓库。
- 仓库自有 Web Client 在固定 Chromium 与 RustDesk 1.4.9 Linux 目标上，从零启动精确正式
  Starry 镜像完成双 origin 验收：直接登录和 admin ready/grant/accepted popup 均建立
  forced-Relay WSS 加密会话，VP9/WebCodecs 画布为 1280x800，远端实际接收鼠标 320x240、
  `K`、`Control_L` 与 `Ctrl+S`，logout 撤销成功；浏览器没有 local/session storage、
  IndexedDB 或 service worker。该结果是正常功能兼容验证，不是渗透、fuzz 或压力测试。
- `scripts/verify-local-admin-candidate.sh` 快照当前前后端同一源树。完整本地候选通过 Go
  binary、前端、tar、DEB 双构建比较，非 root
  runtime image smoke，以及实际 CSP/防嵌入/禁止目录枚举和旧配置泄露路由 `404`；脚本不含
  push/release 命令并清理所有临时容器、镜像和文件。
- 跨仓候选契约校验 Starry OpenAPI digest、9 组响应样例、config/UI Schema，并由 Kessoku
  的真实响应验证器解码；通过。真实联合 E2E 启动 HBBS 与 Control Agent，临时生成 CA、
  server/client cert、URI SAN、JWKS；Provider 完成 capabilities、status、relays、双 IP
  simulation、config/schema、valid/invalid validation、plan、异步 apply、operation poll、
  history、rollback 与 runtime reload；通过。
- 正式发布后的本机复核已下载全部 Starry Release 资产并通过官方 `SHA256SUMS`；tag 与
  source commit 一致，正式 OpenAPI/config/UI schema 与 Kessoku 固定哈希一致。官方 GHCR
  `1.1.16-patch-v1.2.0` 与 `latest` 指向同一 index/amd64 digest，镜像命令入口 smoke 通过。
  官方 release HBBS 通过 native/Secure TCP/WSS 的 enforce/audit 传输矩阵；UDP 保持明确
  unsupported。使用官方 release HBBS/Control Agent 二进制运行 Kessoku Provider 的
  capabilities、status、Relay、simulation、schema、validate、plan/apply/poll/history/
  rollback/reload 真实进程 E2E，以及失败 apply 自动恢复、只读 Agent 和审计检查均通过。
  Starry 自身并发测试的 150ms 延迟仅在 debug build 生效；release-aware 临时夹具允许快速
  完成时出现等价 `PLAN_STALE`，而不是只接受 `OPERATION_IN_PROGRESS`。该调整仅存在于
  `/tmp` 测试夹具，正式制品的 HBBS/Agent 哈希在执行前后保持官方值。
- RustDesk 1.4.9 双客户端验收在刷新公开 JWKS cache 元数据、通过 Kessoku 正常登录签发
  新的 10 分钟连接 token 后，对正式 Starry 镜像完成五条强制 Relay 桌面路径：`audit`
  native/native，以及 `enforce` native/native、WSS/WSS、WSS/native、native/WSS。每条路径
  均验证 Remote Desktop 窗口、截图和已建立 HBBR 连接，并核对正式 tag、source revision、
  image digest 及 HBBS/HBBR/Agent 哈希。该结果不表示覆盖直接 P2P 或独立 Secure TCP case。
- Starry 最新认证修复后的 75 个普通库测试均通过，全部 integration target 编译成功；按约定
  的“不做本地 fuzz/主动安全测试”边界，独立的确定性 mutation-corpus 单元本轮未运行。
  针对性真实进程 native TCP、Secure TCP、WSS、持久 WSS 路由、严格 introspection DTO 与
  TLS 1.3 mTLS JWKS refresh 均通过；此前专用高文件描述符门禁完成 1,000 个已注册 idle WSS
  会话及 100 个断线重连 replacement，普通 WebSocket、local control、mixed Relay、数据库
  并发与真实 Control Agent↔Kessoku E2E 也已通过。
- Starry 固定 `Cargo.lock` 的 RustSec 审计结果为 0 vulnerability、0 unsound、0 yanked；保留
  一个已披露的 upstream-core `sodiumoxide 0.2.7` unmaintained warning。已将 SQLx/deadpool
  数据库路径迁移为 bundled SQLite 的 `tokio-rusqlite`，并更新 TLS/WebSocket/JWT/HTTP
  依赖；锁文件由 overlay 强制复制并在 metadata 前后比较。
- Starry 本地 `x86_64-unknown-linux-gnu`、`x86_64-unknown-linux-musl`、
  `aarch64-unknown-linux-musl` 的四个 release binaries 均构建通过。八个 amd64/arm64 DEB
  连续构建两次字节完全一致；amd64 原生安装运行与 arm64 foreign-architecture 安装后使用
  固定 cross image 提取的 QEMU 执行均通过。amd64 发布镜像的四命令与 HBBS 外部配置引导
  smoke 通过。根据新的发布范围，amd64 是 Starry patch-v1.2.0 唯一阻断平台；已有 arm64
  构建/QEMU 结果仅作为尽力兼容证据，不要求进入发布候选。
- overlay 连续应用两次均成功且没有产生第二次变更。contract/docs/workflow、actionlint、
  shell 与 overlay `rustfmt --check` 通过；workflow 检查器确认 35 个
  Action 使用均固定完整 SHA，Rust 1.97.1、cross revision、cross images、RustSec 数据库与
  扫描工具均为不可变输入。Gitleaks 历史/WIP 扫描无未解释发现，Syft 源码 SBOM 已生成。
- 历史 Codex Security 密封快照已完成两仓库静态审计，全程未启动公开服务、未发送探测流量，也未运行
  fuzz/mutation/压力或利用测试。Kessoku 冻结快照覆盖 27 个 surface、记录 23 项 finding；
  Starry 冻结快照覆盖 40 个 surface、记录 22 项 finding（6 medium、16 low，无 high/critical）。
  Starry 已针对主要认证、边界与资源上限问题实施修复；Kessoku 精确本地候选已逐项静态
  复核并记录唯一获批 residual risk。当前会话再次获准安装插件，但扫描器仍不可调用，
  因而不声称插件已重扫最终树；最终批准提交仍需通过受保护候选 CI。

## 4. 后续联合计划与退出条件

| 优先级 | 工作包 | 负责人/依赖 | 退出条件 |
| --- | --- | --- | --- |
| 完成 | 发布并固定 Starry contract | Starry + Kessoku | `1.1.16-patch-v1.2.0` 已发布；Kessoku 已把同一 OpenAPI digest 提升为 `release_sha256`、设为 `PINNED`，并以官方源码/二进制复核合同与 Provider E2E |
| 本地完成 / CI 待完成 | 验证 Kessoku v2.8.2 单仓候选 | Kessoku；前端已内置 | 本地同源快照已重跑 Go/迁移、`admin-web` 9 tests、`web-client` 46 tests、audit/signatures、双构建、SBOM、Docker linux/amd64、amd64 DEB 安装与正式 Starry 浏览器互操作；最终批准提交仍需受保护 CI |
| 完成 | 正式 Starry 真实客户端 Relay 验收 | Kessoku + Starry | RustDesk 1.4.9 已通过 `audit` native/native 与 `enforce` native/native、WSS/WSS、WSS/native、native/WSS；每项验证桌面窗口、截图与 HBBR 连接，不外推为直接 P2P/独立 Secure TCP 证据 |
| 部署前 P0（非软件发布阻断） | 生产形态恢复演练 | 运维 staging | DB/config/identity/key 备份恢复、Agent read-only→write、ETag conflict、自动 rollback、人工 rollback、token mass invalidation 和重新登录均完成，并由具体部署记录 RTO/RPO |
| P1 | 正式平台/制品矩阵 | 两项目 CI | Docker linux/amd64、Linux x86_64 tar/二进制和 amd64 DEB 完成最终候选 SBOM/provenance/签名/checksum 与干净 CI；ARM/Windows 仅非阻断兼容，不进入本版本承诺 |
| P1 | 安全 finding 闭环与韧性验证 | 两项目 + 获批安全/CI 环境 | 以已完成的 Codex Security 密封静态审计为基线，在精确候选上逐项确认修复或 residual risk；auth/protobuf fuzz、长时间重连/并发负载和 secret/heap diagnostics 不在本地执行，并在已通过 1,000 idle WSS 与 100 次 replacement 基线之上留存外部证据 |
| P1 | 目标拓扑故障演练 | 两项目 + 运维 | 七节点或实际目标拓扑完成 Relay/HBBS failover、key rotation、upgrade 与 rollback |

## 5. 不得跨越的门禁

- 在 Kessoku v2.8.2 同提交前后端候选未通过受保护候选 CI 时，不发布完整 Kessoku
  release；固定的 Starry tag/digest 不得改回移动引用。
- 不从 `lejianwen/rustdesk-api-web@master` 或任何移动分支构建管理前端。
- 不加入、复制、打包或通过 plugin 规避 WebClient2 的非授权闭源资产。
- 只打包仓库自有 `web-client/` 的 `resources/client` 产物；该 MIT MVP 与历史
  WebClient2/V2 没有源码或资产继承关系。
- 不把任意 command、option、URL、文件路径或 Docker socket 暴露给管理请求。
- 已完成的 Codex Security 冻结快照不得冒充最终候选批准；发布前必须在精确候选上完成
  finding 闭环、证据审核与 residual-risk 签字。
- 本检查点不推送 GitHub，不发布镜像、DEB、archive、tag 或 release。
