# 内置 Web 客户端

[English](Web-Client.md) | **简体中文**

Kessoku v3.0.0 从仓库 `web-client/` 源码构建 MIT 许可浏览器客户端，并把产物打包为
`resources/client`。`admin-web/` 管理 UI 仍是独立应用。构建与打包策略永久拒绝历史
`resources/web`、`resources/web2`、WebClient2/V2 和远程下载浏览器客户端。

## MVP 能力

客户端发起 forced-Relay WSS 会话，验证签名 peer 身份，使用 RustDesk 加密会话，通过
WebCodecs 解码 VP9、Canvas 2D 渲染，并发送有界鼠标/基本键盘输入。兼容目标是
RustDesk 1.4.9 与 Starry patch-v1.2.0。

它明确不支持 direct/P2P、host 模式、文件传输、剪贴板、音频、终端、端口转发、打印、
显示器切换、触摸/IME、非 VP9 codec 或软件解码。这些是排除项，不是隐藏配置开关。

## 部署两个 HTTPS origin

API/admin 与 Web Client 使用不同 origin，例如：

```text
https://api.example.com     -> 127.0.0.1:21114
https://client.example.com  -> 127.0.0.1:21122
```

原生 listener 默认 `127.0.0.1:21122`。Docker 镜像暴露 21122，推荐 Compose 只绑定宿主
loopback。容器内 `0.0.0.0:21122` 只能位于这个宿主 loopback 映射和独立 HTTPS 代理后。
不能把客户端放到 API 的某个 path、直接公开 21122、启用 proxy credential 或复用内部
mTLS 端口。

client origin 需要按 CSP 访问 HTTPS API origin，并通过 WSS 访问精确 Rendezvous/Relay
端点。反向代理要支持 WSS upgrade，并保留固定 `/ws/id`、`/ws/relay` path。客户端会拒绝
不在配置精确 map 中的 Relay 名称。

## 启用 profile

设置 `web-client.mode: builtin`，并配置精确 `public-origin`/`api-origin`、listener、WSS
URL、base64 Ed25519 服务端公钥、正数 profile generation，以及不超过一小时和全局 auth
上限的 connection-token TTL。完整示例见
[`WEB-CLIENT.zh-CN.md`](../../WEB-CLIENT.zh-CN.md)与
[配置参数参考](ZH-CN-Configuration-Reference.md)。

client origin 的 `GET /config/v1.json` 只包含公共端点、公钥/fingerprint、schema version 与
profile generation。Fingerprint 是解码后 32-byte Ed25519 key 的 SHA-256，以
`sha256:` 加小写十六进制表示；配置公钥字符串只写入
`PunchHoleRequest.licence_key`/`RequestRelay.licence_key`，强制 Relay 的
`RequestRelay.socket_addr` 保持空。配置不包含 token、密码、私钥、listener、TTL 或
client-origin。

## 启动与认证

用户可以直接在客户端登录。已有认证的管理页面则用 RustAuth bearer 调用
`POST /api/web-client/v1/grants`，只打开部署配置的 `web_client_public_origin`，再用精确
origin `postMessage` 传递短期 connection grant。Peer ID 与 connection token 只在内存
传输，不进入 URL 或持久浏览器存储。Grant 仅有 audience `rustdesk-connect` 和 scope
`connect:initiate`，不是 admin/API bearer。

启动失败时应关闭 popup，并尽可能 logout/revoke connection grant 后修复 origin/配置。
不能回退到 query-string token、通配 `postMessage`、共享 Cookie 或放宽 CORS。

API/admin 响应必须使用 COOP `same-origin-allow-popups`。独立 client 响应会刻意省略 COOP
（默认 `unsafe-none`），直到 ready/grant/accepted 交换完成；两个 origin 不发送相同 COOP
值。客户端同时校验精确 API/admin origin 与 opener source，只接收一次，然后移除 listener
并尝试清除 opener。Admin 超时、导航异常或缺少确认会触发尽力 logout/revoke；收到成功
确认后，token 生命周期责任才交给客户端。

## 验收检查

- 两个前端 lockfile 都通过 lint/test/audit/signatures 和两次构建可复现检查；
- 独立 admin/client dist checksum、CycloneDX SBOM 与 MIT/第三方许可证证据存在；
- 镜像、archive、DEB 包含 `resources/client/index.html` 与完整的
  `resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt`，且不含两个历史目录；
- 21122 只在宿主 loopback，并仅通过配置的 HTTPS client origin 对外；
- profile JSON 无 secret，响应含预期 CSP、no-store/referrer、frame 与 content-type 防护；
- forced-Relay VP9 桌面会话能完成鼠标键盘操作，logout/disconnect 后 token 与密码从内存清除。

详细协议/安全上限见
[`docs/development/WEB-CLIENT-WIRE-SPEC.md`](../development/WEB-CLIENT-WIRE-SPEC.md)。
