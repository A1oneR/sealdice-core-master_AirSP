# SealDice Core AirSP

基于 SealDice Core v1.6.1 的定制版本，主要用于 AirSP 的 LogAI、角色卡分析、跑团统计和 QQ 官方 Bot 工作流。

本目录 `sealdice-core-master_AirSP` 是后续核心源码的唯一维护位置。旧的 `sealdice-core-master161` 及带编号的下载目录只作为历史参考，不再同步修改。

## 项目定位

- 上游项目：[sealdice/sealdice-core](https://github.com/sealdice/sealdice-core)
- 当前版本：`1.6.1-dev+AirSP`
- 核心语言：Go 1.25.0
- 前端：Vue 3 / Element Plus，已包含可供核心嵌入的 `static/frontend`
- 默认定制脚本：`data/default/scripts/QQ官方Bot昵称桥接.js`
- 当前推荐构建产物：`sealdice-core-v161-officialqq.exe`

本分支不只是替换前端或打包插件。QQ 官方 Bot 的身份、文件、分页、角色卡归属等兼容逻辑已经进入 Go 核心，并由 JS 插件和 LogAI 后端共同使用。

## 已完成的工作

### QQ 官方 Bot 核心适配

#### 文件接收

- 接收 QQ 官方 Bot 群聊、群聊 At 消息和 C2C 消息中的附件。
- 将附件统一转换为 SealDice 的文件消息，保留文件名、大小、下载地址和媒体类型。
- 普通文件会触发插件的文件上传处理流程，使 LogAI、角色卡分析器等插件能够读取官方 Bot 上传的日志、DOCX 和表格。
- 对事件顶层附件与消息体附件进行兼容，避免 SDK 版本差异导致附件为空。

#### 富媒体发送

- 支持群聊与 C2C 图片上传，按照 QQ 场景分别获取对应的 `file_info`。
- 本地文件和非公网地址可以转为 Base64 上传；公网地址仍可让 QQ 平台主动拉取。
- 支持普通文本与 Markdown 输出模式。
- 多图片结果按页缓存，每页只发送一张富媒体图片，并提供上一页、下一页及文字翻页兼容。
- 富媒体消息和分页控制按 QQ API 的消息类型限制拆分，避免把不兼容字段塞入同一个 `msg_type=7` 请求。

#### 上传超时保护

- 运行时 OpenAPI 超时由 3 秒提高到 10 秒。
- 获取富媒体 `file_info` 时最多尝试 3 次，退避时间为 250ms、750ms。
- 仅重试请求超时、临时网络错误、限流、服务端错误及 QQ 文档指定的富媒体错误码。
- 最终消息发送不自动重试，避免一次分析结果被重复发送。

#### 身份规范化与迁移

- 区分机器人 UIN、用户 OpenID、群 OpenID、群成员 OpenID 和频道身份。
- 官方 Bot 连接使用稳定的规范化 ID，避免同一用户在私聊和群聊中被错误拆成无关身份。
- 新增官方身份到旧 QQ/QQ群身份的别名解析，原生属性、角色卡和日志查询会优先使用绑定后的旧身份。
- 提供可选的数据迁移流程，可迁移玩家、群、角色卡属性、日志、黑名单等旧数据。
- 迁移失败不会丢弃已经解析出的机器人身份，下次连接仍可重试。
- AppSecret、AccessToken 和原始 OpenID 不应出现在业务日志、图片或公开文档中。

### QQ/QQ群绑定插件

默认脚本 `data/default/scripts/QQ官方Bot昵称桥接.js` 提供以下能力：

- `.QQ绑定 <原QQ号>`：绑定官方 Bot 身份与旧 QQ 身份。
- `.QQ绑定确认`：在可用的原场景中完成确认。
- `.QQ绑定验证`：原 Bot 不可用时，通过角色卡名称选择题验证旧身份。
- `.QQ群绑定 <原QQ群号>`：绑定官方群与旧 QQ 群。
- `.QQ群绑定确认`：在可用的原群场景中完成确认。
- `.QQ群绑定验证`：通过旧群的历史 Log 名称选择题验证群归属。
- 新用户或新群可以选择“没有可用角色卡/历史 Log”，按新身份建立绑定。
- 验证题加入大量不同语言、符号和异常格式的干扰项，并有错误冷却时间。
- 用户绑定可以跨官方群识别；每个官方群仍保存自己的群场景映射。
- 插件恢复绑定后，会向核心注册身份别名，并尝试继承唯一的原生角色卡绑定。
- 暴露 `SealOfficialQQIdentityBridge` 接口，其他插件可查询绑定状态、要求先绑定或解析兼容身份。

依赖旧 QQ 数据的 LogAI、角色卡分析和活跃度插件，在官方 Bot 环境中会先检查绑定状态。未绑定时只给出绑定提示，不会用 OpenID 新建一套孤立数据。

### 原生角色卡兼容

- 身份别名参与原生 `AttrsManager`、角色上下文和玩家属性所有者解析。
- 官方 Bot 用户绑定旧 QQ 后，`.pc list` 可以读取旧身份下的原生角色卡。
- 支持在只有一张候选卡时继承群内角色卡绑定，避免二次绑定后只看到空列表。
- JS 运行时增加角色卡名、历史 Log 名和角色绑定继承接口，供验证插件安全调用。
- 角色卡导入通过 SealCore 原生 `chNew/chBindCur` 角色 API 建卡、绑定，并调用原生 `.st` 录入属性；插件不维护另一套检定属性。

### LogAI 日志分析

相关文件位于项目上级目录：

- `../人工智障Log分析器2_5.js`
- `../logai_server/logai_server2_12.py`
- `../logai_server/token_stats.py`
- `../人工智障系列/logai_server2_13.py`

已完成能力：

- `.logai 链接1 链接2 ...` 按参数顺序下载并拼接多个日志，单次最多 20 个链接。
- 支持群文件和 QQ 官方 Bot 附件直链分析。
- 支持 TXT、DOCX、表格及常见日志导出格式；DOCX 会提取段落和表格文字，不再得到全零统计。
- 后端确定性计算 RP 字数、场外占比、骰点次数、平均值、中位数、可识别成功率和幸运值。
- LLM 只负责识别真正参与的 PC，并排除骰娘、KP/GM/DM、NPC、系统账号、围观者及只有场外发言的 OB。
- 第一页生成玩家统计表和六维雷达图：创新、武力、交涉、幸运、扮演、认真。
- 雷达图使用半透明填充；低于 60 分的图形使用非线性收缩，但显示的原始分数不变。
- 提示词要求使用精确整数评分，避免机械地批量输出整 5 或整 10 分。
- 自定义提示词保留原始换行和 Markdown 表格，并兼容主题标签与 `【分页符】` 的顺序冲突。
- 默认配置和自定义配置都保留玩家识别、后端统计注入与分页能力。
- 支持前文回顾、备用模型、自定义配置和多主题图片输出。

#### Token 统计

- 记录 Log 分析器、模组分析器、角色卡分析器等模块的输入、输出与总 Token。
- 分别生成用户排行图片和模块排行/高频提示词图片。
- 用户榜使用绑定后的真实 QQ 昵称，不使用角色卡名或群昵称。
- 输出图片不显示 QQ 号、OpenID 或群 OpenID。
- 模型未返回 usage 时允许估算，并在账本中标记为估算值。

### 角色卡评分与一键导入

相关文件：`../logai_server/角色卡评分器.js` 和 `../logai_server/logai_server2_12.py`。

- 裸 `.角色卡 导入` 会自动判断 COC 或 DND；也可显式指定 `coc` / `dnd`。
- COC 与 DND 都优先命中工作表名中包含“导入”的工作表。
- 导入前检查当前群房规；COC/DND 不匹配时提示切换规则，并且不创建或修改角色卡。
- 清洗单元格和属性文本，避免说明词、公式残片及无关标签进入 `.st`。
- 通过 QQ 绑定后的用户身份和精确角色名匹配卡库，避免“薇薇”误命中“薇薇安”等前缀角色。
- 属性与技能写入原生 `.st`，导入结果可在原生 `.pc list` 中查看并用于原生判定。
- 记录武器、弹匣、道具和 DND 法术，供后续指令联动。
- `.角色卡 删除 <姓名>` 与 `.角色卡 清空 确认` 只清理 LogAI 卡库及装备记录，不会偷偷删除原生 `.pc` 角色卡。

#### COC 武器联动

- `.武器` / `.道具` 查看当前导入角色的装备、弹匣与物品。
- `.射击 <武器> [次数]` 和武器名 Alias 调用 SealCore 原生 `ra/rc` 判定。
- 双发、三连发使用连续投掷并合并输出。
- 手枪或半自动武器连射时，每发附加规则要求的惩罚骰。
- 全自动按武器对应技能决定每组上限，后续弹幕逐步增加惩罚或难度。
- 玩家声明的优势骰会逐次与规则惩罚骰抵消。
- 按实际发射子弹数扣减弹匣；弹药不足时停止后续射击并提示换弹。
- `.换弹 <武器>` 将导入记录中的弹匣补满。

#### DND 法术联动

- 识别法术环阶、戏法、描述中的伤害骰和需要敌方豁免的法术。
- 有环法术调用 SealCore 原生 `.cast` 消耗对应环位，戏法不消耗环位。
- 敌方豁免只给出提示，不替敌方自动投骰。
- 支持 `.法术位 1环 +1` 一类临时环位补充。
- `.长休` 通过原生规则恢复 HP 和全部法术位。

### 跑团活跃度计时器

相关文件：`../跑团活跃度计时器.js`。

- 跟随 `log on/new` 开始、`log off/end` 结算跑团活动。
- 统计玩家发言量、活跃时段、峰值字数、最长空档等指标。
- 每次结算附带本次骰点次数、平均值、中位数和可识别成功率。
- 成功率只统计能够从 SealCore 原生判定输出中确认结果的骰点，未知结果不会猜测。
- 支持生成图片；后端不可用时自动回退为文字。
- QQ 官方 Bot 场景要求先绑定旧 QQ 身份，避免跨群统计被 OpenID 拆散。

## 核心配置

| 配置项 | 默认值 | 用途 |
| --- | ---: | --- |
| `officialQQFileSendBase64` | `false` | 强制将官方 Bot 发送的本地/非公网文件转为 Base64 |
| `officialQQUseMarkdown` | `false` | 将兼容的官方 Bot 文本消息转为 Markdown |
| `officialQQEnableIdentityMigration` | `false` | 启动旧官方身份数据迁移；确认备份后再开启 |

这些配置已经接入核心配置 API。前端静态资源需要与本目录版本同步，避免后端已支持但设置页仍使用旧字段。

## 目录说明

| 路径 | 说明 |
| --- | --- |
| `dice/platform_adapter_official_qq.go` | QQ 官方 Bot 收发、文件、分页、Markdown、限流和富媒体重试 |
| `dice/official_qq_identity_alias.go` | 官方身份到旧 QQ/QQ群身份的运行时别名 |
| `dice/official_qq_identity_migration.go` | 可选的旧身份数据迁移 |
| `dice/dice_attrs_manager.go` | 原生角色卡绑定继承与身份解析 |
| `dice/dice_jsvm.go` | 提供给绑定插件的核心 JS 接口 |
| `data/default/scripts/QQ官方Bot昵称桥接.js` | QQ/QQ群验证、绑定和昵称同步 |
| `static/frontend` | 编译时嵌入核心的前端静态资源 |
| `sealdice-core-v161-officialqq.exe` | 当前推荐的压缩构建产物 |

## 构建与验证

项目使用 Go 1.25.0。Windows 下若系统临时目录没有写权限，可使用项目内缓存：

```powershell
$buildTmp = (Resolve-Path '.\.gotmp').Path
$env:TEMP = $buildTmp
$env:TMP = $buildTmp
$env:GOCACHE = (Resolve-Path '.\.gocache').Path
$env:GOTMPDIR = $buildTmp

go test ./dice -count=1
go build -buildvcs=false -trimpath -ldflags '-s -w' `
  -o sealdice-core-v161-officialqq.exe .
```

可选的 UPX 压缩：

```powershell
upx --best --lzma sealdice-core-v161-officialqq.exe
upx -t sealdice-core-v161-officialqq.exe
```

最近一次完整验证：

- `go test ./dice -count=1` 通过。
- 构建版本：`1.6.1-dev+AirSP`。
- UPX 校验通过。
- 当前可执行文件大小：`21,253,120` 字节。
- SHA-256：`40667A5E9602692EC27C8BECAD3BBAB9A0BA266A373076D5519C721B6D039657`。

## 后续维护约定

1. 所有 SealDice Core 源码和前端迁移只修改 `sealdice-core-master_AirSP`。
2. `E:/COC` 与 `E:/DND` 中的角色卡只能作为只读测试样本，不得修改或覆盖。
3. LogAI 的自定义提示词、自定义配置和前端配置入口必须保留，不能用硬编码默认提示词替代。
4. 角色卡属性和检定必须优先调用 SealCore 原生 `.pc`、`.st`、`ra/rc`、`.cast` 等能力。
5. QQ 官方 Bot 的 C2C、群聊、频道和群成员 OpenID 不可混用，也不可在排行榜或公开日志中输出。
6. 修改官方 Bot 发送逻辑时，富媒体上传和最终消息发送必须分开考虑；只有 `srv_send_msg=false` 的文件准备阶段适合自动重试。
7. 修改核心后至少运行相关定向测试；身份、角色卡、文件或平台适配改动需要运行完整 `go test ./dice -count=1`。
8. 发布前使用 `-trimpath -ldflags '-s -w'` 构建，并对压缩后的 EXE 执行 `upx -t`。

## 已知边界

- QQ 官方 Bot 不会直接提供旧 QQ 号，旧数据访问依赖用户/群绑定或显式身份迁移。
- 平台上游长时间不可用时，富媒体在 3 次尝试后仍会失败；此时应保留错误并让用户稍后重试，不能无限重发。
- QQ 富媒体消息的字段组合受平台限制，多图片分析结果使用分页发送，而不是把多张图片强塞进一个消息体。
- LogAI 后端的确定性统计取决于日志能否解析出发言者和骰点格式；无法确认成功与否的骰点只计次数，不计入成功率分母。
- LogAI 卡库删除和 SealCore 原生 `.pc` 删除是两套明确分离的操作。

## 上游文档与许可

- [SealDice 使用手册](https://sealdice.github.io/sealdice-manual-next/)
- [SealDice Core](https://github.com/sealdice/sealdice-core)
- [SealDice UI](https://github.com/sealdice/sealdice-ui)
- [SealDice 构建仓库](https://github.com/sealdice/sealdice-build)

除另有说明外，本项目沿用上游 MIT License。AirSP 定制内容也应保留对应文件中的作者、版本和许可证声明。
