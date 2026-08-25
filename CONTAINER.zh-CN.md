# rustdesk-api-kessoku 容器镜像

[English](CONTAINER.md) | **简体中文**

镜像地址：
[`ghcr.io/q1ngyang/rustdesk-api-kessoku`](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)

当前稳定版 `v3.0.1`，正式支持 `linux/amd64`。

## 拉取和固定版本

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.1
docker image inspect ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.1 \
  --format '{{json .RepoDigests}}'
```

生产环境使用明确版本或内容摘要。`latest` 会随新稳定版移动，只适合已经准备自动升级和
回退的环境。

## 镜像约定

| 项目 | 值 |
| --- | --- |
| 运行用户 | `65534:65534` |
| 持久化目录 | `/app/data` |
| 配置文件 | `/app/conf/config.yaml` |
| 只读密钥目录 | `/run/secrets`（由 Compose 挂载） |
| 公共 API/管理后台 | `21114/TCP` |
| 内部认证接口 | `21121/TCP`，默认关闭，不得公开 |
| 浏览器客户端 | `21122/TCP`，使用独立 HTTPS 域名 |

镜像包含 API、管理后台和浏览器客户端，不包含 HBBS、HBBR、数据库密码或部署私钥。

## 推荐 Compose

仅部署 Kessoku：

- [`docker-compose.yaml`](docker-compose.yaml)
- [`examples/compose.env.example`](examples/compose.env.example)
- [`examples/config.docker-builtin.yaml`](examples/config.docker-builtin.yaml)
- [完整教程](docs/wiki/ZH-CN-Getting-Started.md)

Kessoku + Starry HBBS/HBBR：

- [`examples/combined/compose.yaml`](examples/combined/compose.yaml)
- [`examples/combined/.env.example`](examples/combined/.env.example)
- [联合完整教程](docs/wiki/ZH-CN-Complete-Deployment.md)

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
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 120 kessoku-api
```

启动前必须替换 `.env` 和 `config.yaml` 中全部示例域名、公钥和路径。`docker compose
config` 不会把 `example.com` 识别为业务错误。

## 首次管理员密码

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

打开 `https://api.example.com/_admin/`，用户名为 `admin`。保存密码并在后台再次修改，确认新
密码可用后删除一次性文件。Kessoku 不会在日志输出可复用的初始密码。

## Nginx 和防火墙

Compose 默认把 `21114`、`21122` 绑定到宿主机 `127.0.0.1`。使用两个 HTTPS 域名代理：

```text
api.example.com    -> 127.0.0.1:21114
client.example.com -> 127.0.0.1:21122
```

示例见 [`examples/nginx/`](examples/nginx)；完整端口表见
[反向代理与防火墙](docs/wiki/ZH-CN-Reverse-Proxy-and-Firewall.md)。不得公开 `21121`。

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

跨 v2/v3 升降级前阅读[升级与回退](docs/wiki/ZH-CN-Upgrade-and-Rollback.md)。
