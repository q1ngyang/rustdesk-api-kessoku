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

Kessoku ─ 私有 mTLS + scoped JWT ─ Starry Control Agent
```

不得通过公共 API 路径代理端口 `21121` 或 Control Agent。

## 文件

- [`docker-compose.yaml`](../../docker-compose.yaml)
- [`examples/compose.env.example`](../../examples/compose.env.example)
- [`examples/Caddyfile.example`](../../examples/Caddyfile.example)
- [`conf/config.yaml`](../../conf/config.yaml)
- [`CONTAINER.zh-CN.md`](../../CONTAINER.zh-CN.md)

## 准备

```sh
install -d -m 0700 /opt/kessoku/data/kessoku /opt/kessoku/secrets
cd /opt/kessoku
cp /path/to/repository/docker-compose.yaml .
cp /path/to/repository/examples/compose.env.example .env
cp /path/to/repository/examples/Caddyfile.example .
vi .env

umask 077
openssl rand -base64 24 > secrets/bootstrap-admin-password
chown 65534:65534 secrets/bootstrap-admin-password
chmod 0600 secrets/bootstrap-admin-password
```

需要高级认证/控制设置时，复制 `conf/config.yaml`，在变更控制下编辑，并增加只读 Compose
挂载到 `/app/conf/config.yaml`。密钥和证书放入 `secrets/`，权限只允许服务账户读取。

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

确认容器以 `65534:65534` 运行、根文件系统只读、只持久化 `/app/data`，并且不存在
`/app/resources/web` 或 `/app/resources/web2`。把 bootstrap 密码直接转存到批准的密码管理器，
登录并轮换后删除宿主机 secret。Kessoku 不会在日志输出可复用 bootstrap 密码。

## 反向代理与端口

Compose 默认只把 Kessoku `21114` 发布到宿主机回环地址。通过已审核反向代理提供公共
HTTPS；仓库中的 Caddy 范例适用于代理与 Compose 位于同一宿主机的部署。
`gin.trust-proxy` 只配置精确代理地址；保留 Kessoku 输出的安全响应头，并在代理设置明确
请求体上限和超时。

内部 `21121` listener 默认关闭。启用时只绑定私有接口/容器网络，要求 TLS 1.3 和经过验证
的客户端证书，并通过防火墙只允许 Starry 访问。

## 持久化与备份

SQLite 数据库位于 `/app/data/rustdeskapi.db`，应一致备份整个数据目录。MySQL/PostgreSQL
还需使用数据库厂商的一致性备份，并另外保存 Kessoku 密钥、PKI、配置、镜像 digest 与发布
provenance。

## 验证

引入 Starry 认证前先验证管理员登录、普通 API 登录/注销、地址簿、数据库版本与日志。
完整部署继续按照[运维与验证](ZH-CN-Operations-and-Verification.md)分阶段验收。
