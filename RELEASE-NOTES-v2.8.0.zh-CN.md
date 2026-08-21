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

### 类型化 Starry 管理

- Starry 实例 origin 和凭据文件引用由部署固定，浏览器不能选择 Agent URL。
- 能力、状态、Relay 列表与无副作用的双地址分配模拟。
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

- 数据库版本 300 增加 token hash、JTI/key/auth-version 字段与管理员审计事件；覆盖
  SQLite、MySQL 8.4 和 PostgreSQL 16 fixture。
- 迁移是增量的，但旧应用不能使用不含明文 token 的新凭据。v2.8.0 一旦签发令牌，回滚
  旧应用必须恢复匹配且已验证的升级前数据库备份。
- 旧 opaque 凭据只允许有界兼容期；已删除的 HS256 设置不属于受支持连接认证 profile。
- Starry 认证应从 `off` 开始，再进入 `audit`；只有支持客户端真实矩阵不存在无法解释的
  would-deny 后才可开启 `enforce`。
- Control Agent 必须先只读上线，并在开启配置写入前完成回滚演练。

详见[升级与回滚](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)和
[`MIGRATION.md`](MIGRATION.md)。

## 平台范围

- 正式支持：Docker `linux/amd64`、Linux x86_64 archive/binary、amd64 DEB。
- 尽力兼容且不阻断：ARM 源码/构建兼容。
- 不属于 v2.8.0 发布承诺：Windows 制品。

## 发布前状态

本地 Go、race、前端、可复现性、包安装、非 root 镜像、安全响应头、契约和跨项目真实进程
测试已经通过。已按 tag、源码提交、契约/schema 哈希与 amd64 镜像 digest 固定正式发布的
Starry `1.1.16-patch-v1.2.0` Control API。以下剩余证据仍必须属于精确受审核的 Kessoku
提交，发布门禁才可解除：

- 干净候选 CI 和修复后 finding 复核；
- 支持客户端 native/Secure TCP/WSS 的 audit→enforce 验收；
- 备份/恢复、密钥恢复、故障切换与回滚演练；
- 最终发布负责人批准。
