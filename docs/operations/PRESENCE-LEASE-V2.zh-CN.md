# Presence Lease v2 运维说明

Presence Lease v2 让 RustDesk 客户端快速切换网络 Profile 时，旧 activation 的延迟请求
不会改变当前 Profile 的在线状态。它是对 `POST /api/heartbeat` 的补充，不会删除旧接口。

[English](PRESENCE-LEASE-V2.md)

## 身份与线协议

公开端点如下：

- `POST /api/presence/v2/start`
- `POST /api/presence/v2/renew`
- `POST /api/presence/v2/end`
- `POST /api/presence/v2/deactivate`，用于切换 Profile 前退役已经过 Starry 验证的
  activation

`uuid` 是 Profile 独立 `network_identity_uuid` 的标准 base64 编码；每个 Profile 必须
使用不同值。本地 `profile_id` 不是服务端身份，不出现在 OpenAPI 中；请求携带它时
Kessoku 会直接拒绝。

`start` 接收规范化 RustDesk `id`、网络 UUID、activation epoch、客户端随机 16 字节
activation ID，以及一个或多个 32 字节 Starry route lease。Kessoku 先通过 Starry
`peer_registry` capability 2 验证完全一致的活动路由。成功响应原样回传 activation
字段，并返回：

- 随机 16 字节 base64url `lease_id`；
- 随机 32 字节 base64url bearer `lease_token`；
- 绝对时间 `expires_at`、相对时间 `expires_in` 和聚合值 `online_until`。

`renew` 与 `end` 携带相同的 ID、UUID、activation 字段和 bearer token。新客户端还应
携带 `lease_id`，服务端会验证 ID 与 token 命中同一行；为了兼容首个 Presence v2
客户端版本，暂时仍接受仅凭唯一 token 精确定位。`end` 对指定 lease 幂等。含凭据的
响应都带 `Cache-Control: no-store`。

lease TTL 为 45 秒。客户端最迟应在本地到期前 15 秒续约；过期或被拒绝后必须重新
start。客户端崩溃不需要清理请求：到达 `expires_at` 后，该 lease 自动停止贡献在线状态。

## 状态与兼容性

当前 activation 中只要有一条未结束且 `expires_at` 仍在未来的 lease，设备就处于 v2
在线状态。结束一条并行 lease 不会结束其他 lease；更高 activation 会退役旧 epoch；
延迟到达的旧 end 仍被旧 token 绑定，无法改变新 activation。

旧 heartbeat 路由及响应保持不变。Kessoku 保留原有 30 秒 `last_online_time` 写入节流，
v2 renew 不会提高频率。v2 活动后的 90 秒内以 lease 聚合为准，避免近期 inventory 写入
让已 end 的 lease 继续显示在线；降级窗口结束后，当前旧客户端 heartbeat 仍按原逻辑
决定在线状态。

schema 313 中设备 ID 唯一。已绑定的设备 ID 不得改绑到另一个非空网络 UUID，已有账户
归属也不得被另一账户替换。冲突会 fail closed，不修改设备资料、账户归属或在线状态。

## 指标

超级管理员可使用常规 `api-token` 请求
`GET /api/admin/presence/v2/metrics`。响应不包含 token、设备 ID、UUID、IP、用户名或其他
高基数标签。

`*_accepted_total`、`*_rejected_total`、`*_errors_total` 是进程级计数器，重启后清零；
`active_leases`、`online_peers`、`expired_unended_leases` 是数据库级 gauge。抓取多个
Kessoku 副本时对 gauge 使用 `max`，不要使用 `sum`。快照明确返回 `counter_scope`、
`gauge_scope`、`collected_at` 与 `schema_version`。

建议先采用以下告警并根据生产基线调优：

- 任一 `*_errors_total` 在 5 分钟内增长时触发严重告警；
- renew 拒绝率连续 10 分钟超过 5% 时告警；
- start 拒绝率连续 10 分钟超过 10% 时告警，并检查 Starry capability、路由健康、
  客户端时钟与 activation 状态；
- `expired_unended_leases` 连续 30 分钟增长时告警；
- 有预期在线客户端时 `active_leases` 意外降为零，或指标端点无法查询数据库时触发严重告警。

不得把 lease token 放入指标标签、注解、exemplar、trace 或告警文本。lease ID 只可用于
短时诊断关联，也不能作为无界指标标签。

## 存储与迁移

schema 313 新增 `peer_presence_leases` 表、peer activation/聚合字段、随机 lease/token
hash 唯一索引和设备 ID 唯一索引。数据库只保存 token 的 SHA-256 哈希。现有每 6 小时
运行的保留任务会删除超过 24 小时的过期 lease 历史，不会删除活动或近期记录。

迁移前，下列查询必须没有结果：

```sql
SELECT id, COUNT(*)
FROM peers
GROUP BY id
HAVING COUNT(*) > 1;
```

Kessoku 会执行同样的重复 ID 预检；存在冲突时不会创建 schema 313 结构，也不会写入 313
版本标记。只能在负责人确认正确 UUID/账户映射后处理重复记录；不得通过复制 presence 行或
降低 schema 标记来合并身份。

v3.0.6 只理解 schema 312，因此迁移到 313 后只能通过恢复回退：停止全部写入者，恢复匹配
的升级前数据库备份，再启动 v3.0.6。
