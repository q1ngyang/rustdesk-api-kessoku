# Web 客户端

[English](Web-Client.md) | **简体中文**

Kessoku v2.8.0 不包含浏览器远控客户端。内置 `admin-web/` 只用于管理控制台；它不是
WebClient2，也不能建立 RustDesk 桌面会话。

## 已实现内容

`web-client-provider.mode` 默认是 `disabled`。此时 Kessoku 不注册浏览器客户端路由，
也不提供 Provider manifest。

如果运维方已独立取得浏览器客户端授权，并自行完成审核、托管和维护，可以选择
`external` 模式。该模式只会为已登录的 Kessoku 用户启用一个只读端点：

```text
GET /api/admin/config/web-client-provider
```

响应带有 `Cache-Control: no-store`，并且只包含以下已审核的公开 manifest 字段：

```text
id, name, launch_url, allowed_origin,
license, source_url, version, digest
```

配置还必须提供 `authorization-record`，但 Kessoku 绝不会通过 API 返回这项部署记录。
launch/source URL 必须是无凭据、query 和 fragment 的绝对 HTTPS URL；launch origin 必须
与 `allowed_origin` 完全一致；制品 digest 必须使用小写
`sha256:<64 个十六进制字符>`。外部 Provider 配置无效时，服务会拒绝启动。

## 未实现内容

外部 Provider 接口只是启动与治理描述符，不是托管、授权或 SSO 协议。Kessoku 不会：

- 获取、打包、提供、修改或代理 Provider 资产；
- 把 access token 注入 URL、header、脚本或 Provider session；
- 与 Provider origin 共享 Kessoku cookie、localStorage、地址簿、用户身份或 server key；
- 暴露 launch callback、token exchange、隐式登录或 SSO 端点；
- 恢复已删除的 `resources/web`、`resources/web2`、WebClient2、`/api/shared-peer`、
  `/api/server-config` 或 `/api/server-config-v2` 路径；
- 实现许可证检查或下架规避机制。

旧 `app.web-client` 必须保持为 `0`；非零值会导致启动失败。未来 SSO 方案必须单独审核，
并采用短期 authorization code、PKCE 和精确 redirect URI 匹配。v2.8.0 没有实现该流程。

## 运维方责任

启用 `external` 前，应验证并保留 Provider 的许可证、源码版本、构建制品 digest、托管
origin、内容安全策略、更新流程和事故负责人证据。Provider 每次升级都必须重新核验 digest
与批准记录。Provider 可用性和远控会话兼容性不属于 Kessoku v2.8.0 的支持承诺。

完整配置范例与校验规则也记录在
[`WEB-CLIENT-PROVIDER.md`](../../WEB-CLIENT-PROVIDER.md)。
