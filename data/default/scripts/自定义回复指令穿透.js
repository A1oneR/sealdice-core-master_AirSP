// ==UserScript==
// @name         自定义回复指令穿透
// @author       local
// @version      1.0.0
// @description  自动识别海豹全部自定义回复规则，在收到带指令前导符（特别是QQ官方Bot指令面板点击发出的 /xxx）时，忽视指令判定并正常输出自定义回复
// @timestamp    2026-09-28
// @license      MIT
// @sealVersion  1.6.0
// ==/UserScript==

(() => {
  const NAME = '自定义回复指令穿透';
  const AUTHOR = 'local';
  const VERSION = '1.0.0';

  let ext = seal.ext.find(NAME);
  if (!ext) {
    ext = seal.ext.new(NAME, AUTHOR, VERSION);
    seal.ext.register(ext);
  }

  // 1. 注册配置项
  seal.ext.registerBoolConfig(
    ext,
    '启用指令穿透',
    true,
    '是否开启自定义回复指令穿透功能'
  );

  seal.ext.registerBoolConfig(
    ext,
    '仅对斜杠生效',
    false,
    '若开启，则仅对以 / 开头的指令（如QQ官方指令面板点击）进行穿透；若关闭，则对所有指令前导符均生效'
  );

  seal.ext.registerTemplateConfig(
    ext,
    '穿透白名单',
    [],
    '可选。若填写，则仅允许白名单内的词条被穿透（留空表示全量自动穿透所有已启用的自定义回复）'
  );

  seal.ext.registerBoolConfig(
    ext,
    '输出调试日志',
    false,
    '是否在后台控制台打印穿透匹配的详细调试信息'
  );

  // 统计数据
  let stats = {
    totalIntercepts: 0,
    matchedCount: 0,
    lastMatchedTime: 0,
    lastMatchedKeyword: ''
  };

  // 辅助获取配置
  function getConfigBool(key, fallback) {
    try {
      return seal.ext.getBoolConfig(ext, key);
    } catch (e) {
      return fallback;
    }
  }

  function getConfigTemplate(key, fallback) {
    try {
      const val = seal.ext.getTemplateConfig(ext, key);
      return Array.isArray(val) ? val : fallback;
    } catch (e) {
      return fallback;
    }
  }

  // 提取剥离先导符后的干净指令内容
  function extractCleanText(cmdArgs) {
    if (!cmdArgs) return '';
    let cmd = String(cmdArgs.command || '').trim();
    let rawArgs = String(cmdArgs.cleanArgs || cmdArgs.rawArgs || '').trim();
    if (!cmd) return '';
    if (rawArgs) {
      return `${cmd} ${rawArgs}`;
    }
    return cmd;
  }

  // 检查是否命中白名单限制
  function isAllowedByWhitelist(keyword) {
    const list = getConfigTemplate('穿透白名单', []).map(s => String(s).trim()).filter(Boolean);
    if (!list || list.length === 0) return true; // 未设白名单则全放行
    return list.some(item => keyword.toLowerCase().includes(item.toLowerCase()) || item.toLowerCase().includes(keyword.toLowerCase()));
  }

  // 核心钩子：onCommandOverride
  // 海豹在 TriggerCommand 中优先调用此函数。若返回 true，海豹将标记指令已完成并中止后续未知指令报错！
  ext.onCommandOverride = (ctx, msg, cmdArgs) => {
    if (!getConfigBool('启用指令穿透', true)) {
      return false;
    }

    stats.totalIntercepts++;

    // 检查是否限定仅对 / 生效
    if (getConfigBool('仅对斜杠生效', false)) {
      const rawText = String((msg && msg.message) || (cmdArgs && cmdArgs.rawText) || '').trim();
      if (!rawText.startsWith('/')) {
        return false;
      }
    }

    // 获取剥离指令前导符后的文本，例如把 "/今日运势" 提取为 "今日运势"
    const cleanText = extractCleanText(cmdArgs);
    if (!cleanText) {
      return false;
    }

    // 检查白名单
    if (!isAllowedByWhitelist(cleanText)) {
      return false;
    }

    const debugMode = getConfigBool('输出调试日志', false);

    // 模式 A：若环境原生支持 seal.reply API（推荐的高效模式）
    if (typeof seal.reply !== 'undefined' && typeof seal.reply.dispatch === 'function') {
      try {
        // 先检查是否能命中自定义回复
        let isMatch = false;
        if (typeof seal.reply.matches === 'function') {
          isMatch = seal.reply.matches(ctx, msg, cleanText);
        } else {
          // 若无 matches 函数则直接尝试 dispatch
          isMatch = true;
        }

        if (isMatch) {
          // 执行自定义回复输出！
          const executed = seal.reply.dispatch(ctx, msg, cleanText);
          if (executed) {
            stats.matchedCount++;
            stats.lastMatchedTime = Date.now();
            stats.lastMatchedKeyword = cleanText;
            if (debugMode) {
              console.log(`[自定义回复指令穿透] 成功忽视指令并触发自定义回复: "${cleanText}" (原消息: "${msg.message}")`);
            }
            return true; // 告知海豹此指令已被成功解决！
          }
        }
      } catch (err) {
        console.error(`[自定义回复指令穿透] dispatch 处理异常: ${err.message || err}`);
      }
    }

    // 模式 B：兜底尝试（若未开启 seal.reply 原生支持）
    return false;
  };

  // 管理指令：.回复穿透
  const cmd = seal.ext.newCmdItemInfo();
  cmd.name = '回复穿透';
  cmd.help = [
    '自定义回复指令穿透管理：',
    '.回复穿透 status          查看运行状态及穿透统计',
    '.回复穿透 list            列出当前识别到的自定义回复规则',
    '.回复穿透 test <文本>     测试指定文本是否能命中自定义回复'
  ].join('\n');

  cmd.solve = (ctx, msg, cmdArgs) => {
    const ret = seal.ext.newCmdExecuteResult(true);
    const subCmd = String(cmdArgs.getArgN(1) || '').trim().toLowerCase();

    if (!subCmd || subCmd === 'help' || subCmd === '帮助') {
      ret.showHelp = true;
      return ret;
    }

    if (subCmd === 'status' || subCmd === '状态') {
      const enabled = getConfigBool('启用指令穿透', true);
      const onlySlash = getConfigBool('仅对斜杠生效', false);
      const whitelist = getConfigTemplate('穿透白名单', []);
      const hasNativeApi = typeof seal.reply !== 'undefined' && typeof seal.reply.dispatch === 'function';

      let rulesCount = '未知';
      if (hasNativeApi && typeof seal.reply.getRules === 'function') {
        try {
          const rules = seal.reply.getRules();
          rulesCount = Array.isArray(rules) ? rules.length : '0';
        } catch (e) {}
      }

      const lastTimeStr = stats.lastMatchedTime
        ? new Date(stats.lastMatchedTime).toLocaleTimeString()
        : '暂无';

      const replyMsg = [
        '【自定义回复指令穿透运行状态】',
        `• 核心支持状态: ${hasNativeApi ? '✅ 完整原生支持 (seal.reply)' : '⚠️ 降级运行中'}`,
        `• 插件运行开关: ${enabled ? '已启用' : '已关闭'}`,
        `• 范围限制模式: ${onlySlash ? '仅限斜杠 /（面板点击专用）' : '所有前导符均生效'}`,
        `• 白名单规则数: ${whitelist.length ? `${whitelist.length} 条` : '全量自动穿透'}`,
        `• 已识别规则数: ${rulesCount} 条自定义回复`,
        `• 累计拦截判定: ${stats.totalIntercepts} 次`,
        `• 成功穿透输出: ${stats.matchedCount} 次`,
        `• 最近穿透词条: ${stats.lastMatchedKeyword || '无'} (${lastTimeStr})`
      ].join('\n');

      seal.replyToSender(ctx, msg, replyMsg);
      return ret;
    }

    if (subCmd === 'list' || subCmd === '列表') {
      if (typeof seal.reply === 'undefined' || typeof seal.reply.getRules !== 'function') {
        seal.replyToSender(ctx, msg, '当前海豹核心版本暂未导出 seal.reply.getRules() 规则检索接口。');
        return ret;
      }

      try {
        const rules = seal.reply.getRules();
        if (!rules || !rules.length) {
          seal.replyToSender(ctx, msg, '当前未检测到任何已启用的自定义回复规则。');
          return ret;
        }

        const previewList = rules.slice(0, 15).map((r, i) => {
          const condStrs = (r.conditions || []).map(c => {
            if (c.matchType) return `[${c.matchType}] ${c.value}`;
            return `[${c.condType || 'cond'}] ${c.value || ''}`;
          }).join(' 且 ');
          return `${i + 1}. 来自《${r.filename || '自定义回复'}》: ${condStrs || '无具体条件'}`;
        });

        const total = rules.length;
        const extraNote = total > 15 ? `\n... 更多 ${total - 15} 条规则已省略` : '';
        seal.replyToSender(ctx, msg, `已识别的自定义回复规则（展示前15/${total}条）：\n${previewList.join('\n')}${extraNote}`);
      } catch (err) {
        seal.replyToSender(ctx, msg, `获取规则列表失败: ${err.message || err}`);
      }
      return ret;
    }

    if (subCmd === 'test' || subCmd === '测试') {
      const testText = cmdArgs.args.slice(1).join(' ').trim();
      if (!testText) {
        seal.replyToSender(ctx, msg, '用法：.回复穿透 test <测试文本>');
        return ret;
      }

      if (typeof seal.reply === 'undefined' || typeof seal.reply.matches !== 'function') {
        seal.replyToSender(ctx, msg, '当前环境未就绪 seal.reply 预检接口。');
        return ret;
      }

      const matched = seal.reply.matches(ctx, msg, testText);
      seal.replyToSender(ctx, msg, `文本测试：“${testText}”\n匹配结果: ${matched ? '✅ 命中自定义回复规则！可被成功穿透' : '❌ 未命中任何自定义回复规则'}`);
      return ret;
    }

    seal.replyToSender(ctx, msg, '未知子命令，发送 .回复穿透 查看帮助。');
    return ret;
  };

  ext.cmdMap['回复穿透'] = cmd;
})();
