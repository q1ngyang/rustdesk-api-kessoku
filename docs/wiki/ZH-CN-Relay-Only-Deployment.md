# 纯中继节点部署：仅运行 HBBR

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Relay-Only-Deployment) | **简体中文**

本教程面向第一次使用 Docker 的用户，在一台独立 Linux 服务器上只部署 RustDesk 中继
服务器 HBBR。该节点不运行 Kessoku、HBBS 或账户服务，适用于：

- 中心服务器带宽不足，希望把远控中继流量分散到其他地区或线路；
- 已有 Kessoku + Starry/官方 HBBS，需要增加一台或多台中继服务器；
- 希望为浏览器远控提供独立的 `wss://relay-1.example.com/ws/relay` 入口。

纯中继节点兼容官方
[`rustdesk/rustdesk-server:1.1.16`](https://hub.docker.com/r/rustdesk/rustdesk-server/tags?name=1.1.16)
的 HBBR。示例默认推荐
[`q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0`](https://github.com/q1ngyang/rustdesk-server-starry)
镜像内附带的 HBBR：这份 HBBR
仍是未经修改的官方上游程序，但与 Starry HBBS 使用同一固定上游版本，可以减少分别升级
造成的版本差异。仓库的 YAML 和 `.env` 示例同时保留两种镜像写法，官方镜像默认已注释。

## 1. 先理解纯中继节点的职责

```text
RustDesk 客户端 A ── 注册/信令 ──> 中心 HBBS <── 注册/信令 ── 客户端 B
       │                                  │
       └──── 21117/TCP 或 WSS ──> 纯中继 HBBR <────┘
                                  relay-1.example.com

Kessoku：账户、登录和地址簿
HBBS：设备注册、信令和选择中继服务器
HBBR：只转发远控会话数据
```

HBBR 不负责账户登录、设备 ID 分配、地理位置判断或中继选择。中心 HBBS 必须把这台节点的
公网地址返回给客户端。API 登录成功不能证明中继节点可用。

## 2. 部署前需要准备

你需要：

- 一台 `linux/amd64` Debian/Ubuntu 服务器和具有 `sudo` 权限的用户；
- Docker Engine 与 Docker Compose 插件；
- 公网可访问的固定 IPv4 地址；
- 一个域名，例如 `relay-1.example.com`；
- 中心 HBBS 的 `id_ed25519.pub` **完整单行公钥**；
- 修改中心 HBBS 中继服务器列表的权限。

如果只使用原生中继，域名和 `21117/TCP` 即可。Kessoku 内置浏览器远控强制使用 WSS，
因此还需要有效 HTTPS 证书、Nginx、`443/TCP` 和精确的 `/ws/relay` 路径。

检查 Docker：

```sh
docker version
docker compose version
```

为域名添加指向中继节点公网 IPv4 的 DNS `A` 记录。只有服务器确实配置了公网 IPv6 时才
添加 `AAAA`：

```sh
getent ahosts relay-1.example.com
```

## 3. 下载示例文件

```sh
sudo install -d -m 0750 -o "$(id -u)" -g "$(id -g)" /opt/rustdesk-relay
cd /opt/rustdesk-relay

base_url=https://raw.githubusercontent.com/q1ngyang/rustdesk-api-kessoku/master/examples/relay
curl -fsSLo compose.yaml "$base_url/compose.yaml"
curl -fsSLo .env "$base_url/.env.example"
curl -fsSLo nginx-bootstrap.conf "$base_url/nginx-bootstrap.conf.example"
curl -fsSLo nginx-relay.conf "$base_url/nginx.conf.example"

chmod 0600 .env
chmod 0644 compose.yaml nginx-bootstrap.conf nginx-relay.conf
sudo install -d -m 0700 -o root -g root /opt/rustdesk-relay/data
```

需要持久保存的路径：

| 宿主机路径 | 内容 | 要求 |
| --- | --- | --- |
| `/opt/rustdesk-relay/.env` | 镜像选择、中心公钥和带宽限制 | 必须；`0600` |
| `/opt/rustdesk-relay/compose.yaml` | HBBR 编排 | 必须 |
| `/opt/rustdesk-relay/data/` | HBBR 工作目录 | 推荐持久化；root `0700` |
| Nginx 站点文件、`/etc/letsencrypt/` | 可选 WSS 配置和证书 | 启用 WSS 时必须 |

纯中继节点**不需要也不应该保存**中心的 `id_ed25519` 私钥。`KEY` 环境变量只使用中心
`id_ed25519.pub` 的公钥内容。

## 4. 选择 HBBR 镜像

默认的 `compose.yaml` 启用推荐的 Starry 镜像：

```yaml
image: ${STARRY_IMAGE:-ghcr.io/q1ngyang/rustdesk-server-starry}:${STARRY_VERSION:-1.1.16-patch-v1.2.0}
# image: ${OFFICIAL_HBBR_IMAGE:-rustdesk/rustdesk-server:1.1.16}
```

`.env` 同样默认启用 Starry：

```dotenv
STARRY_IMAGE=ghcr.io/q1ngyang/rustdesk-server-starry
STARRY_VERSION=1.1.16-patch-v1.2.0
# OFFICIAL_HBBR_IMAGE=rustdesk/rustdesk-server:1.1.16
```

推荐保持默认，尤其是中心使用同版本 Starry HBBS 时。这样 HBBS 与 HBBR 都来自锁定的
RustDesk Server `1.1.16` 上游版本。

如果需要改用官方 HBBR，必须同时完成：

1. 在 `compose.yaml` 注释 Starry `image:` 行，取消注释官方 `image:` 行；
2. 在 `.env` 注释 `STARRY_IMAGE`、`STARRY_VERSION`，取消注释
   `OFFICIAL_HBBR_IMAGE`；
3. 重新运行 `docker compose config`，确认最终只出现官方镜像。

不要同时保留两个有效的 `image:` 键，也不要使用 `latest`。两种选择都只运行镜像中的
`hbbr` 命令，不会在中继节点启动 HBBS。

## 5. 填写 `.env`

打开文件：

```sh
editor /opt/rustdesk-relay/.env
```

把中心公钥粘贴为一行：

```dotenv
RUSTDESK_PUBLIC_KEY=这里粘贴中心_id_ed25519.pub_的完整单行内容
RELAY_DATA_DIR=/opt/rustdesk-relay/data
```

获取中心公钥时只读取带 `.pub` 后缀的文件。例如中心部署目录为
`/opt/rustdesk-stack`：

```sh
sudo cat /opt/rustdesk-stack/data/starry/id_ed25519.pub
```

不要执行 `cat id_ed25519`，不要复制私钥，也不要为每台中继节点另外生成一套密钥。

带宽参数可先使用示例值：

| 参数 | 默认示例 | 单位和作用 |
| --- | ---: | --- |
| `RELAY_SINGLE_BANDWIDTH` | `128` | 单个中继会话上限，Mb/s |
| `RELAY_TOTAL_BANDWIDTH` | `1024` | HBBR 总带宽上限，Mb/s |
| `RELAY_LIMIT_SPEED` | `32` | 长时间高占用会话降速后的上限，Mb/s |
| `RELAY_DOWNGRADE_START_CHECK` | `1800` | 开始判断降速前的秒数 |
| `RELAY_DOWNGRADE_THRESHOLD` | `0.66` | 相对单会话上限的平均用量比例 |

应根据服务器出口带宽和并发量调整 `TOTAL_BANDWIDTH`，不要把示例值理解为带宽保证。

检查占位值：

```sh
grep -RniE 'example\.com|REPLACE|这里粘贴' .env compose.yaml
```

`compose.yaml` 的注释里可能保留说明文字；`.env` 的公钥占位值必须消失。

## 6. 配置防火墙和云安全组

纯中继节点不需要开放 HBBS 的 `21115`、`21116`，也不需要 Kessoku 的 `21114`。

| 端口 | 公网规则 | 用途 |
| --- | --- | --- |
| 实际 SSH 端口/TCP | 仅可信来源 | 服务器管理 |
| `21117/TCP` | **必须放行** | RustDesk 原生 HBBR 中继 |
| `80/TCP` | 启用 WSS 时放行 | 申请证书和 HTTPS 跳转 |
| `443/TCP` | 启用 WSS 时放行 | `wss://.../ws/relay` |
| `21119/TCP` | **禁止公网访问** | Nginx 到 HBBR 的明文 WebSocket 后端 |

UFW 示例（先把 `22` 换成真实 SSH 端口）：

```sh
sudo ufw allow 22/tcp
sudo ufw allow 21117/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 21119/tcp
sudo ufw enable
sudo ufw status numbered
```

不使用 WSS 时可以不开放 `80/443`。云服务器安全组也必须应用相同的放行规则。示例使用
主机网络，因此 HBBR 直接监听宿主机端口，不能只依赖 Docker 端口映射隐藏 `21119`。

## 7. 启动 HBBR

```sh
cd /opt/rustdesk-relay
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbr
```

确认实际镜像：

```sh
docker inspect rustdesk-relay-hbbr --format '{{.Config.Image}}'
```

日志应显示 HBBR 启动以及带宽参数，没有公钥解析、端口占用或反复重启错误。检查监听：

```sh
sudo ss -lntp | grep -E ':(21117|21119)\b'
```

在另一台公网主机测试原生端口，而不是只在中继服务器本机测试：

```sh
nc -vz relay-1.example.com 21117
```

端口可连接只证明网络入口存在，仍需稍后完成真实中继会话。

## 8. 在中心 HBBS 中加入中继节点

### 中心使用 Starry HBBS

编辑中心 `starry-config.yaml`，把中继节点加入完整候选池：

```yaml
relay_servers:
  - relay-1.example.com:21117
```

多台节点时逐项列出：

```yaml
relay_servers:
  - relay-1.example.com:21117
  - relay-2.example.com:21117
```

启用了 WSS 时，`relay_health.endpoints` 必须精确覆盖每一个 `relay_servers` 项：

```yaml
websocket_signal:
  enabled: true
  relay_health:
    interval_seconds: 60
    timeout_ms: 5000
    success_threshold: 1
    failure_threshold: 2
    endpoints:
      - relay: relay-1.example.com:21117
        url: wss://relay-1.example.com/ws/relay
```

启用了地理位置规则时，规则中的地址也必须与候选池逐字一致：

```yaml
geo:
  enabled: true
  rules:
    - name: 默认中继
      symmetric: true
      match:
        client_a: "*"
        client_b: "*"
      relays:
        - relay-1.example.com:21117
```

重启并检查 HBBS：

```sh
docker compose --env-file .env -f compose.yaml restart hbbs
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbs
```

### 中心使用官方 HBBS

官方 HBBS 的 `-r`/`--relay-servers` 参数接受逗号分隔的中继地址。例如：

```yaml
command:
  - hbbs
  - -r
  - relay-1.example.com:21117,relay-2.example.com:21117
```

修改后重建 HBBS 并检查日志。官方 HBBS 不提供 Starry 的地理位置规则和 WSS 健康选择；
需要这些能力时推荐使用 Starry HBBS。

## 9. 为浏览器远控配置 WSS（可选）

只使用桌面客户端原生 `21117/TCP` 时可跳过本节。Kessoku 内置浏览器远控必须为可能被
HBBS 返回的每台中继节点配置 WSS。

安装 Nginx 和证书工具：

```sh
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

cd /opt/rustdesk-relay
sudo cp nginx-bootstrap.conf /etc/nginx/sites-available/rustdesk-relay.conf
sudo editor /etc/nginx/sites-available/rustdesk-relay.conf
sudo ln -sfn /etc/nginx/sites-available/rustdesk-relay.conf \
  /etc/nginx/sites-enabled/rustdesk-relay.conf
sudo nginx -t
sudo systemctl reload nginx

sudo certbot certonly --nginx -d relay-1.example.com
```

安装最终配置，替换域名和证书路径：

```sh
sudo cp nginx-relay.conf /etc/nginx/sites-available/rustdesk-relay.conf
sudo editor /etc/nginx/sites-available/rustdesk-relay.conf
sudo nginx -t
sudo systemctl reload nginx
```

最终配置只代理精确的 `/ws/relay` 到本机 `127.0.0.1:21119`，其他路径返回 404。
`21117/TCP` 原生中继不经过 Nginx。

检查协议升级：

```sh
curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  https://relay-1.example.com/ws/relay
```

预期看到 HTTP `101 Switching Protocols`。不要使用 `curl -k` 掩盖证书错误。

## 10. 更新 Kessoku 浏览器客户端映射（启用浏览器远控时）

如果中心可能把 `relay-1.example.com:21117` 返回给浏览器客户端，Kessoku
`config.yaml` 必须提供完全相同名称的 WSS 映射：

```yaml
web-client:
  relay-wss-urls:
    "relay-1.example.com:21117": "wss://relay-1.example.com/ws/relay"
  profile-generation: 2
```

多台中继节点都要加入映射。改变地址或映射后递增 `profile-generation`，重启 Kessoku，
再检查：

```sh
curl -fsS https://client.example.com/config/v1.json
```

没有 WSS 的中继节点不能分配给内置浏览器客户端，否则浏览器会得到中继地址但无法建立
数据通道。桌面客户端的原生中继不受此映射限制。

## 11. 完成真实中继验证

在两台 RustDesk 桌面客户端中配置相同的中心 HBBS 和公钥，然后：

1. 确认两台客户端都在线并能登录 Kessoku；
2. 在中心配置中确保本次测试会选中 `relay-1.example.com:21117`；
3. 使用客户端“始终通过中继连接”等功能发起一次强制中继会话；
4. 验证画面、鼠标、键盘和持续连接；
5. 对照中继节点 HBBR 日志中的同一时间段；
6. 启用 WSS 时再完成 WSS↔WSS 及所需混合模式测试。

```sh
docker compose --env-file .env -f compose.yaml logs --since 10m hbbr
```

点对点连接成功不会使用 HBBR，不能作为纯中继节点验收。HTTP `101` 和开放的
`21117/TCP` 也不能代替真实远控会话。

## 12. 更新、备份和回退

至少备份：

```text
/opt/rustdesk-relay/.env
/opt/rustdesk-relay/compose.yaml
/opt/rustdesk-relay/data/
/etc/nginx/sites-available/rustdesk-relay.conf
/etc/letsencrypt/
```

更新时先记录当前镜像，再修改明确版本：

```sh
docker inspect rustdesk-relay-hbbr --format '{{.Config.Image}}'
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml logs --tail 150 hbbr
```

建议先升级中心 HBBS，验证稳定后再逐台升级 HBBR；每次只升级一台中继节点并完成真实
会话测试。回退时恢复原镜像行和版本，不要重新生成中心密钥。

## 13. 常见问题

| 现象 | 检查 |
| --- | --- |
| Compose 提示缺少公钥 | `.env` 的 `RUSTDESK_PUBLIC_KEY` 仍为空或未被读取 |
| HBBR 启动后客户端仍不用它 | 中心 HBBS 尚未返回该地址，或地理位置规则优先选择其他节点 |
| `21117` 连接超时 | 云安全组、主机防火墙、DNS、NAT/端口转发 |
| WSS 返回 502 | HBBR、`127.0.0.1:21119`、Nginx `proxy_pass` |
| WSS 证书错误 | 域名、证书 SAN、完整证书链和客户端时钟 |
| 浏览器提示没有可用中继 | Starry 健康地址和 Kessoku `relay-wss-urls` 名称不一致 |
| 改用官方镜像后仍拉取 Starry | YAML 与 `.env` 没有同时切换，先查看 `docker compose config` |
| P2P 正常、强制中继失败 | HBBR 或 `21117/TCP` 故障；P2P 没有测试中继 |

进一步网络检查见[反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)，中心完整部署见
[Kessoku + Starry 完整部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)。
