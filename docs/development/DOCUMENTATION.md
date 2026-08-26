# 文档维护与 Wiki 发布

使用文档入口见[文档目录](../README.zh-CN.md)。本页供维护文档和发布脚本时参考。

## 链接规则

- `docs/wiki/` 内的页间导航使用完整的 `https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/页面名`
  地址，末尾不加 `.md`。这同样适用于侧边栏和语言切换。
- Wiki 引用主仓库文件时使用完整的 `blob/master/路径` 地址；目录使用 `tree/master/路径`。
  独立 Wiki 仓库无法解析主仓库的 `../../examples/...` 等相对路径。
- 不使用 `raw.githubusercontent.com/wiki/...` 作为阅读入口。代码块中下载 YAML/配置的
  `raw.githubusercontent.com` 命令不属于导航链接，不应改成网页地址。
- `docs/` 的非 Wiki 文档可使用相对链接；迁移文件时必须按新目录重新计算链接。
- 组件许可证、来源记录和生成 API 文档具有构建依赖，不为整理目录而随意移动。

## 本地检查

从仓库根目录运行：

```sh
python3 scripts/check_docs.py
python3 -m unittest discover -s scripts -p 'test_documentation.py'
git diff --check
```

检查覆盖中英文配对、本地文件、Wiki 页名、主仓库文件 URL，以及不应出现在 Wiki 中的
相对链接和原始文件导航。发布说明使用 `scripts/render_release_notes.py`，从源文件实际
目录解析相对链接，生成固定到发布标签的网页链接，不再依赖根目录文件名。

## 发布 Wiki

GitHub Wiki 是独立 Git 仓库。先取得发布授权，提交并推送主仓库，使示例文件和文档新路径
已在线可用；再克隆最新 Wiki，把 `docs/wiki/` 页面复制到 Wiki 仓库根目录。不要修改页面
文件名，不要删除不属于本次源文件的额外 Wiki 内容，也不要在发布时临时重写链接。

比对修改、检查提交身份为 `q1ngyang`，确认没有无关变更后提交并推送 Wiki。最后检查：

1. Wiki 远端提交与预期一致；
2. 页面和侧边栏的实际 HTML `href` 指向无 `.md` 后缀的 Wiki URL；
3. 示例配置指向主仓库 `blob` 页面，目录指向 `tree` 页面；
4. 中英文页面、纯中继指南、Nginx/防火墙和升级入口均可访问。

只查看原始 Markdown 内容不足以验证发布后的导航行为。
