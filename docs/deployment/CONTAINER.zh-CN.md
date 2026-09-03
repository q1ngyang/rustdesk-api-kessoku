# rustdesk-api-kessoku 容器镜像

[English](CONTAINER.md) | **简体中文**

镜像地址：
[`ghcr.io/q1ngyang/rustdesk-api-kessoku`](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)

`v3.0.8` 当前是 **BLOCKED** 的镜像目标，并未发布；所有闸门通过前，生产环境继续固定
已发布 v3.0.7 digest。目标平台仍为 `linux/amd64`。

## 拉取和固定版本

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.8
docker image inspect ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.8 \
  --format '{{json .RepoDigests}}'
```

生产环境使用明确版本或内容摘要。`latest` 会随新稳定版移动，只适合已经准备自动升级和
回退的环境。

## 镜像约定

| 项目 | 值 |
| --- | --- |
| 运行用户 | `65534:65534` |
| 持久化目录 | `/app/data`，含独立 `/app/data/server-control` SP1 registry 与凭据 |
| 配置文件 | `/app/conf/config.yaml` |
| 只读密钥目录 | `/run/secrets`（由 Compose 挂载） |
| 公共 API/管理后台 | `21114/TCP` |
| 内部认证接口 | `21121/TCP`，默认关闭，不得公开 |
| 浏览器客户端 | `21122/TCP`，使用独立 HTTPS 域名 |

镜像包含 API、管理后台和浏览器客户端，不包含 HBBS、HBBR、数据库密码或部署私钥。

## 推荐 Compose

仅部署 Kessoku：

- [`docker-compose.yaml`](../../docker-compose.yaml)
- [`examples/compose.env.example`](../../examples/compose.env.example)
- [`examples/config.docker-builtin.yaml`](../../examples/config.docker-builtin.yaml)
- [完整教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)
- [本地维护 CLI](../operations/LOCAL-MAINTENANCE-CLI.zh-CN.md)

Kessoku + Starry HBBS/HBBR：

- [`examples/combined/compose.yaml`](../../examples/combined/compose.yaml)
- [`examples/combined/.env.example`](../../examples/combined/.env.example)
- [联合完整教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)

## 正确的宿主机权限

```sh
sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/rustdesk-api-kessoku/data/kessoku \
  /opt/rustdesk-api-kessoku/secrets
sudo openssl genpkey -algorithm ED25519 \
  -out /opt/rustdesk-api-kessoku/secrets/kessoku-access-ed25519.pem
openssl rand -base64 24 | sudo tee \
  /opt/rustdesk-api-kessoku/secrets/bootstrap-admin-password >/dev/null
sudo chown 65534:65534 /opt/rustdesk-api-kessoku/secrets/*
sudo chmod 0600 /opt/rustdesk-api-kessoku/secrets/*
```

目录只设 `0700` 但仍属于登录用户时，容器 UID 65534 无法访问。不要使用 `chmod 777`。

## 启动

```sh
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml run --rm kessoku-api \
  ./kessoku-api config validate --config /app/conf/config.yaml --json
docker compose --env-file .env -f compose.yaml run --rm kessoku-api \
  ./kessoku-api database migrate --config /app/conf/config.yaml --json
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 120 kessoku-api
```

启动前必须替换 `.env` 和 `config.yaml` 中全部示例域名、公钥和路径。`docker compose
config` 不会把 `example.com` 识别为业务错误。

配置校验不连接也不写入；迁移命令不会启动 API，并会与其他迁移者串行。现有 schema-312
环境可用 `database status --json` 作为 S6 启动预检。救援命令只能交给可信本地 supervisor。

## 首次管理员密码

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

打开 `https://api.example.com/dash/`，用户名为 `admin`。保存密码并在后台再次修改，确认新
密码可用后删除一次性文件。Kessoku 不会在日志输出可复用的初始密码。

## Nginx 和防火墙

Compose 默认把 `21114`、`21122` 绑定到宿主机 `127.0.0.1`。使用两个 HTTPS 域名代理：

```text
api.example.com    -> 127.0.0.1:21114
client.example.com -> 127.0.0.1:21122
```

示例见 [`examples/nginx/`](../../examples/nginx)；完整端口表见
[反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)。不得公开 `21121`。

## 备份与更新

备份 `/app/data` 对应目录、`/run/secrets` 对应目录、配置、`.env`、Compose、Nginx 和证书。
SQLite 数据库为 `/app/data/rustdeskapi.db`。外部 MySQL/PostgreSQL 还要使用数据库厂商的
一致性备份工具。

更新：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull kessoku-api
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml logs --tail 150 kessoku-api
```

`pull`、`--force-recreate` 和 `down`/`up` 只有在同一个 `KESSOKU_DATA_DIR` 继续挂载到
`/app/data` 时才保留配对。变更前应解析宿主机绝对路径、记录 registry
`installation_id`，并在停止 Kessoku 后备份完整 `server-control/`。不要在没有通过
`docker compose config` 确认卷类型和可恢复备份时执行 `down -v`；路径变化、权限错误和
活跃身份克隆都会关闭失败。完整预检、v3.0.7 往返、跨主机接管和 purge 见
[`MIGRATION-v3.0.8.zh-CN.md`](../releases/v3.0.8/MIGRATION-v3.0.8.zh-CN.md)。
启动和 `registry status` 绝不会初始化缺失的身份状态。只有管理员核对挂载路径后，携带精确
二次确认的新建 `server-control pair create` 才能初始化真正的新 registry。
Compose 还会把 `${KESSOKU_HOST_IDENTITY_FILE:-/etc/machine-id}` 单独只读挂载到
`/run/kessoku-host-machine-id`。该文件有意位于数据树之外；使用镜像内置 machine-id 无法
识别跨宿主机身份克隆。

跨 v2/v3 升降级前阅读[升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback)。
