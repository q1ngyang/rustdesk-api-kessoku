# 快速开始

[English](Getting-Started.md) | **简体中文**

本文完成一个经过基础验证的 Kessoku API 部署。在独立验收完成前，严格连接强制认证和
Starry 配置写入会继续保持关闭。

## 前置条件

- 带受支持 Docker Engine 与 Compose plugin 的 Linux amd64；
- 已运行的 RustDesk ID Server 与 Relay Server；
- 服务端 `id_ed25519.pub` 公钥；
- Kessoku 使用的公网 HTTPS 域名；
- 升级现有 API 时准备好数据库备份。

## 准备和静态检查

```sh
cp examples/compose.env.example .env
mkdir -p data/kessoku secrets
chmod 0700 data/kessoku secrets
umask 077
openssl rand -base64 24 > secrets/bootstrap-admin-password
chown 65534:65534 secrets/bootstrap-admin-password
chmod 0600 secrets/bootstrap-admin-password
vi .env

docker compose --env-file .env -f docker-compose.yaml config
docker compose --env-file .env -f docker-compose.yaml config --quiet
```

确认解析后的镜像、监听地址、公共 API URL、ID/Relay 地址、服务端公钥和持久化路径。
仍有占位值时不得继续。

## 启动 API

```sh
docker compose --env-file .env -f docker-compose.yaml pull
docker compose --env-file .env -f docker-compose.yaml up -d
docker compose --env-file .env -f docker-compose.yaml ps
docker compose --env-file .env -f docker-compose.yaml logs --tail 100 kessoku-api
docker compose --env-file .env -f docker-compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

Kessoku 不会把可复用初始凭据写入日志。把 bootstrap 文件中的值直接转存到批准的密码
管理器，打开 `https://your-api.example/_admin/` 登录并轮换密码，然后删除宿主机 secret
文件。选择一个受支持 RustDesk 客户端，配置相同 API Server、ID Server 和 server 公钥，
再验证登录、地址簿、注销和再次登录。

## 按受控阶段继续

1. 验证并备份数据库版本 301。
2. 配置 Ed25519 access-token 密钥并开启 Kessoku 认证。
3. 在私有 mTLS 路径上线内部 JWKS/introspection listener。
4. 部署配套 Starry release，认证先用 `off`，再进入 `audit`。
5. Control Agent 先只读上线。
6. 完成支持客户端与回滚验收后，才可使用 `enforce` 或写入。

详见[连接认证](ZH-CN-Connection-Authentication.md)、
[Starry 控制](ZH-CN-Starry-Control.md)与
[运维与验证](ZH-CN-Operations-and-Verification.md)。
