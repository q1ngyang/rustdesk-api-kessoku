# Kessoku 管理前端

本目录是 `rustdesk-api-kessoku` v3.0.1 开发线的内置 Vue 3 管理前端源码，直接延续本仓库 v2.8.3 已整合并调整过的前端。它与 Go 后端在同一提交中审核、测试和发布，不依赖独立前端仓库、外部前端分支或 `lejianwen/rustdesk-api-web`。

v3.0.1 的界面设计、响应式行为、企业管理员权限边界和改造理由见 [UI-REDESIGN-v3.0.1.zh-CN.md](../docs/development/UI-REDESIGN-v3.0.1.zh-CN.md)。视觉部分延续既有 Vue 技术路线；权限部分新增 `user/admin/super_admin` 三层角色和服务端资源范围校验，不改变 RustDesk/Starry 安全协议。

品牌 SVG 位于 `src/assets/brand/`：icon 用于 favicon、侧栏项目标识和通用 KESSOKU 身份；完整 StarryLinks Logo 仅用于登录、STARRY 节点控制和关于界面。`light`/`dark` 文件由主题状态自动选择。

安全边界：

- 不包含旧任意 ServerCmd 页面或 `/rustdesk/sendCmd` 等调用；
- 不包含 WebClient/WebClient2 协议、启动、分享或闭源资产；
- Starry 管理仅调用固定的 `/api/admin/server-control/v1` DTO；
- 配置修改必须先验证、生成绑定 ETag 的 plan，再使用幂等键 apply；
- 不接受任意 Agent URL、命令、option、文件路径或 Docker socket。

固定构建环境为 Node.js `24.15.0`、npm `11.12.1`：

```sh
cd admin-web
npm ci
npm run lint
npm test
npm audit --omit=dev --audit-level=high
npm audit signatures
npm run build
```

仓库根目录的候选工作流会重跑上述门禁、比较两次生产构建并生成 CycloneDX SBOM。`dist/` 与 `node_modules/` 都是忽略的派生内容；发布制品只接收审计后构建出的 `dist/`，不接收本目录之外的前端输入。

本地浏览器验收只连接 `tests/mock-control-server.mjs`，不得连接生产环境。来源和许可证见 [PROVENANCE.md](PROVENANCE.md) 与 [LICENSE](LICENSE)。
