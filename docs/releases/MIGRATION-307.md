# Database version 307 migration

Version 307 makes deployment branding theme-aware. It adds independent light
and dark image fields for the administration console logo/icon, sign-in logo,
and WebClient logo/icon. Existing single-theme image URLs are copied into both
new theme slots, so an upgrade preserves the current appearance until an
operator chooses separate artwork.

The bundled defaults now use the StarryDesk light/dark assets. The StarryLinks
logo remains a fixed, non-tenant Server Control identity and is not stored in
the branding record. A new branding save clears the obsolete single-theme
columns after persisting the themed values; the columns remain present for
rollback and older-reader compatibility.

Back up the database and media directory before upgrading. After startup,
confirm that the newest `versions.version` is `307`, then visit **System
management → Branding** and preview both modes. Leaving a field empty selects
the corresponding bundled StarryDesk default; it does not produce a missing
image.

WebClient browser sessions are now API-domain, `Secure`, `HttpOnly`,
`SameSite=None`, partitioned cookies. This supports an independently hosted
WebClient without granting a parent-domain cookie. Keep the configured API and
WebClient origins exact, HTTPS-only, and covered by valid certificates.

## 中文说明

数据库版本 307 为后台 Logo、后台图标、登录页 Logo，以及 WebClient Logo/图标新增标准
模式和夜间模式两套独立字段。升级时，旧的单套图片链接会自动复制到两套新字段，不会改变
现有部署外观。字段留空时会显示随程序发布的对应 StarryDesk 默认素材，不会再出现空白图片。

StarryLinks Logo 继续只作为“服务端控制”页面的固定产品标识，不进入品牌设置，也不可由租户
替换。升级前请备份数据库和媒体目录；启动后确认最新 `versions.version` 为 `307`，并在“系统
管理 → 品牌个性化”中分别检查标准和夜间模式。

WebClient 与 API 使用不同域名时，登录会话不会尝试跨域共享 Cookie；API 会签发 `Secure`、
`HttpOnly`、`SameSite=None` 的分区 Cookie，由 WebClient 顶级站点分区保存。两个公开地址仍须
使用精确配置的 HTTPS Origin 与有效证书。
