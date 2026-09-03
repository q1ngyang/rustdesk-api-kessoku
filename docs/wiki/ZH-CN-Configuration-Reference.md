# 配置参数参考

Kessoku 从 YAML 文件读取配置。Docker 示例把宿主机的 `config.yaml` 只读挂载到
`/app/conf/config.yaml`。完整模板见 [`conf/config.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/conf/config.yaml)，可直接部署的
浏览器客户端模板见
[`examples/config.docker-builtin.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/config.docker-builtin.yaml)，联合部署模板见
[`examples/combined/kessoku-config.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/combined/kessoku-config.yaml)。

## 配置规则

- 时间使用 Go 时长格式，例如 `30s`、`15m`、`168h`。
- `RUSTDESK_API_` 前缀的环境变量可以覆盖 YAML 标量；层级、点号和连字符转换为下划线。
  例如 `auth.internal.request-timeout` 对应
  `RUSTDESK_API_AUTH_INTERNAL_REQUEST_TIMEOUT`。
- 复杂列表、`server-control.instances` 和 `web-client.relay-wss-urls` 建议保留在 YAML 中。
  中继映射的键常含点号和端口，不能假设环境变量能无损表达。
- 私钥只写文件路径，不把 PEM、访问令牌或证书内容直接写进 YAML、`.env` 或日志。
- 修改配置后需要重建或重启容器；Kessoku 不会自动热加载此文件。

推荐先检查：

```sh
docker compose --env-file .env -f compose.yaml config
docker compose --env-file .env -f compose.yaml config --quiet
```

Compose 检查只能发现编排和变量语法问题，不能证明域名、公钥、证书或 Kessoku 配置值正确。

## 最小必填项

推荐的 SQLite + 内置浏览器客户端部署至少需要正确设置：

| 参数 | 要求 |
| --- | --- |
| `gin.api-addr` | 容器内通常为 `0.0.0.0:21114` |
| `rustdesk.id-server` | HBBS 公网地址，通常包含 `:21116` |
| `rustdesk.relay-server` | HBBR 公网地址，通常包含 `:21117` |
| `rustdesk.api-server` | Kessoku 公网 HTTPS 地址 |
| `rustdesk.key` 或 `key-file` | HBBS 的 `id_ed25519.pub` 公钥，二选一 |
| `auth.enabled` | 内置浏览器客户端必须为 `true` |
| `auth.issuer` | 精确的 Kessoku HTTPS 地址 |
| `auth.audiences` | 同时包含 `kessoku-api`、`rustdesk-connect` |
| `auth.current-key.id` | 当前签名密钥的唯一标识 |
| `auth.current-key.private-key-file` | Ed25519 PKCS#8 私钥路径 |
| `web-client.*` | 启用 `builtin` 时必须按下文完整填写 |

## `lang` 与 `app`

| 参数 | 示例/默认 | 说明 |
| --- | --- | --- |
| `lang` | `zh-CN` | 服务默认语言；资源中还包含 `en`、`zh-TW` 等语言 |
| `app.web-client` | `0` | 已废弃的兼容键，必须为 `0`；浏览器客户端改用根级 `web-client.mode` |
| `app.register` | `false` | 是否允许用户从登录页自行注册 |
| `app.register-status` | `1` | 新注册用户状态：`1` 启用，`2` 禁用/待审核 |
| `app.captcha-threshold` | `3` | 同一地址连续失败多少次后要求验证码；小于 `0` 关闭，`0` 始终要求 |
| `app.ban-threshold` | `10`（推荐） | 失败多少次后临时封禁；`0` 关闭封禁。应大于验证码阈值 |
| `app.show-swagger` | `0` | `1` 开放 API/管理接口调试页；公网生产环境保持 `0` |
| `app.token-expire` | `168h` | 数据库会话到期时间；使用新认证方案时建议与 `auth.access-token-ttl` 一致 |
| `app.web-sso` | `true` | 允许已登录管理后台复用网页登录状态 |
| `app.disable-pwd-login` | `false` | 禁用密码登录；必须先验证至少一个 OAuth/OIDC 登录源，避免锁死管理员 |

登录失败计数窗口为 10 分钟，封禁时间为 30 分钟。反向代理场景只有正确配置
`gin.trust-proxy` 才能按真实客户端地址限速。

## `media` 与 `two-factor`

`media.directory` 默认是 `./data/media`，用于保存品牌图片和用户头像。服务端会重新命名
文件，只接受经过验证的 PNG、JPEG、WebP，并按 `media.max-image-bytes`（默认 1 MiB）限制
大小。该目录必须位于持久化的 `/app/data` 中。

`two-factor.enabled` 默认开启。Kessoku 首次启动时会在
`two-factor.key-file`（默认 `./data/totp.key`）生成权限为 `0600` 的 32 字节密钥，并用
AES-GCM 加密数据库中的 TOTP 密钥。必须把它与数据库一起备份；丢失该文件后，已经启用
双重验证的账户将无法校验。`issuer` 会显示在身份验证器中，`challenge-ttl` 允许 1～10
分钟。启用或停用双重验证都会撤销该用户的全部现有会话。

## `admin`

| 参数 | 示例 | 说明 |
| --- | --- | --- |
| `admin.title` | `Kessoku 管理后台` | 管理页面标题 |
| `admin.hello` | 空 | 登录后的欢迎内容；支持 `{{username}}` |
| `admin.hello-file` | `./conf/admin/hello.html` | 欢迎内容文件；可读且非空时优先于 `hello` |
| `admin.id-server-port` | `21116` | 管理页面显示/兼容用途的 HBBS 端口，零值会使用 `21116` |
| `admin.relay-server-port` | `21117` | 管理页面显示/兼容用途的 HBBR 端口，零值会使用 `21117` |

## `gin`

| 参数 | 推荐值 | 说明 |
| --- | --- | --- |
| `gin.api-addr` | `0.0.0.0:21114`（容器内） | API 和管理后台监听地址；Compose 在宿主机只绑定 `127.0.0.1` |
| `gin.mode` | `release` | `release`、`debug` 或 `test`；生产使用 `release` |
| `gin.resources-path` | `resources` | 镜像内管理前端、浏览器客户端和语言资源目录 |
| `gin.trust-proxy` | 空或精确代理地址 | 逗号分隔；空值不信任转发地址。禁止使用 `0.0.0.0/0`、`::/0` |
| `gin.admin-addr` | 不设置 | 当前版本管理后台与 API 共用 `api-addr`；该保留字段不创建第二个服务 |

## `gorm`、`mysql` 与 `postgresql`

### 通用连接设置

| 参数 | 推荐值 | 说明 |
| --- | --- | --- |
| `gorm.type` | `sqlite` | `sqlite`、`mysql` 或 `postgresql` |
| `gorm.max-idle-conns` | `10` | 最大空闲连接数 |
| `gorm.max-open-conns` | `100` | 最大打开连接数；应结合数据库限制调整 |

SQLite 文件固定为 `/app/data/rustdeskapi.db`，没有单独的路径参数。单机部署必须持久化
整个 `/app/data`。

### MySQL

| 参数 | 要求 |
| --- | --- |
| `mysql.username` | 数据库用户 |
| `mysql.password` | 数据库密码；不要提交到 Git |
| `mysql.addr` | `mysql.example.internal:3306`，主机名必须匹配证书 SAN |
| `mysql.dbname` | 数据库名 |
| `mysql.tls` | 必须是字符串 `"true"`，不支持明文或跳过验证 |
| `mysql.ca-file` | 私有 CA 文件路径；公共 CA 时可留空使用系统信任库 |

```yaml
gorm:
  type: mysql
mysql:
  username: "kessoku"
  password: "从部署密钥系统注入"
  addr: "mysql.example.internal:3306"
  dbname: "kessoku"
  tls: "true"
  ca-file: "/run/secrets/mysql-ca.pem"
```

### PostgreSQL

| 参数 | 要求 |
| --- | --- |
| `postgresql.host` | 数据库 DNS 名称，必须匹配证书 SAN |
| `postgresql.port` | 通常 `5432` |
| `postgresql.user`、`password` | 数据库凭据 |
| `postgresql.dbname` | 数据库名 |
| `postgresql.sslmode` | 必须为 `verify-full` |
| `postgresql.ssl-root-cert` | 私有 CA 路径；公共 CA 时可留空 |
| `postgresql.time-zone` | 例如 `Asia/Shanghai` |

```yaml
gorm:
  type: postgresql
postgresql:
  host: "postgres.example.internal"
  port: "5432"
  user: "kessoku"
  password: "从部署密钥系统注入"
  dbname: "kessoku"
  sslmode: "verify-full"
  ssl-root-cert: "/run/secrets/postgres-ca.pem"
  time-zone: "Asia/Shanghai"
```

外部数据库升级前要使用数据库厂商的一致性备份工具。仅备份 Kessoku 容器目录不包含外部
数据库数据。

## `rustdesk`

| 参数 | 示例 | 说明 |
| --- | --- | --- |
| `rustdesk.id-server` | `rustdesk.example.com:21116` | 返回给客户端的 HBBS 地址 |
| `rustdesk.relay-server` | `rustdesk.example.com:21117` | 返回给客户端的 HBBR 地址；Starry 动态选择场景客户端中继栏仍应留空 |
| `rustdesk.api-server` | `https://api.example.com` | 返回给客户端并用于 OAuth 回调的公网 API 地址 |
| `rustdesk.key` | 公钥单行内容 | HBBS 的 `id_ed25519.pub`；设置后优先使用 |
| `rustdesk.key-file` | `/run/secrets/id_ed25519.pub` | 公钥文件路径；仅在 `key` 为空时读取 |
| `rustdesk.personal` | `1` | `1` 向兼容客户端提供个人地址簿；其他值关闭该入口 |

公钥可以公开，私钥 `id_ed25519` 绝不能挂载给 Kessoku 或复制到客户端。

## `logger`

| 参数 | 推荐值 | 说明 |
| --- | --- | --- |
| `logger.path` | `./runtime/log.txt` | Docker 示例把 `/app/runtime` 放在临时内存文件系统；长期日志由容器日志驱动收集 |
| `logger.level` | `info` | `trace`、`debug`、`info`、`warn`、`error`、`fatal` |
| `logger.report-caller` | `false` | 是否记录源码调用位置；排障时可临时开启 |
| `logger.max-size-mb` | `20` | 单个日志文件达到该大小后轮转，最大 `1024` |
| `logger.max-backups` | `5` | 最多保留的轮转文件数量，最大 `100` |
| `logger.max-age-days` | `14` | 轮转文件最长保留天数，最大 `3650` |
| `logger.compress` | `true` | 压缩历史轮转文件 |
| `logger.local-time` | `true` | 轮转文件名使用服务器本地时间 |

示例还将 Docker `json-file` 输出限制为 5 个、每个 20 MiB。不要用 `debug` 长期开启生产日志，也不要把访问令牌或完整配置粘贴到故障报告中。

## `auth`：访问令牌和内部认证接口

### 访问令牌

| 参数 | 推荐/限制 | 说明 |
| --- | --- | --- |
| `auth.enabled` | 新部署为 `true` | 启用 Ed25519/EdDSA 访问令牌；浏览器客户端必需 |
| `auth.issuer` | `https://api.example.com` | 必须是绝对 HTTPS 地址，不含凭据、查询或片段 |
| `auth.audiences` | `kessoku-api`、`rustdesk-connect` | 两项都必须存在且不能重复 |
| `auth.access-token-ttl` | `168h` | 普通登录令牌有效期，不得超过最大值 |
| `auth.maximum-token-ttl` | `168h` | 所有签发令牌的上限 |
| `auth.clock-skew` | `30s` | 时钟容差，范围 0～5 分钟；各服务器仍需 NTP 同步 |
| `auth.max-token-bytes` | `8192` | 最大 8192 字节 |
| `auth.legacy-token-read-enabled` | 新部署 `false` | 仅旧版不透明令牌迁移窗口使用；迁移后关闭 |
| `auth.current-key.id` | 例如 `kessoku-main-2026` | 当前密钥标识，最多 128 个可打印 ASCII 字符 |
| `auth.current-key.private-key-file` | `/run/secrets/kessoku-access-ed25519.pem` | Ed25519 PKCS#8 私钥文件 |
| `auth.previous-keys[].id` | 上一密钥标识 | 轮换重叠期使用 |
| `auth.previous-keys[].public-key-file` | 上一密钥公钥文件 | 只能配置公钥，不再保留旧私钥 |

生成当前密钥：

```sh
openssl genpkey -algorithm ED25519 -out kessoku-access-ed25519.pem
```

轮换时先让验证方取得新公钥，再切换当前签名密钥；保留上一公钥的时间至少覆盖最长令牌
有效期和验证缓存窗口。`current-key` 使用私钥文件，`previous-keys` 使用公钥文件，不能颠倒。

### `auth.internal`

该接口只供 Starry 查询公钥和令牌撤销状态，不是公网 API。

| 参数 | 推荐/限制 |
| --- | --- |
| `enabled` | 默认 `false`；连接认证接入时才开启，且要求 `auth.enabled: true` |
| `listen` | 容器互联时为明确私网地址和 `21121`；不能省略主机部分 |
| `server-cert-file`、`server-key-file` | Kessoku 内部服务端证书和私钥 |
| `client-ca-file` | 只信任签发 Starry 客户端证书的 CA |
| `allowed-uri-sans` | 允许的精确客户端证书 URI SAN 列表 |
| `allowed-dns-sans` | 可替代/补充的精确 DNS SAN；两类列表至少一个非空 |
| `max-body-bytes` | 默认/最大 `1048576` |
| `request-timeout` | 默认 `2s`，最大 `10s` |
| `global-requests-per-second` | 默认 `200`，不能为负数 |
| `per-cert-requests-per-second` | 默认 `100`，不能为负数 |

启用后提供：

```text
GET  /api/internal/v1/auth/jwks
POST /api/internal/v1/auth/introspect
```

必须使用 TLS 1.3、双向证书校验和精确 SAN；不要经过公网 `api.example.com` 代理。完整流程
见[连接认证](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Connection-Authentication)。

## `web-client`

| 参数 | 要求 |
| --- | --- |
| `web-client.mode` | `disabled` 或 `builtin` |
| `listen` | 容器内 `0.0.0.0:21122`；宿主机 Compose 仍只绑定 `127.0.0.1` |
| `public-origin` | 独立浏览器站点，例如 `https://client.example.com`；不带路径 |
| `api-origin` | API 站点，例如 `https://api.example.com`；必须与上一项不同 |
| `rendezvous-wss-url` | 精确 `wss://.../ws/id`，不能带查询或片段 |
| `relay-wss-urls` | 1～64 个“HBBS 返回的中继名称 → 精确 `/ws/relay` WSS 地址”映射 |
| `server-public-key` | base64 编码的 32 字节 Ed25519 HBBS 公钥，即 `id_ed25519.pub` 内容 |
| `profile-generation` | 正整数；端点、映射或公钥变化时递增 |
| `connection-token-ttl` | 默认 `15m`，最大 `1h`，且不超过 `auth.maximum-token-ttl` |

启用 `builtin` 时 `auth.enabled` 必须为 `true`。两个站点来源必须为规范的小写 HTTPS 地址，
默认 443 端口不要显式写出。中继映射示例：

```yaml
web-client:
  mode: builtin
  listen: "0.0.0.0:21122"
  public-origin: "https://client.example.com"
  api-origin: "https://api.example.com"
  rendezvous-wss-url: "wss://rustdesk.example.com/ws/id"
  relay-wss-urls:
    "rustdesk.example.com:21117": "wss://rustdesk.example.com/ws/relay"
  server-public-key: "BASE64_PUBLIC_KEY"
  profile-generation: 1
  connection-token-ttl: 15m
```

## `server-control`

该部分连接可选的 Starry 管理代理。未部署管理代理时保持 `instances: []`。

| 参数 | 推荐/限制 |
| --- | --- |
| `legacy-command-enabled` | `false`；旧通用命令接口已移除，不能用它恢复 shell |
| `read-only` | 首次接入为 `true`；只有经过演练的变更窗口才关闭 |
| `request-timeout` | 默认 `5s`，最大 `30s` |
| `response-max-bytes` | 默认 `1048576`，最大 `4194304` |
| `log-directory` | 可选的日志根目录；配置外部日志来源时必须是绝对、只由部署者控制的目录 |
| `log-sources` | 日志来源白名单；每项包含唯一 `id`、`label`、组件、可选实例 ID 和简单文件名 |
| `registry-directory` | 独立 SP1 registry；容器默认 `/app/data/server-control`，systemd 默认 `/var/lib/kessoku-api/data/server-control` |
| `host-identity-file` | 启用配对时必填；systemd 使用 `/etc/machine-id`，容器使用单独挂载的宿主机 `/run/kessoku-host-machine-id` |
| `pairing.enabled` | 默认 `false`；只有 Broker TLS pin 和精确 Agent allowlist 配置完成后才启用 |
| `pairing.broker-origin` | 对外 Broker 的精确 HTTPS origin，不能含凭据、路径、查询或片段 |
| `pairing.broker-spki-sha256` | Broker 证书 SPKI 的小写 `sha256:` 摘要 |
| `pairing.code-ttl` | 默认 `10m`，最大一小时 |
| `pairing.recovery-ttl` | 响应丢失后的同公钥恢复窗口，默认/最大 `10m` |
| `pairing.agent-origins[]` | 部署预批准的 `id`、显示 `name`、精确 HTTPS `origin` 和 `tls-server-name`；浏览器只能选 ID |

registry 使用独立 schema-v1 SQLite，不升级主业务数据库。完整目录必须持久化，目录权限为
`0700`、文件为 `0600`，且不能使用符号链接。容器和 systemd 的路径、备份恢复、身份克隆
接管与显式 purge 见 [v3.0.8 迁移指南](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/releases/v3.0.8/MIGRATION-v3.0.8.zh-CN.md)。
`host-identity-file` 的原文不会存储或返回，只保存其哈希。容器不能使用镜像内置 machine-id，
否则无法识别 registry 被复制到另一台宿主机；官方 Compose 会单独只读挂载宿主 machine-id。

每个 `instances[]`：

| 参数 | 说明 |
| --- | --- |
| `id` | 部署内唯一的小写标识，以字母/数字开头，可含 `._-` |
| `name` | 后台显示名称 |
| `enabled` | 是否连接该实例；关闭时可以先保留配置 |
| `base-url` | 管理代理的绝对 HTTPS 地址，不含路径/查询/片段 |
| `expected-instance-id` | 管理代理首次初始化产生的固定 UUID |
| `tls-server-name` | 服务端证书中预期的 DNS 名称 |
| `ca-file` | Kessoku 信任的管理代理 CA |
| `client-cert-file`、`client-key-file` | Kessoku 的双向 TLS 客户端身份 |
| `control-key-file` | 独立的 Ed25519 服务令牌签名私钥；不能复用用户访问令牌密钥 |
| `control-key-id` | 服务令牌密钥标识 |
| `control-issuer` | 服务令牌签发者 HTTPS 地址 |
| `authorized-party` | 必须等于 Kessoku 客户端证书的 URI SAN |

所有凭据值都是文件路径。完整示例和上线顺序见[Starry 管理](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control)。

`logger.path` 指向普通文件时，Kessoku 自身日志会自动成为日志来源，无需重复填写。
Starry、Relay 与 Control Agent 通常输出到容器标准输出，Kessoku 不会访问 Docker Socket；
部署者需要让现有日志采集器把所需来源写为普通文本文件，挂载到同一只读目录，再逐项加入
`log-sources`。日志组件只允许 `kessoku`、`starry`、`relay`、`control-agent`，文件名不能
包含路径或目录穿越。例如：

```yaml
server-control:
  log-directory: "/app/deployment-logs"
  log-sources:
    - { id: "starry-center", label: "Starry Center", component: "starry", instance-id: "starry-main", file: "starry.log" }
    - { id: "relay-osaka", label: "Relay · Osaka", component: "relay", instance-id: "starry-main", file: "relay-osaka.log" }
    - { id: "control-agent", label: "Control Agent", component: "control-agent", instance-id: "starry-main", file: "control-agent.log" }
```

如果上述目录与 `logger.path` 不同，Kessoku 自身日志不会被隐式加入；需要让 Kessoku 日志
也落在该目录并显式声明。后台只读取有大小和行数上限的最新窗口，并在展示、导出前脱敏
常见认证头、访问/lease/连接/control token、route lease、密码、nonce、allocation/session
标识、会话 Cookie、客户端密钥、私钥内容及完整 IPv4/IPv6 地址；长期留存仍应交给部署日志
系统。开启控制写入后可以调整当前 Kessoku 进程的日志级别；Starry patch v1.2 尚无安全
的运行时调级接口，仍需在维护窗口修改部署的 `RUST_LOG` 并重启。

## 后台平台设置

超级管理员在 `/dash/` 的“系统管理”中维护公告、IP 信息、登录有效期和数据保留策略。
网页与官方客户端登录时长可以分别缩短，但不能超过部署配置
`auth.maximum-token-ttl`。用户 Token 只会在会话过期或撤销后开始计算保留期，不会清理
仍有效的会话；连接、文件、登录和 Starry 控制审计按创建时间分批清理。GeoIP 只接受
公网 HTTPS MMDB 地址；单文件下载上限 128 MiB，验证成功后原子替换，可设置 1～2160
小时的自动更新周期。默认 City/ASN 数据来自 P3TERX GeoLite 镜像。这些设置保存在数据库
中，不属于品牌配置或 YAML；下载文件位于持久化 `/app/data` 下的 `geoip` 目录，应与
数据库一起备份。

## `ldap`

| 参数 | 要求 |
| --- | --- |
| `ldap.enable` | 默认 `false` |
| `url` | 必须是 `ldaps://`，不能含用户名、密码、查询或额外路径 |
| `tls-ca-file` | 私有 LDAP CA 文件；公共 CA 可留空 |
| `tls-verify` | 启用 LDAP 时必须为 `true` |
| `base-dn` | 搜索根 DN |
| `bind-dn`、`bind-password` | 最小权限查询账户；密码不要提交到 Git |
| `user.base-dn` | 用户搜索根，可比全局根更窄 |
| `user.filter` | 合法 LDAP 过滤器，例如 `(cn=*)` |
| `user.username` | 用户名属性，OpenLDAP 常用 `uid`，AD 常用 `sAMAccountName` |
| `user.email` | 邮箱属性，常用 `mail` |
| `user.first-name`、`last-name` | 常用 `givenName`、`sn` |
| `user.enable-attr`、`enable-attr-value` | 可选启用状态属性；不用时留空 |
| `user.sync` | 是否在登录时同步用户资料 |
| `user.admin-group` | 管理员组的完整 DN |
| `user.allow-group` | 允许登录组的完整 DN |

启用前先用普通用户和管理员各验证一次，并保留本地超级管理员作为身份系统故障时的恢复
入口。若同时设置 `app.disable-pwd-login: true`，更要先完成回退演练。

## OAuth/OIDC

OAuth/OIDC 提供方在管理后台的“登录方式”中创建，而不是写死在 YAML。支持 GitHub、
GitHub、Google 和通用 OIDC。回调基于 `rustdesk.api-server`，通常为：

```text
https://api.example.com/api/oidc/callback
```

提供方地址必须使用 HTTPS，且会拒绝回环、链路本地和其他不安全目标。`proxy.enable` 必须
保持 `false`，因为当前版本拒绝通过通用代理访问身份提供方。先保留密码登录，确认回调、
用户绑定、管理员登录和故障回退都正常后，才考虑禁用密码登录。

## `proxy`、`redis`、`cache` 与 `oss`

| 配置段 | 当前建议 |
| --- | --- |
| `proxy.enable`、`proxy.host` | `enable` 必须为 `false`；启用会在启动校验时被拒绝 |
| `redis.addr/password/db` | 保留的 Redis 客户端参数，当前主要功能不依赖；入门部署不设置 |
| `cache.type` | 可识别 `file`、`redis`；当前业务没有必须依赖该全局缓存的路径，保持未设置 |
| `cache.redis-addr/redis-pwd/redis-db/file-dir` | 仅与上项配套；不要误把数据库替换为缓存 |
| `oss.*` | 保留的对象存储上传策略参数；当前管理路由未启用文件上传入口，入门部署不配置 |

这些保留项不是“打开全部功能”的必要条件。不要为了填满配置模板而部署无用的 Redis 或
对象存储。

## 已删除、拒绝或不应使用的配置

- `app.web-client` 非零会拒绝启动；使用根级 `web-client.mode`。
- 根级 `web-client-provider` 已删除，出现该段或同名环境变量会拒绝启动。
- 旧 `jwt.*` 对称签名配置不属于当前认证方案。
- `proxy.enable: true` 会拒绝启动。
- `server-control.legacy-command-enabled` 不会恢复任意命令执行能力。
- MySQL 明文/跳过验证、PostgreSQL 非 `verify-full`、LDAP 非 `ldaps` 或关闭证书验证都会被
  拒绝。
- 不要在 YAML 中保存私钥正文、访问令牌、数据库备份或完整证书内容。
