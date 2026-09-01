# 本地维护 CLI

Kessoku v3.0.7 为 S6 supervisor 与人工救援提供有边界的本地命令面。这些都是进程本地
Cobra 命令；没有对应的恢复 HTTP 路由、任意 SQL、shell 或任意文件读取能力。

[English](LOCAL-MAINTENANCE-CLI.md)

## 命令与初始化边界

| 命令 | 配置 | 数据库 | 写入 |
| --- | --- | --- | --- |
| `version` | 不加载 | 不打开 | 无 |
| `config validate` | 解析并检查引用 | 不打开 | 无 |
| `database status` | 解析 | 只读连接 | 无 |
| `database migrate` | 解析 | 读写连接及排他迁移锁 | 仅 schema 迁移 |
| `maintenance recover-admin` | 解析 | 必须精确 schema 313 | 一个审计事务 |
| `maintenance reset-2fa` | 解析 | 必须精确 schema 313 | 一个审计事务 |

所有命令支持 `--json`；每个 JSON 对象都含 `schema_version: 1`。JSON 只写 stdout，人工
诊断写 stderr。结构化输出不包含密码、密码哈希、Token、TOTP 密钥、私钥或完整 DSN。

## 版本、配置与数据库

```sh
kessoku-api version --json
kessoku-api config validate --config /app/conf/config.yaml --json
kessoku-api database status --config /app/conf/config.yaml --json
kessoku-api database migrate --config /app/conf/config.yaml --json
```

`database status` 输出 `empty`、`current`、`upgrade_required`、
`newer_than_binary` 或 `invalid`；只有 `current` 的 `safe_to_start` 为 `true`。
`database migrate` 幂等且不会启动 HTTP listener。当工作目录为 `/app` 时，SQLite 文件仍为
`/app/data/rustdeskapi.db`。

## 管理员恢复

必须且只能用一个标识选择用户，再独立确认数据库中完全一致的用户名：

```sh
install -m 0600 /dev/null /run/secrets/kessoku-recovery-password
# 不经过 shell history，把 12–128 字节密码写入该文件。
kessoku-api maintenance recover-admin \
  --config /app/conf/config.yaml \
  --username alice \
  --confirm-username alice \
  --password-file /run/secrets/kessoku-recovery-password \
  --reset-2fa \
  --json
```

`--user-id ID` 可以替代 `--username`，但二者不能同时出现或同时省略。密码文件必须是
owner-only 普通文件；符号链接、group/other 权限、文件替换竞态和 12–128 字节以外内容都会
被拒绝。不得通过参数、环境变量、YAML、日志或镜像层传递密码。

成功恢复会启用账户，设置 `role=super_admin` 和兼容字段 `is_admin`，清除全部遗留管理员
scope 与登录挑战，可选替换密码/TOTP，只提升一次 `auth_version` 并撤销有效 Token。全局
TOTP 加密密钥和其他账户都不改变。

## 双重认证恢复

```sh
kessoku-api maintenance reset-2fa \
  --config /app/conf/config.yaml \
  --user-id 42 \
  --confirm-username alice \
  --json
```

没有 factor 时仍幂等成功，但每次经确认执行仍会提升 `auth_version` 并撤销当前会话。只删除
目标用户已启用/待确认的 factor 行及登录挑战，绝不删除或重新生成全局 TOTP 密钥。

## JSON 错误与退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功 |
| `2` | 参数、用户选择或密码文件输入错误 |
| `3` | 配置解析或校验错误 |
| `4` | 数据库连接或迁移执行错误 |
| `5` | schema 与当前二进制不兼容 |
| `6` | 维护目标或事务操作失败 |
| `1` | 未预期进程/输出错误 |

失败示例：

```json
{"schema_version":1,"operation":"reset_2fa","success":false,"request_id":"0191f6a0-0000-7000-8000-000000000402","password_reset":false,"two_factor_reset":false,"two_factor_was_configured":false,"login_challenges_cleared":0,"scopes_cleared":0,"sessions_revoked":0,"error":{"code":"MAINTENANCE_CONFIRMATION_MISMATCH","message":"confirm-username does not exactly match the stored username"}}
```

每次进入数据库的救援都会先创建 intent 审计，再完成为 success 或 failure。未来 schema 会在
审计和账户写入前被拒绝。应把本地命令执行权以及配置/数据库挂载权限限制在可信 S6 控制面，
不要向 Kessoku 暴露 Docker socket。

历史命令 `reset-admin-pwd --password-file PATH` 与
`reset-pwd --user-id ID --password-file PATH` 继续可用，并复用安全密码读取、认证代际提升、
会话撤销和密码变更审计路径。
