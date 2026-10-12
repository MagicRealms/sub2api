# 用户使用教程

入口：`/guide`，未登录可阅读；用户侧边栏与默认/简洁首页均提供入口。登录后沿用控制台布局。

内容面向当前站点的 OpenAI 分组：准备额度与密钥、Cherry Studio、Codex（含 CC Switch）、OpenCode、流式 Responses API 示例、完成验证、常见错误。

- API 地址从公开设置 `api_base_url` 读取，未设置时使用当前页面 origin；保留部署路径前缀，统一处理末尾 `/v1`，避免重复路径。
- 示例模型为当前站点近期使用的 `gpt-6.1-sol`。页面明确提示实际权限以密钥分组和模型目录为准。
- 教程只包含 `YOUR_API_KEY` 占位符，不读取用户密钥、不自动发起模型调用。Windows 使用 PowerShell，macOS/Linux 使用 bash/zsh。
- Codex 自定义 provider 使用 Responses、`SUB2API_API_KEY` 环境变量；OpenCode 使用 `@ai-sdk/openai` 的 Responses 协议，避免将 Chat Completions 当作 Responses。
- 无需后端接口或数据库迁移，不改已有充值、分组和网关配置。

## 内容参考

2026-10-12 查阅以下公开教程的入门组织方式，以本站实际接入流程重新撰写；未复制对方页面源码、图片或整篇正文：

- [Sub2API 客户端接入教程](https://page.easytokencc.cn/codex-guide.html)
- [Sub2API 开发工具接入文档](https://sub2.h4rvey.com/docs)

页面顶部提供 [AI工程师-DIV 的 CC Switch 视频教程](https://www.bilibili.com/video/BV15PVb6gE9k/)外部链接，新窗口打开并标注作者。视频覆盖供应商配置、客户端切换及 Codex；页面提示使用本站的 API 地址、密钥和模型，并沿用 Codex 章节中的官方工具下载入口。

2026-10-12 检索时，Bilibili 公开页面索引快照显示约 25.1 万播放、7,518 赞、1.7 万收藏，作为选择依据；网页及公开 API 的实时抓取返回 HTTP 412，未核实实时统计或评论口碑。页面不展示易过期的互动数字，不嵌入播放器或加载第三方视频资源。

配置字段和客户端行为核对以下项目的官方文档，页面底部保留官方资料入口：

- [Codex CLI](https://developers.openai.com/codex/cli/)
- [Codex 自定义 provider](https://developers.openai.com/codex/config-advanced/)
- [Cherry Studio 模型服务设置](https://www.cherryai.com/docs/en/pre-basic/settings/providers/)
- [OpenCode providers](https://opencode.ai/docs/providers/)
- [CC Switch](https://github.com/farion1231/cc-switch)

## 验证

检查中英翻译键、首页/侧边栏/路由回归、类型与生产构建。浏览器检查匿名及登录入口、桌面/手机/暗色主题、中英切换、客户端与操作系统切换、展开手动配置、复制地址/代码/测试问题及长代码局部滚动。教程本身不消耗模型额度。
