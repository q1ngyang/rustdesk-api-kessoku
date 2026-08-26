# 常见问题排查

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Troubleshooting) | **简体中文**

排障时先确认是哪一层失败：容器、Kessoku API、账户、HBBS 信令、HBBR 中继、WSS、浏览器
客户端或高级认证。每次只改一项，避免同时关闭防火墙、证书和认证后无法判断真正原因。

## 先收集这些信息

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 200
docker inspect rustdesk-api-kessoku --format '{{.Config.Image}} {{.Config.User}}'
sudo nginx -t
```

联合部署再检查：

```sh
docker inspect rustdesk-starry-hbbs --format '{{.Config.Image}}'
docker inspect rustdesk-starry-hbbr --format '{{.Config.Image}}'
sudo ss -lntup | grep -E ':(80|443|2111[4-9]|2112[0-2])\b'
```

记录发生时间、客户端版本和脱敏错误信息。不要粘贴完整访问令牌、密码、私钥或数据库。

## Compose 或容器无法启动

| 现象/日志 | 常见原因 | 处理 |
| --- | --- | --- |
| `set ...`、变量为空 | `.env` 缺少必填项 | 修改 `.env`，再运行 `docker compose config` |
| 配置路径变成目录 | 宿主机配置文件不存在，Docker 自动创建了同名目录 | 停止容器，确认 `config.yaml` 是普通文件，再启动 |
| `permission denied` 创建数据库 | `data/kessoku` 不属于 `65534:65534` | `chown 65534:65534` 并设目录 `0700` |
| 无法读取签名私钥 | `secrets` 目录不可进入，或文件组/其他权限不为零 | 目录和文件归 `65534:65534`；目录 `0700`、私钥 `0600` |
| `read-only file system` | 日志/临时文件写到了只读根文件系统 | 保持 `logger.path: ./runtime/log.txt` 和 Compose `/app/runtime` 临时挂载 |
| `app.web-client is removed` | 旧配置把 `app.web-client` 设为非零 | 改回 `0`，用根级 `web-client.mode` |
| `web-client-provider is removed` | 仍保留旧配置段或环境变量 | 完整删除旧段，按当前 `web-client` 配置 |
| 容器不断重启 | 配置校验、数据库、证书或密钥失败 | 查看最早一条致命日志；修正输入，不关闭校验 |

检查权限：

```sh
sudo stat -c '%U:%G %a %n' data/kessoku secrets secrets/* config.yaml
```

## 占位值没有替换

Compose 只能检查语法，不知道 `example.com` 或 `REPLACE_*` 是无效业务值。启动前执行：

```sh
grep -RniE 'example\.com|REPLACE|replace-with|这里粘贴' \
  .env config.yaml kessoku-config.yaml starry-config.yaml 2>/dev/null
```

联合部署第一次只启动 HBBS/HBBR，读取 `data/starry/id_ed25519.pub` 后再把公钥填入 `.env`
和 Kessoku 的 `web-client.server-public-key`。

## 管理后台无法访问或登录

| 现象 | 检查与处理 |
| --- | --- |
| `/_admin/` 404 | 使用带末尾斜杠的完整地址；确认镜像包含 `resources/admin/index.html`，Nginx 整站代理到 `21114` |
| 502 Bad Gateway | `docker compose ps`、Kessoku 日志和 `127.0.0.1:21114` 监听 |
| 不知道初始密码 | 创建 12～128 字节、权限 `0600` 的密码文件，执行 `reset-admin-pwd --password-file`；不要重建数据库 |
| 密码文件被拒绝 | 文件必须是普通文件，组/其他权限位都为零，并能被 UID 65534 读取 |
| 多次失败后被验证码/封禁 | 等待封禁期，检查代理真实地址配置；不要把所有用户错误归到同一个不可信代理地址 |
| 禁用密码后无法登录 | 恢复 `app.disable-pwd-login: false`，先修复 OAuth/OIDC，再重新评估 |
| 创建不了用户 | `app.register: false` 只关闭自助注册；管理员应在“用户管理”创建，检查当前账户权限范围 |

管理员密码重置示例：

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

## RustDesk 客户端无法登录

依次确认：

1. API 服务器填写 `https://api.example.com`，不带 `/api` 或 `/_admin/`；
2. 浏览器访问 `https://api.example.com/api/version` 成功且证书有效；
3. 用户状态为启用，密码正确；
4. 客户端、服务器时钟同步；
5. Nginx 转发 `Host`、`X-Forwarded-Proto`；
6. LDAP/OAuth 用户的身份绑定没有重复；
7. 密码重置或管理员撤销会话后，客户端重新登录。

地址簿为空时确认客户端已登录同一 Kessoku，`rustdesk.personal: 1`，管理员共享规则包含该
用户/用户组，并检查客户端是否遗留其他 API 地址。

## 登录成功但无法远控

这通常不是 Kessoku API 故障。检查：

- 两端 ID 服务器和 Key 是否相同；
- 公钥是否为 `id_ed25519.pub`，不是 Kessoku 签名密钥；
- `21116/TCP+UDP` 是否从客户端网络可达；
- 被控端是否仍在线注册；
- Starry 动态分配时客户端“中继服务器”是否留空；
- 强制中继时 `21117/TCP` 是否开放且 HBBR 运行；
- 两端和 HBBS/HBBR 日志是否对应同一连接时间。

点对点成功、强制中继失败时优先检查 HBBR 和 `21117/TCP`。登录成功不能代替远控链路验收。

## 浏览器客户端页面或登录失败

| 现象 | 检查 |
| --- | --- |
| `client` 域名 404/502 | `web-client.mode: builtin`、`21122` 回环映射、Nginx 代理、Kessoku 日志 |
| 启动时报公钥无效 | `server-public-key` 必须是解码后恰好 32 字节的 Ed25519 base64，即 HBBS 公钥内容 |
| 报两个站点相同 | `public-origin` 与 `api-origin` 必须是两个不同的小写 HTTPS 地址，不带路径/默认端口 |
| 浏览器跨域错误 | 地址栏与两个站点地址配置是否逐字一致，Starry `allowed_origins` 是否包含客户端地址 |
| 管理后台弹窗空白 | 浏览器是否阻止弹窗，客户端域名证书/Nginx 是否正常，后台配置返回的客户端地址是否正确 |
| 登录后立即失效 | 两台服务器时钟、`connection-token-ttl`、`auth.maximum-token-ttl` |

检查公开配置：

```sh
curl -fsS https://client.example.com/config/v1.json
```

该响应不能包含密码、私钥或访问令牌。

## WSS 失败或没有可用中继

探测入口：

```sh
curl --http1.1 --include --max-time 5 \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  https://rustdesk.example.com/ws/id
```

对 `/ws/relay` 重复。检查：

- Nginx 是否分别转发到 `127.0.0.1:21118`、`127.0.0.1:21119`；
- 证书域名和完整链是否有效；
- Starry `websocket_signal.enabled: true`；
- `trusted_proxies` 包含实际 Nginx 地址但不过宽；
- 浏览器站点在 `allowed_origins` 中；
- `relay_health.endpoints[].relay` 与 `relay_servers` 完全一致；
- Kessoku `relay-wss-urls` 左侧又与同一中继名称完全一致。

HTTP 101 只说明升级入口正常。浏览器仍无画面时检查 HBBR 会话、被控端密码、VP9 和浏览器
WebCodecs 支持。不要用 `curl -k` 掩盖证书问题。

## MySQL/PostgreSQL 启动失败

| 错误方向 | 处理 |
| --- | --- |
| MySQL 拒绝不安全传输 | `mysql.tls` 必须是字符串 `"true"` |
| PostgreSQL 模式错误 | `postgresql.sslmode` 必须为 `verify-full` |
| CA 无法读取 | 把 CA 只读挂载到 `/run/secrets`，确认容器 UID 可读 |
| 主机名不匹配 | 使用证书 SAN 中的 DNS 名称，不用临时 IP 或别名 |
| 身份重复导致迁移停止 | 保留旧服务停止，在备份副本中明确合并/解绑 OAuth 身份，不随意删行 |

外部数据库故障时不要临时改成明文连接。先恢复证书、DNS、网络或数据库服务。

## LDAP/OAuth/OIDC 问题

- LDAP 必须为 `ldaps://` 且开启证书验证；检查 Base DN、过滤器和属性名；
- 私有 LDAP CA 必须是可读普通文件；
- OAuth/OIDC 回调地址应基于 `rustdesk.api-server`；
- 签发者发现地址必须是可公开访问的 HTTPS，不能指向回环/链路本地；
- 提供方的 Client ID/Secret 与回调地址逐字一致；
- 保持 `proxy.enable: false`；
- 禁用密码登录前先用独立浏览器完成管理员登录和故障回退。

## 连接认证拒绝正常用户

立即把 Starry 从 `enforce` 退回 `audit`，不要设置自动放行。检查：

- Kessoku `auth.issuer` 与 Starry `issuer`；
- `rustdesk-connect` 受众、`connect:initiate` 权限和 `kid`；
- JWKS 最近刷新时间、缓存文件及 `.metadata.json`；
- 双向 TLS CA、服务端名称和客户端证书 URI SAN；
- Kessoku 令牌状态查询是否可达；
- 用户是否已注销、禁用、删除或重置密码；
- Kessoku、Starry 和客户端时钟。

内部 TLS 测试命令见[连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication)。不得公开 `21121` 或
关闭证书验证。

## Starry 管理页面不可用

检查：

- `server-control.instances[].enabled`；
- 管理代理固定地址、TLS 服务端名称和实例 UUID；
- Kessoku 客户端证书、URI SAN、CA 和独立服务令牌私钥；
- 管理代理是否只监听可达的私有地址；
- Kessoku 与代理是否都处于只读模式。

应用配置出现 ETag/计划过期时，重新读取当前配置、合并、校验并生成新计划，不要强制覆盖。
操作返回成功但页面状态旧时重新读取 HBBS 当前配置代号和摘要。不要删除管理代理状态目录。

## 仍无法解决时

提供脱敏后的：

- Kessoku/Starry 镜像版本；
- `docker compose config` 中与问题相关的结构（删除秘密）；
- 问题时间前后的日志；
- Nginx 检查结果、域名与端口；
- RustDesk 客户端版本、平台和 WebSocket 开关；
- 数据库类型以及 Starry 当前配置代号。

不要提供原始令牌、密码、Cookie、`id_ed25519`、Kessoku 签名私钥、客户端证书私钥或完整
数据库。
