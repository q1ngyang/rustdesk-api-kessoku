# Kessoku v3.0.8 升级与回滚

> v3.0.8 是需要主动选择的**预览版**。必须使用精确的 Starry
> `1.1.16-patch-v1.3.1` 预览版，并按不可变 digest 固定两端镜像。稳定版
> v3.0.7/`latest` 仍可用于回退。

## 兼容边界

Kessoku 主业务数据库仍为 schema `313`，从 v3.0.7 升级不需要数据库迁移。Starry 支持
只按 Control API 返回的精确 capability 协商：

| Starry 契约 | 读取行为 | 写入行为 |
| --- | --- | --- |
| patch-v1.2，不含 Relay Quality/Fast capability | 原 Relay 清单可用；新状态显示“不支持” | 禁止 schema-v5 与 Fast 写入 |
| v1.3.0 Relay Quality + FastCompat | 原 Relay Quality 与旧 FastCompat 聚合可用；FastMedia 显示“不支持” | schema-v4 Quality/FastCompat 按 capability 写入；阻止 schema-v5/FastMedia 写入 |
| `1.1.16-patch-v1.3.1` 预览版（契约提交 `6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`） | 校验 Fast 与 Pairing 强类型状态 | Fast 写入要求 capability 版本 1 和 schema 5 |
| 已知 capability 完全缺失 | 明确“不支持”，不是“配置关闭” | 阻止不兼容写入 |
| 已知 capability 出现未知版本 | 整个响应关闭失败 | 阻止所有依赖写入 |

v1.3.1 契约与预览运行时均不可变，并由内置发布摘要固定。预览证据不等于稳定生产批准。

## 新增独立状态

SP1 状态不使用也不迁移 Kessoku 主业务数据库，其位置为：

- 容器：`/app/data/server-control/registry-v1.sqlite`；
- Debian/systemd：`/var/lib/kessoku-api/data/server-control/registry-v1.sqlite`；
- 私有凭据：同级 `instances/` 目录（目录 `0700`，文件 `0600`）；
- v3.0.7 兼容导出：同级 `exports/<managed-id>.static-instance.yaml`。

配置根目录及内容必须属于服务 UID，且不能是符号链接。CLI 与服务使用相同文件锁和单调递增
的 registry generation。
启用配对还必须显式设置 `server-control.host-identity-file`。systemd 使用
`/etc/machine-id`；容器必须把宿主机文件单独挂载到 `/run/kessoku-host-machine-id`，不能
使用镜像内置 machine-id，也不能把它放进随 registry 一起复制的目录。

## 升级前

1. 停止所有可能写同一数据目录的 Kessoku 进程。
2. 记录 `KESSOKU_DATA_DIR` 实际解析出的绝对宿主机路径；切换 Compose 目录或环境文件前
   重新核对。
3. 把完整数据目录、配置和 secrets 作为同一恢复集合备份。外部 SQL 虽不升级 schema，
   仍应同时做数据库厂商支持的一致性备份。
4. 确认 Starry 契约工作区干净且位于精确完整提交，并验证内置摘要：

   ```sh
   sha256sum docs/releases/v3.0.8/STARRY-RELEASE-SUMMARY.json
   # fedeb47ff77bdbc594ddd3ba5b54238a469b02416cfb3410dbd535eff9c7e0ef
   ```

5. 本地校验 Kessoku 配置；首次启动时 Kessoku 和 Starry 两层写入都保持关闭。

## Docker 升级与持久化预检

必须把同一个 bind source 挂载到 `/app/data`：

```sh
test -n "${KESSOKU_DATA_DIR:?请设置原有绝对数据目录}"
test -d "$KESSOKU_DATA_DIR"
test -s "${KESSOKU_HOST_IDENTITY_FILE:-/etc/machine-id}"
docker compose config
docker compose pull kessoku-api
docker compose up -d --force-recreate kessoku-api
docker compose exec kessoku-api ./kessoku-api server-control registry status \
  --config /app/conf/config.yaml --json
```

普通 `pull`、`up --force-recreate` 和 `down`/`up` 只有在解析到同一宿主机目录时才会保留
配对。改变相对路径可能悄悄挂载空目录，但 Kessoku 启动与 `registry status` 会返回
`REGISTRY_NOT_INITIALIZED`，不会生成另一 installation。配对或写入前必须比较记录的绝对
路径和 `installation_id`。

不要把 `docker compose down -v` 作为升级步骤。仓库默认是 bind mount，Docker 不会用
`-v` 删除它，但部署覆盖可能改用会被 `-v` 删除的 named/anonymous volume。执行任何卷删除
前，先检查 `docker compose config`、确认真实卷类型并验证备份可恢复。

容器使用 UID/GID `65534:65534`。属主不匹配、根目录为符号链接、文件权限宽于 `0600`
或目录权限不是 `0700` 时，registry 预检会失败，而不会生成替代身份。

## Debian/systemd 升级

默认根目录是 `/var/lib/kessoku-api/data/server-control`。软件包升级、降级和普通卸载都会
保留 `/var/lib/kessoku-api`；维护脚本不会删除托管身份。确认属主为 `kessoku-api` 服务用户
后执行：

```sh
sudo -u kessoku-api kessoku-api server-control registry status \
  --config /var/lib/kessoku-api/conf/config.yaml --json
```

主机迁移或 purge 时，服务和 CLI 不能并行运行；普通状态与配对写操作共享 registry 文件锁。

## SP1 配对与轮换

在部署配置中预先写入 `server-control.pairing.agent-origins`。CLI/UI 只能选择 allowlist ID，
不能输入任意 URL 或 callback。Broker origin 和 TLS SPKI SHA-256 pin 必须与公开的 Broker
证书一致。

创建短期 code 时，不要把 code 放入命令参数、环境变量、日志或工单：

```sh
kessoku-api server-control pair create --config /path/to/config.yaml \
  --id starry-main --name "Starry main" --agent-origin primary \
  --confirm confirm:pair:starry-main:primary
```

若 code 尚未领取，应先按 enrollment ID 撤销再创建替代 code；命令不需要、也不接受原始 code：

```sh
kessoku-api server-control pair revoke --config /path/to/config.yaml \
  --enrollment-id 019b0000-0000-7000-8000-000000000001 \
  --confirm confirm:revoke-pairing:019b0000-0000-7000-8000-000000000001
```

对真正的全新部署，这个携带精确二次确认的 `pair` 也是唯一允许初始化缺失 registry 的
动作。服务启动、状态查询、`rotate`、`adopt` 和 Relay 命令都不会初始化 registry。因此，
遇到 `down -v` 或路径变化后，应先恢复或核对预期数据根目录，再确认新的 pair；确认后会
有意创建全新的 installation 身份。

新 `pair` 由 Agent 回传并锁定 instance UUID；`adopt` 和 `rotate` 必须预先绑定已有精确 UUID。
轮换会生成独立的新凭据 generation，保留旧文件用于受控回滚，原子更新静态导出，热加载
Provider，并强制恢复 Kessoku 只读。在归档旧 generation 前必须证明新证书和 JWT 密钥可用；
生产证书轮换仍是稳定版晋级门禁。

Relay code 创建前，Kessoku 必须先调用已认证 Agent 的 prepare API。Relay endpoint、池、
证书、secret 和配置授权始终属于 Starry。`--activate-after-health` 只有在创建 code 时携带
精确高风险确认才被接受；否则必须使用成功 operation ID、generation、配置摘要和健康快照
进行显式 activation。

## 备份、恢复与跨主机迁移

复制 `server-control/` 前先停止服务，使 SQLite、WAL、凭据和导出属于同一一致 generation。
必须把完整目录恢复到原绝对路径并恢复权限；不能只恢复 SQLite 或只恢复 `instances/`。

registry 会绑定非秘密的主机指纹。复制到新主机会以 identity-clone 错误失败。只有在确认源
主机已停止、不能再写此身份后，才执行双重接管：

```sh
kessoku-api server-control registry adopt-host --config /path/to/config.yaml \
  --installation-id <记录的UUID> --old-host-stopped \
  --confirm confirm:adopt-host:<记录的UUID>
```

禁止两个主机同时运行同一 registry 身份；活跃身份克隆是安全事件，不是负载均衡方式。

## v3.0.8 → v3.0.7 → v3.0.8

1. 停止 v3.0.8，备份完整数据和配置集合。
2. 对每个托管实例，把 `exports/<id>.static-instance.yaml` 中的条目人工合并到 v3.0.7
   兼容的 `server-control.instances[]` 配置。完成验证前保持 Kessoku 与 Agent 两层只读。
3. 用同一数据挂载启动 v3.0.7。它继续使用静态 mTLS/JWT 文件并忽略托管 registry；不能
   修改或删除 `data/server-control`。
4. 校验 capability/status 并完成真实可靠 Relay 会话。v3.0.7 不提供 schema-v5 FastMedia
   控制或 SP1 管理。
5. 重新升级时先停止 v3.0.7，使用同一个未修改目录和配置启动 v3.0.8。开启写入前核对原
   installation ID、registry generation、instance UUID 和凭据摘要。

两个版本都使用主库 schema 313，因此正常二进制往返不需要恢复主数据库。如果过程外有数据
或 registry 路径变化，则必须从备份恢复。

## 显式身份 purge

卸载软件包、删除容器或关闭 pairing 都不会删除身份。永久删除只能在停止全部进程后提供两项
确认：

```sh
kessoku-api server-control registry purge --config /path/to/config.yaml \
  --installation-id <记录的UUID> --service-stopped \
  --data-loss-understood --confirm confirm:purge:<记录的UUID>
```

命令验证精确 installation identity 后，不可恢复地删除独立 registry、托管凭据、恢复记录
和静态导出；它不删除 Kessoku 主业务数据库。只有经过验证的备份才能恢复已 purge 的身份。

## 开放写入前验收

2026-09-03 隔离精确状态验证已通过 schema-v4 static export 接管并返回 schema v5，
期间 registry SQLite 摘要不变；证书轮换和同数据目录 force-recreate 也已通过。生产批准前
仍必须使用生产 PKI、已停止的源主机、经过恢复验证的备份和最终不可变发布工件重复执行。

- 验证 schema v4/v5 与新旧 capability 行为；
- apply/rollback 后核对 Starry generation、source/effective digest、schema digest 和每个必需
  子系统 ACK；
- 每次 rollback 都要求管理员 RBAC 和精确绑定的
  `confirm:rollback:<instance-id>:<revision-id>` 二次确认；该保护同样覆盖会重新启用既有
  高风险 FastMedia 配置的回滚；
- 确认 UI 只包含 Relay 聚合状态与脱敏 endpoint；
- 完成真实可靠 fallback、自动 FastMedia 重入、Akari 双角色、NAT/UDP 故障、轮换、备份恢复
  和跨主机迁移测试；
- 预览部署必须主动选择且新增能力默认关闭。Hosted CI、安全审查、SBOM、签名、
  provenance 与 attestation 是预览发布门禁；真实 Akari/网络/PKI 矩阵是后续稳定版门禁。

[English](MIGRATION-v3.0.8.md)
