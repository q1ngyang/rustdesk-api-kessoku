# 升级与回退

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Upgrade-and-Rollback) | **简体中文**

Kessoku 启动时会自动升级数据库结构。跨数据库版本升级后，直接把镜像改回旧版并继续写入
可能造成权限错误或旧程序无法识别新会话，因此升级前必须准备匹配的数据库备份。

## 升级前清单

记录当前状态：

```sh
docker compose --env-file .env -f compose.yaml ps
docker inspect rustdesk-api-kessoku --format '{{.Config.Image}}'
docker inspect rustdesk-starry-hbbs --format '{{.Config.Image}}' 2>/dev/null
docker inspect rustdesk-starry-hbbr --format '{{.Config.Image}}' 2>/dev/null
```

保存：

- 当前 Kessoku 和 Starry 的完整镜像版本/内容摘要；
- `compose.yaml`、`.env`、Kessoku 和 Starry YAML；
- Kessoku 数据库或外部数据库一致性备份；
- Kessoku 当前/上一访问令牌密钥；
- Starry `id_ed25519`、数据库和 MMDB；
- 内部认证证书、JWKS 缓存及元数据；
- 管理代理实例 ID、凭据、状态和配置历史；
- Nginx 配置与 TLS 证书。

在隔离环境恢复一次备份，确认管理员和普通用户能登录。生产维护窗口开始前完成，而不是
升级失败后才第一次测试恢复。

## 普通版本更新

1. 阅读目标版本说明；
2. 保持 Starry 连接认证为 `off` 或 `audit`，管理代理保持只读；
3. 如需发现未登录 API 的客户端，先把中心 HBBS 及其 Control Agent
   升级到 Starry `1.1.16-patch-v1.2.2`，并保留数据、身份密钥、实例 ID、
   证书及服务 JWT 信任；
4. 修改 `.env` 中 Kessoku 的固定版本；
5. 检查并拉取镜像；
6. 只重建 Kessoku；
7. 验证后再更新其余 Starry 节点。

```sh
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull kessoku-api
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml logs --tail 200 kessoku-api
```

验证：

- `https://api.example.com/api/version`；
- 管理员和普通用户登录/注销；
- 地址簿、设备和权限范围；
- 原生点对点、强制中继和 WSS；
- 浏览器远控登录、画面、鼠标、键盘和退出。

更新 Starry 时 HBBS/HBBR 必须同时使用相同 `STARRY_VERSION`，但可以先重建 HBBS、完成
健康检查后再重建 HBBR。不要把 HBBR 换成独立移动版本的官方镜像。

## 从 v2 升级到 v3.0.6

v3.0.6 把数据库升级到版本 `312`，包含企业角色和管理员资源范围，并新增品牌、TOTP、
公告、GeoIP、用户界面偏好、WebClient 审计及可信设备发现字段：

- 旧 `is_admin=true` 账户迁移为 `super_admin`，避免原管理员意外失去权限；
- 普通账户迁移为 `user`；
- 新的范围管理员角色为 `admin`，只能访问明确授予的资源；
- 保留 `is_admin` 兼容列，但 v3 权限判断使用新 `role`。
- `/app/data/totp.key` 与数据库共同保护已绑定的 TOTP 密钥，必须成套备份；
- `/app/data/media` 保存头像和品牌图片，也必须纳入升级和回退集合；
- 迁移会移除不再支持的 LinuxDo OAuth 提供商及身份绑定。
- 当前有效的原生客户端会话可认领已登录设备；未登录 API 的客户端只有在 Starry
  精确确认 ID 与机器 UUID 后才会进入设备管理。客户端启动、信息变化或超过 24 小时
  未完整上报时会刷新清单，并同步同一 ID 的地址簿设备信息。

升级步骤：

1. 停止旧 Kessoku 写入并创建完整数据库备份；
2. 对恢复副本先执行升级，处理 OAuth/OIDC 身份重复或空字段；
3. 生产环境启动 v3.0.6，检查日志中的数据库迁移；
4. 确认至少一个启用的 `super_admin`；
5. 分别测试普通用户、范围管理员和超级管理员；
6. 测试范围管理员只能看到获授的用户组、用户、公共地址簿和设备；
7. 确认角色/范围变更会撤销旧管理会话；
8. 再启用新的 Ed25519 认证、浏览器客户端或 Starry 高级集成。

详细预检和数据库查询见
[`MIGRATION-v3.0.6.zh-CN.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.6/MIGRATION-v3.0.6.zh-CN.md)及
[`MIGRATION.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/MIGRATION.md)。身份冲突必须由管理员明确决定合并或解绑，不能只为
通过唯一索引而随意删除记录。

## 推荐的高级功能启用顺序

升级 API 后不要一次打开所有新功能：

1. 保持浏览器客户端关闭、Starry 认证 `off`、管理代理只读；
2. 验证数据库和桌面客户端账户功能；
3. 启用 Ed25519 访问令牌并验证登录/注销；
4. 部署独立浏览器域名和 WSS，启用浏览器客户端；
5. 部署 Kessoku 内部双向 TLS 接口；
6. Starry 进入 `audit`，覆盖一个完整业务周期；
7. 只读接入管理代理；
8. 小范围启用 `enforce`；
9. 只在单独维护窗口开放管理代理写入。

每次只改变一层，失败时才能明确回退哪一项。

## 何时可以只回退镜像

如果目标版本没有改变数据库结构或凭据格式，并且版本说明明确支持原地回退，可以恢复
上一固定镜像后重建容器。即便如此，也要先备份当前数据库并验证登录与远控。

## v3 回退到 v2 的重要警告

不要让 v2 与 v3 同时写同一数据库。版本 312 仍为兼容保留 `is_admin=true`，v2 可能把 v3
的范围管理员当成无限制管理员。v3 新签发的令牌只在数据库保存摘要，旧程序也无法恢复或
认证这些新会话。

首选回退方法：

1. 停止 Kessoku；
2. 把 Starry 从 `enforce` 改回 `audit` 或 `off`；
3. 恢复升级前的完整数据库备份；
4. 恢复与该备份匹配的旧配置和旧镜像；
5. 启动旧版并要求升级后创建/登录的用户重新登录；
6. 验证管理员权限、普通用户、地址簿和真实远控会话。

版本 312 不能通过删表或降低版本号原地回退。必须恢复升级前匹配的数据库、TOTP 密钥、
媒体目录、配置、签名密钥与旧镜像；保留失败数据库副本用于排查。

## 联合部署的有序回退

1. Starry 连接认证从 `enforce` 降到 `audit`；
2. Kessoku 与管理代理恢复只读；
3. 浏览器客户端故障时设置 `web-client.mode: disabled`，不影响桌面客户端；
4. 恢复 Starry 上一固定版本和上一份已验证配置；
5. 验证 HBBS 与 HBBR 使用相同镜像版本；
6. 恢复匹配的 Kessoku 程序/数据库/密钥；
7. 完成登录、点对点、中继和 WSS 会话；
8. 故障原因明确前不要重新启用强制认证或管理写入。

不要通过删除 `data/`、重新生成 `id_ed25519`、删除管理代理状态或关闭证书验证来回退。

## 回退后验证

- Kessoku 容器稳定运行，数据库版本与程序匹配；
- `admin`/超级管理员可登录，普通用户权限正确；
- 升级后签发的令牌已失效或用户已重新登录；
- Starry 当前配置与磁盘配置一致；
- HBBS/HBBR 身份公钥没有变化；
- API、原生 TCP/UDP、HBBR、WSS 和浏览器客户端均按预期工作；
- 防火墙和 Nginx 没有因临时排障留下公开后端端口。

更详细的应急流程见 [`ROLLBACK-RUNBOOK.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/operations/ROLLBACK-RUNBOOK.md)。
