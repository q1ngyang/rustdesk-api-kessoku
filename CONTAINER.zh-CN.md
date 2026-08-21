# rustdesk-api-kessoku 容器镜像

[English](CONTAINER.md) | **简体中文**

本文是从 GHCR package 页面进入项目时使用的版本化入口。推荐在 Linux amd64 上使用
Docker Compose 部署。

> 在受保护发布流程完成前，本文中的 `v2.8.3` 镜像仍只是发布目标。正式发布的 `latest`
> 将指向最新成功发布的稳定版；不要用本地工作树镜像替代。

部署链接：

- [GHCR 镜像页面](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [推荐 Docker 部署](docs/wiki/ZH-CN-Docker-Deployment.md)
- [Compose 范例](docker-compose.yaml)
- [环境变量范例](examples/compose.env.example)
- [内置 Web Client 配置范例](examples/config.docker-builtin.yaml)
- [Caddy HTTPS 范例](examples/Caddyfile.example)
- [快速开始](docs/wiki/ZH-CN-Getting-Started.md)
- [Starry 集成](docs/wiki/ZH-CN-Starry-Control.md)
- [升级与回滚](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)

## 镜像范围

v2.8.3 镜像包含一个非特权 `kessoku-api` 进程、从同一源码提交构建的已审核管理前端与
Web Client、API 文档和运行配置模板。该镜像：

- 目标平台为 `linux/amd64`；
- 使用 UID/GID `65534:65534` 运行；
- 在 `/app/data` 持久化应用数据；
- 公共 API 使用端口 `21114`；
- 独立内置 Web Client listener 暴露 `21122`；
- 明确启用时可在 `21121` 使用独立内部 mTLS listener；
- 包含已审核 `resources/client`，但不包含历史 WebClient2/V2、`resources/web` 或
  `resources/web2`；
- 不包含私钥或部署凭据。

Kessoku 不是 HBBS/HBBR。请独立部署配套 Starry HBBS 和官方 HBBR。

## 拉取与检查

发布后执行：

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v2.8.3
docker image inspect \
  ghcr.io/q1ngyang/rustdesk-api-kessoku:v2.8.3 \
  --format '{{json .RepoDigests}}'
```

发布 workflow 会把不可变 `v2.8.3` 与移动的 `latest` 推送为同一镜像。只有在明确希望跟随
最新稳定版时才使用 `latest`；生产变更控制与回滚应解析并固定版本 tag 的 digest。

## Compose 快速开始

在 v2.8.3 源码或下载的部署文件目录中：

```sh
cp examples/compose.env.example .env
cp examples/config.docker-builtin.yaml config.yaml
mkdir -p data/kessoku secrets
chmod 0700 data/kessoku secrets

# 创建一次性 bootstrap secret，不把密码写入 Compose 或 shell 参数。
# 镜像内非特权用户的 UID 是 65534。
umask 077
openssl rand -base64 24 > secrets/bootstrap-admin-password
openssl genpkey -algorithm ED25519 \
  -out secrets/kessoku-access-ed25519.pem
chown 65534:65534 secrets/bootstrap-admin-password
chown 65534:65534 secrets/kessoku-access-ed25519.pem
chmod 0600 secrets/bootstrap-admin-password secrets/kessoku-access-ed25519.pem

# 继续前修改所有占位值。config.yaml 中的 relay-wss-urls 必须保留为精确 YAML map，
# 不能尝试编码到 .env。
vi .env config.yaml

docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

范例会移除 Linux capabilities、启用 `no-new-privileges`、使用只读根文件系统、把运行日志
放入 tmpfs，并只持久化应用数据目录。`.env` 中的 RustDesk server 公钥不是私钥。

## 首次登录

新数据库会创建 `admin`，但使用不可达的随机 bootstrap credential，应用日志不会输出可复用
密码。上面的 reset 命令通过 group/other 权限位均为零的普通文件设置第一个可用密码。请把该
值直接转存到批准的密码管理器，通过已审核反向代理访问
`https://your-api.example/_admin/`，登录后再次轮换密码，然后删除宿主机 secret 文件。不得
把密码放在命令参数或环境变量中。

Compose 默认只把 `21114` 与 `21122` 发布到 `127.0.0.1`。仓库 Caddy 范例
[`examples/Caddyfile.example`](examples/Caddyfile.example)把 API/admin 与 Web Client 配置
为两个不同 HTTPS origin。Compose 会只读挂载 `KESSOKU_CONFIG_FILE`；随附 builtin 范例会
明确启用 `web-client.mode: builtin`。启动前必须编辑精确 Relay 名称到 WSS 的 YAML map、两个
origin、公钥、generation 与认证密钥路径；Relay map 不使用 Viper 环境变量解码。不能把
客户端作为 API origin 的一个 path。`gin.trust-proxy` 只应配置精确代理地址，内部端口
`21121` 不得通过该代理公开。

除非这是明确且受保护的运维决定，否则不要在生产环境公开 Swagger。

内置 MVP 支持强制 Relay WSS、VP9、鼠标和基本键盘；不支持 P2P/direct、被控模式、
文件/剪贴板/音频、显示器切换与非 VP9 codec。短期 connection-only token 只在内存传给
精确 client origin。详见 [Web 客户端](docs/wiki/ZH-CN-Web-Client.md)。

## Secret 与高级配置

基本 API 可仅用环境变量运行。连接认证和 Starry 控制需要已审核配置，并在
`/run/secrets` 只读挂载彼此独立的：

- Kessoku access-token Ed25519 私钥；
- 内部 listener 服务端证书/私钥与 Starry 客户端 CA；
- Kessoku 访问 Control Agent 的客户端证书/私钥与 CA；
- Control Agent service-JWT 签名私钥。

access-token 与 Control Agent 签名密钥不得复用。不要把 secret 内容写入 Compose YAML
或 Git。

内部 listener 不得经过公共 API 反向代理。通过容器网络访问时，应明确配置容器内监听地址，
并只允许经过批准的 Starry 身份访问。

## 部署验收

容器启动和 HTTP 可达只是部分证据。在开启连接强制认证或 Agent 写入前，必须验证：

1. 数据库迁移与备份恢复；
2. 管理员和普通用户登录/注销；
3. 专用 mTLS 路径上的 JWKS 与 introspection；
4. 每种支持客户端传输的 Starry `audit` 结果；
5. Relay 列表与无副作用分配模拟；
6. Control Agent 只读行为，以及 staging 中的 plan/apply/rollback；
7. 最终 native、Secure TCP、WSS 与 Relay 桌面会话矩阵。
8. 浏览器 forced-Relay VP9 鼠标/键盘会话、grant 到期/logout，以及独立 origin/CSP。

详见[运维与验证](docs/wiki/ZH-CN-Operations-and-Verification.md)。

## 升级与回滚

升级前备份数据库、认证密钥、内部 PKI、配置、当前镜像 digest 和 Starry generation。
Kessoku 数据库版本 301 是增量迁移，但旧程序无法认证 v2.8.3 新签发的仅 hash token。
如果 v2.8.3 已签发令牌，回滚旧程序时必须恢复匹配的升级前数据库备份。

外部 MySQL/PostgreSQL 必须使用验证证书和主机名的 TLS；需要时只读挂载私有 CA。详见
[配置参数参考](docs/wiki/ZH-CN-Configuration-Reference.md)。

不得覆盖或移动已经公开的版本 tag。详见
[升级与回滚](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)。
