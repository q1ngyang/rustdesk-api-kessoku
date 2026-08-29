# 文档目录

[English](README.md) | **简体中文**

部署和日常使用请优先阅读
[在线 Wiki](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Home)。
本目录按用途收纳使用指南、运维手册、发布历史、安全设计和开发参考资料。

## 快速入口

- [已有 HBBS/HBBR，单独部署 Kessoku](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Getting-Started)
- [从零部署 Kessoku + Starry](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Complete-Deployment)
- [增加纯中继 HBBR 节点](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Relay-Only-Deployment)
- [Nginx 与防火墙](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Reverse-Proxy-and-Firewall)
- [容器镜像说明](deployment/CONTAINER.zh-CN.md)
- [浏览器客户端配置与限制](deployment/WEB-CLIENT.zh-CN.md)

## 分类收纳

| 目录 | 内容 |
| --- | --- |
| [wiki/](wiki/) | GitHub Wiki 的中英文源文件 |
| [deployment/](deployment/) | 容器部署和浏览器客户端参考 |
| [operations/](operations/) | 运维、恢复和故障回退手册 |
| [security/](security/) | 安全模型与信任边界 |
| [releases/](releases/) | 发布流程、检查清单和迁移历史 |
| [releases/v3.0.4/](releases/v3.0.4/) | 当前 v3.0.4 发布说明和数据库迁移指南 |
| [releases/v3.0.3/](releases/v3.0.3/) | 上一个受支持的 v3.0.3 发布文档 |
| [releases/v3.0.2/](releases/v3.0.2/) | 发布失败且未公开的 v3.0.2 历史记录 |
| [releases/v3.0.1/](releases/v3.0.1/) | 已撤回 v3.0.1 的历史发布文档 |
| [releases/v2.8.3/](releases/v2.8.3/) | v2.8.3 历史发布说明 |
| [development/](development/) | 界面设计、协议细节、来源记录和文档维护说明 |
| [api/](api/) 与 [admin/](admin/) | 自动生成的 API 文档；路径与 Go 导入有关，不随手工文档一起移动 |

可运行的部署模板继续保留在 [examples/](../examples/)。组件 README、许可证与构建依赖的
来源说明随组件保存。根目录 README 只作为项目入口，其余专题文档统一放入本目录。

移动文档或发布 Wiki 前，请阅读[文档维护说明](development/DOCUMENTATION.md)。
