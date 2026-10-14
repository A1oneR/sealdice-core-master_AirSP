# AirSP QQ 聊天增强（2026-10-13）

本次修改已接入当前 `sealdice-core-master_AirSP` 源码，另行生成新版 EXE，保留已有可执行文件。改动前文件保存在 `tmp/qq-chat-enhancement-20261013/before/`；修改前后源码和测试脚本另行打包，便于对照和回退。

## 功能与使用方式

- **真实群名**：选中群聊时读取群资料。官方 Bot 调用 `GET /v2/groups/{group_openid}/info`，使用返回的群名和成员人数；成功缓存十分钟，失败退避一分钟，同一适配器请求间隔不少于两秒。后台保留群名缓存，连接不可用时展示已有资料。
- **群和群友头像**：普通 QQ 群号、QQ 号使用 QQ 头像地址；官方消息实际提供 `author.avatar` 时显示并保存该图片。图片加载失败后显示原来的文字头像。
- **双击群友头像 @**：在群聊输入框光标位置插入可发送的 CQ 提及标记。普通 QQ 使用数字账号，官方 Bot 保留完整内部身份交给现有适配器转换。私聊双击不插入 @。
- **加号 → 图片资源**：浏览海豹资源管理已有的 PNG/JPG/JPEG/GIF 图片，按名称或路径搜索，选图后点击“发送图片”。图片发往打开面板时选中的会话；切换会话会关闭面板，防止误发。图片单独发送，保留文字草稿，失败后保留选中项供重试。
- **加号 → KaTeX 符号**：点击符号将公式插入草稿。公式外自动包裹 `$...$`，在已有行内公式中插入命令。官方 Bot 会自动勾选“Markdown / KaTeX”，只影响当前聊天发送，无需修改全局配置。
- **加号 → 点击填入指令**：分别编辑表面显示文字 `show` 和点击后填入内容 `text`，可勾选 `reference`。实时生成 URL 编码后的 `<qqbot-cmd-input ... />`，点击“插入到消息”放入草稿。内容和展示文字分别限制为 100 字符；最终草稿继续受聊天框的 1000 字符限制。
- **点击填入预览**：工具内预览按钮只把内层内容插入聊天输入框；聊天气泡中的同类标签也显示表面文字，点击后填入内层内容。`reference=true` 且消息有原始 ID 时，本地预览追加对应回复标记。所有这些点击均不会自动发送消息。QQ 客户端中的原生效果由官方 Markdown 消息负责。

24 个 KaTeX 快捷项：∞、π、α、β、θ、Δ、±、×、÷、≠、≈、≤、≥、→、↔、⇒、∑、∫、√x、a/b、★、♥、✓、☑。分数使用 `\dfrac{a}{b}`；快捷项没有 `\f` 开头指令，避免海豹的 FormFeed 分段转换。

此外修复了原聊天界面在群聊/私聊切换时丢失状态徽章选择器引发的空对象错误，防止较旧的列表、消息和群资料响应覆盖新会话。

## 能力边界

官方群信息文档没有群头像字段，官方群消息的作者对象也不保证返回头像。无法仅凭群 OpenID 或成员 OpenID 推导真实 QQ 群号或 QQ 账号，因此这两种情况下继续显示文字头像。已实现返回字段的接收、显示和持久化，不能声称官方群及所有群友头像已经实测可获取。

本次验证使用本地测试数据，没有向真实 QQ 群发送测试消息，也没有运行新版核心连接用户的正式机器人。真实群资料接口的权限和返回结果、QQ 客户端公式渲染及原生点击填入行为仍需部署后验证。

## 验证结果

- JS 语法检查通过；本次文件的 `git diff --check` 通过。
- Go 专项回归通过：聊天 API、头像身份边界、官方群信息请求路径和响应解析、头像事件接收、旧聊天数据库迁移和重启持久化、资源路径边界和特殊文件名、KaTeX/点击填入标签的 Markdown JSON，以及本地文件相关测试。
- Playwright + Edge 浏览器回归通过：群名/人数与头像渲染、头像失败回退、普通/官方提及、资源搜索和发送、草稿保留、失败重试、会话切换、私聊保护、旧响应保护、窄窗口布局；24 个 KaTeX 快捷项、公式内插入和 Markdown 发送参数；指令编码、预览、引用和点击后不自动发送。
- `go build -trimpath -ldflags '-s -w'` 成功；新版 EXE 的 `--version` 输出 `1.6.1-dev+AirSP`；已检查 EXE 嵌入的 QQ 聊天 JS/CSS 与最终源码一致。
- 扩大的 `go test ./api ./dice ./message -count=1` 中 API 和 message 包通过，dice 包未全部通过：部分测试未关闭聊天数据库，导致 Windows 临时目录清理遇到文件占用；另有一次扩展包目录重命名权限失败。使用本次修改前文件的 Go overlay 复查，`TestDismissMilkyOwnerRequiresConfirmationBeforeQuit` 仍发生相同数据库占用失败；扩展包重装测试在这次复查中通过，因此该次重命名错误只能记录为间歇性环境失败。没有将全套测试标为通过。

浏览器测试：`scripts/qq-chat-ui.test.cjs`，使用本机已安装的 Playwright 包和 Edge；所有接口响应均为本地测试数据。测试图保存在 `tmp/qq-chat-enhancement-20261013/`。

## 交付与部署

新版：`sealdice-core-v1613-AirSP-qq-chat-20261013.exe`。

SHA-256：`6CB5C19528E261329A7B216999FD89F91011EEEA066CC776115C0B071CF793B2`。

正式部署时停止旧核心后，将新版放入原运行目录并使用原数据目录；本次未启动正式服务或替换旧 EXE。新版内嵌最新聊天页面。如果运行目录存在 `frontend_overwrite`，核心会优先加载该目录，需同时同步本次 `qq-chat.js` 和 `qq-chat.css`。重启后刷新浏览器缓存再打开“QQ 聊天”。

旧聊天库首次打开会增加 `sender_avatar` 字段，历史消息保留；旧程序使用原有列读取仍可正常工作。

## 接口依据

- 官方群资料：`C:/Users/11209/.codex/skills/qq-official-bot-dev/references/official-docs/server-inter/group/manage/get-group-info.md`。
- 官方消息作者字段：`C:/Users/11209/.codex/skills/qq-official-bot-dev/references/official-docs/server-inter/message/send-receive/event.md`。
- 点击填入标签、URL 编码、100 字符限制和引用字段：`C:/Users/11209/.codex/skills/qq-official-bot-dev/references/official-docs/server-inter/message/trans/text-chain.md`。
- Markdown 发送：`C:/Users/11209/.codex/skills/qq-official-bot-dev/references/official-docs/server-inter/message/type/markdown.md`。
- 普通 QQ 群资料和头像：[go-cqhttp 官方 API 文档](https://docs.go-cqhttp.org/api/)。
- KaTeX 海豹约束：本仓库 `.agents/skills/qqbot-katex-ui/SKILL.md`，并已用实际 `TranslateSplit` 路径验证快捷项不会触发 FormFeed 分段。
