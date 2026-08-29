# rustdesk-api-kessoku 中文文档

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Home) | **简体中文**

Kessoku 是非官方 RustDesk 账户与管理服务，提供客户端登录接口、用户和设备管理、个人及
公共地址簿、审计记录、分级管理员权限、内置管理后台，以及可选的浏览器远程控制页面。

Kessoku **不包含** RustDesk ID 服务器（HBBS）和中继服务器（HBBR）。它可以搭配官方
HBBS/HBBR 使用；为了获得安全 TCP、按地理位置选择中继服务器、连接令牌校验和管理接口等
扩展能力，推荐搭配同一开发者维护的
[`q1ngyang/rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)。

## 主要功能

| 分类 | 功能 |
| --- | --- |
| 账户 | 用户名/密码登录、注册开关、会话注销、密码重置、LDAP 与后台配置的 OAuth/OIDC 登录 |
| 地址簿 | 个人地址簿、共享地址簿、标签、地址簿集合及匹配规则 |
| 设备 | 设备上报、在线信息、设备分组和按范围授权 |
| 管理 | 用户、用户组、设备、登录记录、连接/文件审计、会话撤销 |
| 权限 | 普通用户、范围管理员、超级管理员 |
| 浏览器远控 | 独立 HTTPS 域名，强制通过 WSS 中继，支持 VP9 画面、鼠标和基本键盘 |
| Starry 集成 | 可选连接令牌认证，以及通过私有管理代理查看中继状态和安全修改配置 |

浏览器远控目前不支持点对点直连、被控端模式、文件传输、剪贴板、音频、终端、端口转发、
多显示器切换、输入法组合输入或 VP9 以外的视频编码。桌面端 RustDesk 不受这些浏览器端
限制。

## 从哪里开始

| 你的情况 | 建议阅读 |
| --- | --- |
| 已有官方或第三方 HBBS/HBBR，只需要增加账户 API | [快速开始：单独部署 Kessoku](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started) |
| 从空白服务器搭建 API + HBBS + HBBR | [完整部署：Kessoku + Starry](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment) |
| 为现有中心增加一台独立 HBBR | [纯中继节点部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Relay-Only-Deployment) |
| 需要查询 Compose、目录和更新命令 | [Docker 部署参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment) |
| 需要配置域名、HTTPS、Nginx 或防火墙 | [反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall) |
| 需要查某个 YAML 参数 | [配置参数参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Configuration-Reference) |
| 需要配置 RustDesk 客户端或浏览器远控 | [客户端使用方法](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Web-Client) |
| 需要启用 Starry 连接令牌校验 | [连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication) |
| 需要在后台查看或修改 Starry 配置 | [Starry 管理](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control) |
| 需要备份、巡检、升级或排障 | [日常运维](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Operations-and-Verification) · [升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback) · [常见问题](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Troubleshooting) |

## 推荐部署原则

- 生产环境固定具体镜像版本，不直接使用 `latest`。
- API 和浏览器远控分别使用两个 HTTPS 域名，例如 `api.example.com` 与
  `client.example.com`。
- `21114`（API）和 `21122`（浏览器客户端）只监听宿主机回环地址，由 Nginx 对外提供
  `443/TCP`。
- `21121`（Kessoku 内部认证接口）和 `21120`（Starry 管理代理）不得暴露到公网。
- Kessoku 的 `/app/data`、Starry 的 `/root`、配置、签名密钥和 TLS 证书都要持久保存并
  定期备份。
- 首次接入 Starry 时保持连接认证为 `off`；准备好双向 TLS 后先使用 `audit`（只记录不
  拦截），确认没有误判再改为 `enforce`（强制拦截）。

## 版本与项目关系

本文档随 Kessoku `v3.0.4` 维护，联合部署示例固定 Starry
`1.1.16-patch-v1.2.0`。示例中的域名、路径和密钥都必须替换后才能使用。Kessoku 与
RustDesk 官方项目没有隶属关系，项目按 MIT 许可证发布。
