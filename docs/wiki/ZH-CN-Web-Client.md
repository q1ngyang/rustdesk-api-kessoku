# 客户端使用方法

Kessoku 同时服务两类客户端：RustDesk 桌面/移动客户端使用账户 API；内置浏览器客户端从
独立网页发起远程控制。本页说明两者的配置与使用差异。

## RustDesk 桌面客户端

在 RustDesk 中打开“设置 → 网络”（不同版本文字可能略有差异），填写：

| 设置项 | 示例 | 说明 |
| --- | --- | --- |
| ID 服务器 | `rustdesk.example.com` | HBBS；非默认端口时写 `:21116` |
| API 服务器 | `https://api.example.com` | Kessoku 公网地址，不要写 `/dash/` 或 `/api` |
| Key | `id_ed25519.pub` 的完整单行内容 | HBBS 公钥，不是私钥、登录令牌或商业许可证 |
| 中继服务器 | Starry 场景留空 | 让 HBBS 动态返回中继；固定填写可能绕过 Starry 选择规则 |
| 使用 WebSocket | 第一次测试先关闭 | 服务器 WSS 验证完成后再逐端开启 |

保存后重启或重新连接客户端，用 Kessoku 普通用户登录。应能看到地址簿、个人设备和管理员
共享的地址。两台客户端必须使用相同的 ID 服务器和公钥。

桌面客户端的正常验收顺序：

1. 未登录时先验证两端能从 HBBS 获得 ID；
2. 填写 API 并登录，验证地址簿同步；
3. 完成一例点对点会话；
4. 使用“始终通过中继连接”或受限网络完成一例 HBBR 会话；
5. 启用 WebSocket 后完成 WSS 会话；
6. 注销并再次登录，确认旧会话已撤销且新会话正常。

登录成功只证明 Kessoku 可用，不代表 HBBS/HBBR 已正确开放端口。

## 内置浏览器客户端能做什么

Kessoku `v3.0.5` 内置浏览器远控页面，当前支持：

- 用户名和密码登录；
- 输入目标 RustDesk ID 和被控端密码；
- 通过 WSS 强制中继连接；
- 校验服务端身份和 RustDesk 加密会话；
- VP9 画面、鼠标和基本键盘输入；
- 退出时撤销短期连接令牌并清理内存凭据。

当前不支持：

- 点对点直连或浏览器作为被控端；
- 文件传输、剪贴板、音频、终端、端口转发、远程打印；
- 多显示器切换、触摸和输入法组合输入；
- VP9 以外的视频编码或没有 WebCodecs 的浏览器软件解码。

桌面客户端不受这些浏览器端限制。

## 服务端前置条件

浏览器客户端必须同时满足：

1. `auth.enabled: true`，并提供可读的 Ed25519 签名私钥；
2. `web-client.mode: builtin`；
3. API 和浏览器页面使用两个不同的 HTTPS 域名；
4. HBBS 提供 `wss://.../ws/id`；
5. 每个可能返回的 HBBR 名称都有精确 `wss://.../ws/relay` 映射；
6. WSS 证书受浏览器信任，不能使用自签名证书后手工忽略警告。

Kessoku 可以搭配支持这些 WSS 路径的官方 HBBS/HBBR。推荐使用 Starry，联合配置见
[完整部署教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)。

## Kessoku 配置示例

```yaml
auth:
  enabled: true
  issuer: "https://api.example.com"
  audiences:
    - "kessoku-api"
    - "rustdesk-connect"
  access-token-ttl: 168h
  maximum-token-ttl: 168h
  current-key:
    id: "kessoku-main-2026"
    private-key-file: "/run/secrets/kessoku-access-ed25519.pem"

web-client:
  mode: "builtin"
  listen: "0.0.0.0:21122"
  public-origin: "https://client.example.com"
  api-origin: "https://api.example.com"
  rendezvous-wss-url: "wss://rustdesk.example.com/ws/id"
  relay-wss-urls:
    "rustdesk.example.com:21117": "wss://rustdesk.example.com/ws/relay"
  server-public-key: "这里填写 id_ed25519.pub 的完整内容"
  profile-generation: 1
  connection-token-ttl: 15m
```

`relay-wss-urls` 左侧是 HBBS 返回给客户端的中继名称，必须逐字一致；右侧是浏览器实际
连接的 WSS 地址。多中继示例：

```yaml
relay-wss-urls:
  "relay-sg.example.com:21117": "wss://relay-sg.example.com/ws/relay"
  "relay-jp.example.com:21117": "wss://relay-jp.example.com/ws/relay"
```

改变 WSS 地址、中继映射或公钥后递增 `profile-generation`，便于浏览器识别新配置。

## Starry 配置示例

Starry 必须允许浏览器站点的精确来源，并为所有中继提供健康检查地址：

```yaml
version: 3
relay_servers:
  - rustdesk.example.com:21117

websocket_signal:
  enabled: true
  trusted_proxies:
    - 127.0.0.1/32
    - ::1/128
  allowed_origins:
    - https://client.example.com
  relay_health:
    interval_seconds: 60
    timeout_ms: 5000
    success_threshold: 1
    failure_threshold: 2
    endpoints:
      - relay: rustdesk.example.com:21117
        url: wss://rustdesk.example.com/ws/relay
```

桌面 RustDesk 通常不发送浏览器 `Origin` 请求头，仍可使用 WSS。不要把
`allowed_origins` 写成 `*`。

## 域名与 Nginx

```text
https://api.example.com     -> 127.0.0.1:21114
https://client.example.com  -> 127.0.0.1:21122
wss://rustdesk.example.com/ws/id    -> 127.0.0.1:21118
wss://rustdesk.example.com/ws/relay -> 127.0.0.1:21119
```

完整示例见
[`examples/combined/nginx.conf.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/combined/nginx.conf.example)和
[反向代理与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)。不能直接向公网开放
`21118`、`21119` 或 `21122`。

## 使用浏览器客户端

直接使用：

1. 打开 `https://client.example.com/`；
2. 输入 Kessoku 普通用户的用户名和密码；
3. 输入目标 RustDesk ID 与被控端密码；
4. 连接后验证画面、鼠标和基本键盘；
5. 结束会话并退出登录。

从管理后台发起时，后台会打开配置中的浏览器客户端域名，并通过精确站点来源传递一个
短期、只能发起 RustDesk 连接的令牌。该令牌不是管理 API 令牌，不会放进 URL 或浏览器
持久存储。请允许该站点弹出窗口；不要把两个站点合并来绕过浏览器限制。

## 检查公开配置

```sh
curl -fsS https://client.example.com/config/v1.json
```

响应应包含：

- 配置结构版本和 `profile_generation`；
- API、HBBS WSS 和 HBBR WSS 地址；
- HBBS 公钥和 SHA-256 指纹。

响应不应包含：

- 用户名、密码或访问令牌；
- Ed25519 私钥；
- 容器监听地址和内部端口；
- 令牌有效期或 Starry 管理凭据。

## 常见问题

| 现象 | 检查 |
| --- | --- |
| `client` 域名 404/502 | Kessoku 容器日志、`21122` 回环绑定、Nginx `proxy_pass`、`web-client.mode` |
| Kessoku 启动即退出 | `auth.enabled`、私钥路径/权限、公钥是否为有效 32 字节 Ed25519 base64、两个站点是否不同 |
| 浏览器登录被跨域拦截 | `public-origin`、`api-origin` 与地址栏是否逐字一致；不能多斜杠、路径或显式 `:443` |
| `/ws/id` 连接失败 | Nginx 是否代理到 HBBS `21118`，Starry `websocket_signal.enabled`，证书是否有效 |
| 找不到可用中继 | HBBS 返回名称与 `relay-wss-urls` 左侧是否一致，Starry 中继健康检查是否成功 |
| 连接后无画面 | 被控端是否支持 VP9，是否禁用了硬件编码进行排障，HBBR 日志是否有同一会话 |
| 中文输入不完整 | 当前浏览器端不支持完整输入法组合输入，改用桌面客户端 |

排障时不要使用 `curl -k`、关闭证书验证、把令牌放进查询参数、使用通配跨域或直接公开
后端端口。这些做法会掩盖配置错误并扩大凭据泄露风险。
