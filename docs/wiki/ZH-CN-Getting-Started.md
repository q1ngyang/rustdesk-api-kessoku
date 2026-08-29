# 快速开始：单独部署 Kessoku

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Getting-Started) | **简体中文**

本教程面向第一次使用 Docker 的用户，从一台已经运行 HBBS/HBBR 的 Linux 服务器开始，
部署 Kessoku API、管理后台和内置浏览器远控页面。完成后可以使用 RustDesk 客户端登录、
管理用户和设备、同步地址簿，并通过独立网页发起远程控制。

如果你还没有 HBBS/HBBR，直接阅读
[完整部署：Kessoku + Starry](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)。Kessoku 可以搭配官方
HBBS/HBBR；本项目更推荐同一开发者维护的
[`q1ngyang/rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)。

本文使用以下示例，请在所有文件中换成自己的值：

| 示例 | 含义 |
| --- | --- |
| `api.example.com` | Kessoku API 与管理后台域名 |
| `client.example.com` | 内置浏览器远控的独立域名 |
| `rustdesk.example.com` | 已有 HBBS/HBBR 的域名 |
| `/opt/rustdesk-api-kessoku` | 宿主机部署目录 |

## 1. 部署前准备

你需要：

- `linux/amd64` 的 Debian 或 Ubuntu 服务器，以及具有 `sudo` 权限的用户；
- 已安装 Docker Engine 和 Docker Compose 插件；
- 两个指向该服务器的域名：`api.example.com`、`client.example.com`；
- 已运行的 HBBS/HBBR 地址和 `id_ed25519.pub` 公钥；
- HBBS 的 `/ws/id` 与 HBBR 的 `/ws/relay` 已通过有效证书提供 WSS。内置浏览器远控必须
  使用这两条路径；如果暂时没有 WSS，可以先把浏览器客户端关闭。

检查 Docker：

```sh
docker version
docker compose version
```

为两个域名添加指向服务器公网地址的 DNS `A` 记录。只有服务器确实配置了公网 IPv6 时
才添加 `AAAA` 记录：

```sh
getent ahosts api.example.com
getent ahosts client.example.com
```

## 2. 下载部署文件

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" \
  /opt/rustdesk-api-kessoku
cd /opt/rustdesk-api-kessoku

curl -fsSLo compose.yaml \
  https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/docker-compose.yaml
curl -fsSLo .env \
  https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/compose.env.example
curl -fsSLo config.yaml \
  https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/config.docker-builtin.yaml
curl -fsSLo nginx-bootstrap.conf \
  https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/nginx/kessoku-bootstrap.conf.example
curl -fsSLo nginx-kessoku.conf \
  https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/nginx/kessoku.example.conf

chmod 0600 .env
chmod 0644 compose.yaml config.yaml nginx-bootstrap.conf nginx-kessoku.conf
```

## 3. 创建持久化目录和签名密钥

Kessoku 镜像使用 UID/GID `65534:65534`，宿主机目录必须允许该用户读写。不要只创建
`0700` 目录而忘记修改所有者，否则容器会因无法创建数据库或读取密钥而启动失败。

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

各路径的作用：

| 宿主机路径 | 容器内路径 | 要求 |
| --- | --- | --- |
| `data/kessoku` | `/app/data` | **必须持久化。** SQLite 数据库位于其中 |
| `secrets` | `/run/secrets` | **必须保存并备份。** 只读挂载签名密钥和可选证书 |
| `config.yaml` | `/app/conf/config.yaml` | 必须保存；配置中不要直接写密码或私钥内容 |
| `.env` | 无 | 必须保存；权限建议为 `0600` |

## 4. 修改 `.env`

打开文件：

```sh
editor .env
```

至少修改以下内容：

```dotenv
KESSOKU_IMAGE=ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.5
KESSOKU_PUBLIC_URL=https://api.example.com

RUSTDESK_ID_SERVER=rustdesk.example.com:21116
RUSTDESK_RELAY_SERVER=rustdesk.example.com:21117
RUSTDESK_SERVER_PUBLIC_KEY=这里粘贴_id_ed25519.pub_的完整单行内容

KESSOKU_CONFIG_FILE=./config.yaml
KESSOKU_DATA_DIR=./data/kessoku
KESSOKU_SECRETS_DIR=./secrets
```

下列值建议保持不变：

```dotenv
KESSOKU_BIND_ADDRESS=127.0.0.1
KESSOKU_API_PORT=21114
KESSOKU_CLIENT_BIND_ADDRESS=127.0.0.1
KESSOKU_CLIENT_PORT=21122
KESSOKU_LANG=zh-CN
```

`RUSTDESK_SERVER_PUBLIC_KEY` 是公钥，不是 `id_ed25519` 私钥，也不是登录令牌。不要在终端、
工单或聊天中输出私钥。

`KESSOKU_TRUST_PROXY` 可以先留空，此时日志会记录直接连接 Kessoku 的代理地址。确实需要
记录真实客户端地址时，先查明 Nginx 访问容器时使用的精确 Docker 网关地址，再只信任该
地址；不要设置为整个互联网网段。

## 5. 修改 `config.yaml`

```sh
editor config.yaml
```

必须修改：

1. `web-client.public-origin` → `https://client.example.com`；
2. `web-client.api-origin` 和 `auth.issuer` → `https://api.example.com`；
3. `web-client.rendezvous-wss-url` → `wss://rustdesk.example.com/ws/id`；
4. `web-client.relay-wss-urls` 的键必须与 HBBS 返回的中继名称完全相同，值为
   `wss://rustdesk.example.com/ws/relay`；
5. `web-client.server-public-key` → `id_ed25519.pub` 的完整单行内容；
6. `rustdesk.id-server`、`relay-server`、`api-server` 中的示例值。Compose 会用 `.env`
   覆盖这些标量，但保留正确值有助于人工检查。

推荐保持：

```yaml
app:
  register: false          # 由管理员创建用户；确需开放注册时再改为 true
  register-status: 1       # 新注册用户立即启用；需要审核时改为 2
  captcha-threshold: 3
  ban-threshold: 10
  show-swagger: 0          # 公网部署不要开放接口调试页

auth:
  enabled: true            # 内置浏览器远控和 Starry 连接认证的基础
  legacy-token-read-enabled: false

server-control:
  read-only: true
  instances: []            # 未部署 Starry 管理代理时保持为空

proxy:
  enable: false
```

如果已有 HBBS/HBBR 没有可用的 WSS 路径，先设置：

```yaml
web-client:
  mode: "disabled"
```

这样桌面客户端的账户、地址簿和管理功能仍可使用，但 `client.example.com` 不提供浏览器
远控。部署 WSS 后再按[客户端使用方法](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Web-Client)启用。

检查是否还残留占位值：

```sh
grep -RniE 'example\.com|REPLACE|replace-with|这里粘贴' .env config.yaml
```

命令没有输出后再继续。

## 6. 配置防火墙

如果这台服务器只运行 Kessoku，对公网只需开放 SSH、HTTP 和 HTTPS：

| 端口 | 是否开放 | 用途 |
| --- | --- | --- |
| 实际 SSH 端口/TCP | 是，仅允许可信来源更好 | 服务器管理 |
| `80/TCP` | 是 | 申请证书和跳转到 HTTPS |
| `443/TCP` | 是 | API、管理后台和浏览器客户端 |
| `21114/TCP` | **否** | Kessoku API 后端，仅本机 Nginx 访问 |
| `21122/TCP` | **否** | 浏览器客户端后端，仅本机 Nginx 访问 |
| `21121/TCP` | **否** | 可选内部认证接口，只允许 Starry 私有通道访问 |

UFW 示例（SSH 不是 `22` 时先替换第一条）：

```sh
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 21114/tcp
sudo ufw deny 21121/tcp
sudo ufw deny 21122/tcp
sudo ufw enable
sudo ufw status numbered
```

如果 HBBS/HBBR 也在同一主机，还要按
[反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)开放 `21115/TCP`、
`21116/TCP+UDP` 和 `21117/TCP`。云服务器安全组需要同步设置；主机防火墙不会自动修改
云平台安全组。

## 7. 安装 Nginx 并申请证书

```sh
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
sudo cp nginx-bootstrap.conf /etc/nginx/sites-available/kessoku.conf
sudo editor /etc/nginx/sites-available/kessoku.conf
sudo ln -sfn /etc/nginx/sites-available/kessoku.conf \
  /etc/nginx/sites-enabled/kessoku.conf
sudo nginx -t
sudo systemctl reload nginx

sudo certbot certonly --nginx -d api.example.com
sudo certbot certonly --nginx -d client.example.com
```

复制最终配置并再次替换域名和证书路径：

```sh
sudo cp nginx-kessoku.conf /etc/nginx/sites-available/kessoku.conf
sudo editor /etc/nginx/sites-available/kessoku.conf
sudo nginx -t
sudo systemctl reload nginx
```

API 与浏览器客户端必须使用两个不同的 HTTPS 域名，不能把浏览器客户端放在
`https://api.example.com/client/` 之类的子路径中。Nginx 只代理 `127.0.0.1:21114` 和
`127.0.0.1:21122`，不得代理内部端口 `21121`。

## 8. 检查并启动 Kessoku

```sh
cd /opt/rustdesk-api-kessoku
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 120 kessoku-api
```

如果日志出现 `permission denied`，检查 `data/kessoku`、`secrets` 及其中文件是否属于
`65534:65534`。如果出现配置校验错误，不要删除配置项来绕过校验，应按错误信息修正域名、
公钥、签名密钥路径或 WSS 映射。

设置首次管理员密码：

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

把密码保存到密码管理器，然后登录 `https://api.example.com/dash/`，用户名为 `admin`。
登录后立即在后台再次修改密码，确认成功后删除一次性密码文件：

```sh
sudo rm /opt/rustdesk-api-kessoku/secrets/bootstrap-admin-password
```

如果 `app.register: false`，请在管理后台的“用户管理”中创建普通用户。

## 9. 配置 RustDesk 桌面客户端

在 RustDesk 客户端中打开“设置 → 网络”，填写：

| 设置项 | 值 |
| --- | --- |
| ID 服务器 | `rustdesk.example.com`，非默认端口时写 `:21116` |
| API 服务器 | `https://api.example.com` |
| Key | `id_ed25519.pub` 的完整单行内容 |
| 中继服务器 | Starry 动态分配时留空；其他 HBBS 按其文档设置 |
| 使用 WebSocket | 第一次先关闭；确认 WSS 正常后再测试 |

用刚创建的普通用户登录，验证地址簿同步、设备显示、注销和再次登录。至少用两台客户端
完成一次真实远程控制和一次强制中继连接；仅登录成功不能证明 HBBS/HBBR 链路正常。

## 10. 验证浏览器远控

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
```

第二个响应应只包含公开的 API/WSS 地址、公钥指纹和配置代号，不应包含密码、私钥或访问
令牌。打开 `https://client.example.com/`，使用普通用户登录，输入目标 RustDesk ID 和
被控端密码，验证画面、鼠标与基本键盘。

浏览器端强制使用 WSS 中继，不会尝试点对点连接。无法打开时，依次检查：

1. `web-client.mode` 是否为 `builtin`；
2. `auth.enabled` 是否为 `true`；
3. `public-origin`、`api-origin` 是否与浏览器地址完全一致；
4. `/ws/id`、`/ws/relay` 是否返回有效的 WebSocket 升级；
5. `relay-wss-urls` 的键是否与 HBBS 返回的中继名称完全一致。

## 11. 备份与日常更新

至少备份：

```text
/opt/rustdesk-api-kessoku/data/kessoku/
/opt/rustdesk-api-kessoku/secrets/
/opt/rustdesk-api-kessoku/.env
/opt/rustdesk-api-kessoku/config.yaml
/opt/rustdesk-api-kessoku/compose.yaml
/etc/nginx/sites-available/kessoku.conf
/etc/letsencrypt/
```

SQLite 数据库是 `data/kessoku/rustdeskapi.db`。签名私钥丢失会让已有令牌失效，因此数据库
和密钥要在同一套备份策略中保存。更新前先备份，再修改固定镜像版本并执行：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml logs --tail 120 kessoku-api
```

需要同时部署 HBBS/HBBR、WSS 和完整 Nginx 的用户，继续阅读
[Kessoku + Starry 完整部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)。
