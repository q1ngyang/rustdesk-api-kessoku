# Docker 部署参考

推荐在 `linux/amd64` 主机上使用 Docker Compose 部署 Kessoku。第一次部署请先选择：

- 已有 HBBS/HBBR：[单独部署 Kessoku](ZH-CN-Getting-Started.md)；
- 还没有 HBBS/HBBR：[Kessoku + Starry 完整部署](ZH-CN-Complete-Deployment.md)；
- 为现有中心增加 HBBR：[纯中继节点部署](ZH-CN-Relay-Only-Deployment.md)。

本页供部署后查询编排文件、目录、网络、数据库和更新命令。

## 三套编排文件

| 场景 | 文件 | 包含服务 |
| --- | --- | --- |
| 仅 Kessoku | [`docker-compose.yaml`](../../docker-compose.yaml)、[`examples/compose.env.example`](../../examples/compose.env.example) | Kessoku API、管理后台、可选内置浏览器客户端 |
| Kessoku + Starry | [`examples/combined/compose.yaml`](../../examples/combined/compose.yaml)、[`examples/combined/.env.example`](../../examples/combined/.env.example) | Kessoku、Starry HBBS、同版本 HBBR |
| 纯中继节点 | [`examples/relay/compose.yaml`](../../examples/relay/compose.yaml)、[`examples/relay/.env.example`](../../examples/relay/.env.example) | 一台独立 HBBR；默认 Starry 镜像，已注释官方替代项 |

Kessoku 可以搭配官方 `rustdesk/rustdesk-server` 的 HBBS/HBBR。联合示例推荐
`q1ngyang/rustdesk-server-starry`，并让 HBBS/HBBR 使用同一个固定 Starry 镜像版本，避免
两者独立升级后版本不一致。

## Kessoku 容器默认安全设置

仓库 Compose 默认：

- 镜像内进程使用 UID/GID `65534:65534`；
- 根文件系统只读；
- 删除全部 Linux capabilities；
- 启用 `no-new-privileges`；
- `/app/runtime` 使用临时内存文件系统；
- 只把 `/app/data` 作为应用数据持久化；
- 把配置和 `/run/secrets` 只读挂载；
- 把宿主机 `21114`、`21122` 绑定到 `127.0.0.1`。

不要为了排障删除这些限制。出现权限错误时修正宿主机属主，而不是改用 root 或
`chmod 777`。

## 宿主机目录权限

```sh
sudo install -d -m 0700 -o 65534 -g 65534 \
  /opt/rustdesk-api-kessoku/data/kessoku \
  /opt/rustdesk-api-kessoku/secrets
sudo chown 65534:65534 /opt/rustdesk-api-kessoku/secrets/*
sudo chmod 0600 /opt/rustdesk-api-kessoku/secrets/*
```

仅执行 `mkdir` 和 `chmod 0700` 不够：如果目录仍属于宿主机登录用户，容器内 UID 65534
无法进入目录。

联合部署中 Starry 的数据目录由其 root 进程管理：

```sh
sudo install -d -m 0700 -o root -g root /opt/rustdesk-stack/data/starry
```

## 挂载与备份清单

### Kessoku

| 宿主机 | 容器内 | 用途 |
| --- | --- | --- |
| `data/kessoku` | `/app/data` | SQLite 数据库 `rustdeskapi.db` |
| `secrets` | `/run/secrets:ro` | 访问令牌私钥、内部证书、数据库/LDAP 私有 CA |
| `config.yaml` | `/app/conf/config.yaml:ro` | 主配置 |
| `/app/runtime` | 临时内存文件系统 | 运行日志和临时文件，不作为持久备份 |

SQLite 用户应整体一致备份 `data/kessoku`。使用 MySQL/PostgreSQL 时，此目录仍可保存应用
运行数据，但账户数据库要用数据库厂商工具另行备份。

### Starry

Starry 单机示例把持久目录挂载到 `/root`，其中常见文件：

| 文件 | 用途 |
| --- | --- |
| `id_ed25519` | HBBS 身份私钥，严格保密 |
| `id_ed25519.pub` | 公钥，可填写到 Kessoku 和客户端 |
| `db_v2.sqlite3` | RustDesk Server 数据 |
| `mmdb/*.mmdb` | 部署者提供的地理位置数据库 |

联合示例把 `starry-config.yaml` 单独只读挂载到 HBBS。备份时要同时保存该文件和 Starry
数据目录。

## 配置文件和环境变量的分工

`.env` 适合保存镜像版本、宿主机路径、监听地址和少量标量；`config.yaml` 适合保存完整
功能结构。尤其注意：

- `web-client.relay-wss-urls` 必须留在 YAML；
- `server-control.instances` 建议留在 YAML；
- PEM 私钥和证书内容只放文件，配置中只引用路径；
- `.env` 默认不会自动把任意变量传入容器，只有 Compose `environment` 中声明的项才会
  传入；增加外部数据库密码等变量时要同时修改 Compose 映射。

环境变量覆盖规则和所有参数见
[配置参数参考](ZH-CN-Configuration-Reference.md)。

## 网络模式

### Kessoku

Kessoku 使用普通 Docker 网桥并发布两个本机端口：

```yaml
ports:
  - 127.0.0.1:21114:21114
  - 127.0.0.1:21122:21122
```

Nginx 在宿主机代理这两个地址。可选内部认证接口 `21121` 默认关闭；启用连接认证时应通过
专用 Docker 网络或受限私网访问，并使用双向 TLS，不能添加到公网 Nginx。

### Starry

联合示例的 HBBS/HBBR 使用 Linux 主机网络，以保留 NAT 和地理位置选择所需的真实对端
地址。该示例不适用于 Docker Desktop。主机网络意味着 `21115`～`21119` 直接在宿主机
监听，必须正确设置主机防火墙和云安全组。

完整端口表见[反向代理与防火墙](ZH-CN-Reverse-Proxy-and-Firewall.md)。

## 启动、停止和查看日志

仅 Kessoku：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 120 kessoku-api
```

联合部署：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 180 hbbs hbbr kessoku-api
```

持续查看日志：

```sh
docker compose --env-file .env -f compose.yaml logs -f --tail 100
```

停止但保留容器和数据：

```sh
docker compose --env-file .env -f compose.yaml stop
```

删除容器和网络不会删除绑定挂载的宿主机数据，但仍应先备份。不要使用带 `-v` 的清理命令
处理生产部署。

## 首次管理员密码

新数据库会创建 `admin`，但不会在日志中输出可复用初始密码。使用只有 UID 65534 可读的
普通文件设置密码：

```sh
docker compose --env-file .env -f compose.yaml exec kessoku-api \
  ./kessoku-api reset-admin-pwd \
  --password-file /run/secrets/bootstrap-admin-password
```

密码必须为 12～128 字节，文件必须是普通文件，且组/其他用户权限位均为零。登录后台并
再次修改密码后删除该一次性文件。

## 使用外部数据库

单机和小规模部署优先使用 SQLite。切换 MySQL/PostgreSQL 前：

1. 创建专用数据库用户和数据库；
2. 为数据库部署主机名匹配的证书；
3. 把私有 CA 只读挂载到 `/run/secrets`；
4. 修改 `gorm.type` 和对应配置；
5. 在 Compose 中安全传入数据库密码；
6. 先在测试环境完成迁移和恢复演练。

MySQL 必须 `tls: "true"`，PostgreSQL 必须 `sslmode: "verify-full"`。证书/SAN 不匹配、
CA 无法读取或使用不安全模式时，Kessoku 会拒绝启动。

## 更新镜像

生产环境在 `.env` 中固定具体版本。更新流程：

1. 阅读版本说明和数据库迁移说明；
2. 备份数据库、配置、签名密钥和 Starry 身份数据；
3. 修改一个镜像版本；
4. 执行 `docker compose config --quiet`；
5. `pull` 后 `up -d`；
6. 检查日志并逐项验证登录、地址簿、原生连接、中继和 WSS；
7. 确认稳定后再更新另一组件。

联合部署应确保 HBBS 与 HBBR 始终使用同一个 `STARRY_VERSION`。详细回退步骤见
[升级与回退](ZH-CN-Upgrade-and-Rollback.md)。

## 部署后检查

```sh
docker inspect rustdesk-api-kessoku --format '{{.Config.User}}'
docker inspect rustdesk-api-kessoku --format '{{json .HostConfig.PortBindings}}'
curl -fsS https://api.example.com/api/version
curl -fsS https://client.example.com/config/v1.json
```

预期 Kessoku 用户为 `65534:65534`，两个宿主机端口都绑定回环地址。联合部署还应确认
HBBS/HBBR 镜像完全一致，并使用两台真实客户端完成原生和中继会话。容器显示“运行中”、
HTTP 返回 200 都不能单独证明远控链路可用。
