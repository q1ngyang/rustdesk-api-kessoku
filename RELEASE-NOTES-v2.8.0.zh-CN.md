# Kessoku v2.8.0 发布说明（未发布）

[English](RELEASE-NOTES-v2.8.0.md) | **简体中文**

v2.8.0 将原通用 RustDesk API 服务收紧为有明确边界的账户与管理控制面，并面向
`rustdesk-server-starry patch-v1.2.0` 配套使用。

本文是供发布负责人确认的内容草案，不表示 tag、镜像、Wiki、package 或 GitHub Release
已经发布。

## 推荐 Docker 部署

推荐在 Linux amd64 使用 Docker Compose。

- [GHCR 镜像页面](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [容器镜像指南](CONTAINER.zh-CN.md)
- [Docker 部署文档](docs/wiki/ZH-CN-Docker-Deployment.md)
- [Compose 范例](docker-compose.yaml)
- [环境变量范例](examples/compose.env.example)
- [Starry 集成文档](docs/wiki/ZH-CN-Starry-Control.md)

本版本把 `ghcr.io/q1ngyang/rustdesk-api-kessoku:v2.8.0` 与 `:latest` 发布为同一镜像。
版本 tag 不可变；`latest` 只会在稳定版成功发布后移动。生产部署应解析并固定版本 tag 的
digest。

## v2.8.0 新特性

### 认证与令牌生命周期

- Ed25519/EdDSA access token，固定校验 `at+jwt` 类型、kid、issuer、audience、
  subject/user 绑定、JTI、scope、认证版本和有界时间字段。
- current/previous JWKS 重叠机制，支持受控密钥轮换。
- 新签发凭据只存 token hash，不再存可复用明文 token。
- 单会话注销、密码变更、用户禁用与全局会话失效具有明确撤销语义。
- 独立 TLS 1.3 mTLS 内部 listener 只向精确批准的 Starry 证书身份提供有界 JWKS
  和 token introspection。
- OIDC 要求非空 ID-token subject，并与 UserInfo subject 完全一致。Callback state 只能
  原子 claim 一次；provider body/code 有上限；拒绝尾随 JSON；callback origin 必须是固定
  的公网 HTTPS origin。应用代理无法保留目标地址校验，因此 OAuth/OIDC 拒绝代理模式。
- provider/subject 与 user/provider 绑定唯一；升级预检会报告旧重复绑定，不会静默删除或
  合并。

### 类型化 Starry 管理

- Starry 实例 origin 和凭据文件引用由部署固定，浏览器不能选择 Agent URL。
- 能力、状态、Relay 列表，以及绑定明确非零配置 generation 的无副作用双地址分配模拟。
- 通过 Control API v1 提供配置 schema/read/validate/plan/apply/operation/history/
  rollback 与同步 runtime reload。
- 每个请求同时使用 mTLS 和短期最小 scope service JWT。
- ETag、plan 身份、idempotency、响应大小、超时、错误标准化与脱敏意图/结果审计边界。
- 默认只读控制。

### 管理与前端供应链

- 所有控制路由仅管理员可达。
- 通用 legacy ServerCmd 执行已从运行时 API 删除；兼容端点只能返回 `410 Gone`。
- 管理前端源码位于 `admin-web/`，与后端共享 commit/tag，并从 lockfile 使用 `npm ci`
  构建。
- 不使用移动前端分支，不允许外部替换编译产物。
- 用户创建/删除、角色/状态变更、会话撤销和审计记录删除都会留下管理员审计事件。角色或
  状态变更会撤销现有会话，数据库级共享不变量可防止并发操作删除最后一个启用管理员。
- 地址簿、collection、tag 与 peer 元数据按 owner 隔离；不接受客户端持久化 ID 或嵌套
  ORM association；地址簿/tag 同步在同一事务中提交，tag 只能是有界字符串数组。

### 浏览器客户端边界

- Kessoku 不包含 WebClient2，也没有下载/代理路径。
- 可选 External Web Client Provider 默认关闭，必须提供来源、许可证、版本和 digest，
  并且不会接收 bearer token。
- Provider 只是独立托管 HTTPS 客户端的启动/治理描述符。v2.8.0 不托管浏览器远控客户端，
  不共享 cookie/localStorage，也不实现 SSO 或 token exchange。详见
  [Web 客户端](docs/wiki/ZH-CN-Web-Client.md)。

### 制品与自动化

- v2.8.0 正式支持 Linux amd64：GHCR 镜像、Linux x86_64 archive/binary 与 amd64 DEB。
- 镜像使用非特权用户，并排除浏览器客户端资产。
- 候选 workflow 运行 Go/race、三数据库迁移、内置前端测试/审计/可复现检查、SBOM、
  secret/依赖扫描、镜像 smoke 和真实 DEB 安装。
- 发布流程只消费一个精确成功的非发布候选 run，并输出 checksum 与 Sigstore
  provenance/SBOM attestations。

## 兼容性与迁移

- 数据库版本 301 增加 token hash、JTI/key/auth-version 字段、管理员审计事件、OAuth
  身份唯一约束和最后管理员共享不变量；覆盖 SQLite、MySQL 8.4 和 PostgreSQL 16 fixture。
- 外部 MySQL 现在要求 `mysql.tls: "true"`；可用 `mysql.ca-file` 把私有 CA 加入系统信任
  池。PostgreSQL 要求 `postgresql.sslmode: verify-full`，可配置 `ssl-root-cert`。不安全或
  不验证主机名的数据库传输会导致启动失败。
- 迁移是增量的，但旧应用不能使用不含明文 token 的新凭据。v2.8.0 一旦签发令牌，回滚
  旧应用必须恢复匹配且已验证的升级前数据库备份。
- 旧 opaque 凭据只允许有界兼容期；已删除的 HS256 设置不属于受支持连接认证 profile。
- Starry 认证应从 `off` 开始，再进入 `audit`；只有支持客户端真实矩阵不存在无法解释的
  would-deny 后才可开启 `enforce`。
- Control Agent 必须先只读上线，并在开启配置写入前完成回滚演练。

详见[升级与回滚](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)和
[`MIGRATION.md`](MIGRATION.md)。

## v2.8.0 接受的已知限制

RustDesk 1.4.9 的 audit/sysinfo 上传不携带认证头。Kessoku 保留有界兼容路由，并要求已经
登记的 peer ID 与精确 UUID；但 UUID 不是秘密，已知这一组合仍可提交伪造运维 telemetry。
需要不可抵赖性时，应把记录导出到 append-only 或不可变存储。完整处置与证据边界见
[安全发现闭环](docs/wiki/ZH-CN-Security-Finding-Closure.md)。

## 平台范围

- 正式支持：Docker `linux/amd64`、Linux x86_64 archive/binary、amd64 DEB。
- 尽力兼容且不阻断：ARM 源码/构建兼容。
- 不属于 v2.8.0 发布承诺：Windows 制品。

## 发布前状态

本地 Go、race、前端、可复现性、包安装、非 root 镜像、安全响应头、契约和跨项目真实进程
测试已经通过。已按 tag、源码提交、契约/schema 哈希与 amd64 镜像 digest 固定正式发布的
Starry `1.1.16-patch-v1.2.0` Control API。RustDesk 1.4.9 强制 Relay 桌面会话已通过
`audit` native-to-native，以及 `enforce` native-to-native、WSS-to-WSS、WSS-to-native、
native-to-WSS，并检查 Remote Desktop 窗口/截图和已建立 HBBR 连接。本矩阵不表示已经覆盖
直接 P2P 或独立 Secure TCP case。精确本地候选验证器与修复后静态复核现已通过。每个部署
在上线前仍须自行记录备份/恢复、密钥恢复、故障切换、回滚、RTO/RPO 与 go/no-go
负责人；Kessoku 不承诺统一恢复 SLA。该部署门禁与软件发布门禁相互独立。软件发布仅剩
以下批准/workflow 操作：

- 批准最终文档与新特性文案；
- 在发布不可变 tag、GHCR `v2.8.0`/`latest`、GitHub Release 与 Wiki 前，对批准提交运行
  受保护的非发布候选 CI。
