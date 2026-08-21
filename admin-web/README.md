# Kessoku 管理前端

本目录是 `rustdesk-api-kessoku` v2.8.0 的内置 Vue 3 管理前端源码。它与 Go 后端在同一提交中审核、测试和发布，不依赖独立前端仓库或移动分支。

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
