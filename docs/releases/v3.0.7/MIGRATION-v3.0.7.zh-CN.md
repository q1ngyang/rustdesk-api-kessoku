# Kessoku v3.0.7 数据库迁移与回退

v3.0.7 把数据库从 schema `312` 增量升级到 `313`。迁移新增 Presence Lease v2 存储并
强制设备 ID 唯一，不会重写账户归属或自动合并设备身份。机器可读契约见
[`migration.yaml`](migration.yaml)。

## 兼容前提

- 保留 `POST /api/heartbeat`，旧客户端继续可用。
- Presence Lease v2 客户端要求 Starry `1.1.16-patch-v1.3.0` 与
  `peer_registry` capability 2，使 `start` 在常规 Kessoku 身份验证后校验完全一致的
  activation 路由。
- 每个客户端 Profile 必须使用不同的 `network_identity_uuid`；不得发送本地
  `profile_id`，Kessoku 会拒绝该字段。
- 不得让 v3.0.6 与 v3.0.7 同时写同一数据库。

## 可选的 Adaptive Relay Quality 管理接入

此接入不需要 Kessoku 数据库迁移。Starry patch-v1.2 及没有 `relay_quality` 的实例仍受
支持；管理页面会把新字段显示为“**不支持**”，不会让旧 Relay 清单整体失败。

不得从移动分支或 dirty Starry 工作区启用这条可选管理路径。应先把已审核的 patch-v1.3
Control v1 契约发布到不可变标签，并在 `internal/starrycontrol/CONTRACT_VERSION` 固定其
OpenAPI digest，然后执行：

1. 确认 `/control/v1/capabilities` 把 `relay_quality`、`relay_active_probe`、
   `relay_probe_protocol` 和 `relay_load_protocol` 都报告为版本 1。
2. 在只读模式核对质量聚合快照、每个 Relay 报告的协议、遥测新鲜度和
   `quality_candidate` 状态。
3. 确认配置表单直接来自 Starry config v4 JSON Schema/UI Schema，并且 YAML/表单往返
   不会丢失未知字段。
4. 在预发布环境执行 `validate -> plan`；patch-v1.3 必须把所有 `/relay_quality` 变更至少
   标为 `medium`。如果旧版或不符合契约的 Agent 返回 `low`，Kessoku 仍保留同一防御下限，
   且不改变 plan ID 或 candidate digest。
5. 应用已审核的准确计划；关闭变更窗口前，必须收到成功的 activation ACK，并确认其
   generation、source/effective digest 与当前运行状态一致。

告警只使用契约明确状态：Relay 遥测过期，以及策略已启用但质量候选数为零。timeout、
invalid/late 报告和回退原因是累计计数，应根据部署基线监控增量或速率，不能自行发明绝对
门槛。管理页面必须始终只展示脱敏聚合数据。

## 升级前检查

1. 停止连接该数据库的全部 Kessoku 写入者。
2. 把数据库、配置、TOTP 加密密钥、访问令牌签名密钥、内部 PKI、上传媒体和当前镜像
   digest 作为一个恢复集合备份，并实际恢复验证。
3. 对容器实际挂载的配置执行无副作用校验：

   ```sh
   kessoku-api config validate --config /app/conf/config.yaml --json
   ```

4. 只读检查数据库。v3.0.6 数据库必须输出 `installed_schema: 312`、
   `target_schema: 313`、`state: upgrade_required` 与 `migration_required: true`：

   ```sh
   kessoku-api database status --config /app/conf/config.yaml --json
   ```

5. 确认设备 ID 唯一性预检没有结果：

   ```sql
   SELECT id, COUNT(*)
   FROM peers
   GROUP BY id
   HAVING COUNT(*) > 1;
   ```

存在重复项时，Kessoku 会在创建 schema-313 对象或写版本标记前拒绝迁移。必须由负责人
确认正确的 UUID/账户映射；不得只为通过索引而删除、合并或改绑身份。

## 升级

保持其他写入者停止，只运行一个迁移命令：

```sh
kessoku-api database migrate --config /app/conf/config.yaml --json
```

Kessoku 会为 SQLite 获取 `flock`、为 MySQL 获取 `GET_LOCK`、为 PostgreSQL 获取
advisory lock。随后执行重复 ID 预检、创建 `peer_presence_leases`、增加 peer
activation/聚合字段和设备 ID 唯一索引，最后才记录 schema 313 并验证结果。到达 313 后
重复运行命令是幂等的。

启动 v3.0.7 并再次运行 `database status`，必须得到 `installed_schema: 313`、
`target_schema: 313` 与 `state: current`。然后验证：

- 旧 heartbeat 客户端仍能显示在线；
- Presence v2 客户端完成 A→B→A 后，旧 activation 的延迟 `end` 不改变当前 activation；
- 强制结束客户端后，设备在 45 秒 lease TTL 到期时自动离线；
- 使用不同网络身份 UUID 的两个 Profile 保持为独立设备，账户归属不变；
- 超级管理员能读取 `GET /api/admin/presence/v2/metrics`，响应不含 lease token 或
  高基数身份标签。

完整线协议、保留策略、指标范围和告警契约见
[Presence Lease v2 运维说明](../../operations/PRESENCE-LEASE-V2.zh-CN.md)。

## 失败处理

只有全部迁移操作成功后才记录目标版本。初始化失败可能留下可重试的增量对象和旧版本
标记；修正根因后重新运行同一个带锁命令。不得手工修改版本、删除 lease 对象或执行任意
修复 SQL。

`newer_than_binary` 对 status、migrate、maintenance 与默认启动都是硬拒绝。应恢复匹配的
程序/数据库恢复集合，不能强迫 Kessoku 读取未知 schema。日志、迁移输出、备份、指标、
trace 与支持包都不得包含原始 lease token。

## 恢复回退到 v3.0.6

v3.0.6 只理解 schema 312，不理解 313；迁移后不支持只换镜像的原地回退。

1. 停止全部 v3.0.7 写入者，并保留失败 schema-313 数据库的诊断快照。
2. 恢复完整且匹配的升级前 schema-312 恢复集合，包括数据库、TOTP 密钥、媒体、签名
   密钥、配置和 PKI。
3. 只启动一个 v3.0.6 进程，验证 schema 标记、登录、管理员角色、地址簿及真实客户端
   连接后再增加副本。

[English](MIGRATION-v3.0.7.md)
