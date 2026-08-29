# 项目元数据

[English](PROJECT-METADATA.md) | **简体中文**

本文记录发布负责人已批准的 v3.0.5 外部 GitHub 元数据；远端变更仍只能按受保护发布流程执行。

## Repository About

描述：

> Unofficial RustDesk account and enterprise administration API with a
> responsive UI, scoped administrators, Starry control, and a built-in
> Relay-only Web Client.

网站：

```text
https://github.com/q1ngyang/rustdesk-api-kessoku/wiki
```

Topics：

```text
rustdesk
rustdesk-api
self-hosted
remote-desktop
docker
golang
authentication
oidc
ldap
```

## GHCR package 页面

镜像描述：

> Kessoku RustDesk administration API with Starry control and a Relay-only
> WebClient.

发布 workflow 会设置 OCI/index title、source、release URL、documentation、version、
revision、licence 与 description annotation。documentation 指向
[`CONTAINER.md`](../docs/deployment/CONTAINER.md)，其中提供以下可见链接：

- 推荐 [Docker 部署](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Docker-Deployment)；
- [Compose 范例](../docker-compose.yaml)；
- [环境变量范例](../examples/compose.env.example)；
- [Starry 集成](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Starry-Control)；
- [内置 Web Client](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Web-Client)。

发布流程会把不可变 `v3.0.5` 与移动的 `latest` 发布为同一镜像。`latest` 指向最新成功
发布的稳定版；生产运维人员解析并固定版本 tag 的 digest。

## Wiki 发布

已审核 Wiki 源文件位于 [`docs/wiki/`](../docs/wiki)。GitHub Wiki 是独立 Git 仓库；得到
明确批准后，才能把这些文件复制到 `rustdesk-api-kessoku.wiki.git`，并作为独立发布操作
推送。`_Sidebar.md` 提供双语索引。

## Release 内容

受保护发布 workflow 已准备：

- 在 `master` 构建精确的 v3.0.5 candidate，完成发布就绪验证后才创建不可变 tag；
- 附加 Compose/环境变量范例与双语容器/发布文档；
- 发布不超过 12 行的 GitHub Release 摘要，并用 Read more 链接指向中英文详细说明；
- 把一个带 OCI provenance 与 SBOM 的 linux/amd64 GHCR 镜像同时发布为 `v3.0.5`
  与 `latest`；
- 保留打标前 commit、candidate run、受保护环境、签名、镜像仓库认证、contract、checksum、
  前端源码与发布批准的 fail-closed 检查。

文档/新特性文案和发布门禁已经批准；Repository About、Wiki、package、tag、镜像与
Release 仍须按 [`RELEASE-PROCESS.md`](../docs/releases/RELEASE-PROCESS.md) 的可审计顺序执行。
