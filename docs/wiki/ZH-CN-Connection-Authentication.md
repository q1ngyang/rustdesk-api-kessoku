# 连接认证

Kessoku 可以签发可撤销的 Ed25519 访问令牌，Starry HBBS 可以在控制端发起远控或直接请求
中继时验证该令牌。这样，只有登录状态有效、账户未禁用且令牌未撤销的用户才能发起新连接。

这是高级安全功能，不是基础远控、账户登录或浏览器客户端的前置条件。第一次联合部署应
保持 Starry `connection_auth.mode: off`，先确认原生、HBBR 和 WSS 都正常。

## 三种工作模式

| Starry 模式 | 行为 | 使用阶段 |
| --- | --- | --- |
| `off` | 不校验连接令牌 | 首次部署、故障回退 |
| `audit` | 完整校验并记录“本应拒绝”的原因，但不阻断连接 | 必须经过的观察期 |
| `enforce` | 校验失败时拒绝发起连接 | 仅在观察期无误判后小范围启用 |

不要从 `off` 直接改为 `enforce`。Kessoku 登录成功也不能证明所有 RustDesk 客户端版本都会
在原生 TCP、安全 TCP 和 WSS 请求中正确携带令牌。

## 覆盖范围

Starry 对控制端发起的 `PunchHoleRequest` 和直接 `RequestRelay` 进行校验，覆盖：

- 原生 `21116/TCP`；
- `21116/TCP` 上的安全 TCP；
- WSS `/ws/id`。

UDP 不用于发起已认证的控制请求。被控端的注册和心跳也不要求用户登录令牌，因此连接认证
不会阻止无人值守设备保持在线。

## 令牌要求

Kessoku 签发的令牌使用：

- Ed25519/EdDSA 签名；
- `typ=at+jwt` 和明确的 `kid`；
- `kessoku-api`、`rustdesk-connect` 受众；
- `connect:initiate` 权限范围；
- 用户编号、唯一 JTI、认证版本和有界的签发/生效/到期时间。

注销会撤销当前 JTI；禁用用户、重置密码或“注销全部设备”会让已有令牌失效。Kessoku
数据库只保存令牌摘要和标识，不保存可再次使用的完整令牌。

## 网络结构

```text
Starry HBBS
  └─ 私有 TLS 1.3 + 客户端证书
       └─ https://kessoku.internal:21121
            ├─ GET  /api/internal/v1/auth/jwks
            └─ POST /api/internal/v1/auth/introspect

公网 api.example.com:443 不能转发到 21121
```

同机 Docker 部署可以把 Kessoku `21121` 仅发布到宿主机 `127.0.0.1`，并让使用主机网络的
Starry 通过内部名称访问。跨主机部署应使用专用私网地址和防火墙白名单。

## 1. 准备内部证书

建议由现有内部 PKI 签发。如果没有 PKI，可在测试环境创建专用 CA。需要：

| 文件 | 使用方 |
| --- | --- |
| `internal-ca.pem` | Starry 用来验证 Kessoku 服务端证书；Kessoku 用来验证 HBBS 客户端证书 |
| `kessoku-internal-server.pem`、`...-key.pem` | Kessoku `21121` 服务端身份，DNS SAN 为 `kessoku.internal` |
| `hbbs-client.pem`、`hbbs-client-key.pem` | Starry 客户端身份，URI SAN 例如 `spiffe://rustdesk.example.com/starry/hbbs` |

测试环境的 OpenSSL 3 示例：

```sh
openssl genpkey -algorithm ED25519 -out internal-ca-key.pem
openssl req -x509 -new -key internal-ca-key.pem -days 3650 \
  -subj '/CN=Kessoku Starry Internal CA' -out internal-ca.pem

openssl genpkey -algorithm ED25519 -out kessoku-internal-server-key.pem
openssl req -new -key kessoku-internal-server-key.pem \
  -subj '/CN=kessoku.internal' \
  -addext 'subjectAltName=DNS:kessoku.internal' \
  -addext 'extendedKeyUsage=serverAuth' \
  -out kessoku-internal-server.csr
openssl x509 -req -in kessoku-internal-server.csr \
  -CA internal-ca.pem -CAkey internal-ca-key.pem -CAcreateserial \
  -days 825 -copy_extensions copy -out kessoku-internal-server.pem

openssl genpkey -algorithm ED25519 -out hbbs-client-key.pem
openssl req -new -key hbbs-client-key.pem \
  -subj '/CN=Starry HBBS' \
  -addext 'subjectAltName=URI:spiffe://rustdesk.example.com/starry/hbbs' \
  -addext 'extendedKeyUsage=clientAuth' \
  -out hbbs-client.csr
openssl x509 -req -in hbbs-client.csr \
  -CA internal-ca.pem -CAkey internal-ca-key.pem -CAcreateserial \
  -days 825 -copy_extensions copy -out hbbs-client.pem
```

生产环境应离线保存 CA 私钥，签发完成后不要把 `internal-ca-key.pem` 挂载进任何容器。
Kessoku 和 Starry 只获得各自需要的证书、私钥和 CA 公钥。

## 2. 安装文件并设置权限

联合部署目录示例：

```text
/opt/rustdesk-stack/secrets/kessoku/
  internal-ca.pem
  kessoku-internal-server.pem
  kessoku-internal-server-key.pem

/opt/rustdesk-stack/starry-auth/secrets/
  internal-ca.pem
  hbbs-client.pem
  hbbs-client-key.pem

/opt/rustdesk-stack/starry-auth/cache/
  jwks.json（Starry 自动刷新后写入）
  jwks.json.metadata.json
```

Kessoku 文件由 `65534:65534` 读取；Starry HBBS 以 root 运行：

```sh
sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/rustdesk-stack/secrets/kessoku
sudo install -d -m 0700 -o root -g root \
  /opt/rustdesk-stack/starry-auth/secrets \
  /opt/rustdesk-stack/starry-auth/cache
sudo chown 65534:65534 /opt/rustdesk-stack/secrets/kessoku/*
sudo chmod 0600 /opt/rustdesk-stack/secrets/kessoku/*
sudo chown -R root:root /opt/rustdesk-stack/starry-auth
sudo chmod 0600 /opt/rustdesk-stack/starry-auth/secrets/*
```

CA 公钥可以是 `0644`，私钥保持 `0600`。

## 3. 开启 Kessoku 内部接口

在 `kessoku-config.yaml` 中设置：

```yaml
auth:
  enabled: true
  issuer: "https://api.example.com"
  audiences:
    - "kessoku-api"
    - "rustdesk-connect"
  access-token-ttl: 168h
  maximum-token-ttl: 168h
  clock-skew: 30s
  max-token-bytes: 8192
  legacy-token-read-enabled: false
  current-key:
    id: "kessoku-main-2026"
    private-key-file: "/run/secrets/kessoku-access-ed25519.pem"
  previous-keys: []
  internal:
    enabled: true
    listen: "0.0.0.0:21121"
    server-cert-file: "/run/secrets/kessoku-internal-server.pem"
    server-key-file: "/run/secrets/kessoku-internal-server-key.pem"
    client-ca-file: "/run/secrets/internal-ca.pem"
    allowed-uri-sans:
      - "spiffe://rustdesk.example.com/starry/hbbs"
    allowed-dns-sans: []
    max-body-bytes: 1048576
    request-timeout: 2s
    global-requests-per-second: 200
    per-cert-requests-per-second: 100
```

在联合 `compose.yaml` 的 `kessoku-api.ports` 增加**仅回环**映射：

```yaml
- 127.0.0.1:21121:21121
```

不得改成 `0.0.0.0:21121:21121`，也不得把它加入 Nginx 公网站点。

## 4. 给 Starry 挂载证书和缓存

在 `hbbs.volumes` 增加：

```yaml
- type: bind
  source: /opt/rustdesk-stack/starry-auth/secrets
  target: /run/secrets/starry-auth
  read_only: true
- type: bind
  source: /opt/rustdesk-stack/starry-auth/cache
  target: /var/lib/starry-auth
```

在 `hbbs` 增加内部名称：

```yaml
extra_hosts:
  - "kessoku.internal:127.0.0.1"
```

该名称必须与 Kessoku 服务端证书 DNS SAN 相同。

## 5. 先配置 `audit`

把 `starry-config.yaml` 的 `connection_auth` 改为：

```yaml
connection_auth:
  mode: audit
  issuer: https://api.example.com
  audience: rustdesk-connect
  token_use: access
  required_scope: connect:initiate
  max_token_bytes: 8192
  clock_skew_seconds: 30
  jwks:
    file: /var/lib/starry-auth/jwks.json
    url: https://kessoku.internal:21121/api/internal/v1/auth/jwks
    refresh_interval_seconds: 300
    max_stale_seconds: 3600
    ca_file: /run/secrets/starry-auth/internal-ca.pem
    cert_file: /run/secrets/starry-auth/hbbs-client.pem
    key_file: /run/secrets/starry-auth/hbbs-client-key.pem
    server_name: kessoku.internal
  introspection:
    required: true
    url: https://kessoku.internal:21121/api/internal/v1/auth/introspect
    timeout_ms: 1000
    positive_cache_seconds: 10
    negative_cache_seconds: 1
    max_cache_entries: 100000
    ca_file: /run/secrets/starry-auth/internal-ca.pem
    cert_file: /run/secrets/starry-auth/hbbs-client.pem
    key_file: /run/secrets/starry-auth/hbbs-client-key.pem
    server_name: kessoku.internal
```

先重建 Kessoku，再重启 HBBS：

```sh
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml restart hbbs
docker compose --env-file .env -f compose.yaml logs --tail 200 kessoku-api hbbs
```

验证双向 TLS：

```sh
curl --resolve kessoku.internal:21121:127.0.0.1 \
  --cacert /opt/rustdesk-stack/starry-auth/secrets/internal-ca.pem \
  --cert /opt/rustdesk-stack/starry-auth/secrets/hbbs-client.pem \
  --key /opt/rustdesk-stack/starry-auth/secrets/hbbs-client-key.pem \
  https://kessoku.internal:21121/api/internal/v1/auth/jwks
```

不带客户端证书、使用错误证书或错误名称都必须失败。检查缓存目录出现
`jwks.json` 和 `jwks.json.metadata.json`，并把两者一起纳入备份。

## 6. 观察期必须验证的情况

至少保持 `audit` 一个完整业务周期，并测试：

- 正常登录用户的原生点对点、原生中继、WSS/WSS 和两种混合 WSS 会话；
- 未登录、令牌过期、令牌格式错误、错误签名和错误受众；
- 用户注销、管理员撤销会话、禁用用户、删除用户、重置密码；
- Kessoku 暂停、内部接口超时、证书过期和 JWKS 刷新失败；
- 当前/上一签名公钥的轮换重叠期；
- 所有实际使用的 RustDesk 客户端版本和平台。

HBBS 日志中的 `audit_would_deny` 必须都有可解释原因。正常用户出现无法解释的本应拒绝、
公钥状态不是就绪、证书即将过期或令牌状态查询失败时，不得进入强制模式。

## 7. 切换到 `enforce`

只有观察期全部通过后，才把：

```yaml
connection_auth:
  mode: enforce
```

先在少量客户端和维护窗口中启用，重启 HBBS，立即验证真实原生和 WSS 会话。强制模式下
公钥未知、内部查询失败或令牌已撤销都会拒绝连接，不会自动放行。

## 回退

连接认证导致业务异常时：

1. 把 Starry 模式改回 `audit`；紧急情况下改为 `off`；
2. 重启 HBBS并确认新配置被接受；
3. 再次完成原生和中继会话；
4. 保留日志、JWKS 缓存和元数据用于排查，不要删除账户数据库或重新生成 HBBS 身份密钥。

不要通过关闭 TLS 校验、公开 `21121`、把旧公钥立即删除或为未知令牌放行来“修复”故障。
