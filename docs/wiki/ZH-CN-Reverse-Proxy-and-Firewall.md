# 反向代理与防火墙

本页集中说明 Kessoku、HBBS/HBBR 和 Starry 联合部署时的公网入口。第一次部署请先按
[单独部署教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)或
[Kessoku + Starry 完整教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)操作。

## 端口总表

| 端口/协议 | 服务 | 公网开放 | 说明 |
| --- | --- | --- | --- |
| `21114/TCP` | Kessoku | 否 | API 和管理后台的明文后端，只允许本机反向代理访问 |
| `21115/TCP` | HBBS | 是 | NAT 类型测试 |
| `21116/TCP` | HBBS | 是 | 注册、信令、安全 TCP 和发起远控 |
| `21116/UDP` | HBBS | 是 | ID 注册和打洞 |
| `21117/TCP` | HBBR | 是 | 原生中继数据 |
| `21118/TCP` | HBBS | 否 | `/ws/id` 明文 WebSocket 后端 |
| `21119/TCP` | HBBR | 否 | `/ws/relay` 明文 WebSocket 后端 |
| `21120/TCP` | Starry 管理代理 | 否 | 只允许本机或受限私有管理网访问 |
| `21121/TCP` | Kessoku 内部认证接口 | 否 | 只允许持有客户端证书的 Starry 私有通道访问 |
| `21122/TCP` | Kessoku 浏览器客户端 | 否 | 独立网页后端，只允许本机反向代理访问 |
| `80/TCP` | Nginx | 是 | ACME 证书验证和跳转到 HTTPS |
| `443/TCP` | Nginx | 是 | API、管理后台、浏览器客户端和 WSS |

只部署 Kessoku 时，对公网只需 SSH、`80/TCP` 和 `443/TCP`；HBBS/HBBR 端口由它们所在
主机开放。联合单机部署则按上表同时开放原生 RustDesk 端口。

## 推荐域名

```text
api.example.com       Kessoku API 和 /dash/
client.example.com    Kessoku 浏览器远控
rustdesk.example.com  HBBS/HBBR 原生地址及 /ws/id、/ws/relay
```

三个域名可以解析到同一个 IPv4 地址。`api` 和 `client` 必须是不同的站点来源，不能使用
同一域名下的不同路径。浏览器的跨域、内容安全策略和短期连接令牌都按精确站点来源校验。

使用 Cloudflare 等 DNS/CDN 服务时，普通 HTTP/WSS 域名可在确认 WebSocket 和长连接支持
后代理；`21115`～`21117` 不是普通网站流量，免费 HTTP 代理通常不会转发这些端口。没有
专用四层代理产品时，应让 RustDesk 域名使用“仅 DNS”，并直接把原生端口开放到服务器。

## Nginx 参考文件

仓库提供可直接替换域名的示例：

| 文件 | 场景 |
| --- | --- |
| [`examples/nginx/kessoku-bootstrap.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/nginx/kessoku-bootstrap.conf.example) | Kessoku 两个域名首次申请证书 |
| [`examples/nginx/kessoku.example.conf`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/nginx/kessoku.example.conf) | 仅 Kessoku API 与浏览器客户端 |
| [`examples/combined/nginx-bootstrap.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/combined/nginx-bootstrap.conf.example) | 联合部署三个域名首次申请证书 |
| [`examples/combined/nginx.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/combined/nginx.conf.example) | Kessoku + Starry 完整反向代理 |
| [`examples/relay/nginx-bootstrap.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/relay/nginx-bootstrap.conf.example) | 纯中继节点首次申请证书 |
| [`examples/relay/nginx.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/relay/nginx.conf.example) | 纯中继节点 `/ws/relay` |

复制后必须替换全部 `example.com` 和证书路径，并在每次重新加载前执行：

```sh
sudo nginx -t
sudo systemctl reload nginx
```

不要把 `21120` 或 `21121` 添加到公网 Nginx 配置。

## Kessoku API 代理要点

核心配置：

```nginx
server {
    listen 443 ssl;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    client_max_body_size 16m;

    location / {
        proxy_pass http://127.0.0.1:21114;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
```

管理后台位于同一站点的 `/dash/`，不需要单独的 `location`。不要覆盖 Kessoku 自己输出
的内容安全策略、禁止嵌入、禁止嗅探等响应头。

## 浏览器客户端代理要点

浏览器客户端使用独立域名，整个站点代理到 `21122`：

```nginx
server {
    listen 443 ssl;
    server_name client.example.com;

    ssl_certificate /etc/letsencrypt/live/client.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/client.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://127.0.0.1:21122;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

`web-client.public-origin` 必须与浏览器地址完全一致，`web-client.api-origin` 必须与 API
地址完全一致；都使用小写 HTTPS 域名，不带路径和默认 `:443`。

## Starry WSS 代理要点

联合部署的两条路径不能互换或改写：

```nginx
location = /ws/id {
    proxy_pass http://127.0.0.1:21118;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_socket_keepalive on;
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;
}

location = /ws/relay {
    proxy_pass http://127.0.0.1:21119;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_socket_keepalive on;
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;
}
```

Starry 配置中的中继健康检查地址必须使用有效证书和精确 `/ws/relay` 路径。浏览器客户端
配置中的 `rendezvous-wss-url` 必须为 `/ws/id`，中继映射的值必须为 `/ws/relay`。

## 可信代理设置

Kessoku 默认不信任任何转发地址，这比信任所有代理更安全。`gin.trust-proxy` 或
`.env` 中的 `KESSOKU_TRUST_PROXY` 只填写实际访问 Kessoku 的代理地址。Nginx 在宿主机、
Kessoku 在 Docker 网桥时，应用看到的来源地址可能是该 Compose 网络的网关，而不是
`127.0.0.1`。

可以查看容器使用的网关：

```sh
docker inspect rustdesk-api-kessoku \
  --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{end}}'
```

确认地址稳定且确实是本机代理后再填写。不要使用 `0.0.0.0/0`、`::/0` 或未经限制的整个
内网，否则攻击者可能伪造 `X-Forwarded-For`，影响登录限速和审计地址。

Starry 的 `websocket_signal.trusted_proxies` 与此相同：同机 Nginx 使用回环地址时保持
`127.0.0.1/32` 和 `::1/128`；代理在其他主机时只加入其精确私网地址或受控网段。

## UFW 示例

联合部署：

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

请先把 `22` 换成实际 SSH 端口，并确认当前 SSH 会话不会被切断。Docker 的端口发布可能
绕过部分 UFW 转发规则，因此 Kessoku Compose 同时从根源上绑定到 `127.0.0.1`；不要只
依赖一层防火墙。云安全组仅开放表中标记为公网的端口。

## 验证

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json

curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
  https://rustdesk.example.com/ws/id
```

对 `/ws/relay` 重复最后一项。证书检查不要使用 `curl -k`；忽略证书错误会掩盖真实客户端
同样无法连接的问题。HTTP 可达和 101 升级只证明入口正常，最后还要用真实 RustDesk 客户端
验证原生连接、中继连接和 WSS 会话。
