// Standalone browser regression test; all QQ/API responses are local fixtures.
const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');

(async () => {
  const root = path.resolve(__dirname, '..');
  const image = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jJawAAAAASUVORK5CYII=', 'base64');
  const sent = [], errors = [];
  let failSend = false;
  let delayedProfile = false;
  const groups = [
    { groupId: 'QQ-Group:123456', groupName: '旧群名', avatarUrl: '/invalid-avatar', memberNum: 3, active: true },
    { groupId: 'OpenQQ-Group:fixture-group', groupName: '官 Bot 群', isOfficial: true, active: true },
  ];
  const resources = [
    { type: 'image', name: '测试图片.png', path: 'data/images/测试图片.png' },
    { type: 'image', name: '另一个.gif', path: 'data/images/另一个.gif' },
  ];
  const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const json = value => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(value)); };
    if (url.pathname === '/') {
      res.setHeader('Content-Type', 'text/html; charset=utf-8');
      res.end('<!doctype html><html><head><link rel="stylesheet" href="/qq-chat.css"><style>body{margin:0}.main-container{height:100vh!important}</style></head><body><div id="app"><div class="main-container"></div></div><script>localStorage.setItem("t","fixture-token")</script><script src="/qq-chat.js"></script></body></html>');
    } else if (url.pathname === '/qq-chat.js' || url.pathname === '/qq-chat.css') {
      res.setHeader('Content-Type', url.pathname.endsWith('.js') ? 'text/javascript' : 'text/css');
      res.end(fs.readFileSync(path.join(root, 'static/frontend/assets', url.pathname.slice(1))));
    } else if (url.pathname === '/sd-api/chat/groups') {
      const items = url.searchParams.get('sessionType') === 'private' ? [{ groupId: 'QQ:555555', groupName: '私聊用户' }] : groups;
      json({ items, total: items.length, page: 1, totalPages: 1 });
    } else if (url.pathname === '/sd-api/chat/profile') {
      const id = url.searchParams.get('groupId');
      if (delayedProfile && id === groups[0].groupId) await new Promise(r => setTimeout(r, 350));
      json({ groupName: id === groups[0].groupId ? '真实测试群名' : '官 Bot 群', memberNum: 42, avatarUrl: `http://127.0.0.1:${server.address().port}/avatar.png` });
    } else if (url.pathname === '/sd-api/chat/messages') {
      const id = url.searchParams.get('groupId') || url.searchParams.get('targetId') || url.searchParams.get('sessionId');
      json({ messages: [
        { id: 'm1', senderId: id?.startsWith('OpenQQ') ? 'OpenQQ:fixture-member' : 'QQ:888888', senderName: '<群友>', senderAvatar: `http://127.0.0.1:${server.address().port}/avatar.png`, content: '测试消息', timestamp: 1000 },
        { id: 'm2', senderId: 'QQ:999999', senderName: '失败头像', senderAvatar: `http://127.0.0.1:${server.address().port}/missing.png`, content: '头像失败时保留文字', timestamp: 1001 },
        { id: 'm3', senderId: 'QQ:777777', senderName: '指令预览', content: '<qqbot-cmd-input text="%2Er%201d100" show="%E7%82%B9%E5%87%BB%E6%8E%B7%E9%AA%B0" reference="true" />', rawId: 'fixture-raw-id', timestamp: 1002 },
      ], total: 2, hasMore: false });
    } else if (url.pathname === '/sd-api/resource/page') {
      json({ result: true, data: resources });
    } else if (url.pathname === '/sd-api/chat/send') {
      let body = '';
      for await (const chunk of req) body += chunk;
      sent.push(JSON.parse(body));
      res.statusCode = failSend ? 400 : 200;
      json(failSend ? { error: '模拟发送失败' } : { ok: true });
    } else if (url.pathname === '/avatar.png' || url.pathname === '/sd-api/chat/image') {
      res.setHeader('Content-Type', 'image/png'); res.end(image);
    } else { res.statusCode = 404; res.end(); }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  let browser;
  try {
    browser = await chromium.launch({ executablePath: process.env.CHAT_TEST_BROWSER || 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', headless: true });
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    page.on('pageerror', error => { errors.push(error.message); console.error('Page error:', error.message); });
    await page.goto(`http://127.0.0.1:${server.address().port}/#/qq-chat`);
    await page.waitForFunction(() => document.querySelector('.qq-chat-header-title')?.textContent === '真实测试群名');
    assert.equal(await page.locator('.qq-chat-header-members').innerText(), '· 42 人');
    await page.locator('.qq-chat-msg-avatar[data-sender-id="QQ:888888"]').dblclick();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), '[CQ:at,qq=888888] ');
    await page.locator('.qq-chat-send-btn').click();
    await page.waitForFunction(() => !document.querySelector('.qq-chat-send-btn').textContent.includes('发送中'));
    assert.equal(sent[0].message, '[CQ:at,qq=888888]');
    await page.locator('.qq-chat-msg-avatar[data-sender-id="QQ:999999"] img').waitFor({ state: 'detached' });
    assert.equal(await page.locator('.qq-chat-msg-avatar[data-sender-id="QQ:999999"]').innerText(), '失');
    await page.locator('.qq-chat-textarea').fill('保留草稿');
    await page.locator('.qq-chat-add-btn').click();
    await page.locator('.qq-chat-resource-item').first().waitFor();
    await page.locator('.qq-chat-resource-search').fill('测试');
    assert.equal(await page.locator('.qq-chat-resource-item').count(), 1);
    await page.locator('.qq-chat-resource-item').click();
    await page.screenshot({ path: path.join(root, 'tmp/qq-chat-enhancement-20261013/resource-picker.png') });
    await page.locator('.qq-chat-resource-send').click();
    await page.locator('.qq-chat-resource-panel').waitFor({ state: 'hidden' });
    assert.equal(sent[1].resourcePath, resources[0].path);
    assert.equal(sent[1].targetId, groups[0].groupId);
    assert.equal(sent[1].message, '');
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), '保留草稿');
    failSend = true;
    await page.locator('.qq-chat-add-btn').click();
    await page.locator('.qq-chat-resource-item').first().click();
    await page.locator('.qq-chat-resource-send').click();
    await page.waitForFunction(() => document.querySelector('.qq-chat-toast')?.textContent === '模拟发送失败');
    assert.ok(await page.locator('.qq-chat-resource-panel').isVisible());
    await page.waitForFunction(() => !document.querySelector('.qq-chat-resource-send').disabled);
    failSend = false;
    await page.locator(`.qq-chat-group-item[data-id="${groups[1].groupId}"]`).click();
    assert.ok(await page.locator('.qq-chat-resource-panel').isHidden());
    await page.locator('.qq-chat-msg-avatar[data-sender-id="OpenQQ:fixture-member"]').dblclick();
    assert.ok((await page.locator('.qq-chat-textarea').inputValue()).includes('[CQ:at,qq=OpenQQ:fixture-member]'));
    await page.locator('.qq-chat-type-btn[data-type="private"]').click();
    await page.locator('.qq-chat-msg-avatar[data-sender-id="QQ:888888"]').waitFor();
    const draft = await page.locator('.qq-chat-textarea').inputValue();
    await page.locator('.qq-chat-msg-avatar[data-sender-id="QQ:888888"]').dblclick();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), draft);
    await page.locator('.qq-chat-type-btn[data-type="group"]').click();
    await page.locator(`.qq-chat-group-item[data-id="${groups[0].groupId}"]`).waitFor();
    delayedProfile = true;
    await page.locator(`.qq-chat-group-item[data-id="${groups[0].groupId}"]`).click();
    await page.locator(`.qq-chat-group-item[data-id="${groups[1].groupId}"]`).click();
    await page.waitForFunction(() => document.querySelector('.qq-chat-header-title')?.textContent === '官 Bot 群');
    // Await the outstanding fixture response, then verify it cannot replace the active header.
    await page.waitForTimeout(450);
    assert.equal(await page.locator('.qq-chat-header-title').innerText(), '官 Bot 群');
    await page.locator('.qq-chat-textarea').fill('');
    await page.locator('.qq-chat-add-btn').click();
    await page.locator('.qq-chat-quick-tab[data-tab="katex"]').click();
    assert.equal(await page.locator('.qq-chat-katex-item').count(), 24);
    await page.getByRole('button', { name: '插入无穷', exact: true }).click();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), String.raw`$\infty$ `);
    assert.ok(await page.locator('.qq-chat-markdown-mode').isChecked());
    await page.locator('.qq-chat-textarea').fill('$x + $');
    await page.locator('.qq-chat-textarea').evaluate(input => input.setSelectionRange(5, 5));
    await page.getByRole('button', { name: '插入分数', exact: true }).click();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), String.raw`$x + \dfrac{a}{b} $`);
    await page.locator('.qq-chat-textarea').fill('');
    for (const button of await page.locator('.qq-chat-katex-item').all()) {
      await button.click();
      assert.ok(!(await page.locator('.qq-chat-textarea').inputValue()).includes(String.raw`\f`));
      await page.locator('.qq-chat-textarea').fill('');
    }
    await page.getByRole('button', { name: '插入无穷', exact: true }).click();
    await page.screenshot({ path: path.join(root, 'tmp/qq-chat-enhancement-20261013/katex-shortcuts.png') });
    await page.locator('.qq-chat-send-btn').click();
    await page.waitForFunction(() => document.querySelector('.qq-chat-textarea').value === '');
    assert.equal(sent.at(-1).markdown, true);
    assert.equal(sent.at(-1).message, String.raw`$\infty$`);
    await page.locator('.qq-chat-quick-tab[data-tab="command"]').click();
    await page.locator('.qq-chat-command-text').fill('.r 1d100 "中文" & <');
    await page.locator('.qq-chat-command-show').fill('点我 <掷骰>');
    await page.locator('.qq-chat-command-reference').check();
    const generated = await page.locator('.qq-chat-command-code').inputValue();
    assert.ok(generated.includes(`text="${encodeURIComponent('.r 1d100 "中文" & <')}"`));
    assert.ok(generated.includes(`show="${encodeURIComponent('点我 <掷骰>')}"`));
    assert.ok(generated.includes('reference="true"'));
    const countBeforePreview = sent.length;
    await page.locator('.qq-chat-command-preview').click();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), '.r 1d100 "中文" & <');
    assert.equal(sent.length, countBeforePreview);
    await page.locator('.qq-chat-textarea').fill('');
    await page.screenshot({ path: path.join(root, 'tmp/qq-chat-enhancement-20261013/command-input.png') });
    await page.locator('.qq-chat-command-insert').click();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), generated + ' ');
    assert.ok(await page.locator('.qq-chat-resource-panel').isHidden());
    await page.locator('.qq-chat-textarea').fill('');
    await page.locator('.qq-chat-inline-command').click();
    assert.equal(await page.locator('.qq-chat-textarea').inputValue(), '[CQ:reply,id=fixture-raw-id].r 1d100');
    assert.equal(sent.length, countBeforePreview);
    await page.locator('.qq-chat-textarea').fill('');
    await page.setViewportSize({ width: 720, height: 800 });
    await page.locator('.qq-chat-add-btn').click();
    await page.locator('.qq-chat-quick-tab[data-tab="images"]').click();
    await page.locator('.qq-chat-resource-item').first().waitFor();
    const panel = await page.locator('.qq-chat-resource-panel').boundingBox();
    assert.ok(panel.x >= 0 && panel.x + panel.width <= 720);
    assert.deepEqual(errors, []);
    console.log('PASS: profile and avatars, avatar fallback, numeric/official mentions, image search/send, draft preservation, send retry, session switching, private guard, stale responses, narrow layout; 24 KaTeX symbols, formula insertion, Markdown payload; command-input encoding, preview, reference, inline click without auto-send.');
  } finally {
    if (browser) await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
