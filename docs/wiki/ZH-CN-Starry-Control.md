# Starry 管理

Kessoku 可以通过可选的 Starry 管理代理，在管理后台查看单台 HBBS 的状态、中继服务器和
当前配置，并在明确授权后执行有校验、可回退的配置修改。

管理代理不是 HBBS/HBBR 或账户 API 的必需组件。基础联合部署不需要它；停止管理代理也
不会停止远控服务或清除 Starry 当前生效配置。

## 可用功能

连接管理代理后，超级管理员可以：

- 查看代理能力、实例状态和连接认证状态；
- 查看已配置中继服务器及 WSS 健康状态；
- 输入两端地址和传输方式，模拟中继选择而不改变线上状态；
- 读取当前 Starry YAML 和配置结构；
- 校验候选配置并查看变更计划；
- 在写入窗口中应用配置、查询操作结果；
- 查看配置历史并发起受审计的回退；
- 请求 HBBS 重新加载已写入的配置。

它不提供 shell、任意命令、任意 URL、Docker Socket、systemd 控制、通用文件读写或跳过
版本检查的强制覆盖。

## Adaptive Relay Quality

Kessoku 只根据 Starry Control v1 capability 协商 Relay Quality。当前 adaptive 契约由
`relay_quality: 1` 标识，Relay 清单会在 `quality.protocol_version: 1` 再次报告已冻结的
wire 版本；Kessoku 不根据 Starry 或 HBBR 的版本字符串猜测支持情况。

| Starry 响应 | Kessoku 行为 |
| --- | --- |
| patch-v1.2 capability 中没有 `relay_quality` | 旧 Relay 清单继续可用，质量字段明确显示为“**不支持**”。 |
| `relay_quality: 1` 且包含协议、候选与新鲜度字段 | 校验并展示 adaptive/eager 的脱敏聚合状态。 |
| 已声明质量 capability，但缺少必填状态或数值越界 | 将其作为无效上游契约拒绝，不展示不完整数据。 |
| 新增的未知 JSON 字段 | 忽略且不从 Kessoku 强类型响应中反射出去。 |
| 已知 capability 出现尚不支持的版本 | 关闭失败，直到 Kessoku 明确支持该版本。 |

管理页面展示当前策略和协议、primary accepted 与 expansion triggered 比例、P2P 取消数、
预计节省探测次数、timeout/invalid/late 计数、固定的回退原因聚合，以及每个 Relay 明确上报
的探测/负载协议、遥测新鲜度和质量候选状态。页面不会渲染单次客户端报告、nonce、
allocation/session UUID、完整客户端 IP 或连接令牌。

配置继续完全由 schema 驱动。Kessoku 渲染 `/control/v1/config/schema` 返回的原始 JSON
Schema 与 UI Schema，不另外保存一套 Relay Quality 字段规范；本地化帮助只解释
adaptive/eager、primary samples、accept score、loss gate 和 P2P grace，最终校验仍以 Starry
为准。所有变更必须经过 `validate -> plan -> apply -> generation ACK`。冻结的 patch-v1.3
分类器保证所有 `/relay_quality` 变更至少为 `medium`；如果旧版或不符合契约的 Agent 返回
`low`，Kessoku 仍会保留防御性的 `medium` 下限，且不改变服务端签发的 plan ID 与
candidate digest。

当 Starry 明确把 Relay 遥测标为过期，或 Relay Quality 已启用但当前没有质量候选时，页面
会显示状态告警。timeout、invalid/late 报告和回退原因是累计快照；应按部署基线监控其增量
或速率，不应自行设定一个伪造的绝对门槛。告警不包含单次客户端报告或高基数 allocation
数据。

Allocation Simulation 必须明确标为 non-binding。它可以展示匹配的 GEO 规则、GEO primary、
候选顺序、传输资格和可能的 adaptive/eager 流程，但没有真实客户端探测数据，不能解释为
最终质量分、最终 Relay 选择或预测客户端 RTT。

发布包含 adaptive 支持的 Kessoku 前，必须先把完全相同的 Starry 契约发布到不可变标签，
把 `internal/starrycontrol/CONTRACT_VERSION` 从 `LOCAL_CANDIDATE_VALIDATED` 改为 `PINNED`，
记录匹配的 release digest，并重跑跨仓库与真实实例测试。移动分支或 dirty Starry 工作区不能
作为发布依据。

## 安全结构

```text
超级管理员浏览器
  └─ HTTPS -> Kessoku API
       └─ 私有 HTTPS + 双向 TLS + 短期服务令牌
            └─ Starry 管理代理 :21120
                 └─ 本机受令牌保护的内部通道 -> HBBS
```

浏览器从不直接访问管理代理。`21120` 只监听同机回环地址或受防火墙限制的私有管理地址，
不能加入公网 Nginx。

每个请求同时要求：

1. Kessoku 客户端证书由代理信任的 CA 签发，且 URI SAN 精确匹配；
2. Kessoku 使用独立 Ed25519 私钥签发最长五分钟的服务令牌；
3. 服务令牌的签发者、受众、授权方和操作权限与实例配置一致。

用户访问令牌密钥、连接认证密钥与管理代理服务令牌密钥必须分别生成，不能复用。

## 先部署只读管理代理

以 Starry 项目的官方示例为准：

- [管理代理完整指南](https://github.com/q1ngyang/rustdesk-server-starry/wiki/ZH-CN-Control-Agent)
- [Docker Compose 示例](https://github.com/q1ngyang/rustdesk-server-starry/tree/main/examples/control-agent)
- [代理配置模板](https://github.com/q1ngyang/rustdesk-server-starry/blob/main/examples/control-agent/control-agent.yaml)

首次部署保持：

```yaml
listen: 127.0.0.1:21120

config:
  write_enabled: false
```

管理代理、HBBS 和 HBBR 使用同一个固定 Starry 镜像版本。管理代理与 HBBS 共享网络命名
空间，只为访问本机内部控制通道；它不应获得 Docker Socket 或宿主机服务管理权限。

需要持久保存：

- 管理代理实例 UUID；
- 管理代理状态和操作记录；
- Kessoku 客户端 CA、代理服务端证书/私钥；
- 服务令牌公共 JWKS；
- HBBS 与代理共享的本机控制令牌；
- 受管 Starry 配置和配置历史。

## Kessoku 配置示例

把所需文件放入 Kessoku 的只读 `/run/secrets`，然后配置一个固定实例：

```yaml
server-control:
  legacy-command-enabled: false
  read-only: true
  request-timeout: 5s
  response-max-bytes: 1048576
  instances:
    - id: "starry-main"
      name: "主节点"
      enabled: true
      base-url: "https://starry-control.internal:21120"
      expected-instance-id: "替换为管理代理生成的 UUID"
      tls-server-name: "starry-control.internal"
      ca-file: "/run/secrets/starry-control-ca.pem"
      client-cert-file: "/run/secrets/kessoku-control-client.pem"
      client-key-file: "/run/secrets/kessoku-control-client-key.pem"
      control-key-file: "/run/secrets/kessoku-control-jwt-ed25519.pem"
      control-key-id: "kessoku-control-2026"
      control-issuer: "https://api.example.com"
      authorized-party: "spiffe://api.example.com/starry-control"
```

对应的代理配置必须使用同样的信任关系：

```yaml
tls:
  allowed_client_uri_sans:
    - spiffe://api.example.com/starry-control

service_jwt:
  issuer: https://api.example.com
  audience_prefix: "urn:starry-control:"
```

`expected-instance-id` 必须来自目标管理代理的持久实例文件，不能每次部署随机填写。
`authorized-party` 必须与 Kessoku 客户端证书中的 URI SAN 完全一致。

同机联合部署若让管理代理只监听 `127.0.0.1:21120`，需要为 Kessoku 容器提供一条受限的
宿主机访问路径；不要为了方便把代理改为公网 `0.0.0.0`。可以使用专用 Docker 网络，或
绑定受防火墙限制的私有地址并在证书中使用对应 DNS 名称。

## 只读验收

重建 Kessoku 后，在后台的 Starry 管理页面逐项验证：

1. 实例 UUID、版本和能力与部署一致；
2. 状态、中继服务器、配置和配置结构可读取；
3. 中继分配模拟不会改变轮换序号、健康状态或实际配置；
4. 校验错误能返回明确字段，不泄露私钥文件内容；
5. 计划、应用、回退和重新加载在只读模式下不可用；
6. 无客户端证书、错误 URI SAN、过期/错误受众的服务令牌均被拒绝；
7. 公网无法连接 `21120`。

Kessoku 的 `server-control.read-only: true` 与代理的 `config.write_enabled: false` 是两层独立
保护。首次接入应同时保持只读。

## 开启配置写入

只有在测试环境完成应用、失败自动回退和人工恢复演练后，才在维护窗口：

1. 确认受管配置文件和状态目录属于管理代理的固定 UID/GID；
2. 设置代理 `config.write_enabled: true`；
3. 重启代理并确认写入能力已公布；
4. 暂时把 Kessoku `server-control.read-only` 改为 `false`；
5. 在后台读取当前配置和 ETag；
6. 校验候选配置并查看计划；
7. 确认影响、摘要、目标实例和有效期后应用；
8. 等待操作成功，再核对磁盘配置、HBBS 当前配置代号和实际中继状态；
9. 完成真实客户端会话；
10. 变更结束后恢复 Kessoku 和管理代理只读。

管理代理使用原子替换写入配置。仅给 root 所有的文件增加组写权限通常不够，因为替换后
会产生新文件；应严格按 Starry 管理代理文档设置固定数字 UID/GID、目录属主和权限。

## 回退和故障处理

后台返回 HTTP 成功不等于 HBBS 已启用新配置。必须核对操作状态、配置摘要、HBBS 当前
配置代号和真实会话。

出现失败、磁盘与运行配置不一致或“需要人工处理”时：

1. 把 Kessoku 和代理恢复只读，必要时停止代理；
2. 保留操作、审计、恢复和配置历史目录；
3. 对比受管文件内容/属主/权限与 HBBS 当前配置摘要；
4. 恢复最后一份已验证配置，并在本机请求 HBBS 重新加载；
5. 完成真实原生和中继会话后再恢复管理代理。

不要删除代理状态目录来清除错误，也不要让管理代理重启 Docker/HBBS。HBBS 即使在代理
停止时也会继续使用最后一份有效配置。
