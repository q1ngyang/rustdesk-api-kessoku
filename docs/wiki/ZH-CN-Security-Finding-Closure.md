# 安全发现闭环

[English](Security-Finding-Closure.md) | **简体中文**

本文记录 Kessoku v2.8.2 发布决策使用的防御性、纯静态安全审查。它不是渗透测试报告，
也不包含利用说明。

## 证据边界

- Kessoku 封存审查快照为
  `codex-security-snapshot/v1:sha256:d504807864f052238881f7e0e18548763d8e1b0134567f95ee0d08b497bef68d`，
  覆盖 27 个 surface、记录 23 项 finding。
- Starry 封存快照为
  `codex-security-snapshot/v1:sha256:4b5ffa3ce6bc819a9a72e9f6e9ec7fd9dc63c0aee4c74645b1d67472d5b6aaac`，
  覆盖 40 个 surface、记录 22 项 finding（6 medium、16 low），没有 high/critical。
- Codex Security 插件安装已获批准，但最终 Kessoku 候选会话中没有出现可调用的扫描器。
  因此，封存快照属于历史证据，不能表述成插件已对最终工作树重新扫描。
- 2026-08-21，发布负责人接受了这一证据边界，并取消“重新运行插件”这一发布前置条件。
- 精确修复后源码由静态代码复核，以及常规功能、race、迁移、前端、容器、打包和真实
  客户端兼容性测试覆盖。本证据不包含渗透、利用、fuzz/mutation、压力或公网目标测试。

## Kessoku finding 处置

| Finding | 处置 | 发布证据 |
| --- | --- | --- |
| `KS-ADMIN-LOGOUT-REVOCATION` | 已关闭 | 注销与管理员生命周期变更会撤销数据库会话并轮换认证版本。 |
| `KS-ANONYMOUS-AUDIT-MUTATION` | 接受残余 | RustDesk 1.4.9 audit/sysinfo 上传没有认证头；保留的有界兼容路由见下文。 |
| `KS-ANONYMOUS-PEER-STORAGE` | 已关闭 | 请求大小、数量和字段均有上限；请求不能重分配已持久化 peer 身份或所有权。 |
| `KS-BOOTSTRAP-PASSWORD-LOG` | 已关闭 | 启动时生成不可达随机凭据；运维通过 mode `0600` 文件设置密码，日志不输出可复用密码。 |
| `KS-CAPTCHA-ALLOCATION` | 已关闭 | CAPTCHA 状态有界，客户端绕过路径已删除。 |
| `KS-CSV-FORMULA-INJECTION` | 已关闭 | 可能被电子表格解释为公式的导出单元格会被中和。 |
| `KS-DB-BOOTSTRAP-TLS` | 已关闭 | 外部 MySQL 强制验证 TLS，PostgreSQL 强制 `verify-full`；DSN 能正确保留编码后的凭据。 |
| `KS-LDAP-FILTER-INJECTION` | 已关闭 | LDAP 查询值在进入 filter 前会转义。 |
| `KS-LDAP-IDENTITY-COLLISION` | 已关闭 | provider/subject 与 provider/user 身份使用唯一约束。 |
| `KS-LDAP-INSECURE-TRANSPORT` | 已关闭 | LDAP 要求证书验证 TLS，并拒绝不安全传输配置。 |
| `KS-OAUTH-BIND-STATE` | 已关闭 | 绑定状态与发起认证的用户绑定，跳转完成前先保存已验证身份。 |
| `KS-OAUTH-CACHE-AMPLIFICATION` | 已关闭 | OAuth 状态数量/寿命、state/code 长度和 provider 响应体均有界。 |
| `KS-OIDC-ISSUER-SSRF` | 已关闭 | Provider endpoint 必须为公网 HTTPS，拒绝本地/私网地址和 redirect，并使用有界客户端。 |
| `KS-OIDC-STATE-TRANSFER` | 已关闭 | Callback state 只能原子 claim 一次，登录结果仍与发起设备 ID/UUID 绑定。 |
| `KS-OIDC-UNVERIFIED-EMAIL` | 已关闭 | OIDC 身份以必需的 ID-token subject 和完全一致的 UserInfo subject 为准，不依赖未验证邮箱。 |
| `KS-PEER-IDENTITY-HIJACK` | 已关闭 | 地址簿/peer 元数据读写按认证 owner 隔离，丢弃请求中的 row/user/collection ID。 |
| `KS-REGISTRATION-STORAGE` | 已关闭 | 注册输入与状态有界，安全默认值关闭公网注册。 |
| `KS-REQUEST-CARDINALITY` | 已关闭 | 持久化前限制 API body、peer、tag、batch、字段和序列化元数据大小。 |
| `KS-STARRY-ASYNC-AUDIT` | 已关闭 | Provider 工作前记录控制意图，之后用关联 ID 结束 success/failure。 |
| `KS-STARRY-OPERATION-BINDING` | 已关闭 | 类型化 DTO 把操作绑定到 deployment、actor、预期 operation/plan、ETag 与幂等信息。 |
| `KS-STARRY-RELOAD-DIGEST` | 已关闭 | 只有 reload/apply 响应给出非零且符合预期的 generation/digest 才接受成功。 |
| `KS-TRUSTED-PROXY-DEFAULT` | 已关闭 | 默认不信任代理，部署必须配置精确代理地址。 |
| `KS-USER-DIRECTORY-OVEREXPOSURE` | 已关闭 | 普通用户/组目录使用最小 DTO，不返回管理或认证字段。 |

## 接受的兼容残余

RustDesk 1.4.9 的连接审计、文件传输审计和 sysinfo 上传请求不携带认证头；删除这些路由会
破坏支持客户端的现有行为。Kessoku 因此保留很窄的兼容 surface，并实施：

- 64 KiB 请求上限和严格字段限制；
- 必须精确匹配已持久化的 peer ID 与 UUID；
- 请求不能修改 owner；
- 删除已存审计记录时另写独立管理员审计事件。

Peer UUID 不是秘密。已经知道有效 peer ID 与 UUID 的人仍可能提交伪造的兼容审计记录。
这些记录应视为运维 telemetry，而非不可抵赖证据；需要更强审计保证时，应导出到
append-only 或不可变存储。v2.8.2 为兼容性接受这一残余；受支持 RustDesk 客户端提供带
认证的审计上传后必须重新评估。

## 已发布 Starry 兼容性证据

本机发布矩阵使用已发布镜像
`ghcr.io/q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0`，repository digest 为
`sha256:3685543aee6e60c27bed5db1df2fa32af83e61a58e9bc4c0ea3464664863811b`，源码 revision
为 `5e73b3af1423acf5ee20ca32a2d747eef6df3494`。使用前已核对官方 HBBS、HBBR 与
Control Agent 二进制 hash。

使用 RustDesk 1.4.9，矩阵通过 Starry `audit` native-to-native；再在 `enforce` 下覆盖
native-to-native、WSS-to-WSS、WSS-to-native、native-to-WSS 控制端/被控端组合。每项都
打开 Remote Desktop 会话并观察到预期 HBBR Relay 连接。这是正常兼容性验证，不代表进攻
性安全评估。

## 发布决策规则

发布负责人必须能看到上述已接受残余、数据库/OAuth 迁移预检、精确制品身份和所有常规
候选检查。负责人已通过受审核流程批准源码候选；发布仍由 `RELEASE-PROCESS.md` 中的
不可变 tag 与受保护候选工作流保持 fail-closed。
