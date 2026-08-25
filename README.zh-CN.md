# RustDesk API Kessoku

[English](README.md) | **简体中文**

Kessoku 是一个非官方的 RustDesk 账户、管理和策略控制面。它提供客户端 API、内置管理
前端和开源浏览器远控 MVP，并通过类型化、版本化 Control API 与
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
集成。

> **v3.0.0 稳定版。** 功能实现和 Linux amd64 检查已经完成，Starry 正式
> 契约也已固定，已发布 Starry 的原生客户端矩阵与内置浏览器 forced-Relay 夹具均在本机
> 通过。不可变 tag 只通过受保护候选/发布工作流生成；生产部署前请验证 GitHub Release
> checksum，并固定带版本的 GHCR digest。

## 组件边界

| 组件 | 责任 |
| --- | --- |
| Kessoku API | 登录、用户、地址簿、设备、令牌生命周期、管理与审计。 |
| 内置 `admin-web/` | 与 Kessoku 同一提交、使用 `npm ci` 构建的已审核管理前端。 |
| 内置 `web-client/` | MIT 浏览器 MVP：强制 Relay WSS、VP9、鼠标和基本键盘。 |
| Starry HBBS | 连接认证、Relay 分配与信令。 |
| Starry Control Agent | 可选的 mTLS/scoped-JWT Relay 可观测与安全配置事务 API。 |
| 官方 HBBR | 承载远控数据的 Relay；Kessoku 不替代它。 |

Kessoku 不暴露 shell、任意命令、任意 Agent URL、Docker Socket 或浏览器提供的文件路径。
浏览器客户端是仓库自有源码；历史 WebClient2/V2 与 `resources/web*` 资产继续排除。

## v3.0.0 新特性

- 全新响应式明暗主题管理界面，针对桌面、平板和手机优化，并使用仓库内置的
  Kessoku/StarryLinks 品牌资源。
- 三级企业权限：普通用户、范围管理员 `admin` 和无限制超管 `super_admin`；范围管理员
  可管理获授的用户组、用户、公共地址簿和 ID 设备。
- 严格 Ed25519/EdDSA access token：校验 issuer、audience、kid、JTI、生命周期、scope
  和认证版本。
- 在独立 TLS 1.3 mTLS listener 上提供支持撤销的 JWKS 与 introspection。
- 类型化 Starry 操作：能力、状态、Relay、分配模拟、配置校验、plan/apply、历史、回滚
  和运行时 reload。
- 控制路由仅管理员可达，并持久记录脱敏的意图/结果审计。
- 从运行时攻击面删除通用 legacy ServerCmd 执行。
- 管理前端源码内置并可复现构建，不再使用移动的前端分支。
- Web Client 内置并可复现构建，使用独立 origin/listener 与只在内存传递的短期连接 grant。
  MVP 支持强制 Relay WSS、VP9、鼠标和基本键盘；不支持 P2P、被控模式、文件传输、
  剪贴板、音频、显示器切换与非 VP9 codec。
- SQLite、MySQL、PostgreSQL 迁移；外部 MySQL/PostgreSQL 强制验证 TLS 证书与主机名。
- v3.0.0 正式制品范围为 Docker `linux/amd64`、Linux x86_64 archive/binary 和
  amd64 DEB；ARM 仅尽力兼容且不阻断发布。

> **升级提示：** v3 将 Go module 路径迁移到 `/v3`，并在数据库版本 302 调整角色语义。
> 升级前请先阅读[破坏性变更](RELEASE-NOTES-v3.0.0.zh-CN.md#破坏性变更breaking-changes)。

## 推荐部署

推荐在 Linux amd64 使用 Docker Compose。先使用不可变版本 tag，再把解析出的 digest
记录到部署配置：

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.0
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
# 先编辑 .env/config.yaml，并提供其中引用的签名私钥。
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
```

Compose 默认把 API 21114 和 Web Client 21122 绑定到 `127.0.0.1`，请使用两个不同的
HTTPS origin，并参考反代范例
[`examples/Caddyfile.example`](examples/Caddyfile.example)对外发布。release 还会发布
`latest`，供明确希望跟随最新稳定版的用户使用；生产回滚仍应固定 `v3.0.0` digest。
精确 `relay-wss-urls` map 位于挂载的 YAML，不放在环境变量中；启动前请按 Docker 详细
文档完成配置。

首次启动时不要立即开启 Starry `enforce` 或配置写入。认证应从 `off`/`audit` 迁移，
Control Agent 应先以只读模式上线，并先完成真实客户端和回滚演练。

## 文档

| 主题 | 文档 |
| --- | --- |
| 首次部署 | [快速开始](docs/wiki/ZH-CN-Getting-Started.md) |
| GHCR 镜像用户 | [容器镜像指南](CONTAINER.zh-CN.md) |
| 推荐 Compose 部署 | [Docker 部署](docs/wiki/ZH-CN-Docker-Deployment.md) |
| 配置 | [配置参数参考](docs/wiki/ZH-CN-Configuration-Reference.md) |
| JWT/JWKS/introspection 灰度 | [连接认证](docs/wiki/ZH-CN-Connection-Authentication.md) |
| Starry 集成 | [Starry 控制](docs/wiki/ZH-CN-Starry-Control.md) |
| 浏览器客户端部署与排除项 | [Web 客户端](docs/wiki/ZH-CN-Web-Client.md) |
| 安全审查与已接受残余 | [安全发现闭环](docs/wiki/ZH-CN-Security-Finding-Closure.md) |
| 验收证据 | [运维与验证](docs/wiki/ZH-CN-Operations-and-Verification.md) |
| 升级与回滚 | [升级与回滚](docs/wiki/ZH-CN-Upgrade-and-Rollback.md) |
| 故障诊断 | [常见问题排查](docs/wiki/ZH-CN-Troubleshooting.md) |
| English | [English documentation](docs/wiki/Home.md) |

已审核的 Wiki 源文件位于 [`docs/wiki/`](docs/wiki/)。将它们发布到 GitHub Wiki 是独立的
发布负责人操作。

## 发布状态

权威门禁是 [`RELEASE_STATUS`](RELEASE_STATUS)，证据要求见
[`RELEASE-CHECKLIST.md`](RELEASE-CHECKLIST.md)，v3.0.0 功能和兼容性说明见
[`RELEASE-NOTES-v3.0.0.zh-CN.md`](RELEASE-NOTES-v3.0.0.zh-CN.md)。

本地开发验证不等于发布授权。tag、push、GHCR、GitHub Release 和 Wiki 发布都需要单独
明确批准。

## 许可证与致谢

Kessoku 使用 MIT 许可证，与 RustDesk 无隶属关系。项目延续
`lejianwen/rustdesk-api` 贡献者的工作；内置管理前端的 MIT 来源记录见
[`ADMIN-WEB-PROVENANCE.md`](ADMIN-WEB-PROVENANCE.md)。仓库自有 Web Client 使用 MIT
许可证，依赖许可证记录在 release SBOM 中。
