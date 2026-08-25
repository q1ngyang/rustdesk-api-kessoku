# RustDesk API Kessoku

[English](README.md) | **简体中文**

Kessoku 是非官方 RustDesk 账户与管理服务，提供客户端登录接口、用户与设备管理、个人和
公共地址簿、审计、分级管理员权限、内置管理后台，以及可选的浏览器远程控制页面。

Kessoku 不包含 HBBS/HBBR。它可以搭配官方 RustDesk Server；推荐搭配同一开发者维护的
[`q1ngyang/rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)，
获得安全 TCP、WSS、按地理位置选择中继服务器、连接令牌认证和可选管理代理。

当前稳定版：`v3.0.1`，正式支持 Docker/Linux `amd64`。

## 功能

- RustDesk 客户端账户登录、注销和会话撤销；
- 用户、用户组、设备、设备组和分级管理员范围；
- 个人地址簿、公共地址簿、标签、地址簿集合和共享规则；
- 登录记录、连接审计、文件审计和管理操作审计；
- 密码登录、LDAP、GitHub/Linux.do OAuth 和通用 OIDC；
- Ed25519/EdDSA 访问令牌、JWKS 和可撤销令牌状态查询；
- 与 Starry 的连接认证和类型化管理接口；
- 内置响应式管理后台；
- 独立浏览器远控：强制 WSS 中继、VP9 画面、鼠标和基本键盘。

浏览器客户端目前不支持点对点、被控端、文件传输、剪贴板、音频、终端、多显示器切换、
输入法组合输入或 VP9 以外的视频编码。桌面 RustDesk 客户端不受这些限制。

## 选择部署方式

| 场景 | 教程 |
| --- | --- |
| 已有官方/第三方 HBBS/HBBR，只部署 Kessoku | [手把手快速开始](docs/wiki/ZH-CN-Getting-Started.md) |
| 从空白服务器部署 API + HBBS + HBBR | [Kessoku + Starry 完整教程](docs/wiki/ZH-CN-Complete-Deployment.md) |
| 为现有中心增加独立 HBBR | [纯中继节点教程](docs/wiki/ZH-CN-Relay-Only-Deployment.md) |
| 查询 Compose、目录和更新命令 | [Docker 部署参考](docs/wiki/ZH-CN-Docker-Deployment.md) |

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
开放；详细规则见[反向代理与防火墙](docs/wiki/ZH-CN-Reverse-Proxy-and-Firewall.md)。

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

| 主题 | 文档 |
| --- | --- |
| 中文首页与功能概览 | [Wiki 首页](docs/wiki/ZH-CN-Home.md) |
| 单独部署 Kessoku | [快速开始](docs/wiki/ZH-CN-Getting-Started.md) |
| Kessoku + Starry | [完整部署](docs/wiki/ZH-CN-Complete-Deployment.md) |
| 独立 HBBR | [纯中继节点](docs/wiki/ZH-CN-Relay-Only-Deployment.md) |
| Docker 镜像 | [镜像使用](CONTAINER.zh-CN.md) |
| Docker 参考 | [Docker 部署](docs/wiki/ZH-CN-Docker-Deployment.md) |
| Nginx 和端口 | [反向代理与防火墙](docs/wiki/ZH-CN-Reverse-Proxy-and-Firewall.md) |
| 所有 YAML 参数 | [配置参数参考](docs/wiki/ZH-CN-Configuration-Reference.md) |
| 桌面与浏览器客户端 | [客户端使用方法](docs/wiki/ZH-CN-Web-Client.md) |
| Starry 连接令牌 | [连接认证](docs/wiki/ZH-CN-Connection-Authentication.md) |
| Starry 管理代理 | [Starry 管理](docs/wiki/ZH-CN-Starry-Control.md) |
| 安全、运维和故障 | [安全配置](docs/wiki/ZH-CN-Security-Finding-Closure.md) · [日常运维](docs/wiki/ZH-CN-Operations-and-Verification.md) · [排障](docs/wiki/ZH-CN-Troubleshooting.md) |
| 升级 | [升级与回退](docs/wiki/ZH-CN-Upgrade-and-Rollback.md) |
| English | [English documentation](docs/wiki/Home.md) |

## 升级提示

v3.0.1 使用数据库版本 302，新增企业角色和范围授权。v2 程序可能把范围管理员误认为无限制
管理员，不能让 v2/v3 同时写一个数据库，也不能在没有匹配备份时直接降级。升级前阅读
[`MIGRATION-v3.0.1.zh-CN.md`](MIGRATION-v3.0.1.zh-CN.md)。

## 许可证

Kessoku 使用 MIT 许可证，与 RustDesk 官方项目没有隶属关系。项目延续
`lejianwen/rustdesk-api` 贡献者的工作；管理前端来源记录见
[`ADMIN-WEB-PROVENANCE.md`](ADMIN-WEB-PROVENANCE.md)。
