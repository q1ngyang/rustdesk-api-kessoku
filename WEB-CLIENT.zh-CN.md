# 内置 Kessoku Web 客户端

[English](WEB-CLIENT.md) | **简体中文**

Kessoku v2.8.0 内置仓库自有、MIT 许可的浏览器远控 MVP。已审核 TypeScript 源码位于
`web-client/`，release 只把可复现生产产物放入 `resources/client`。实现不使用历史
`resources/web`、`resources/web2`、托管 WebClient2/V2、Flutter Web 输出、外部构建
JavaScript 或下载的 WASM。

## MVP 支持范围

- 仅通过强制 Relay over WSS 发起远控会话；
- 面向 Starry patch-v1.2.0 的 RustDesk 1.4.9 协议兼容；
- 签名 peer key 校验和 fail-closed 加密会话；
- 通过浏览器 WebCodecs 解码 VP9，并用 Canvas 2D 渲染；
- 有界鼠标与基本键盘输入。

v2.8.0 MVP 不包含 direct/P2P TCP/UDP、被控/host 模式、文件传输、剪贴板、音频、终端、
端口转发、打印、显示器切换、移动触摸、IME、H.264/H.265/AV1 或软件解码。

## Origin 与令牌边界

原生 Linux 部署的客户端 listener 默认是 `127.0.0.1:21122`。容器内可监听
`0.0.0.0:21122`，但 Compose 只把宿主端口绑定到 `127.0.0.1`。应为它配置独立 HTTPS
反向代理与主机名。`web-client.public-origin` 必须不同于 `web-client.api-origin`，不能把
客户端作为管理/API origin 下的路径发布。

客户端从同源 `GET /config/v1.json` 读取非 secret 连接端点和服务端公钥，并通过以下接口
取得或撤销连接凭据：

- `POST /api/web-client/v1/login`：严格 username/password JSON；
- `POST /api/web-client/v1/grants`：使用现有 RustAuth bearer；
- `POST /api/web-client/v1/logout`：使用 connection bearer。

返回 token 的 audience 是 `rustdesk-connect`，scope 是 `connect:initiate`，默认有效期
15 分钟，配置上限一小时。管理端从 Kessoku 应用配置读取受信任 client origin，打开该
精确 origin，并用严格 `postMessage` target origin 发送
`kessoku.web-client.grant.v1`。Peer ID、token、密码、密钥与会话状态只存在内存，不进入
URL、Cookie、localStorage、sessionStorage、IndexedDB、Cache API、service worker 或
日志；client origin 也不会得到 API/admin 的环境 Cookie。

API/admin 响应使用 `Cross-Origin-Opener-Policy: same-origin-allow-popups`，仅让认证
opener 保留到精确 origin 的 ready/grant/accepted 握手完成。独立 client listener 刻意不
发送 COOP（浏览器默认是 `unsafe-none`），不会复制 admin 的 header。接收一次 grant 后，
client 会移除 listener 并尝试清除 `window.opener`；超时、导航异常或未收到确认时，admin
会关闭 popup，在清空本地 token 引用前尽力调用 `/api/web-client/v1/logout`。

## 配置

```yaml
web-client:
  mode: builtin
  listen: "127.0.0.1:21122"
  public-origin: "https://client.example.com"
  api-origin: "https://api.example.com"
  rendezvous-wss-url: "wss://rustdesk.example.com/ws/id"
  relay-wss-urls:
    "rustdesk.example.com:21117": "wss://rustdesk.example.com/ws/relay"
  server-public-key: "BASE64_ED25519_PUBLIC_KEY"
  profile-generation: 1
  connection-token-ttl: 15m
```

所有 URL 都是精确 HTTPS/WSS。Rendezvous 返回的 Relay 名称必须存在于
`relay-wss-urls`，浏览器不会从不可信输入推导目标。已审核端点/密钥 profile 变化时应
递增 `profile-generation`。

## 构建、许可与证据

Node 24.15.0 与 npm 11.12.1 固定。CI 执行 `npm ci`、lint、测试、生产依赖审计、registry
签名审计、两次相同构建、独立 CycloneDX SBOM、产物 checksum 和 MIT/第三方许可证
证据。`web-client/LICENSE` 覆盖仓库自有客户端代码；客户端完整依赖/构建图及其许可证
记录在独立 SBOM 与 release 的 `WEB-CLIENT-NOTICE.md` 中。运行时依赖要求的完整
Apache-2.0 与 BSD-3-Clause 文本会随每个 archive、Debian 包和容器镜像发布在
`resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt`。

详见 [Web 客户端部署](docs/wiki/ZH-CN-Web-Client.md)、
[安全模型](SECURITY-MODEL.md)和
[wire profile](docs/development/WEB-CLIENT-WIRE-SPEC.md)。
