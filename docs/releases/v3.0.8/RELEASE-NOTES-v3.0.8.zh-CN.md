# Kessoku v3.0.8 发布说明

> 发布状态：**PREVIEW_APPROVED**。v3.0.8 以 GitHub prerelease、不可变
> `v3.0.8` 镜像和移动 `preview` 镜像发布，不替换稳定版 v3.0.7 Release 与
> `latest` 镜像。

v3.0.8 从已冻结的 v3.0.7 基线增加 Starry v1.3.1 FastCompat/FastMedia 与
Pairing v1 冻结契约的强类型管理和脱敏聚合观察。实现固定到 Starry 不可变预览版
`1.1.16-patch-v1.3.1` 及本目录逐字节一致的运行时发布摘要，不根据版本字符串猜测
能力；线协议本身仍冻结于提交
`6f5a31008ab7761d8557c8cf9fefcb5be11c49e6`。

## Relay FastCompat 与 FastMedia

- 已知能力只接受精确版本：`fast_relay_authorization=1`、
  `fast_media_relay_udp=1`、`config_schema=5`。完全缺失时显示“**不支持**”；
  已知能力出现未知版本时关闭失败。
- schema v5 表单继续直接渲染 Starry 返回的 JSON Schema/UI Schema，Kessoku
  不复制一套 Relay 配置规范。
- `fast_compat_enabled` 与 `fast_media_v1_enabled` 相互独立且默认关闭。启用
  FastMedia 前，当前已认证遥测必须证明至少有一个端口匹配、新鲜、健康且支持 UDP
  的 Relay 候选，否则拒绝变更。
- 任何 `/fast_mode` 计划至少为中风险；打开 FastMedia，或改变 UDP endpoint、
  datagram、限速及相关防火墙配置属于高风险，必须经过精确二次确认、管理员 RBAC、
  before/after 审计绑定、schema/generation 摘要和所有子系统接受的 activation ACK。
- Relay 页面只显示强类型脱敏聚合：能力、遥测新鲜度、候选资格、bind 结果、
  cookie/grant 与 role/session/allocation 拒绝分类、rebind、forward/drop/rate-limit、
  expiry 和可靠回退计数。服务端事件不能被描述成某个客户端已经进入 FastMedia。

Starry patch-v1.2 和 v1.3.0 的原有 Relay/Relay Quality 页面继续可用。新增配置写入会
同时检查 capability 和 schema，不能向不兼容目标写入 schema-v5 FastMedia 字段。

## SP1 Broker 与 Relay enrollment

- Control Agent Broker 只接受部署时 allowlist 中的 HTTPS origin；浏览器不能提交
  Agent URL 或 callback。
- code 短期、单次且绑定用途。Kessoku 只保留 secret 的 SHA-256 摘要；claim 精确绑定
  purpose/action、配置摘要、CSR 公钥和实例身份，并支持响应丢失后的同公钥恢复。
- 未领取 Control Agent code 可在 UI/CLI 中按 enrollment ID 撤销，无需向 Kessoku 回传
  原始 code，随后可安全重建。
- 每个受管实例生成独立 mTLS 客户端凭据和 service-JWT Ed25519 密钥；学习后锁定
  Starry instance UUID；首次配对和每次轮换后 Kessoku 都回到只读状态。
- Relay enrollment 必须由已认证 Starry Agent 先授权再完成。
  `activate_after_health` 只能在创建 code 时成为不可变的高风险预授权；否则保持待人工
  批准，并要求精确 generation/health 证据。
- Kessoku 不保存原始 code、Relay 私钥、遥测 secret、签名 grant、客户端地址、
  session/allocation UUID 或媒体内容；Kessoku 不进入 Relay 数据面，停止 Kessoku
  不会停止已有远控。

配对数据在 `data/server-control/registry-v1.sqlite` 独立版本化；
`data/server-control/instances` 中的私钥文件仅属主可读写。主业务数据库仍为 schema
313。每个受管实例都会持续刷新 `server-control.instances[]` 静态导出，使 v3.0.7 在人工
合并后可以接管。
配对必须显式配置主机身份文件。Compose 把宿主机的
`${KESSOKU_HOST_IDENTITY_FILE}` 挂载到 `/run/kessoku-host-machine-id`，systemd 使用
`/etc/machine-id`；registry 只保存加域分隔的 SHA-256 指纹。复制到另一主机的 registry
会关闭失败，直到管理员用精确 installation ID 确认执行 `registry adopt-host`。

## 升级与兼容

从 v3.0.7 升级不改变数据库 schema。镜像 pull、force-recreate 和 down/up 必须保持同一
完整 `KESSOKU_DATA_DIR`，尤其是 `server-control/`。v3.0.7 只忽略托管 registry，不能
修改它；重新升级 v3.0.8 后应恢复原 registry generation 与凭据。

容器、systemd、备份恢复、主机接管、证书轮换、回滚和显式 purge 步骤见
[迁移指南](MIGRATION-v3.0.8.zh-CN.md)。

## 精确状态诊断验证

2026-09-03 的隔离候选验证已关闭此前互锁门禁中的本地部分：

- Go 全量测试、race、vet、生成 API 稳定性、文档和发布身份检查通过；管理前端通过
  lint、全部 33 项测试、零漏洞/registry 签名审计、两次逐字节一致生产构建及生产依赖
  SBOM licence 检查；WebClient 通过 lint/typecheck、全部 63 项测试、同样的审计/签名与
  可复现构建门禁，以及 62-component licence 完整 SBOM；
- migration fixture 通过 SQLite 以及 CI 固定摘要的 PostgreSQL 16.4、MySQL 8.4.2，覆盖
  schema 检查、维护恢复和各方言 migration lock；
- `golang.org/x/crypto` 0.56.0 通过 module verify、vet、全量测试和全量 race；当前
  `govulncheck` 为 0 可达、0 imported-package 漏洞，只剩 Kessoku 未导入的无维护
  `openpgp` package module-only 提示；
- 缺少 rollback 二次确认时返回 HTTP 428，generation、ETag 和 history 均不变；提供精确
  revision 绑定确认后，schema/generation digest 与全部子系统 ACK 均接受；
- SP1 Control 证书轮换、Agent 重启、Kessoku force-recreate 和 Provider 热加载后，独立
  registry 仍为 schema 1、generation 30，凭据权限仅属主可用，托管写入保持只读；
- 同一持久状态通过生成的 static export 运行 Kessoku v3.0.7 + Starry v1.3.0/schema 4，
  status/capabilities/Relay inventory 均为 HTTP 200，registry 逐字节不变；返回
  v3.0.8/v1.3.1/schema 5 后 UDP telemetry 新鲜且健康；
- v3.0.8 对外 Relay inventory 不含被禁止的 process/allocation/session UUID、地址、token、
  nonce、grant、私钥或媒体字段名；
- 使用当前 Akari FastMedia library 双角色和精确 HBBR 候选的协议层 harness 保持可靠
  TCP，在 UDP probe 超时后回退，并在同一 session 自动重入 FastMedia。

这些结果与受保护的干净 commit 候选/发布流程共同构成预览版就绪证据，但不能替代稳定版
所需的干净不可变 Akari 构建、完整 HBBS 信令/GUI 客户端、真实设备网络以及生产
PKI/多主机证据。

## 预览边界与稳定版剩余门禁

Kessoku 预览版要求固定 Starry 不可变预览运行时及 provenance，并由自身干净 commit 的
Hosted 流程完成安全、构建、软件包、SBOM、provenance 和 attestation 检查；它不会等待
Akari 发布。以下项目保留为稳定版晋级门禁，发现问题时可继续迭代第三位版本号：

- 干净不可变 Akari 候选，以及两个真实 Akari 客户端跨 Native、WSS、mixed signalling 的
  HBBS/HBBR 双角色全链路；
- 在上述真实客户端上验证可靠 fallback 与自动重入，不能只依赖已通过的协议层同 session
  harness；
- 真实设备 NAT/AP 切换/UDP 阻断/丢包/过载/rebind/重启及持续媒体/温控 soak 矩阵；
- 生产 PKI 下响应丢失恢复、证书与 enrolled Relay 轮换、停源备份恢复和多主机迁移/克隆
  演练；
- 对预览软件包和镜像的持续生产观察。

[English](RELEASE-NOTES-v3.0.8.md)
