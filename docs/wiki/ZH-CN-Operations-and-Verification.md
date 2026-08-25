# 日常运维与验证

[English](Operations-and-Verification.md) | **简体中文**

本页提供部署后的日常检查、备份和完整验收清单。容器显示“运行中”、API 返回 200 或用户
登录成功都只证明一部分链路正常。

## 每日或告警触发时

查看服务状态：

```sh
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --since 24h kessoku-api
```

联合部署：

```sh
docker compose --env-file .env -f compose.yaml logs --since 24h hbbs hbbr kessoku-api
```

关注：

- 容器反复重启、配置校验失败或数据库连接失败；
- 登录失败、临时封禁和异常来源地址；
- JWT 公钥加载、证书到期、内部认证查询失败；
- Starry 配置被拒绝、中继服务器离线或 WSS 健康检查失败；
- 管理代理操作失败、回退或需要人工处理；
- 日志是否意外包含访问令牌、私钥或完整密码。

## 每周检查

```sh
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
sudo nginx -t
sudo certbot renew --dry-run
```

同时检查：

- Kessoku `21114`、`21122` 仍只绑定宿主机回环地址；
- `21118`～`21121` 未在公网安全组中开放；
- 数据目录剩余空间和备份任务结果；
- 至少一个启用的超级管理员仍可登录；
- 浏览器公开配置不含秘密；
- 使用 Starry 时 HBBS/HBBR 仍为同一固定镜像版本。

检查端口绑定：

```sh
docker inspect rustdesk-api-kessoku \
  --format '{{json .HostConfig.PortBindings}}'
docker inspect rustdesk-starry-hbbs --format '{{.Config.Image}}'
docker inspect rustdesk-starry-hbbr --format '{{.Config.Image}}'
```

## 备份

### SQLite 部署

在低写入时段停止 Kessoku，复制整个数据目录和密钥，再恢复服务，是最容易得到一致备份的
方法：

```sh
docker compose --env-file .env -f compose.yaml stop kessoku-api
# 使用你的备份工具保存 data/kessoku、secrets、.env 和配置文件
docker compose --env-file .env -f compose.yaml start kessoku-api
```

也可以使用支持 SQLite 在线备份的工具，但不能在数据库正在写入时只复制单个文件并假定
一定一致。

### MySQL/PostgreSQL

使用 `mysqldump`/物理备份或 `pg_dump`/物理备份等数据库厂商方法，同时保存 Kessoku 配置、
签名密钥和私有 CA。外部数据库不在 `/app/data` 中。

### Starry

备份整个 Starry 数据目录，至少包括：

- `id_ed25519` 与 `id_ed25519.pub`；
- `db_v2.sqlite3`；
- MMDB 文件；
- Starry YAML；
- 使用连接认证时的 `jwks.json` 与 `jwks.json.metadata.json`；
- 使用管理代理时的实例 ID、状态、配置历史和本机控制令牌。

`id_ed25519` 丢失时不要让容器生成新身份后继续运行，应停止服务并从备份恢复。

## 恢复演练

至少每季度在隔离主机上演练：

1. 恢复 Kessoku 数据库、配置和签名密钥；
2. 使用固定旧镜像启动；
3. 验证管理员和普通用户登录；
4. 验证地址簿和设备数据；
5. 恢复 Starry 身份数据并确认公钥未变化；
6. 完成原生点对点和强制中继会话；
7. 验证 WSS 与浏览器远控；
8. 记录实际恢复时间和可接受的数据丢失范围。

没有实际恢复过的备份不能视为可靠备份。

## 用户与权限检查

管理后台定期检查：

- 已离职或不再使用的账户是否禁用/删除；
- 管理员是否只拥有所需用户组、用户、公共地址簿和设备范围；
- 是否至少保留两个受控的超级管理员恢复入口；
- 用户密码重置后旧会话是否已撤销；
- OAuth/OIDC 或 LDAP 身份与本地账户绑定是否符合预期；
- 登录、连接、文件和管理操作审计是否在保留期限内。

普通用户、范围管理员和超级管理员要分别测试。只在前端隐藏菜单不代表后端权限正确，应用
应返回拒绝结果。

## 真实客户端验收矩阵

每次升级 Kessoku、Starry、Nginx、证书或认证设置后，至少记录：

| 维度 | 最少测试 |
| --- | --- |
| 客户端 | 实际使用的每个 RustDesk 版本和平台 |
| 登录 | 登录、注销、再次登录、密码重置后的旧会话 |
| 原生传输 | 点对点和强制 HBBR 中继 |
| WSS | 两端都开，以及一端开一端关的两个方向 |
| 浏览器 | 登录、VP9 画面、鼠标、基本键盘、退出 |
| 账户状态 | 禁用用户、删除用户、管理员撤销全部会话 |
| 依赖故障 | API 暂停、内部认证超时、中继离线、证书错误 |

对齐两端客户端、HBBS、HBBR 和 Kessoku 日志时使用同一连接时间戳。不要把另一场会话的
日志当作当前会话证据。

## 连接认证巡检

Starry 处于 `audit` 或 `enforce` 时检查：

- 当前模式与实际生效模式；
- 公钥数量、最近刷新时间和最大陈旧时间；
- 校验次数、允许、拒绝和 `audit_would_deny` 增量；
- 令牌状态查询请求、失败和缓存命中；
- Kessoku 与 Starry 服务器时钟；
- 当前/上一公钥的轮换重叠期。

`audit` 中存在无法解释的“本应拒绝”时不能启用 `enforce`。强制模式中内部认证故障会拒绝
新连接，不会自动放行。

## 管理代理巡检

- 平时 Kessoku `server-control.read-only` 和代理 `write_enabled` 都保持只读；
- 实例 UUID、证书和服务令牌密钥没有意外变化；
- 配置读取的摘要与 HBBS 当前生效摘要一致；
- 操作、审计、恢复和历史目录未达到容量限制；
- `21120` 仍不可从公网访问；
- 每次应用或回退后都完成真实会话，而不只看操作返回成功。

## 证书和密钥轮换

建立到期提醒，至少覆盖：

- Nginx 公网证书；
- Kessoku 内部认证服务端证书；
- Starry HBBS 客户端证书；
- 管理代理服务端和 Kessoku 客户端证书；
- Kessoku 访问令牌 Ed25519 密钥；
- 管理代理服务令牌 Ed25519 密钥。

轮换访问令牌密钥时先分发新公钥，再切换签名私钥，并保留上一公钥直至所有旧令牌与缓存
到期。不要一次同时轮换 DNS、TLS、JWT 和镜像，否则故障时难以定位。

## 支持信息的脱敏

报告问题时可以提供：

- Kessoku/Starry 镜像版本；
- 脱敏后的域名和配置结构；
- 容器状态、错误时间和相关日志行；
- 数据库类型/版本、RustDesk 客户端版本；
- Starry 当前配置代号和中继健康状态。

不得提供：

- `id_ed25519`、Kessoku/管理代理签名私钥；
- 管理员或用户密码；
- 完整访问令牌、Cookie、客户端证书私钥；
- 未脱敏的完整数据库、LDAP 密码或 OAuth 客户端密钥。
