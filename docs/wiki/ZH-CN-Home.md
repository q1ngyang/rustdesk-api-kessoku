# rustdesk-api-kessoku 文档

[English](Home.md) | **简体中文**

Kessoku 是一个非官方 RustDesk 账户、管理和策略控制面，提供客户端 API 与内置管理前端。
Starry HBBS 仍是信令、连接授权与 Relay 分配的权威实现。

## 先理解组件边界

| 组件 | 是否包含 | 作用 |
| --- | --- | --- |
| Kessoku API 和管理前端 | 是 | 账户、登录、地址簿、设备、令牌生命周期、审计和类型化管理。 |
| Starry HBBS | 否 | 信令、严格连接 JWT 强制认证与 Relay 决策。 |
| Starry Control Agent | 否 | 可选的单 HBBS 最小权限 Relay/配置 API。 |
| 官方 HBBR | 否 | 承载远控数据的 Relay。 |
| 内置浏览器远控 MVP | 是 | 仓库自有强制 Relay WSS、VP9、鼠标和基本键盘客户端，使用独立 origin。 |

## 选择起点

| 场景 | 文档 |
| --- | --- |
| 首次部署 | [快速开始](ZH-CN-Getting-Started.md) |
| 从 GHCR 进入项目 | [Docker 镜像使用](ZH-CN-Docker-Image-Usage.md) |
| 推荐单机 API 部署 | [Docker 部署](ZH-CN-Docker-Deployment.md) |
| 了解全部配置边界 | [配置参数参考](ZH-CN-Configuration-Reference.md) |
| 开启连接认证 | [连接认证](ZH-CN-Connection-Authentication.md) |
| Relay 可观测或配置事务 | [Starry 控制](ZH-CN-Starry-Control.md) |
| 了解当前浏览器客户端边界 | [Web 客户端](ZH-CN-Web-Client.md) |
| 收集 release/staging 证据 | [运维与验证](ZH-CN-Operations-and-Verification.md) |
| 升级或准备回滚 | [升级与回滚](ZH-CN-Upgrade-and-Rollback.md) |
| 部署出现故障 | [常见问题排查](ZH-CN-Troubleshooting.md) |

## 安全默认值

- 使用不可变 v2.8.0 镜像 tag，再固定解析出的 digest。
- 注册、Swagger、内置 Web Client 和旧 token 兼容保持关闭，直到对应部署 profile 经过
  明确审核；客户端只能在独立 HTTPS origin 上启用。
- Starry 连接认证必须从 `off` 或 `audit` 开始，不能直接进入 `enforce`。
- Control Agent 先只读上线，并保持在私有网络。
- access-token、内部 PKI 与 Control Agent 密钥位于镜像外且彼此独立。
- 容器健康、HTTP 200 和登录成功都只是部分证据；最终必须完成真实 native/Secure TCP/
  WSS/P2P/Relay 客户端会话。

## 发布与法律状态

虽然源码候选已批准，但不可变 tag、受保护候选 CI 和发布操作仍未执行，因此 v2.8.0
尚未发布。已审核源码使用 MIT 许可证，与 RustDesk 无隶属关系。仓库自有 Web Client
使用 MIT，第三方依赖许可证记录在 release SBOM；不包含历史 WebClient2/V2 资产。
