# RustDesk API Kessoku v3.0.1 数据库迁移与回退

v3.0.1 将数据库版本从 301 升级到 302。迁移是增量的，但角色语义发生了安全相关变化；正式升级前必须使用生产备份演练升级和回退。

## 302 新增内容

- `users.role`：`user`、`admin`、`super_admin`；
- `admin_resource_scopes`：记录范围管理员获授的用户组、用户、公共地址簿和 ID 设备；
- `(admin_user_id, scope_type, scope_id)` 唯一索引及目标查询索引。

首次升级时，原 `is_admin=true` 账号全部迁移为 `super_admin`，因此旧管理员不会被降权；其他账号迁移为 `user`。`is_admin` 列继续保留为 v2 客户端兼容镜像，v3 的授权判断只使用 `role`。即使此前迁移在添加 `role` 列后中断，版本 301 的再次启动也会按旧 `is_admin` 恢复角色。

## 升级步骤

1. 停止写流量并备份数据库；记录当前最新数据库版本和所有 `is_admin=true` 账号。
2. 使用备份副本启动 v3.0.1，确认最新版本为 302。
3. 验证原管理员均为 `super_admin`，且至少有一个启用状态的超管。
4. 创建一个 `admin` 测试账号，保持空范围并确认其看不到企业资源；随后分别授予四类资源，验证并集和批量越权拒绝。
5. 验证角色或范围变更后目标管理员的旧会话立即失效，需要重新登录。
6. 验证无误后再恢复生产流量。

建议核对：

```sql
SELECT id, username, role, is_admin, status FROM users ORDER BY id;
SELECT admin_user_id, scope_type, scope_id FROM admin_resource_scopes ORDER BY admin_user_id, scope_type, scope_id;
```

## 回退注意事项

首选方案是停止 v3 并恢复升级前的完整数据库备份。

如果必须让 v2.8.x 临时读取已升级数据库，不能直接启动旧程序：v2 只认识 `is_admin`，而兼容镜像对 `admin` 和 `super_admin` 都为 `true`，会把范围管理员误当成全局管理员。必须先停止所有 v3 实例，备份当前 302 数据库，并将范围管理员的旧镜像降为普通用户：

```sql
UPDATE users SET is_admin = FALSE WHERE role = 'admin';
```

然后才能启动 v2.8.x。该回退只改变旧程序看到的兼容镜像，不删除 `role` 和范围表；重新启用 v3 前应再次备份并确认每个账号的 `role`。禁止在 v3 与 v2 进程同时写入同一数据库。

## 验收与恢复

- `go test ./cmd ./service` 覆盖正常迁移、中断恢复、角色范围和授权清理；
- 升级失败时不得手工删除 302 新列或范围表，应保留失败日志并从备份恢复；
- 删除用户、用户组、设备或公共地址簿后，确认对应范围授权同步消失；
- 所有授权变更和越权拒绝应在 `admin_audit_events` 中留下结果记录。
