# Database version 308 migration

Version 308 adds the optional `preference_language` and `preference_theme`
columns to `users`. They synchronize a signed-in user's presentation choices
between the administration console and a WebClient hosted on a different
domain. No credential, authorization, or connection data is moved.

The schema change is additive. Existing users keep an empty preference and
continue to use their current first-party browser setting until they select a
language or color scheme. Before upgrading, take a normal database backup.
After startup, confirm that the newest `versions.version` is `308` and verify
one language and theme change in both applications.

Cookies are deliberately not shared across unrelated domains. The WebClient
keeps its own first-party display cookie, while its API-domain browser session
uses `Secure`, `SameSite=None`, and `Partitioned`. Account preference fields
provide synchronization after authentication without broadening cookie scope.

## 中文说明

数据库版本 308 在 `users` 表中新增可选的 `preference_language` 和
`preference_theme` 字段，使已登录用户在不同域名部署的后台与 WebClient 之间同步语言和
显示模式。本次迁移不会移动凭据、权限或连接数据。

升级是增量式的。现有用户的偏好字段默认为空，仍沿用各浏览器原有的首方设置，直到用户
主动切换语言或显示模式。升级前请正常备份数据库；启动后确认最新
`versions.version` 为 `308`，并分别在后台和 WebClient 验证一次语言与主题切换。

两个无关域名不会共享 Cookie。WebClient 使用自己的首方显示偏好 Cookie；API 域名的
浏览器会话使用 `Secure`、`SameSite=None` 和 `Partitioned`。登录后的账户偏好字段负责
同步，不扩大 Cookie 的可见范围。
