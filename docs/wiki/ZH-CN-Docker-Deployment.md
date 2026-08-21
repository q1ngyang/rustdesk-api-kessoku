# Docker 部署

[English](Docker-Deployment.md) | **简体中文**

推荐在 Linux amd64 上使用 Docker Compose 部署 Kessoku。仓库范例只运行 Kessoku；
Starry HBBS 和官方 HBBR 继续作为拥有独立数据与端口的服务。

## 架构

```text
RustDesk 客户端 ─ HTTPS 443 ─ 反向代理 ─ Kessoku 21114
                                             │
Starry HBBS ─ 私有 TLS 1.3/mTLS ─────────────┤ 21121 JWKS/introspection
                                             │
管理浏览器 ─ HTTPS /_admin/ ─────────────────┘

浏览器远控客户端 ── 独立 HTTPS origin ── Kessoku 21122

Kessoku ─ 私有 mTLS + scoped JWT ─ Starry Control Agent
```

不得通过公共 API 路径代理端口 `21121` 或 Control Agent。

## 文件

- [`docker-compose.yaml`](../../docker-compose.yaml)
- [`examples/compose.env.example`](../../examples/compose.env.example)
- [`examples/config.docker-builtin.yaml`](../../examples/config.docker-builtin.yaml)
- [`examples/Caddyfile.example`](../../examples/Caddyfile.example)
- [`conf/config.yaml`](../../conf/config.yaml)
- [`CONTAINER.zh-CN.md`](../../CONTAINER.zh-CN.md)

## 准备

```sh
install -d -m 0700 /opt/kessoku/data/kessoku /opt/kessoku/secrets
cd /opt/kessoku
cp /path/to/repository/docker-compose.yaml .
cp /path/to/repository/examples/compose.env.example .env
cp /path/to/repository/examples/config.docker-builtin.yaml config.yaml
cp /path/to/repository/examples/Caddyfile.example .
vi .env config.yaml

umask 077
openssl rand -base64 24 > secrets/bootstrap-admin-password
openssl genpkey -algorithm ED25519 \
  -out secrets/kessoku-access-ed25519.pem
chown 65534:65534 secrets/bootstrap-admin-password
chown 65534:65534 secrets/kessoku-access-ed25519.pem
chmod 0600 secrets/bootstrap-admin-password secrets/kessoku-access-ed25519.pem
```

Compose 会把 `KESSOKU_CONFIG_FILE` 只读挂载到 `/app/conf/config.yaml`。随附 builtin 范例
会启用客户端，并把 `relay-wss-urls` 保持为精确 YAML map；应在变更控制下编辑该 map、两个
origin、全部 WSS endpoint、公钥、generation 与认证密钥路径。不要假设 Viper 能从环境变量
安全解码 Relay map。密钥和证书放入 `secrets/`，权限只允许服务账户读取。

## 检查和启动

```sh
docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
docker compose --env-file .env -f docker-compose.yaml pull
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml ps
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

确认容器以 `65534:65534` 运行、根文件系统只读、只持久化 `/app/data`，存在
`/app/resources/client/index.html`，并且不存在 `/app/resources/web` 或
`/app/resources/web2`。把 bootstrap 密码直接转存到批准的密码管理器，
登录并轮换后删除宿主机 secret。Kessoku 不会在日志输出可复用 bootstrap 密码。

## 反向代理与端口

Compose 默认只把 Kessoku `21114` 与 Web Client `21122` 发布到宿主机回环地址。通过已审核
反向代理把它们发布为两个不同的公共 HTTPS origin；仓库 Caddy 范例适用于同机部署。
只有在配置精确 public/API origin、WSS map、服务端公钥和正数 generation 后才启用
`web-client.mode: builtin`。
`gin.trust-proxy` 只配置精确代理地址；保留 Kessoku 输出的安全响应头，并在代理设置明确
请求体上限和超时。

内部 `21121` listener 默认关闭。启用时只绑定私有接口/容器网络，要求 TLS 1.3 和经过验证
的客户端证书，并通过防火墙只允许 Starry 访问。

## 持久化与备份

SQLite 数据库位于 `/app/data/rustdeskapi.db`，应一致备份整个数据目录。MySQL/PostgreSQL
还需使用数据库厂商的一致性备份，并另外保存 Kessoku 密钥、PKI、配置、镜像 digest 与发布
provenance。

首次用 v2.8.1 启动前必须配置外部数据库：MySQL 要求 `tls: "true"`，PostgreSQL 要求
`sslmode: "verify-full"`。使用私有 PKI 时，把 CA 放入 `secrets/`，只读挂载到
`/run/secrets`，再把 `mysql.ca-file` 或 `postgresql.ssl-root-cert` 指向容器内路径。数据库
地址/host 必须匹配证书 SAN。不安全模式、无法读取/不受信任的 CA 或主机名不匹配都会让
Kessoku 主动退出。详见[配置参数参考](ZH-CN-Configuration-Reference.md)。

## 验证

引入 Starry 认证前先验证管理员登录、普通 API 登录/注销、地址簿、数据库版本 301、OAuth
身份索引/不变量与日志。
完整部署继续按照[运维与验证](ZH-CN-Operations-and-Verification.md)分阶段验收。
还要验证浏览器客户端公共 profile 不含 secret、grant 只交给精确 origin，并完成一例
forced-Relay VP9 鼠标/键盘会话与 logout。详见[内置 Web 客户端](ZH-CN-Web-Client.md)。
