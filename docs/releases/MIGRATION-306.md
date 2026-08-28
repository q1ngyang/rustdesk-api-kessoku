# Database version 306 migration

Version 306 separates the super-administrator settings navigation, adds an
independent GeoLite2 Country database source, and refreshes the default login
brand lockup and footer. The migration is additive:

- `system_settings.geo_ip_country_url` is added and backfilled with the P3TERX
  GeoLite2 Country MMDB download URL;
- an unchanged legacy login title is converted to the new two-line format;
- an unchanged `RustDesk API Kessoku · v3` footer is replaced by the project
  name and GitHub link, without exposing a version number;
- operator-customized titles, footer HTML, images, announcements, and MMDB
  addresses are preserved.

Back up the database and media directory before upgrading. After startup,
confirm that the newest `versions.version` is `306`, visit **System management
→ IP geolocation**, and run **Update now** once if the Country database has not
yet been downloaded. Existing `/branding`, `/server-control`, and
`/my/settings` bookmarks redirect to the new routes.

## 中文说明

数据库版本 306 新增独立的 GeoLite2 Country MMDB 地址，并重新整理仅超级管理员可见的
系统管理导航。迁移会为 `system_settings.geo_ip_country_url` 写入默认下载地址；仅当登录页
标题和页脚仍是旧版原始默认值时，才会分别升级为两行标题和“项目名称 + Github 链接”，
不会覆盖管理员已经自定义的标题、HTML、图片、公告或 MMDB 地址。

升级前请备份数据库与媒体目录。启动后确认最新 `versions.version` 为 `306`，进入“系统管理
→ IP 地理信息”；若 Country 数据库尚未下载，请执行一次“立即更新”。旧的品牌、服务端控制
和系统设置地址会自动跳转到新路径。
