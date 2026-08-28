# RustDesk API Kessoku v3.0.3 数据库迁移与回退

v3.0.3 会将 Kessoku 数据库依次升级到版本 `309`。迁移主要为增量变更，但会引入持久化
加密密钥、媒体文件、身份清理和旧程序无法识别的新字段。生产维护前必须用生产备份恢复
副本并完整演练升级。

## 可升级来源

- v3.0.1 数据库从版本 `302` 开始执行 303–309 迁移；
- 更早的 v3 候选数据库可能先执行已有的 301/302 令牌与角色迁移，再继续到 309；
- 全新数据库会直接创建为版本 309。

每一阶段的详细记录见
[`MIGRATION-303.md`](../MIGRATION-303.md)、
[`MIGRATION-304.md`](../MIGRATION-304.md)、
[`MIGRATION-305.md`](../MIGRATION-305.md)、
[`MIGRATION-306.md`](../MIGRATION-306.md)、
[`MIGRATION-307.md`](../MIGRATION-307.md)、
[`MIGRATION-308.md`](../MIGRATION-308.md) 和
[`MIGRATION-309.md`](../MIGRATION-309.md)。

## 升级前

1. 停止所有 Kessoku 写入实例，记录当前镜像摘要、数据库版本、启用的管理员、外部身份
   和关键表行数。
2. 创建并验证一致性数据库备份，同时备份配置、访问令牌签名密钥、内部 PKI 和当前镜像
   摘要。
3. 备份整个数据卷，包括 `media.directory` 与已经存在的 `two-factor.key-file`。数据库、
   TOTP 密钥和媒体目录必须作为同一版本的恢复集合保存。
4. 曾使用 LinuxDo 时，先记录受影响账户并准备其他已批准的登录/恢复方式；迁移 305 会
   删除 LinuxDo 提供商和身份绑定记录。
5. 确认 WebClient 公共/API 地址是两个精确、不同的 HTTPS Origin，证书有效。若浏览器
   客户端中断会影响维护，可先保持 `web-client.mode: disabled`，完成 API/后台迁移后再开。
6. Starry 连接认证保持 `off` 或 `audit`，服务端控制保持只读，直到新版 API 和真实客户端
   验收通过。

## 升级步骤

1. 在隔离环境恢复备份并只启动一个 v3.0.3 实例，禁止新旧版本并发写入。
2. 确认 `versions.version` 最新值为 `309`，启动日志没有迁移失败。
3. 确认至少一个启用的 `super_admin`，并验证普通用户、范围管理员、原生客户端登录、设备
   系统信息上报、地址簿和会话撤销。
4. 确认 `/app/data/totp.key`（或配置路径）是 32 字节、权限 `0600` 的普通文件；使用测试
   账户绑定和验证 TOTP，确保密钥不会进入日志或配置。
5. 验证媒体上传、头像裁剪、所有浅色/深色默认和自定义品牌素材、公告，以及一次成功的
   Country/City/ASN MMDB 更新。
6. 在独立 WebClient 域名验证登录保持、后台授权、强制 Relay 会话、远程主机名、协助聊天、
   注销和连接审计记录。
7. 验证 Starry 能力、状态和只读日志。只有在测试环境完成计划/应用/回退后，才可开启
   Agent 配置写入；Kessoku 运行时日志级别仅在限定的排障窗口内调整。
8. 在生产维护窗口按相同步骤再次执行并验收。

可使用以下只读查询核对：

```sql
SELECT version, created_at FROM versions ORDER BY id DESC LIMIT 10;
SELECT id, username, role, is_admin, status FROM users ORDER BY id;
SELECT admin_user_id, scope_type, scope_id
FROM admin_resource_scopes
ORDER BY admin_user_id, scope_type, scope_id;
```

## 回退

不得让 v3.0.1、v3.0.0 或任何 v2 镜像读取已经迁移到 309 的数据库。停止全部 v3.0.3
实例后，恢复升级前完整且相互匹配的数据库、TOTP 密钥、媒体目录、配置、签名密钥和镜像
摘要，并保留失败的 309 数据库副本用于排查。

只恢复数据库会破坏 TOTP 与图片引用，只恢复密钥也可能使会话失效。不要删除新表或降低
版本记录来强迫旧程序启动。

[English](MIGRATION-v3.0.3.md)
