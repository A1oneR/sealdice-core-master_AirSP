const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');

(async () => {
  const root = path.resolve(__dirname, '..');
  const frontend = path.join(root, 'static/frontend');
  let baseReads = 0;
  const stall = process.env.CHAT_CONNECTION_STALL === '1';
  const baseline = process.env.CHAT_CONNECTION_BASELINE === '1';
  const expectOffline = process.env.CHAT_CONNECTION_EXPECT_OFFLINE === '1';
  const evidenceName = baseline ? 'baseline' : stall ? 'fixed-stalled' : 'fixed-healthy';
  const stalledResponses = new Set();
  let peakStalls = 0;
  let failHeartbeat = false;
  const requests = [], errors = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const json = value => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(value)); };
    if (url.pathname.startsWith('/sd-api/')) {
      requests.push(url.pathname);
      if (url.pathname === '/sd-api/signin/salt') return json({ salt: 'fixture-salt' });
      if (url.pathname === '/sd-api/baseInfo') { baseReads++; if (failHeartbeat) { res.statusCode=403; return json(null); } return json({ appName: 'SealDice', appChannel: 'dev', version:'1.6.1-dev', versionSimple:'1.6.1-dev', versionDetail:[], versionNew: {}, os:'windows', arch:'amd64', memoryAlloc: 1000, memoryUsedSys: 2000, extraTitle:'', justForTest:false, containerMode:false }); }
      if (url.pathname === '/sd-api/checkSecurity') return json({ isOk:true });
      if (url.pathname === '/sd-api/configs/customText') return json({ texts:{}, helpInfo:{}, previewInfo:{} });
      if (url.pathname === '/sd-api/utils/news') return json({ result:true, checked:true, news:'fixture', newsMark:'fixture' });
      if (url.pathname === '/sd-api/chat/groups') return json({ items:[{ groupId:'QQ-Group:123456', groupName:'连接回归测试群', active:true }], total:1, page:1, totalPages:1 });
      if (url.pathname === '/sd-api/chat/messages') {
        if (stall) {
          stalledResponses.add(res);
          peakStalls = Math.max(peakStalls, stalledResponses.size);
          res.on('close', () => stalledResponses.delete(res));
          if (process.env.CHAT_CONNECTION_PARTIAL_BODY === '1') { res.setHeader('Content-Type','application/json'); res.write('{"messages":'); }
          return;
        }
        return json({ messages:[{ id:'fixture', senderId:'QQ:123456', senderName:'测试用户', content:'测试消息', timestamp:1000 }], total:1 });
      }
      if (url.pathname === '/sd-api/chat/profile') return json({ groupName:'连接回归测试群', memberNum:10 });
      return json({ result:true, show:false });
    }
    const file = path.join(frontend, url.pathname === '/' ? 'index.html' : url.pathname);
    if (!file.startsWith(frontend) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.statusCode = 404; return res.end(); }
    res.setHeader('Content-Type', file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : file.endsWith('.html') ? 'text/html; charset=utf-8' : 'image/png');
    res.end(fs.readFileSync(baseline && url.pathname === '/assets/qq-chat.js' ? path.join(root,'tmp/qq-chat-connection-20261013/before/qq-chat.js') : file));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  let browser;
  try {
    browser = await chromium.launch({ executablePath: 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', headless:true });
    const page = await browser.newPage({ viewport:{ width:1280, height:900 } });
    page.on('pageerror', e => { errors.push(e.stack); console.error('PAGE ERROR', e.stack); });
    page.on('console', message => { if (message.type() === 'error') console.error('CONSOLE', message.text().slice(0, 500)); });
    await page.addInitScript(() => localStorage.setItem('t', 'fixture-token'));
    await page.goto(`http://127.0.0.1:${server.address().port}/#/qq-chat`);
    await page.locator('.qq-chat-header-title').waitFor();
    await page.waitForTimeout(stall ? 52000 : 16500);
    console.log('BASE READS', baseReads, 'ERRORS', errors.length, 'PEAK STALLED CHAT REQUESTS', peakStalls);
    assert.deepEqual(errors, []);
    const offline = page.getByText('与主程序断开连接，请耐心等待连接恢复', {exact:true});
    assert.equal(await offline.isVisible(), expectOffline);
    assert.ok(baseReads >= 3);
    if (stall && !expectOffline) assert.equal(peakStalls, 1);
    const evidence = { baseline, stall, partialBody:process.env.CHAT_CONNECTION_PARTIAL_BODY === '1', baseReads, peakStalls, offlineVisible:await offline.isVisible(), pageErrors:errors.length, requests };
    await page.screenshot({ path:path.join(root,`tmp/qq-chat-connection-20261013/${evidenceName}.png`) });
    if (!expectOffline) {
      failHeartbeat = true;
      await offline.waitFor({ state:'visible', timeout:12000 });
      failHeartbeat = false;
      await offline.waitFor({ state:'hidden', timeout:12000 });
      evidence.actualFailureDetected = true;
      evidence.actualFailureRecovered = true;
      const countChatReads = () => requests.filter(url => url === '/sd-api/chat/messages').length;
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable:true, get:() => true });
        document.dispatchEvent(new Event('visibilitychange'));
      });
      const hiddenCount = countChatReads();
      await page.waitForTimeout(3100);
      assert.equal(countChatReads(), hiddenCount);
      assert.equal(stalledResponses.size, 0);
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable:true, get:() => false });
        document.dispatchEvent(new Event('visibilitychange'));
      });
      await page.waitForTimeout(200);
      assert.ok(countChatReads() > hiddenCount);
      evidence.hiddenPageStopsChatRequests = true;
      evidence.visiblePageResumesChatRequests = true;
    }
    fs.writeFileSync(path.join(root,`tmp/qq-chat-connection-20261013/${evidenceName}-evidence.json`), JSON.stringify(evidence,null,2));
    console.log(expectOffline ? 'PASS: baseline reproduced false offline while healthy heartbeats queued behind stalled chat requests.' : 'PASS: real Vue layout, bounded chat polling and body timeout preserve heartbeat; actual heartbeat failure is shown and automatically recovers.');
  } finally {
    if (browser) await browser.close();
    for (const response of stalledResponses) response.destroy();
    await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
