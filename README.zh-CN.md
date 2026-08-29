# RustDesk API Kessoku

[English](README.md) | **简体中文**

Kessoku 是非官方 RustDesk 账户与管理服务，提供客户端登录接口、用户与设备管理、个人和
公共地址簿、审计、分级管理员权限、内置管理后台，以及可选的浏览器远程控制页面。

Kessoku 不包含 HBBS/HBBR。它可以搭配官方 RustDesk Server；推荐搭配同一开发者维护的
[`q1ngyang/rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)，
获得安全 TCP、WSS、按地理位置选择中继服务器、连接令牌认证和可选管理代理。

当前稳定版：`v3.0.4`；v3.0.3 是首个正式支持的 v3 版本，正式支持 Docker/Linux `amd64`。
此前 `v3.0.1` 因重大集成缺陷已撤回；`v3.0.2` 仅保留未公开发布尝试的标签，没有受支持
的 Release 制品或容器镜像，请勿将这两个版本用于新部署。

## 功能

- RustDesk 客户端账户登录、注销和会话撤销；
- 用户、用户组、设备、设备组和分级管理员范围；
- 个人地址簿、公共地址簿、标签、地址簿集合和共享规则；
- 登录记录、连接审计、文件审计和管理操作审计；
- 密码登录、TOTP 双重认证、LDAP、GitHub/Google OAuth 和通用 OIDC；
- Ed25519/EdDSA 访问令牌、JWKS 和可撤销令牌状态查询；
- 与 Starry 的连接认证和类型化管理接口；
- 内置响应式管理后台、集中品牌设置、头像、公告、GeoIP 与多语言；
- 独立浏览器远控：登录保持、强制 WSS 中继、VP9 画面、鼠标、基本键盘和协助聊天。

浏览器客户端目前不支持点对点、被控端、文件传输、剪贴板、音频、终端、多显示器切换、
输入法组合输入或 VP9 以外的视频编码。桌面 RustDesk 客户端不受这些限制。

## 选择部署方式

| 场景 | 教程 |
| --- | --- |
| 已有官方/第三方 HBBS/HBBR，只部署 Kessoku | [手把手快速开始](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started) |
| 从空白服务器部署 API + HBBS + HBBR | [Kessoku + Starry 完整教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment) |
| 为现有中心增加独立 HBBR | [纯中继节点教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Relay-Only-Deployment) |
| 查询 Compose、目录和更新命令 | [Docker 部署参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment) |

基础 Kessoku Compose：

```sh
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
# 修改全部域名、公钥和路径，并创建 data/secrets 及 Ed25519 私钥。
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
```

请不要只复制以上四行后直接上线。首次部署还需要设置 UID 65534 目录权限、管理员密码、
防火墙、Nginx/HTTPS、持久化和真实客户端验收，完整命令都在快速开始中。

## 推荐网络结构

```text
https://api.example.com       -> Nginx -> 127.0.0.1:21114
https://client.example.com    -> Nginx -> 127.0.0.1:21122
wss://rustdesk.example.com/ws/id    -> Nginx -> HBBS 21118
wss://rustdesk.example.com/ws/relay -> Nginx -> HBBR 21119

RustDesk 原生端口：21115/TCP、21116/TCP+UDP、21117/TCP
私有端口：21120（Starry 管理代理）、21121（Kessoku 内部认证）
```

API 和浏览器客户端必须使用两个不同的 HTTPS 域名。`21114`、`21118`～`21122` 不对公网
开放；详细规则见[反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)。

## 配置与持久化

推荐的 SQLite 部署至少保存：

- `/app/data` 对应的宿主机目录，其中包含 `rustdeskapi.db`；
- `/run/secrets` 对应目录，其中包含 Kessoku 访问令牌 Ed25519 私钥；
- `config.yaml`、`.env` 和 Compose 文件；
- Nginx 配置和 TLS 证书；
- 联合部署时 Starry `/root` 数据目录中的 `id_ed25519`、`db_v2.sqlite3` 和 MMDB。

Kessoku 容器使用 UID/GID `65534:65534`。数据和密钥目录必须属于该用户且建议为 `0700`，
私钥文件为 `0600`。不要使用 root/特权容器或 `chmod 777`。

## 文档

完整分类见[文档目录](docs/README.zh-CN.md)：部署、运维、安全、发布历史与开发参考统一
收纳在 `docs/`，下方使用指南链接直接打开在线 Wiki。

| 主题 | 文档 |
| --- | --- |
| 中文首页与功能概览 | [Wiki 首页](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Home) |
| 单独部署 Kessoku | [快速开始](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started) |
| Kessoku + Starry | [完整部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment) |
| 独立 HBBR | [纯中继节点](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Relay-Only-Deployment) |
| Docker 镜像 | [镜像使用](docs/deployment/CONTAINER.zh-CN.md) |
| Docker 参考 | [Docker 部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment) |
| Nginx 和端口 | [反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall) |
| 所有 YAML 参数 | [配置参数参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Configuration-Reference) |
| 桌面与浏览器客户端 | [客户端使用方法](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Web-Client) |
| Starry 连接令牌 | [连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication) |
| Starry 管理代理 | [Starry 管理](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control) |
| 安全、运维和故障 | [安全配置](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Security-Finding-Closure) · [日常运维](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Operations-and-Verification) · [排障](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Troubleshooting) |
| 升级 | [升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback) |
| English | [English documentation](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Home) |

## 升级提示

v3.0.4 使用数据库版本 309，除企业角色和范围授权外，还新增 TOTP、品牌与媒体、公告、
GeoIP、界面偏好和 WebClient 审计字段。v2 程序可能把范围管理员误认为无限制管理员，不能
让 v2/v3 同时写一个数据库，也不能在没有匹配数据库、TOTP 密钥、媒体和配置备份时直接
降级。升级前阅读
[`MIGRATION-v3.0.4.zh-CN.md`](docs/releases/v3.0.4/MIGRATION-v3.0.4.zh-CN.md)。

## 许可证

Kessoku 使用 MIT 许可证，与 RustDesk 官方项目没有隶属关系。项目延续
`lejianwen/rustdesk-api` 贡献者的工作；管理前端来源记录见
[`ADMIN-WEB-PROVENANCE.md`](docs/development/ADMIN-WEB-PROVENANCE.md)。
