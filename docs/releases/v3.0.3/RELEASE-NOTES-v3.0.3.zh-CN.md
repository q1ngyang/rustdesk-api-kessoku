# Kessoku v3.0.3

v3.0.3 是首个公开发布并正式支持的 Kessoku v3 版本。此前 v3.0.1 被确认存在后台界面、
客户端信息上报和 WebClient 集成等重大缺陷，其 GitHub Release 已撤回；v3.0.2 标签仅保留
一次发布失败且未公开的历史记录，没有受支持的 Release 制品或容器镜像。新部署请勿使用
这两个版本；已经运行 v3.0.1 的环境，应先完整备份数据库、密钥、媒体目录和配置，再升级
到 v3.0.3。

Kessoku 仍是非官方 RustDesk 账户、管理与策略服务，并与固定版本的
[`rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)
Control API 集成。后台面板、Relay-only 浏览器客户端与 Go 后端均从同一已审核提交构建。

## 主要更新

- 恢复完整的工作区和系统管理导航，修复弹窗、表格在 PC、平板和手机上的响应式布局，
  并统一优化标准模式与夜间模式。
- 修复 RustDesk 客户端设备信息接收与展示，后台可正确显示设备 ID、主机名、用户名、
  操作系统、CPU、内存、UUID 和版本；登录、连接、文件、分享及 WebClient 审计记录采用
  一致的用户与设备归属逻辑。
- 重构独立 HTTPS 域名下的 WebClient 登录保持、后台一次性授权和连接审计；新增密码
  明文切换、按连接状态收起表单、远程主机名展示、语言/主题同步，以及简洁的远程协助
  文字聊天窗口。浏览器远控仍强制使用 Relay WSS 与 VP9。
- 新增集中式明暗主题品牌配置：登录页、后台、关于页和 WebClient 共用一套浅色/深色
  Logo 与图标；登录页和 WebClient 分别支持浅色/深色背景；三端共用页脚 HTML，并支持
  文案、HTML、CSS、图片链接、上传、预览和内置 StarryDesk 默认素材。服务端控制页面的
  StarryLinks 标识保持固定。
- 新增加密保存的 TOTP 双重认证、头像上传裁剪与缩放、个人资料编辑、会话撤销，以及
  跨后台和 WebClient 同步的语言/主题偏好；界面语言新增日语。
- 扩展类型化 Starry 服务端控制：更清晰的状态与参数说明、基于能力协商的配置审核计划、
  完整 Schema/YAML 编辑、审计历史、Kessoku/Starry/Relay 日志查看与导出，以及 Agent
  控制模式下受保护的 Kessoku 运行时日志级别切换。
- 新增管理员公告、GeoLite2 Country/City/ASN 数据源、MMDB 定时更新与明确的成功/失败
  提示；活动列表中的 IP 可快速查看国家、城市和 ASN。
- 完全移除 LinuxDo OAuth 登录及其数据库绑定。仍可按配置使用 GitHub、Google、LDAP
  和标准 OIDC。
- 发布 Linux amd64 容器、压缩包/二进制与 DEB，并提供校验和、前端及源码 SBOM 和
  Sigstore 构建来源证明。

## 兼容性与破坏性变更

- Go 模块路径为 `github.com/q1ngyang/rustdesk-api-kessoku/v3`，下游 Go 导入和依赖需
  使用 `/v3`。
- 数据库版本为 `309`，包含 v3 分级角色，并依次新增品牌、TOTP、登录挑战、公告、GeoIP
  策略、用户界面偏好、明暗主题素材、共用页脚和 WebClient 审计归属字段。
- 旧 `is_admin=true` 账户会先迁移为不受范围限制的 `super_admin`；v3 权限以 `role`
  （`user`、`admin`、`super_admin`）为准。不得让 v2 与 v3 同时写入同一数据库。
- 启用双重认证后，`/app/data/totp.key`（或 `two-factor.key-file` 指定文件）用于解密已
  绑定的 TOTP 密钥，必须与数据库一起备份，现有部署不得重新生成。
- 上传的品牌图片和头像位于 `media.directory`（通常为 `/app/data/media`），必须纳入
  备份和回退集合。
- 内置 WebClient 要求 `web-client.public-origin` 与 `web-client.api-origin` 是两个精确、
  不同的 HTTPS Origin。无关域名之间不会直接共享 Cookie；Kessoku 使用受限的 API 域名
  浏览器会话，并在登录后同步界面偏好。
- LinuxDo 配置与身份绑定不再支持，并会在迁移时删除；曾使用该提供商的部署应在升级前
  核对受影响身份。
- 不要从数据库版本 309 原地降级。必须停止全部写入实例，并恢复升级前匹配的数据库、
  TOTP 密钥、媒体、配置和签名密钥。

生产升级前请先完成备份和恢复演练，阅读
[升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback)
及 [v3.0.3 数据库迁移说明](MIGRATION-v3.0.3.zh-CN.md)。

[English](RELEASE-NOTES-v3.0.3.md)
