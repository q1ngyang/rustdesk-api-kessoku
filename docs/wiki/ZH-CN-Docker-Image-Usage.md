# Docker 镜像使用

[English](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Docker-Image-Usage) | **简体中文**

Kessoku 镜像发布于
[`ghcr.io/q1ngyang/rustdesk-api-kessoku`](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)，
当前正式支持 `linux/amd64`。

## 拉取镜像

```sh
docker pull ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.2
docker image inspect ghcr.io/q1ngyang/rustdesk-api-kessoku:v3.0.2 \
  --format '{{json .RepoDigests}}'
```

生产环境使用具体版本或解析后的内容摘要，不直接跟随 `latest`。`latest` 会随新稳定版移动，
不适合作为可预测的回退点。

## 镜像内容

- Kessoku API 可执行程序；
- 内置管理后台；
- 内置浏览器远控页面；
- 运行配置模板和语言资源；
- 公共 API 端口 `21114`；
- 可选内部认证端口 `21121`；
- 独立浏览器客户端端口 `21122`。

镜像不包含 HBBS、HBBR、部署私钥、数据库密码或 TLS 证书。HBBS/HBBR 可以使用官方
RustDesk Server，推荐使用
[`q1ngyang/rustdesk-server-starry`](https://github.com/q1ngyang/rustdesk-server-starry)。

## 不建议直接 `docker run`

Kessoku 需要持久化数据、只读配置、签名密钥、端口回环绑定和安全限制。推荐使用仓库提供的
Compose：

- [已有 HBBS/HBBR 的完整教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)；
- [Kessoku + Starry 联合教程](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)；
- [Docker 部署参考](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment)。

相关文件：

- [`docker-compose.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docker-compose.yaml)
- [`examples/compose.env.example`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/compose.env.example)
- [`examples/config.docker-builtin.yaml`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/examples/config.docker-builtin.yaml)
- [`examples/combined/`](https://github.com/q1ngyang/rustdesk-api-kessoku/tree/master/examples/combined)
- [`examples/nginx/`](https://github.com/q1ngyang/rustdesk-api-kessoku/tree/master/examples/nginx)

## 更新镜像

先备份数据库、配置和签名密钥，修改 `.env` 中的固定版本，再执行：

```sh
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml pull kessoku-api
docker compose --env-file .env -f compose.yaml up -d kessoku-api
docker compose --env-file .env -f compose.yaml logs --tail 150 kessoku-api
```

升级跨越 v2/v3 时必须阅读[升级与回退](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Upgrade-and-Rollback)，不能只替换镜像后
在同一数据库上反复切换新旧版本。
