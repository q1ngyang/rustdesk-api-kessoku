# 完整部署：Kessoku + Starry HBBS/HBBR

本教程从一台空白 Linux 服务器开始，在同一台主机上部署：

- Kessoku API 和管理后台；
- Kessoku 内置浏览器远控页面；
- Starry HBBS（ID 服务器）；
- 与 Starry 同版本镜像内附带的 HBBR（中继服务器）；
- Nginx、HTTPS 和 WSS。

Kessoku 也可以搭配官方 HBBS/HBBR 使用，已有服务的用户可阅读
[单独部署 Kessoku](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)。新部署推荐 Starry，是因为它与 Kessoku
由同一开发者维护，并额外提供安全 TCP、WSS 信令、按地理位置选择中继服务器、连接令牌
校验和可选管理代理。Starry 镜像中的 HBBR 是同一锁定上游版本构建的原版 HBBR，不包含
额外修改。

以下命令以 `linux/amd64` 的 Debian/Ubuntu、具有 `sudo` 权限的用户为例。示例使用三个
域名，它们可以同时指向一台服务器：

| 域名 | 服务 |
| --- | --- |
| `rustdesk.example.com` | HBBS/HBBR 的 WSS 入口；原生端口也使用该域名 |
| `api.example.com` | Kessoku API 与管理后台 |
| `client.example.com` | Kessoku 浏览器远控页面 |

## 1. 部署完成后的网络结构

```text
RustDesk 桌面客户端
  ├─ 21115/TCP        HBBS NAT 类型测试
  ├─ 21116/TCP+UDP    注册、信令、打洞和安全 TCP
  ├─ 21117/TCP        HBBR 原生中继
  └─ 443/TCP          Nginx
       ├─ rustdesk.example.com/ws/id    -> 127.0.0.1:21118
       ├─ rustdesk.example.com/ws/relay -> 127.0.0.1:21119
       ├─ api.example.com/*             -> 127.0.0.1:21114
       └─ client.example.com/*          -> 127.0.0.1:21122

Kessoku 持久数据 -> /app/data
Starry 持久数据  -> /root
```

`21114`、`21118`、`21119` 和 `21122` 都是反向代理后端，不应对公网开放。可选的
Kessoku 内部认证端口 `21121` 与 Starry 管理代理端口 `21120` 也不得公开。

## 2. 安装 Docker 并配置 DNS

按照 [Docker 官方文档](https://docs.docker.com/engine/install/)安装 Docker Engine 和
Compose 插件，然后检查：

```sh
docker version
docker compose version
```

为三个域名建立指向服务器公网 IPv4 地址的 `A` 记录。只有服务器已经正确配置公网 IPv6
时才添加 `AAAA`：

```sh
getent ahosts rustdesk.example.com
getent ahosts api.example.com
getent ahosts client.example.com
```

三个结果都正确后再申请证书。错误的 `AAAA` 记录会让部分客户端优先连接到不可达地址。

## 3. 下载联合部署文件

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" /opt/rustdesk-stack
cd /opt/rustdesk-stack

base_url=https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/combined
curl -fsSLo compose.yaml "$base_url/compose.yaml"
curl -fsSLo .env "$base_url/.env.example"
curl -fsSLo kessoku-config.yaml "$base_url/kessoku-config.yaml"
curl -fsSLo starry-config.yaml "$base_url/starry-config.yaml"
curl -fsSLo nginx-bootstrap.conf "$base_url/nginx-bootstrap.conf.example"
curl -fsSLo nginx-rustdesk.conf "$base_url/nginx.conf.example"

chmod 0600 .env
chmod 0644 compose.yaml kessoku-config.yaml starry-config.yaml \
  nginx-bootstrap.conf nginx-rustdesk.conf
```

生产环境可把下载地址中的 `master` 换成经过审核的发布标签，从而固定部署文件版本。

## 4. 创建持久化目录和 Kessoku 密钥

```sh
sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/rustdesk-stack/data/kessoku \
  /opt/rustdesk-stack/secrets/kessoku
sudo install -d -m 0700 -o root -g root \
  /opt/rustdesk-stack/data/starry

sudo openssl genpkey -algorithm ED25519 \
  -out /opt/rustdesk-stack/secrets/kessoku/kessoku-access-ed25519.pem
openssl rand -base64 24 | sudo tee \
  /opt/rustdesk-stack/secrets/kessoku/bootstrap-admin-password >/dev/null
sudo chown 65534:65534 /opt/rustdesk-stack/secrets/kessoku/*
sudo chmod 0600 /opt/rustdesk-stack/secrets/kessoku/*
```

为什么使用不同所有者：Kessoku 镜像以 `65534:65534` 非特权用户运行，需要写入自己的
数据库并读取签名密钥；HBBS/HBBR 镜像把 `/root` 作为数据目录。不要用 `chmod 777` 解决
权限问题。

需要持久保存和备份的内容：

| 宿主机路径 | 内容 | 重要性 |
| --- | --- | --- |
| `data/kessoku/` | Kessoku SQLite 数据库 | **必须** |
| `secrets/kessoku/` | 访问令牌签名私钥；首次密码文件使用后应删除 | **必须** |
| `data/starry/` | HBBS 身份密钥、服务器数据库和可选 MMDB | **必须** |
| `kessoku-config.yaml` | Kessoku 功能配置 | 必须 |
| `starry-config.yaml` | Starry 中继/WSS/地理位置/认证配置 | 必须 |
| `.env`、`compose.yaml` | 镜像版本、路径和服务编排 | 必须 |
| `/etc/letsencrypt/`、Nginx 站点文件 | TLS 证书及反向代理 | **必须** |

## 5. 修改 `.env`

```sh
editor /opt/rustdesk-stack/.env
```

先修改域名、公开地址和绝对路径：

```dotenv
RUSTDESK_DOMAIN=rustdesk.example.com
KESSOKU_PUBLIC_URL=https://api.example.com

KESSOKU_CONFIG_FILE=/opt/rustdesk-stack/kessoku-config.yaml
KESSOKU_DATA_DIR=/opt/rustdesk-stack/data/kessoku
KESSOKU_SECRETS_DIR=/opt/rustdesk-stack/secrets/kessoku
STARRY_CONFIG_FILE=/opt/rustdesk-stack/starry-config.yaml
STARRY_DATA_DIR=/opt/rustdesk-stack/data/starry
```

镜像建议固定为：

```dotenv
KESSOKU_IMAGE=ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.1
STARRY_IMAGE=ghcr.io/q1ngyang/rustdesk-server-starry
STARRY_VERSION=1.1.16-patch-v1.2.0
```

此时暂时保留：

```dotenv
RUSTDESK_SERVER_PUBLIC_KEY=REPLACE_AFTER_FIRST_HBBS_START
```

HBBS 第一次启动后会生成公钥，稍后再替换。不要提前自行生成另一套公钥，也不要把
Kessoku 的令牌签名公钥误填到这里。

## 6. 修改两个 YAML 配置

编辑 `starry-config.yaml`，把所有 `rustdesk.example.com` 换成实际 RustDesk 域名，把
`https://client.example.com` 换成实际浏览器客户端地址。

初次部署建议设置：

| 参数 | 初始值 | 原因 |
| --- | --- | --- |
| `secure_tcp.mode` | `auto` | 为兼容客户端提供安全 TCP，同时保留有效的普通首帧兼容 |
| `websocket_signal.enabled` | `true` | 内置浏览器客户端需要 `/ws/id` 和 `/ws/relay` |
| `websocket_signal.allowed_origins` | 精确的 `https://client.example.com` | 允许该浏览器站点发起 WSS；不要使用通配符 |
| `connection_auth.mode` | `off` | 尚未配置双向 TLS，不能直接强制拦截 |
| `geo.enabled` | `false` | 尚未提供有合法来源的 MMDB 文件 |

编辑 `kessoku-config.yaml`，替换所有三个示例域名。公钥项暂时保留占位值。主要设置：

| 参数 | 初始值 | 说明 |
| --- | --- | --- |
| `app.register` | `false` | 安全默认；由管理员创建用户。需要开放注册时改为 `true` |
| `app.register-status` | `1` | 注册后立即启用；设为 `2` 可由管理员审核 |
| `app.show-swagger` | `0` | 不在公网开放接口调试页 |
| `web-client.mode` | `builtin` | 启用内置浏览器远控 |
| `auth.enabled` | `true` | 使用 Ed25519 签发访问令牌，浏览器客户端必需 |
| `auth.current-key.private-key-file` | `/run/secrets/kessoku-access-ed25519.pem` | 对应刚生成的只读文件 |
| `gorm.type` | `sqlite` | 单机入门最简单；数据库在持久化目录 |
| `server-control.read-only` | `true` | 未部署管理代理时保持只读 |
| `server-control.instances` | `[]` | 未部署管理代理时保持为空 |

API 域名和浏览器客户端域名必须不同。`web-client.relay-wss-urls` 的键必须与
`starry-config.yaml` 中的 `relay_servers` 项完全一致，包括 `:21117`。

## 7. 配置防火墙和云安全组

远程修改防火墙前，先放行服务器实际使用的 SSH 端口。

| 端口 | 公网规则 | 用途 |
| --- | --- | --- |
| `21115/TCP` | 放行 | HBBS NAT 类型测试 |
| `21116/TCP` | 放行 | 注册、信令和安全 TCP |
| `21116/UDP` | 放行 | ID 注册和打洞 |
| `21117/TCP` | 放行 | HBBR 原生中继数据 |
| `80/TCP` | 放行 | 申请证书和 HTTP 跳转 |
| `443/TCP` | 放行 | API、管理后台、浏览器客户端和 WSS |
| `21114/TCP` | **禁止公网访问** | Kessoku API 后端 |
| `21118/TCP`、`21119/TCP` | **禁止公网访问** | 明文 WebSocket 后端 |
| `21120/TCP`、`21121/TCP`、`21122/TCP` | **禁止公网访问** | 管理代理、内部认证、浏览器页面后端 |

UFW 示例（SSH 不是 `22` 时先替换第一条）：

```sh
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 21115/tcp
sudo ufw allow 21116/tcp
sudo ufw allow 21116/udp
sudo ufw allow 21117/tcp
sudo ufw deny 21114/tcp
sudo ufw deny 21118/tcp
sudo ufw deny 21119/tcp
sudo ufw deny 21120/tcp
sudo ufw deny 21121/tcp
sudo ufw deny 21122/tcp
sudo ufw enable
sudo ufw status numbered
```

云服务器安全组也要配置相同的公网放行端口。Kessoku Compose 已把 `21114`、`21122`
绑定到 `127.0.0.1`；不要改成 `0.0.0.0`。Starry 使用主机网络，外部访问控制依赖主机
防火墙和云安全组。

## 8. 安装 Nginx 并申请三个证书

```sh
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

cd /opt/rustdesk-stack
sudo cp nginx-bootstrap.conf /etc/nginx/sites-available/rustdesk-stack.conf
sudo editor /etc/nginx/sites-available/rustdesk-stack.conf
sudo ln -sfn /etc/nginx/sites-available/rustdesk-stack.conf \
  /etc/nginx/sites-enabled/rustdesk-stack.conf
sudo nginx -t
sudo systemctl reload nginx

sudo certbot certonly --nginx -d rustdesk.example.com
sudo certbot certonly --nginx -d api.example.com
sudo certbot certonly --nginx -d client.example.com
```

取得证书后安装最终配置，并逐一检查域名和证书路径：

```sh
sudo cp nginx-rustdesk.conf /etc/nginx/sites-available/rustdesk-stack.conf
sudo editor /etc/nginx/sites-available/rustdesk-stack.conf
sudo nginx -t
sudo systemctl reload nginx
```

如果 `nginx -t` 失败，应修正错误后再重新加载，不能删除 TLS 或代理指令来绕过检查。
最终配置只允许精确的 `/ws/id`、`/ws/relay`，并保留 API 和浏览器客户端两个独立站点。

## 9. 首次启动 HBBS 和 HBBR

`.env` 中的公钥仍是占位值，因此先只启动 Starry 服务：

```sh
cd /opt/rustdesk-stack
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull hbbs hbbr
docker compose --env-file .env -f compose.yaml up -d hbbs hbbr
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbs hbbr
```

确认身份文件已生成：

```sh
sudo test -s /opt/rustdesk-stack/data/starry/id_ed25519
sudo test -s /opt/rustdesk-stack/data/starry/id_ed25519.pub
sudo cat /opt/rustdesk-stack/data/starry/id_ed25519.pub
```

最后一条只显示公钥。绝对不要显示或复制没有 `.pub` 后缀的私钥。

把公钥的完整单行内容粘贴到两个位置：

1. `.env` 的 `RUSTDESK_SERVER_PUBLIC_KEY=`；
2. `kessoku-config.yaml` 的 `web-client.server-public-key`。

`kessoku-config.yaml` 中的 `relay-wss-urls` 应保持：

```yaml
relay-wss-urls:
  "rustdesk.example.com:21117": "wss://rustdesk.example.com/ws/relay"
```

检查所有文件是否仍有占位值：

```sh
grep -RniE 'example\.com|REPLACE|replace-with' \
  .env kessoku-config.yaml starry-config.yaml \
  /etc/nginx/sites-available/rustdesk-stack.conf
```

命令没有输出后再启动 API。

## 10. 启动 Kessoku

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 150 kessoku-api
```

设置首次管理员密码：

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

打开 `https://api.example.com/_admin/`，用 `admin` 和密码文件中的值登录。把密码保存到
密码管理器，在后台再次修改密码，确认新密码可登录后删除一次性文件：

```sh
sudo rm /opt/rustdesk-stack/secrets/kessoku/bootstrap-admin-password
```

在“用户管理”中创建一个普通用户。需要允许用户自行注册时，再把
`kessoku-config.yaml` 的 `app.register` 改为 `true` 并重建 Kessoku 容器。

## 11. 验证原生 RustDesk 客户端

在两台 RustDesk 桌面客户端中打开“设置 → 网络”：

| 设置项 | 内容 |
| --- | --- |
| ID 服务器 | `rustdesk.example.com` |
| API 服务器 | `https://api.example.com` |
| Key | `id_ed25519.pub` 完整单行内容 |
| 中继服务器 | 留空，让 Starry HBBS 返回配置的中继服务器 |
| 使用 WebSocket | 第一次测试先关闭 |

重新启动或重新连接两台客户端，用普通 Kessoku 用户登录，然后依次验证：

1. 两台客户端都获得 ID；
2. 登录、地址簿同步、注销和再次登录正常；
3. 发起真实远程控制，画面、鼠标和键盘正常；
4. 临时使用客户端的“始终通过中继连接”功能，完成一次真实中继会话；
5. 对照同一时间段的 HBBS/HBBR 日志。

```sh
docker compose --env-file .env -f compose.yaml logs --since 10m hbbs hbbr kessoku-api
```

点对点连接成功不能证明 HBBR 正常；API 登录成功也不能证明 HBBS 信令正常。

## 12. 验证 WSS 和浏览器远控

先检查两个 WebSocket 入口。HTTP `101 Switching Protocols` 表示 TLS 与协议升级初步正常；
升级后的连接会保持打开，所以 `curl` 最后可能因五秒上限退出。

```sh
for path in ws/id ws/relay; do
  curl --http1.1 --include --no-buffer --max-time 5 \
    -H 'Connection: Upgrade' \
    -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' \
    -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
    "https://rustdesk.example.com/$path"
done
```

检查 Kessoku：

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
```

浏览器配置响应中不应出现密码、私钥或访问令牌。打开
`https://client.example.com/`，用普通账户登录，输入一台在线 RustDesk 设备的 ID 和被控端
密码，完成画面、鼠标、基本键盘和退出登录测试。

然后在两台桌面客户端中启用“使用 WebSocket”，验证 WSS↔WSS 会话；需要兼容受限网络时，
再分别测试一端开启、一端关闭的两种组合。仅有 HTTP 101 不能代替真实桌面会话。

## 13. 功能验收清单

基础部署完成后，建议逐项确认：

- 管理员登录、修改密码和注销；
- 创建、禁用、删除用户和撤销用户会话；
- 普通用户登录 RustDesk 客户端；
- 个人地址簿、标签、公共地址簿、地址簿集合；
- 设备信息、用户组、设备组和管理员范围；
- 登录记录、连接审计和文件审计；
- 原生点对点、原生中继、WSS 中继；
- 浏览器远控的登录、短期连接令牌、鼠标、键盘和退出；
- 数据库、Starry 身份密钥和 Kessoku 签名密钥的备份恢复。

LDAP、OAuth/OIDC、外部 MySQL/PostgreSQL、Starry 连接强制认证和管理代理需要额外的身份
系统或证书，不适合用示例密码直接启用。配置方法见
[配置参数参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Configuration-Reference)、
[连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication)和
[Starry 管理](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control)。

## 14. 地理位置规则（可选）

Starry 镜像不包含 GeoLite2/MMDB 数据库。请自行选择合法、可信的数据来源并遵守许可证。
把文件放到：

```text
/opt/rustdesk-stack/data/starry/mmdb/GeoLite2-Country.mmdb
/opt/rustdesk-stack/data/starry/mmdb/GeoLite2-City.mmdb
/opt/rustdesk-stack/data/starry/mmdb/GeoLite2-ASN.mmdb
```

国家匹配需要 Country 或包含国家信息的 City 数据库；城市需要 City；ASN/运营商需要 ASN。
准备好文件并写好规则后才把 `starry-config.yaml` 的 `geo.enabled` 改为 `true`。保留最后一条
全匹配兜底规则，然后重启 HBBS 并检查日志是否明确接受配置：

```sh
docker compose --env-file .env -f compose.yaml restart hbbs
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbs
```

规则按顺序选择第一台可用中继服务器，不是随机负载均衡。

## 15. 备份和更新

至少备份：

```text
/opt/rustdesk-stack/data/kessoku/
/opt/rustdesk-stack/secrets/kessoku/
/opt/rustdesk-stack/data/starry/
/opt/rustdesk-stack/.env
/opt/rustdesk-stack/compose.yaml
/opt/rustdesk-stack/kessoku-config.yaml
/opt/rustdesk-stack/starry-config.yaml
/etc/nginx/sites-available/rustdesk-stack.conf
/etc/letsencrypt/
```

特别保护：

- `data/starry/id_ed25519`：Starry 服务器身份私钥；
- `data/starry/db_v2.sqlite3`：RustDesk Server 数据；
- `data/kessoku/rustdeskapi.db`：Kessoku 账户和地址簿数据；
- `secrets/kessoku/kessoku-access-ed25519.pem`：Kessoku 令牌签名私钥。

更新时每次只改一个层次，先备份并检查 Compose：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbs hbbr kessoku-api
```

不要删除持久化目录来解决升级问题，也不要在故障时重新生成 `id_ed25519`。详细恢复步骤见
[升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback)。

## 16. 下一步：安全启用高级集成

本教程有意让 `connection_auth.mode` 保持 `off`，`server-control.instances` 保持为空。
这些设置不会影响账户、地址簿、原生远控、WSS 或浏览器远控。

连接认证需要 Kessoku 在私有 `21121` 上提供双向 TLS 的 JWKS/令牌状态查询接口，并让
Starry 先运行一段时间的 `audit`。管理代理需要独立的双向 TLS 证书和服务令牌密钥，默认
只能读取。不要公开这两个端口，也不要直接从 `off` 切换到 `enforce`。

继续阅读：

- [增加纯中继节点](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Relay-Only-Deployment)
- [反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)
- [配置参数参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Configuration-Reference)
- [连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication)
- [Starry 管理](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control)
- [日常运维与验证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Operations-and-Verification)
