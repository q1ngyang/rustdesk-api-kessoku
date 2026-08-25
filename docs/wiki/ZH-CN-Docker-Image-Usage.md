# Docker 镜像使用

[English](Docker-Image-Usage.md) | **简体中文**

完整的版本化 package 页面指南见
[`CONTAINER.zh-CN.md`](../../CONTAINER.zh-CN.md)，其中说明镜像内容、不可变 tag、Compose、
secret、端口、验收与回滚。

快速链接：

- [GHCR package](https://github.com/q1ngyang/rustdesk-api-kessoku/pkgs/container/rustdesk-api-kessoku)
- [Docker 部署](ZH-CN-Docker-Deployment.md)
- [Compose 范例](../../docker-compose.yaml)
- [环境变量范例](../../examples/compose.env.example)
- [Caddy HTTPS 范例](../../examples/Caddyfile.example)
- [v3.0.0 发布说明](../../RELEASE-NOTES-v3.0.0.zh-CN.md)

受支持镜像平台为 `linux/amd64`。发布流程会把不可变 `v3.0.0` 与移动的 `latest` 推送为
同一镜像。生产部署应检查并固定版本 tag 的 digest；只有在明确跟随最新稳定版且已准备
回滚时才使用 `latest`。
