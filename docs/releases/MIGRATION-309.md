# Database version 309 migration

Version 309 consolidates the administration, sign-in and WebClient artwork into
one light/dark logo and icon set. It also adds independent light/dark background
fields for the sign-in page and WebClient, a shared footer HTML field, and
WebClient ownership metadata on connection-audit rows.

The migration is additive. Existing administration artwork is preferred when
the older per-surface settings differ; otherwise the first configured legacy
asset is retained. A legacy single sign-in background is copied into both theme
slots, and the previous sign-in footer becomes the shared footer. Legacy
columns remain populated as a rollback mirror.

Before upgrading, take a normal database backup. After startup, confirm the
latest `versions.version` is `309`, open Branding once, and verify the light and
dark assets on the sign-in page, administration console and WebClient. Existing
connection, login, file and share records are not rewritten.

## 中文说明

数据库版本 309 将后台面板、登录页与 WebClient 原先分别设置的素材合并为一套明暗主题
Logo 和图标，并新增登录页与 WebClient 各自独立的浅色/深色背景、三端共用的页脚 HTML，
以及 WebClient 连接审计的用户与客户端来源字段。

本次迁移为增量升级。若旧版各页面素材不同，优先保留后台面板素材；否则采用首个已配置的
旧素材。旧版单一登录背景会复制到两个主题，原登录页页脚会迁移为全局页脚。旧字段仍会
作为可回退镜像保留。升级前请备份数据库；启动后确认最新 `versions.version` 为 `309`，
并在品牌个性化页面分别检查登录页、后台和 WebClient 的明暗主题效果。现有连接、登录、
文件及分享记录不会被改写。
