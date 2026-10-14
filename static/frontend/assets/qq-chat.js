(function () {
  const CHAT_ROUTE = "/qq-chat";
  let pollTimer = null;
  let currentSessionType = sessionStorage.getItem("qq_chat_session_type") || "group";
  let currentGroupId = sessionStorage.getItem("qq_chat_current_group") || "";
  let currentPrivateId = sessionStorage.getItem("qq_chat_current_private") || "";
  let sessionsCache = [];
  let currentFilter = "all";
  let searchQuery = "";
  let autoScroll = true;
  let isSending = false;
  let sessionPage = 1;
  let sessionPageSize = 20;
  let sessionTotalPages = 1;
  let sessionTotal = 0;
  let onlyWithMsgs = true;
  let messagesLimit = 35;
  let currentOldestTimestamp = 0;
  let hasMoreHistory = false;
  let isLoadingHistory = false;
  const knownSessionsMap = new Map();
  let lastRenderedSignature = "";
  let isChatRouteActive = false;
	let resourceImages = [];
	let resourceSelection = null;
	let resourceTarget = null;
	let profilePollCount = 0;
  let sessionRequestId = 0;
  let messagesRequestId = 0;
  const chatReads = new Map();

  function cancelChatRead(key) {
    const controller = chatReads.get(key);
    if (controller) {
      chatReads.delete(key);
      controller.abort();
    }
  }

  // Limit each read stream to one request, including response-body download.
  // A stalled chat poll must not occupy every connection used by the UI heartbeat.
  async function fetchChatJson(key, url, timeoutMs = 8000, replace = false) {
    if (chatReads.has(key)) {
      if (!replace) return null;
      cancelChatRead(key);
    }
    const controller = new AbortController();
    chatReads.set(key, controller);
    const timer = window.setTimeout(() => controller.abort(), timeoutMs);
    try {
      const res = await fetch(url, { headers: getHeaders(), signal: controller.signal });
      const data = await res.json();
      if (controller.signal.aborted || chatReads.get(key) !== controller) return null;
      return { res, data };
    } catch (err) {
      if (err.name === "AbortError") return null;
      throw err;
    } finally {
      window.clearTimeout(timer);
      if (chatReads.get(key) === controller) chatReads.delete(key);
    }
  }
  const katexShortcuts = [
    ["∞", "无穷", String.raw`\infty`], ["π", "圆周率", String.raw`\pi`],
    ["α", "阿尔法", String.raw`\alpha`], ["β", "贝塔", String.raw`\beta`],
    ["θ", "西塔", String.raw`\theta`], ["Δ", "德尔塔", String.raw`\Delta`],
    ["±", "正负", String.raw`\pm`], ["×", "乘号", String.raw`\times`],
    ["÷", "除号", String.raw`\div`], ["≠", "不等于", String.raw`\neq`],
    ["≈", "约等于", String.raw`\approx`], ["≤", "小于等于", String.raw`\leq`],
    ["≥", "大于等于", String.raw`\geq`], ["→", "右箭头", String.raw`\rightarrow`],
    ["↔", "双向箭头", String.raw`\leftrightarrow`], ["⇒", "推出", String.raw`\Rightarrow`],
    ["∑", "求和", String.raw`\sum`], ["∫", "积分", String.raw`\int`],
    ["√x", "平方根", String.raw`\sqrt{x}`], ["a/b", "分数", String.raw`\dfrac{a}{b}`],
    ["★", "星星", String.raw`\bigstar`], ["♥", "爱心", String.raw`\heartsuit`],
    ["✓", "对勾", String.raw`\checkmark`], ["☑", "选中框", String.raw`\boxtimes`],
  ];

  function updateMarkdownMode() {
    const input = document.querySelector(".qq-chat-markdown-mode");
    if (!input) return;
    const id = getCurrentActiveId();
    const supported = /^(OpenQQ:|OpenQQ-Group:|OpenQQCH:|OpenQQCH-Channel:)/.test(id);
    input.disabled = !supported;
    if (!supported) input.checked = false;
  }

  function insertDraftText(code, markdown = false) {
    const input = document.querySelector(".qq-chat-textarea");
    if (!input || !getCurrentActiveId() || isSending) return false;
    const start = input.selectionStart, end = input.selectionEnd;
    if (input.value.length - (end - start) + code.length > input.maxLength) {
      showToast("输入内容已达到长度限制", "info"); return false;
    }
    input.setRangeText(code, start, end, "end");
    const mode = document.querySelector(".qq-chat-markdown-mode");
    if (markdown && !mode.disabled) mode.checked = true;
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.focus();
    return true;
  }

  function insertKatex(formula) {
    const input = document.querySelector(".qq-chat-textarea");
    const before = input.value.slice(0, input.selectionStart).replace(/\\\$/g, "");
    const inFormula = (before.match(/\$/g) || []).length % 2 === 1;
    insertDraftText(inFormula ? formula + " " : "$" + formula + "$ ", true);
  }

  function commandInputTag(panel = document.querySelector(".qq-chat-command-content")) {
    const text = panel.querySelector(".qq-chat-command-text").value;
    const show = panel.querySelector(".qq-chat-command-show").value;
    const reference = panel.querySelector(".qq-chat-command-reference").checked;
    if (!text.trim()) throw new Error("请填写点击后输入的内容");
    if (Array.from(text).length > 100 || Array.from(show).length > 100) throw new Error("内容和展示文字分别最多 100 字符");
    return `<qqbot-cmd-input text="${encodeURIComponent(text)}" show="${encodeURIComponent(show || text)}" reference="${reference}" />`;
  }

  function updateCommandPreview(root = document) {
    const panel = root.querySelector(".qq-chat-command-content");
    const text = panel.querySelector(".qq-chat-command-text").value;
    const show = panel.querySelector(".qq-chat-command-show").value;
    panel.querySelector(".qq-chat-command-preview").textContent = show || text || "点击填入内容";
    try { panel.querySelector(".qq-chat-command-code").value = commandInputTag(panel); }
    catch (err) { panel.querySelector(".qq-chat-command-code").value = err.message; }
  }

  function selectQuickTab(tab) {
    const panel = document.querySelector(".qq-chat-resource-panel");
    panel.querySelectorAll(".qq-chat-quick-tab").forEach(button => {
      const active = button.dataset.tab === tab;
      button.classList.toggle("active", active);
      button.setAttribute("aria-selected", String(active));
    });
    panel.querySelector(".qq-chat-resource-content").hidden = tab !== "images";
    panel.querySelector(".qq-chat-katex-content").hidden = tab !== "katex";
    panel.querySelector(".qq-chat-command-content").hidden = tab !== "command";
  }

  function avatarHtml(url, initial) {
    let safe = "";
    try { const parsed = new URL(url); if (["http:", "https:"].includes(parsed.protocol)) safe = parsed.href; } catch (_) {}
    return `<span class="qq-chat-avatar-initial">${escapeHtml(initial)}</span>${safe ? `<img class="qq-chat-avatar-image" src="${escapeHtml(safe)}" alt="" loading="lazy" referrerpolicy="no-referrer">` : ""}`;
  }

  function insertMention(senderId, name) {
    if (currentSessionType !== "group" || !getCurrentActiveId() || isSending) return;
    if (!/^(QQ:\d+|OpenQQ:[\w-]+|OpenQQCH:[\w-]+)$/.test(senderId)) {
      showToast("此账号暂不支持快捷 @", "info"); return;
    }
    const input = document.querySelector(".qq-chat-textarea");
    const target = senderId.startsWith("QQ:") ? senderId.slice(3) : senderId;
    const code = `[CQ:at,qq=${target}] `;
    const start = input.selectionStart, end = input.selectionEnd;
    if (input.value.length - (end - start) + code.length > input.maxLength) {
      showToast("输入内容已达到长度限制", "info"); return;
    }
    input.setRangeText(code, start, end, "end");
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.focus();
    showToast(`已插入 @${name || "群友"}`);
  }

  async function refreshSessionProfile(sessionId) {
    if (currentSessionType !== "group") return;
    try {
      const result = await fetchChatJson("profile", `/sd-api/chat/profile?groupId=${encodeURIComponent(sessionId)}`, 6000);
      if (!result) return;
      const { res, data } = result;
      if (!res.ok || currentSessionType !== "group" || getCurrentActiveId() !== sessionId) return;
      if (currentSessionType !== "group" || getCurrentActiveId() !== sessionId) return;
      const session = knownSessionsMap.get(sessionId) || sessionsCache.find(s => s.groupId === sessionId);
      if (!session) return;
      if (data.groupName && data.groupName !== "%未知群名%") session.groupName = data.groupName;
      session.avatarUrl = data.avatarUrl || session.avatarUrl;
      if (data.memberNum > 0) session.memberNum = data.memberNum;
      document.querySelector(".qq-chat-header-title").textContent = session.groupName || sessionId;
      const avatar = document.querySelector(".qq-chat-header-avatar");
      avatar.innerHTML = avatarHtml(session.avatarUrl, getInitials(session.groupName, "群"));
      document.querySelector(".qq-chat-header-members").textContent = session.memberNum > 0 ? `· ${session.memberNum} 人` : "";
      renderSessionList();
    } catch (_) { /* Keep cached metadata when the connection is unavailable. */ }
  }

  function closeResourcePicker() {
    cancelChatRead("resources");
    const panel = document.querySelector(".qq-chat-resource-panel");
    if (panel) panel.hidden = true;
    document.querySelector(".qq-chat-add-btn")?.setAttribute("aria-expanded", "false");
    resourceSelection = null;
    resourceTarget = null;
  }

  function renderResourceImages() {
    const panel = document.querySelector(".qq-chat-resource-panel");
    const query = panel.querySelector(".qq-chat-resource-search").value.trim().toLowerCase();
    const filtered = resourceImages.filter(img => `${img.name} ${img.path}`.toLowerCase().includes(query));
    const grid = panel.querySelector(".qq-chat-resource-grid");
    grid.replaceChildren();
    if (!filtered.length) { grid.textContent = resourceImages.length ? "没有匹配的图片" : "暂无图片，请先在资源管理中上传图片"; }
    filtered.forEach(img => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "qq-chat-resource-item";
      button.classList.toggle("selected", resourceSelection?.path === img.path);
      button.title = img.path;
      button.setAttribute("aria-pressed", String(resourceSelection?.path === img.path));
      button.innerHTML = `<img loading="lazy" src="/sd-api/chat/image?path=${encodeURIComponent(img.path)}&token=${encodeURIComponent(getToken())}" alt="${escapeHtml(img.name)}"><span>${escapeHtml(img.name)}</span>`;
      button.addEventListener("click", () => { resourceSelection = img; renderResourceImages(); });
      grid.appendChild(button);
    });
    const send = panel.querySelector(".qq-chat-resource-send");
    send.disabled = !resourceSelection || isSending;
    panel.querySelector(".qq-chat-resource-selected").textContent = resourceSelection ? `已选择：${resourceSelection.name}` : `共 ${resourceImages.length} 张图片，选择后发送`;
  }

  async function openResourcePicker() {
    const id = getCurrentActiveId();
    if (!id) { showToast("请先选择会话"); return; }
    const panel = document.querySelector(".qq-chat-resource-panel");
    if (!panel.hidden) { closeResourcePicker(); return; }
    resourceTarget = { id, type: currentSessionType };
    const target = resourceTarget;
    resourceSelection = null;
    resourceImages = [];
    panel.hidden = false;
    document.querySelector(".qq-chat-add-btn").setAttribute("aria-expanded", "true");
    panel.querySelector(".qq-chat-resource-search").value = "";
    panel.querySelector(".qq-chat-resource-send").disabled = true;
    panel.querySelector(".qq-chat-resource-selected").textContent = "";
    panel.querySelector(".qq-chat-resource-grid").textContent = "正在加载图片资源...";
    try {
      const result = await fetchChatJson("resources", "/sd-api/resource/page", 10000, true);
      if (!result) {
        if (resourceTarget === target && !panel.hidden) panel.querySelector(".qq-chat-resource-grid").textContent = "图片资源暂未加载，请关闭后重新打开重试";
        return;
      }
      const { res, data } = result;
      if (!res.ok) throw new Error("图片资源读取失败，请检查登录状态");
      if (resourceTarget !== target || panel.hidden) return;
      if (data.result === false) throw new Error(data.err || "图片资源读取失败");
      resourceImages = (data.data || []).filter(img => img.type === "image");
      renderResourceImages();
      panel.querySelector(".qq-chat-resource-search").focus();
    } catch (err) { if (resourceTarget === target) panel.querySelector(".qq-chat-resource-grid").textContent = err.message; }
  }

  function getCurrentActiveId() {
    return currentSessionType === "private" ? currentPrivateId : currentGroupId;
  }

  function getToken() {
    return localStorage.getItem("t") || "";
  }

  function getHeaders() {
    const token = getToken();
    return {
      "Content-Type": "application/json",
      Authorization: token,
      token: token,
    };
  }

  function showToast(message, type = "info") {
    const existing = document.querySelector(".qq-chat-toast");
    if (existing) existing.remove();

    const toast = document.createElement("div");
    toast.className = `qq-chat-toast ${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    window.setTimeout(() => {
      toast.style.opacity = "0";
      window.setTimeout(() => toast.remove(), 300);
    }, 2800);
  }

  function escapeHtml(str) {
    if (!str) return "";
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function formatTime(timestamp) {
    if (!timestamp) return "";
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const isToday =
      date.getFullYear() === now.getFullYear() &&
      date.getMonth() === now.getMonth() &&
      date.getDate() === now.getDate();

    const hh = String(date.getHours()).padStart(2, "0");
    const mm = String(date.getMinutes()).padStart(2, "0");

    if (isToday) {
      return `${hh}:${mm}`;
    }

    const month = date.getMonth() + 1;
    const day = date.getDate();
    return `${month}月${day}日 ${hh}:${mm}`;
  }

  function formatFullTime(timestamp) {
    if (!timestamp) return "";
    const date = new Date(timestamp * 1000);
    const yyyy = date.getFullYear();
    const mm = String(date.getMonth() + 1).padStart(2, "0");
    const dd = String(date.getDate()).padStart(2, "0");
    const hh = String(date.getHours()).padStart(2, "0");
    const min = String(date.getMinutes()).padStart(2, "0");
    const ss = String(date.getSeconds()).padStart(2, "0");
    return `${yyyy}-${mm}-${dd} ${hh}:${min}:${ss}`;
  }

  function getAvatarColor(str) {
    const colors = [
      "#1890ff", "#2f54eb", "#722ed1", "#13c2c2", "#52c41a",
      "#fa8c16", "#eb2f96", "#faad14", "#fa541c", "#1677ff",
    ];
    let hash = 0;
    for (let i = 0; i < (str || "").length; i++) {
      hash = (hash << 5) - hash + str.charCodeAt(i);
      hash |= 0;
    }
    return colors[Math.abs(hash) % colors.length];
  }

  function getInitials(name, defaultChar = "群") {
    if (!name) return defaultChar;
    const clean = name.replace(/^[\[\(（【]/, "").trim();
    return clean ? clean.slice(0, 1) : defaultChar;
  }

  function unescapeCQ(val) {
    if (!val) return "";
    return String(val)
      .replace(/&amp;/g, "&")
      .replace(/&#91;/g, "[")
      .replace(/&#93;/g, "]")
      .replace(/&#44;/g, ",")
      .replace(/&lt;/g, "<")
      .replace(/&gt;/g, ">")
      .replace(/&quot;/g, '"')
      .replace(/&#039;/g, "'");
  }

  function parseCqParams(paramStr) {
    const params = {};
    if (!paramStr) return params;
    const parts = paramStr.split(",");
    for (const part of parts) {
      const eqIdx = part.indexOf("=");
      if (eqIdx !== -1) {
        const k = part.slice(0, eqIdx).trim().toLowerCase();
        const v = part.slice(eqIdx + 1).trim();
        params[k] = v;
      }
    }
    return params;
  }

  function normalizeImageSrc(rawSrc) {
    if (!rawSrc) return "";
    let src = unescapeCQ(rawSrc).trim();
    if (/^file:\/\//i.test(src)) {
      try {
        const file = new URL(src);
        src = decodeURIComponent(file.pathname);
        if (/^\/[a-z]:\//i.test(src)) src = src.slice(1);
      } catch (_) { return ""; }
    }
    if (src.startsWith("base64://")) {
      return "data:image/png;base64," + src.slice(9);
    }
    if (src.startsWith("data:image/")) {
      return src;
    }
    if (src.startsWith("http://") || src.startsWith("https://")) {
      return src;
    }
    // Local file path e.g. data/images/xxx.png or cache/xxx.png or relative path
    if (src.startsWith("data/") || src.startsWith("cache/") || src.includes("/") || src.includes("\\")) {
      const token = getToken();
      return `/sd-api/chat/image?path=${encodeURIComponent(src)}${token ? `&token=${encodeURIComponent(token)}` : ""}`;
    }
    return src;
  }

  function renderImageCardHtml(imgUrl, altText = "图片") {
    if (!imgUrl) return "";
    const safeUrl = escapeHtml(imgUrl);
    const safeAlt = escapeHtml(altText);
    return `<div class="qq-chat-img-card" data-src="${safeUrl}">` +
      `<div class="qq-chat-img-thumb-wrap">` +
        `<img src="${safeUrl}" alt="${safeAlt}" class="qq-chat-img" loading="lazy" referrerpolicy="no-referrer" onerror="this.onerror=null;const c=this.closest('.qq-chat-img-card');if(c)c.classList.add('img-load-error');" />` +
        `<div class="qq-chat-img-overlay">` +
          `<span class="qq-chat-img-zoom-tag">` +
            `<svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14zm.5-7h-1v2h-2v1h2v2h1v-2h2V9h-2z"/></svg>` +
            `点击放大` +
          `</span>` +
        `</div>` +
      `</div>` +
      `<div class="qq-chat-img-error-hint">` +
        `<svg viewBox="0 0 24 24" width="15" height="15" fill="currentColor"><path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/></svg>` +
        `<span>图片加载失败</span>` +
        `<a href="${safeUrl}" target="_blank" rel="noopener noreferrer" class="qq-chat-img-retry-link">新窗口打开</a>` +
      `</div>` +
    `</div>`;
  }

  function decodeFaceTag(tag) {
    if (!tag) return "";
    const m = tag.match(/ext=(?:\\?["'])([^"'>\\]+)(?:\\?["'])/i) || tag.match(/ext=([^"'>\s,]+)/i);
    if (m && m[1]) {
      try {
        const b64 = m[1].replace(/\\/g, "").trim();
        let decodedStr = "";
        try {
          const binary = atob(b64);
          const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
          decodedStr = new TextDecoder("utf-8").decode(bytes);
        } catch (e) {
          decodedStr = atob(b64);
        }
        const data = JSON.parse(decodedStr);
        if (data && typeof data.text === "string" && data.text.trim()) {
          return data.text.trim();
        }
      } catch (e) {
        // ignore decode error
      }
    }
    return "";
  }

  function formatGroupSnippet(content) {
    if (!content) return "暂无消息";
    let s = String(content);

    const hasImages = /\[CQ:image/i.test(s) || /\[(?:图|img):/i.test(s) || /!\[[^\]]*\]\(/i.test(s);
    if (hasImages) {
      s = s.replace(/<faceType[\s=][^>]*>\s*/gi, "");
    } else {
      s = s.replace(/<faceType[\s=][^>]*>/gi, (tag) => {
        const decoded = decodeFaceTag(tag);
        return decoded || "[表情]";
      });
    }

    s = s
      .replace(/\[CQ:image,[^\]]*\]/gi, "[图片]")
      .replace(/\[(?:图|img):[^\]]+\]/gi, "[图片]")
      .replace(/!\[[^\]]*\]\([^)]+\)/g, "[图片]")
      .replace(/\[CQ:file,[^\]]*\]/gi, "[文件]")
      .replace(/\[CQ:record,[^\]]*\]/gi, "[语音]")
      .replace(/\[CQ:video,[^\]]*\]/gi, "[视频]")
      .replace(/\[CQ:face,[^\]]*\]/gi, "[表情]")
      .replace(/\r?\n/g, " ")
      .trim();
    return escapeHtml(s) || "[图片]";
  }

  function parseContentToHtml(content) {
    if (!content) return "";
    const images = [];
    const commands = [];

    // 1. [CQ:image, ...]
    let text = String(content).replace(/<qqbot-cmd-input\s+([^>]*?)\s*\/>/gi, (match, attributes) => {
      const params = {};
      for (const attr of attributes.matchAll(/(text|show|reference)="([^"]*)"/g)) params[attr[1]] = attr[2];
      try {
        const value = decodeURIComponent(unescapeCQ(params.text || ""));
        const label = decodeURIComponent(unescapeCQ(params.show || params.text || ""));
        if (!value || Array.from(value).length > 100 || Array.from(label).length > 100) return match;
        const token = `__QQ_CHAT_CMD_${commands.length}__`;
        commands.push({ text: value, show: label, reference: params.reference === "true" });
        return token;
      } catch (_) { return match; }
    });
    text = text.replace(/\[CQ:image(?:,([^\]]*))?\]/gi, (match, paramStr) => {
      const params = parseCqParams(paramStr || "");
      let rawUrl = params.url || params.file || "";
      if (params.url && (params.url.startsWith("http://") || params.url.startsWith("https://"))) {
        rawUrl = params.url;
      }
      const finalUrl = normalizeImageSrc(rawUrl);
      if (finalUrl) {
        const token = `__QQ_CHAT_IMG_${images.length}__`;
        images.push({ url: finalUrl, alt: "图片" });
        return token;
      }
      return match;
    });

    // 2. [图:...] or [img:...]
    text = text.replace(/\[(?:图|img):([^\]]+)\]/gi, (match, rawPath) => {
      const finalUrl = normalizeImageSrc(rawPath);
      if (finalUrl) {
        const token = `__QQ_CHAT_IMG_${images.length}__`;
        images.push({ url: finalUrl, alt: "海豹插图" });
        return token;
      }
      return match;
    });

    // 3. Markdown images: ![alt](url)
    text = text.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (match, alt, url) => {
      const finalUrl = normalizeImageSrc(url);
      if (finalUrl) {
        const token = `__QQ_CHAT_IMG_${images.length}__`;
        images.push({ url: finalUrl, alt: alt || "图片" });
        return token;
      }
      return match;
    });

    // 4. [CQ:file, ...] if file is image
    text = text.replace(/\[CQ:file(?:,([^\]]*))?\]/gi, (match, paramStr) => {
      const params = parseCqParams(paramStr || "");
      const fileName = (params.file || params.name || "").toLowerCase();
      if (/\.(jpe?g|png|gif|webp|bmp|svg)$/i.test(fileName)) {
        const rawUrl = params.url || params.file;
        const finalUrl = normalizeImageSrc(rawUrl);
        if (finalUrl) {
          const token = `__QQ_CHAT_IMG_${images.length}__`;
          images.push({ url: finalUrl, alt: params.file || "图片文件" });
          return token;
        }
      }
      return match;
    });

    // 4.5 Filter out / decode face tags (<faceType=...>)
    if (images.length > 0) {
      text = text.replace(/<faceType[\s=][^>]*>\s*/gi, "");
    } else {
      text = text.replace(/<faceType[\s=][^>]*>/gi, (tag) => {
        const decoded = decodeFaceTag(tag);
        return decoded || "[表情]";
      });
    }
    text = text.replace(/<faceType[\s=][^>]*>\s*/gi, "");
    text = text.replace(/\[CQ:face,id=\d+\]/gi, "[表情]");

    // 5. Escape HTML for ordinary text
    let html = escapeHtml(text);

    // 6. At mentions @xxx
    html = html.replace(/@([^\s@]+)/g, '<span class="qq-chat-at">@$1</span>');

    // 7. Regular URLs (that are not tokens)
    html = html.replace(
      /(https?:\/\/[a-zA-Z0-9_\-\.\/\?%&=+#~]+)/g,
      '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
    );

    // 8. Newlines
    html = html.replace(/\r?\n/g, "<br/>");

    // 9. Re-insert images
    images.forEach((img, idx) => {
      const token = `__QQ_CHAT_IMG_${idx}__`;
      html = html.split(token).join(renderImageCardHtml(img.url, img.alt));
    });
    commands.forEach((command, index) => {
      html = html.split(`__QQ_CHAT_CMD_${index}__`).join(`<button type="button" class="qq-chat-inline-command" data-command-text="${escapeHtml(command.text)}" data-reference="${command.reference}" title="点击填入输入框">${escapeHtml(command.show)}</button>`);
    });

    return html;
  }

  // --------------------------------------------------------------------------
  // Lightbox Image Viewer
  // --------------------------------------------------------------------------
  let lightboxEl = null;
  let lightboxImgEl = null;
  let lightboxScaleValEl = null;
  let lightboxDimValEl = null;
  let currentLightboxSrc = "";
  let currentScale = 1.0;
  let currentRotate = 0;
  let panX = 0;
  let panY = 0;
  let isPanning = false;
  let panStartX = 0;
  let panStartY = 0;

  function initLightbox() {
    if (document.getElementById("qq-chat-lightbox")) return;

    lightboxEl = document.createElement("div");
    lightboxEl.id = "qq-chat-lightbox";
    lightboxEl.className = "qq-chat-lightbox";
    lightboxEl.style.display = "none";
    lightboxEl.innerHTML = `
      <div class="qq-chat-lightbox-backdrop"></div>
      <div class="qq-chat-lightbox-container">
        <header class="qq-chat-lightbox-topbar">
          <div class="qq-chat-lightbox-meta">
            <span class="qq-chat-lightbox-zoom-indicator">100%</span>
            <span class="qq-chat-lightbox-dim-indicator"></span>
          </div>
          <div class="qq-chat-lightbox-top-actions">
            <button type="button" class="qq-chat-lightbox-btn qq-chat-lightbox-close-btn" title="关闭 (ESC)">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
            </button>
          </div>
        </header>

        <div class="qq-chat-lightbox-stage">
          <div class="qq-chat-lightbox-stage-inner">
            <img class="qq-chat-lightbox-image" src="" alt="放大图片" draggable="false" referrerpolicy="no-referrer" />
          </div>
        </div>

        <footer class="qq-chat-lightbox-toolbar">
          <button type="button" class="qq-chat-lightbox-btn" data-action="zoom-out" title="缩小 (-)">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M19 13H5v-2h14v2z"/></svg>
          </button>
          <span class="qq-chat-lightbox-zoom-val">100%</span>
          <button type="button" class="qq-chat-lightbox-btn" data-action="zoom-in" title="放大 (+)">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/></svg>
          </button>
          <div class="qq-chat-lightbox-divider"></div>
          <button type="button" class="qq-chat-lightbox-btn" data-action="reset" title="适应窗口 / 还原 (0)">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"/></svg>
          </button>
          <button type="button" class="qq-chat-lightbox-btn" data-action="rotate-left" title="向左旋转 90° ([)">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M7.11 8.53L5.7 7.11C4.8 8.27 4.24 9.61 4.07 11h2.02c.14-.87.49-1.72 1.02-2.47zM6.09 13H4.07c.17 1.39.72 2.73 1.62 3.89l1.41-1.42c-.52-.75-.87-1.59-1.01-2.47zm1.01 5.32c1.16.9 2.51 1.44 3.9 1.61V17.9c-.87-.15-1.71-.49-2.46-1.01L7.1 18.32zM13 4.07V1L8.45 5.55 13 10V6.09c3.37.5 6 3.41 6 6.91 0 3.87-3.13 7-7 7-1.39 0-2.69-.41-3.79-1.11l-1.42 1.42C8.27 21.39 10.05 22 12 22c5.52 0 10-4.48 10-10 0-5.18-3.95-9.45-8.99-9.93z"/></svg>
          </button>
          <button type="button" class="qq-chat-lightbox-btn" data-action="rotate-right" title="向右旋转 90° (])">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M15.55 5.55L11 1v3.07C5.96 4.55 2 8.82 2 14c0 5.52 4.48 10 10 10 1.95 0 3.73-.61 5.21-1.69l-1.42-1.42C14.69 21.59 13.39 22 12 22c-3.87 0-7-3.13-7-7 0-3.5 2.63-6.41 6-6.91V12l4.55-4.45zM19.93 11h-2.02c-.14-.87-.49-1.72-1.02-2.47l1.41-1.42c.9 1.16 1.46 2.5 1.63 3.89zm-1.63 5.89l-1.41-1.41c.53-.75.88-1.6 1.02-2.48h2.02c-.17 1.39-.73 2.73-1.63 3.89zM13 17.9v2.02c1.39-.17 2.74-.71 3.9-1.61l-1.41-1.41c-.75.51-1.59.85-2.49 1z"/></svg>
          </button>
          <div class="qq-chat-lightbox-divider"></div>
          <button type="button" class="qq-chat-lightbox-btn" data-action="open-tab" title="在新标签页中打开原图">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M19 19H5V5h7V3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7h-2v7zM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7z"/></svg>
          </button>
          <button type="button" class="qq-chat-lightbox-btn" data-action="download" title="保存/下载图片">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM17 13l-5 5-5-5h3V9h4v4h3z"/></svg>
          </button>
          <button type="button" class="qq-chat-lightbox-btn" data-action="close" title="关闭 (ESC)">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
          </button>
        </footer>
      </div>
    `;

    document.body.appendChild(lightboxEl);

    lightboxImgEl = lightboxEl.querySelector(".qq-chat-lightbox-image");
    lightboxScaleValEl = lightboxEl.querySelector(".qq-chat-lightbox-zoom-val");
    lightboxDimValEl = lightboxEl.querySelector(".qq-chat-lightbox-dim-indicator");

    // Close on backdrop or close button
    lightboxEl.querySelector(".qq-chat-lightbox-backdrop").addEventListener("click", closeImageLightbox);
    lightboxEl.querySelector(".qq-chat-lightbox-close-btn").addEventListener("click", closeImageLightbox);

    // Toolbar actions
    lightboxEl.querySelector(".qq-chat-lightbox-toolbar").addEventListener("click", (e) => {
      const btn = e.target.closest("button[data-action]");
      if (!btn) return;
      const action = btn.getAttribute("data-action");
      if (action === "zoom-in") zoomLightbox(0.25);
      else if (action === "zoom-out") zoomLightbox(-0.25);
      else if (action === "reset") resetLightboxTransform();
      else if (action === "rotate-left") rotateLightbox(-90);
      else if (action === "rotate-right") rotateLightbox(90);
      else if (action === "open-tab") {
        if (currentLightboxSrc) window.open(currentLightboxSrc, "_blank");
      } else if (action === "download") {
        downloadLightboxImage();
      } else if (action === "close") {
        closeImageLightbox();
      }
    });

    // Mouse wheel zoom
    const stage = lightboxEl.querySelector(".qq-chat-lightbox-stage");
    stage.addEventListener("wheel", (e) => {
      e.preventDefault();
      const delta = e.deltaY < 0 ? 0.25 : -0.25;
      zoomLightbox(delta);
    }, { passive: false });

    // Drag to pan
    stage.addEventListener("mousedown", (e) => {
      if (e.button !== 0) return;
      if (e.target.closest(".qq-chat-lightbox-toolbar, .qq-chat-lightbox-topbar")) return;
      isPanning = true;
      panStartX = e.clientX - panX;
      panStartY = e.clientY - panY;
      stage.classList.add("is-dragging");
    });

    window.addEventListener("mousemove", (e) => {
      if (!isPanning) return;
      panX = e.clientX - panStartX;
      panY = e.clientY - panStartY;
      updateLightboxTransform();
    });

    window.addEventListener("mouseup", () => {
      if (isPanning) {
        isPanning = false;
        stage.classList.remove("is-dragging");
      }
    });

    // Double click to toggle 1x and 2x
    lightboxImgEl.addEventListener("dblclick", (e) => {
      e.preventDefault();
      if (Math.abs(currentScale - 1.0) < 0.05) {
        currentScale = 2.0;
      } else {
        currentScale = 1.0;
        panX = 0;
        panY = 0;
      }
      updateLightboxTransform();
    });

    // Keyboard shortcuts
    window.addEventListener("keydown", (e) => {
      if (!lightboxEl || lightboxEl.style.display === "none") return;
      if (e.key === "Escape") {
        closeImageLightbox();
      } else if (e.key === "+" || e.key === "=") {
        zoomLightbox(0.25);
      } else if (e.key === "-") {
        zoomLightbox(-0.25);
      } else if (e.key === "0") {
        resetLightboxTransform();
      } else if (e.key === "[") {
        rotateLightbox(-90);
      } else if (e.key === "]") {
        rotateLightbox(90);
      }
    });
  }

  function openImageLightbox(src) {
    if (!src) return;
    initLightbox();
    currentLightboxSrc = src;
    currentScale = 1.0;
    currentRotate = 0;
    panX = 0;
    panY = 0;
    isPanning = false;

    lightboxImgEl.src = src;
    updateLightboxTransform();

    if (lightboxDimValEl) lightboxDimValEl.textContent = "";

    lightboxImgEl.onload = () => {
      if (lightboxDimValEl && lightboxImgEl.naturalWidth) {
        lightboxDimValEl.textContent = `· ${lightboxImgEl.naturalWidth} × ${lightboxImgEl.naturalHeight} px`;
      }
    };

    lightboxEl.style.display = "flex";
    document.body.classList.add("qq-chat-lightbox-open");
  }

  function closeImageLightbox() {
    if (!lightboxEl) return;
    lightboxEl.style.display = "none";
    document.body.classList.remove("qq-chat-lightbox-open");
  }

  function updateLightboxTransform() {
    if (!lightboxImgEl) return;
    lightboxImgEl.style.transform = `translate3d(${panX}px, ${panY}px, 0) scale(${currentScale}) rotate(${currentRotate}deg)`;
    const pct = `${Math.round(currentScale * 100)}%`;
    if (lightboxScaleValEl) lightboxScaleValEl.textContent = pct;
    const topIndicator = lightboxEl?.querySelector(".qq-chat-lightbox-zoom-indicator");
    if (topIndicator) topIndicator.textContent = pct;
  }

  function zoomLightbox(delta) {
    currentScale = Math.max(0.2, Math.min(6.0, currentScale + delta));
    updateLightboxTransform();
  }

  function rotateLightbox(deg) {
    currentRotate = (currentRotate + deg + 360) % 360;
    updateLightboxTransform();
  }

  function resetLightboxTransform() {
    currentScale = 1.0;
    currentRotate = 0;
    panX = 0;
    panY = 0;
    updateLightboxTransform();
  }

  function downloadLightboxImage() {
    if (!currentLightboxSrc) return;
    try {
      const a = document.createElement("a");
      a.href = currentLightboxSrc;
      a.download = `qq-chat-image-${Date.now()}.png`;
      a.target = "_blank";
      document.body.appendChild(a);
      a.click();
      a.remove();
    } catch (e) {
      window.open(currentLightboxSrc, "_blank");
    }
  }

  function hookVueRouter() {
    const appEl = document.getElementById("app");
    const router = appEl?.__vue_app__?.config?.globalProperties?.$router;
    if (router && !router.__qq_chat_hooked) {
      router.__qq_chat_hooked = true;
      if (!router.hasRoute("qq-chat")) {
        try {
          router.addRoute({
            path: "/qq-chat",
            name: "qq-chat",
            component: { render: () => null },
          });
        } catch (e) {
          console.warn("addRoute error:", e);
        }
      }
      router.afterEach(() => {
        handleRouteChange();
      });
    }
  }

  // --------------------------------------------------------------------------
  // Navigation injection
  // --------------------------------------------------------------------------
  function injectNavTab() {
    const menu = document.querySelector(".el-menu");
    if (!menu) return;
    if (document.querySelector(".sealdice-nav-qq-chat")) {
      updateNavActiveState();
      return;
    }

    const navItem = document.createElement("li");
    navItem.className = "el-menu-item sealdice-nav-qq-chat";
    navItem.setAttribute("role", "menuitem");
    navItem.setAttribute("tabindex", "-1");
    navItem.setAttribute("index", "/qq-chat");
    navItem.innerHTML = `
      <i class="el-icon" style="margin-right: 8px; font-size: 17px; display: inline-flex; align-items: center;">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
          <path d="M20 2H4c-1.1 0-1.99.9-1.99 2L2 22l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 9h12v2H6V9zm8 5H6v-2h8v2zm4-6H6V6h12v2z"/>
        </svg>
      </i>
      <span>QQ 聊天</span>
    `;

    navItem.addEventListener("click", (e) => {
      e.preventDefault();
      const appEl = document.getElementById("app");
      const router = appEl?.__vue_app__?.config?.globalProperties?.$router;
      if (router) {
        if (!router.hasRoute("qq-chat")) {
          router.addRoute({
            path: "/qq-chat",
            name: "qq-chat",
            component: { render: () => null },
          });
        }
        router.push("/qq-chat").catch(() => {});
      } else {
        window.location.hash = `#${CHAT_ROUTE}`;
      }
    });

    const groupNavItem = Array.from(menu.querySelectorAll(".el-menu-item, .el-sub-menu"))
      .find((el) => el.textContent.includes("群管理") || el.textContent.includes("群组"));

    if (groupNavItem && groupNavItem.parentNode) {
      groupNavItem.insertAdjacentElement("afterend", navItem);
    } else {
      menu.appendChild(navItem);
    }

    updateNavActiveState();
  }

  function updateNavActiveState() {
    const navItem = document.querySelector(".sealdice-nav-qq-chat");
    if (!navItem) return;
    const isChat = window.location.hash.includes(CHAT_ROUTE);
    if (isChat) {
      navItem.classList.add("is-active");
      // Remove is-active from other menu items
      document.querySelectorAll(".el-menu .el-menu-item:not(.sealdice-nav-qq-chat)").forEach((el) => {
        el.classList.remove("is-active");
      });
    } else {
      navItem.classList.remove("is-active");
    }
  }

  // --------------------------------------------------------------------------
  // View Container
  // --------------------------------------------------------------------------
  function getOrCreateChatView() {
    let view = document.getElementById("sealdice-qq-chat-view");
    if (view) return view;

    view = document.createElement("div");
    view.id = "sealdice-qq-chat-view";
    view.style.display = "none";
    view.innerHTML = `
      <!-- Left: Sidebar -->
      <aside class="qq-chat-sidebar">
        <div class="qq-chat-type-switcher">
          <button type="button" class="qq-chat-type-btn ${currentSessionType === 'group' ? 'active' : ''}" data-type="group">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" style="vertical-align: -2px; margin-right: 4px;"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 9h12v2H6V9zm8 5H6v-2h8v2zm4-6H6V6h12v2z"/></svg>群聊
          </button>
          <button type="button" class="qq-chat-type-btn ${currentSessionType === 'private' ? 'active' : ''}" data-type="private">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" style="vertical-align: -2px; margin-right: 4px;"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>私聊
          </button>
        </div>
        <div class="qq-chat-search-wrap">
          <input type="text" class="qq-chat-search-input" placeholder="${currentSessionType === 'private' ? '搜索私聊用户 / ID...' : '搜索群聊名称 / ID...'}" />
          <button type="button" class="qq-chat-icon-btn qq-chat-refresh-groups-btn" title="刷新列表">&#x21bb;</button>
        </div>
        <div class="qq-chat-scope-tabs" style="${currentSessionType === 'private' ? 'display:none;' : ''}">
          <button type="button" class="qq-chat-scope-tab active" data-scope="active">仅看有发言群</button>
          <button type="button" class="qq-chat-scope-tab" data-scope="all">全部群聊</button>
        </div>
        <div class="qq-chat-filter-tabs">
          <button type="button" class="qq-chat-filter-tab active" data-filter="all">全部</button>
          <button type="button" class="qq-chat-filter-tab" data-filter="official">官Bot</button>
          <button type="button" class="qq-chat-filter-tab qq-chat-filter-tab-active" data-filter="active" style="${currentSessionType === 'private' ? 'display:none;' : ''}">已开启</button>
        </div>
        <div class="qq-chat-group-list">
          <div class="qq-chat-empty-hint">${currentSessionType === 'private' ? '正在加载私聊...' : '正在加载群聊...'}</div>
        </div>
        <div class="qq-chat-pagination-bar">
          <button type="button" class="qq-chat-page-btn qq-chat-page-prev" disabled>上一页</button>
          <span class="qq-chat-page-indicator">1 / 1 (0项)</span>
          <button type="button" class="qq-chat-page-btn qq-chat-page-next" disabled>下一页</button>
        </div>
      </aside>

      <!-- Right: Main Chat Area -->
      <main class="qq-chat-main">
        <header class="qq-chat-header">
          <div class="qq-chat-header-info">
            <div class="qq-chat-header-avatar" style="display:none;"></div>
            <div>
              <div class="qq-chat-header-title-row">
                <span class="qq-chat-header-title">请选择左侧会话</span>
                <span class="qq-chat-badge qq-chat-badge-type" style="display:none;">私聊</span>
                <span class="qq-chat-badge qq-chat-badge-official" style="display:none;">官Bot</span>
                <span class="qq-chat-badge qq-chat-badge-status" style="display:none;"></span>
              </div>
              <div class="qq-chat-header-subtitle">
                <span class="qq-chat-header-id"></span>
                <button type="button" class="qq-chat-copy-id-btn" style="display:none;" title="复制ID">复制</button>
                <span class="qq-chat-header-members"></span>
              </div>
            </div>
          </div>
          <div class="qq-chat-header-actions">
            <label class="qq-chat-auto-scroll-label">
              <input type="checkbox" checked id="qq-chat-autoscroll-chk" /> 自动滚屏
            </label>
            <button type="button" class="qq-chat-btn qq-chat-btn-refresh" title="刷新消息">&#x21bb; 刷新</button>
            <button type="button" class="qq-chat-btn qq-chat-btn-clear" title="清空历史记录">&#x1f5d1; 清空</button>
          </div>
        </header>

        <div class="qq-chat-messages-container">
          <div class="qq-chat-messages-stream">
            <div class="qq-chat-empty-hint">请从左侧选择会话开始查看消息</div>
          </div>
        </div>

        <footer class="qq-chat-footer">
          <section class="qq-chat-resource-panel" hidden aria-label="图片资源、KaTeX 符号与点击填入指令">
            <div class="qq-chat-resource-heading"><strong>快捷工具</strong><button type="button" class="qq-chat-icon-btn qq-chat-resource-close" aria-label="关闭快捷工具">×</button></div>
            <div class="qq-chat-quick-tabs" role="tablist" aria-label="快捷工具分类">
              <button type="button" class="qq-chat-quick-tab active" data-tab="images" role="tab" aria-selected="true">图片资源</button>
              <button type="button" class="qq-chat-quick-tab" data-tab="katex" role="tab" aria-selected="false">KaTeX 符号</button>
              <button type="button" class="qq-chat-quick-tab" data-tab="command" role="tab" aria-selected="false">点击填入指令</button>
            </div>
            <div class="qq-chat-resource-content">
            <input class="qq-chat-resource-search" type="search" placeholder="搜索图片名称 / 路径" aria-label="搜索图片资源">
            <div class="qq-chat-resource-grid"></div>
            <div class="qq-chat-resource-actions"><span class="qq-chat-resource-selected"></span><button type="button" class="qq-chat-btn qq-chat-resource-send" disabled>发送图片</button></div>
            </div>
            <div class="qq-chat-katex-content" hidden>
              <div class="qq-chat-katex-hint">点击插入公式，可继续编辑。官 Bot 会自动开启 Markdown / KaTeX 发送。</div>
              <div class="qq-chat-katex-grid">${katexShortcuts.map(([symbol, name, formula], index) => `<button type="button" class="qq-chat-katex-item" data-index="${index}" title="${escapeHtml(formula)}" aria-label="插入${name}"><strong>${symbol}</strong><span>${name}</span><code>${escapeHtml(formula)}</code></button>`).join("")}</div>
            </div>
            <div class="qq-chat-command-content" hidden>
              <div class="qq-chat-katex-hint">官 Bot Markdown：展示表面文字，点击后把内容填入 QQ 输入框，不会直接发送。</div>
              <label>展示文字（show）<input class="qq-chat-command-show" type="text" maxlength="100" value="点击掷骰" placeholder="消息中显示的文字"></label>
              <label>点击后输入（text）<textarea class="qq-chat-command-text" rows="2" maxlength="100" placeholder="填入用户聊天框的指令或文字">.r 1d100</textarea></label>
              <label class="qq-chat-command-reference-label"><input class="qq-chat-command-reference" type="checkbox">引用原消息（reference）</label>
              <div class="qq-chat-command-preview-row"><span>效果预览：</span><button type="button" class="qq-chat-command-preview">点击掷骰</button></div>
              <textarea class="qq-chat-command-code" rows="3" readonly aria-label="生成的点击填入代码"></textarea>
              <button type="button" class="qq-chat-btn qq-chat-command-insert">插入到消息</button>
            </div>
          </section>
          <div class="qq-chat-input-toolbar">
            <button type="button" class="qq-chat-icon-btn qq-chat-add-btn" title="图片、KaTeX 与点击填入指令" aria-label="打开快捷工具" aria-expanded="false">+</button>
            <label class="qq-chat-markdown-label"><input type="checkbox" class="qq-chat-markdown-mode" disabled> Markdown / KaTeX</label>
            <span class="qq-chat-toolbar-tip">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>
              以骰娘身份发送 · 双击群友头像快捷 @
            </span>
            <span class="qq-chat-char-counter">0/1000</span>
          </div>
          <div class="qq-chat-input-box">
            <textarea class="qq-chat-textarea" placeholder="${currentSessionType === 'private' ? '输入要发送给私聊用户的信息... (Enter 发送, Ctrl+Enter 换行)' : '输入要发送到群聊的信息... (Enter 发送, Ctrl+Enter 换行)'}" rows="2" maxlength="1000"></textarea>
            <button type="button" class="qq-chat-send-btn" disabled>
              <span>发送</span>
              <svg viewBox="0 0 24 24" width="15" height="15" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
            </button>
          </div>
        </footer>
      </main>
    `;

    bindChatViewEvents(view);
    initLightbox();
    return view;
  }

  function updateSessionTypeUI(view) {
    if (!view) view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;
    const isPrivate = currentSessionType === "private";
    const sInput = view.querySelector(".qq-chat-search-input");
    if (sInput) {
      sInput.placeholder = isPrivate ? "搜索私聊用户 / ID..." : "搜索群聊名称 / ID...";
    }
    const scopeTabs = view.querySelector(".qq-chat-scope-tabs");
    if (scopeTabs) {
      scopeTabs.style.display = isPrivate ? "none" : "flex";
    }
    const activeFilterTab = view.querySelector(".qq-chat-filter-tab-active");
    if (activeFilterTab) {
      activeFilterTab.style.display = isPrivate ? "none" : "";
    }
    if (isPrivate && currentFilter === "active") {
      currentFilter = "all";
      const filterTabs = view.querySelectorAll(".qq-chat-filter-tab");
      filterTabs.forEach((t) => {
        t.classList.toggle("active", t.getAttribute("data-filter") === "all");
      });
    }
    const textarea = view.querySelector(".qq-chat-textarea");
    if (textarea) {
      textarea.placeholder = isPrivate
        ? "输入要发送给私聊用户的信息... (Enter 发送, Ctrl+Enter 换行)"
        : "输入要发送到群聊的信息... (Enter 发送, Ctrl+Enter 换行)";
    }
    const tipText = view.querySelector(".qq-chat-toolbar-tip");
    if (tipText) {
      tipText.innerHTML = `
        <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>
        ${isPrivate ? "以骰娘身份发送私聊消息" : "以骰娘身份发送群聊消息 · 双击群友头像快捷 @"}
      `;
    }
  }

  function bindChatViewEvents(view) {
    view.addEventListener("error", e => {
      if (e.target.matches?.(".qq-chat-avatar-image")) e.target.remove();
    }, true);
    view.querySelector(".qq-chat-add-btn").addEventListener("click", openResourcePicker);
    view.querySelector(".qq-chat-resource-close").addEventListener("click", closeResourcePicker);
    view.querySelector(".qq-chat-resource-search").addEventListener("input", renderResourceImages);
    view.querySelectorAll(".qq-chat-quick-tab").forEach(button => button.addEventListener("click", () => selectQuickTab(button.dataset.tab)));
    view.querySelectorAll(".qq-chat-katex-item").forEach(button => button.addEventListener("click", () => insertKatex(katexShortcuts[Number(button.dataset.index)][2])));
    view.querySelectorAll(".qq-chat-command-show, .qq-chat-command-text, .qq-chat-command-reference").forEach(input => input.addEventListener("input", () => updateCommandPreview()));
    view.querySelector(".qq-chat-command-insert").addEventListener("click", () => {
      try { if (insertDraftText(commandInputTag() + " ", true)) closeResourcePicker(); }
      catch (err) { showToast(err.message, "info"); }
    });
    view.querySelector(".qq-chat-command-preview").addEventListener("click", () => insertDraftText(view.querySelector(".qq-chat-command-text").value));
    updateCommandPreview(view);
    view.querySelector(".qq-chat-resource-send").addEventListener("click", () => {
      if (!resourceSelection || !resourceTarget || resourceTarget.id !== getCurrentActiveId() || resourceTarget.type !== currentSessionType) return;
      sendMessage(resourceSelection.path);
    });
    view.addEventListener("keydown", e => { if (e.key === "Escape") closeResourcePicker(); });
    const typeBtns = view.querySelectorAll(".qq-chat-type-btn");
    typeBtns.forEach((btn) => {
      btn.addEventListener("click", () => {
        const type = btn.getAttribute("data-type");
        if (type === currentSessionType) return;
        closeResourcePicker();
        currentSessionType = type;
        sessionStorage.setItem("qq_chat_session_type", type);
        typeBtns.forEach((b) => b.classList.toggle("active", b.getAttribute("data-type") === type));

        updateSessionTypeUI(view);

        sessionPage = 1;
        searchQuery = "";
        const sInput = view.querySelector(".qq-chat-search-input");
        if (sInput) sInput.value = "";
        sessionsCache = [];

        const activeId = getCurrentActiveId();
        if (activeId) {
          selectSession(activeId, true);
        } else {
          resetMainHeader(view);
        }
        fetchSessions(false);
      });
    });

    const searchInput = view.querySelector(".qq-chat-search-input");
    searchInput.addEventListener("input", (e) => {
      searchQuery = e.target.value.trim().toLowerCase();
      renderSessionList();
    });

    const refreshGroupsBtn = view.querySelector(".qq-chat-refresh-groups-btn");
    refreshGroupsBtn.addEventListener("click", () => {
      fetchSessions(true);
    });

    const groupListEl = view.querySelector(".qq-chat-group-list");
    if (groupListEl) {
      groupListEl.addEventListener("click", (e) => {
        const item = e.target.closest(".qq-chat-group-item");
        if (!item) return;
        const sid = item.getAttribute("data-id");
        if (sid) {
          selectSession(sid, true);
        }
      });
    }

    const filterTabs = view.querySelectorAll(".qq-chat-filter-tab");
    filterTabs.forEach((tab) => {
      tab.addEventListener("click", () => {
        filterTabs.forEach((t) => t.classList.remove("active"));
        tab.classList.add("active");
        currentFilter = tab.getAttribute("data-filter");
        renderSessionList();
      });
    });

    const copyBtn = view.querySelector(".qq-chat-copy-id-btn");
    copyBtn.addEventListener("click", () => {
      const idText = view.querySelector(".qq-chat-header-id").textContent;
      if (idText && navigator.clipboard) {
        navigator.clipboard.writeText(idText).then(() => {
          showToast(currentSessionType === "private" ? "已复制用户ID" : "已复制群号", "success");
        });
      }
    });

    const autoScrollChk = view.querySelector("#qq-chat-autoscroll-chk");
    autoScrollChk.addEventListener("change", (e) => {
      autoScroll = e.target.checked;
      if (autoScroll) scrollToBottom();
    });

    const messagesContainer = view.querySelector(".qq-chat-messages-container");
    messagesContainer.addEventListener("dblclick", e => {
      const avatar = e.target.closest(".qq-chat-msg-avatar[data-sender-id]");
      if (avatar) insertMention(avatar.dataset.senderId, avatar.dataset.senderName);
    });
    messagesContainer.addEventListener("scroll", () => {
      const atBottom =
        messagesContainer.scrollHeight - messagesContainer.scrollTop - messagesContainer.clientHeight < 40;
      if (autoScrollChk.checked !== atBottom) {
        autoScroll = atBottom;
        autoScrollChk.checked = atBottom;
      }
    });

    // Image click delegation to open Lightbox
    messagesContainer.addEventListener("click", (e) => {
      const command = e.target.closest(".qq-chat-inline-command");
      if (command) {
        const rawId = command.closest(".qq-chat-msg-row")?.dataset.rawId;
        const replyId = rawId ? rawId.replace(/&/g, "&amp;").replace(/\[/g, "&#91;").replace(/\]/g, "&#93;").replace(/,/g, "&#44;") : "";
        const prefix = command.dataset.reference === "true" && replyId ? `[CQ:reply,id=${replyId}]` : "";
        insertDraftText(prefix + command.dataset.commandText);
        return;
      }
      if (e.target.closest(".qq-chat-img-retry-link, a")) return;
      const card = e.target.closest(".qq-chat-img-card");
      if (card) {
        const src = card.getAttribute("data-src") || card.querySelector("img")?.src;
        if (src) {
          openImageLightbox(src);
          return;
        }
      }
      const img = e.target.closest("img.qq-chat-img");
      if (img && img.src) {
        openImageLightbox(img.src);
      }
    });

    const refreshMsgBtn = view.querySelector(".qq-chat-btn-refresh");
    refreshMsgBtn.addEventListener("click", () => {
      const activeId = getCurrentActiveId();
      if (activeId) fetchMessages(activeId, false);
    });

    const clearMsgBtn = view.querySelector(".qq-chat-btn-clear");
    clearMsgBtn.addEventListener("click", () => {
      const activeId = getCurrentActiveId();
      if (!activeId) return;
      const confirmText = currentSessionType === "private"
        ? "确定要清空与此用户的私聊本地记录吗？（不影响聊天平台记录）"
        : "确定要清空此群在面板中的聊天记录吗？（不影响群内消息）";
      if (confirm(confirmText)) {
        clearSessionMessages(activeId);
      }
    });

    const textarea = view.querySelector(".qq-chat-textarea");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const counter = view.querySelector(".qq-chat-char-counter");

    textarea.addEventListener("input", () => {
      const len = textarea.value.trim().length;
      counter.textContent = `${len}/1000`;
      sendBtn.disabled = len === 0 || isSending || !getCurrentActiveId();
    });

    textarea.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        sendMessage();
      }
    });

    const scopeTabs = view.querySelectorAll(".qq-chat-scope-tab");
    scopeTabs.forEach((tab) => {
      tab.addEventListener("click", () => {
        scopeTabs.forEach((t) => t.classList.remove("active"));
        tab.classList.add("active");
        onlyWithMsgs = tab.getAttribute("data-scope") === "active";
        sessionPage = 1;
        fetchSessions(false);
      });
    });

    const prevBtn = view.querySelector(".qq-chat-page-prev");
    if (prevBtn) {
      prevBtn.addEventListener("click", () => {
        if (sessionPage > 1) {
          sessionPage--;
          fetchSessions(false);
        }
      });
    }

    const nextBtn = view.querySelector(".qq-chat-page-next");
    if (nextBtn) {
      nextBtn.addEventListener("click", () => {
        if (sessionPage < sessionTotalPages) {
          sessionPage++;
          fetchSessions(false);
        }
      });
    }

    sendBtn.addEventListener("click", () => {
      sendMessage();
    });
  }

  // --------------------------------------------------------------------------
  // Data Fetching
  // --------------------------------------------------------------------------
  async function fetchSessions(showNotice = false) {
    const requestId = ++sessionRequestId;
    const requestedType = currentSessionType;
    try {
      let url = `/sd-api/chat/groups?page=${sessionPage}&pageSize=${sessionPageSize}&sessionType=${currentSessionType}`;
      if (currentSessionType === "group") {
        url += `&hasMessagesOnly=${onlyWithMsgs}`;
      }
      if (searchQuery) {
        url += `&search=${encodeURIComponent(searchQuery)}`;
      }
      const result = await fetchChatJson("sessions", url, 12000, true);
      if (!result) {
        if (requestId === sessionRequestId && requestedType === currentSessionType && !sessionsCache.length && isChatRouteActive) {
          const list = document.querySelector(".qq-chat-group-list");
          if (list) list.innerHTML = '<div class="qq-chat-empty-hint">会话列表暂未加载，请点击刷新列表重试</div>';
        }
        return;
      }
      const { res, data } = result;
      if (!res.ok) {
        if (res.status === 403) showToast("登录已失效，请重新登录", "error");
        return;
      }
      if (requestId !== sessionRequestId || requestedType !== currentSessionType) return;
      sessionsCache = data.items || [];
      sessionTotal = data.total || sessionsCache.length;
      sessionPage = data.page || 1;
      sessionTotalPages = data.totalPages || 1;

      sessionsCache.forEach((s) => knownSessionsMap.set(s.groupId, s));

      renderSessionList();
      renderPaginationBar();
      if (showNotice) showToast(currentSessionType === "private" ? "私聊列表已刷新" : "群列表已刷新", "success");

      // Auto select only on initial load when no session has been selected yet
      const activeId = getCurrentActiveId();
      if (!activeId && sessionsCache.length > 0) {
        const stored = currentSessionType === "private"
          ? sessionStorage.getItem("qq_chat_current_private")
          : sessionStorage.getItem("qq_chat_current_group");
        const foundStored = stored && sessionsCache.find((s) => s.groupId === stored);
        const target = foundStored ? stored : sessionsCache[0].groupId;
        selectSession(target, false);
      } else if (activeId && knownSessionsMap.has(activeId) && document.querySelector(".qq-chat-header-id")?.textContent !== activeId) {
        selectSession(activeId, true);
      }
    } catch (err) {
      console.warn("fetchSessions error:", err);
    }
  }

  function renderPaginationBar() {
    const barEl = document.querySelector(".qq-chat-pagination-bar");
    if (!barEl) return;
    const prevBtn = barEl.querySelector(".qq-chat-page-prev");
    const nextBtn = barEl.querySelector(".qq-chat-page-next");
    const indicator = barEl.querySelector(".qq-chat-page-indicator");
    const unit = currentSessionType === "private" ? "人" : "群";

    if (indicator) {
      indicator.textContent = `${sessionPage} / ${sessionTotalPages} (共 ${sessionTotal} ${unit})`;
    }
    if (prevBtn) {
      prevBtn.disabled = sessionPage <= 1;
    }
    if (nextBtn) {
      nextBtn.disabled = sessionPage >= sessionTotalPages;
    }
  }

  function renderSessionList() {
    const listEl = document.querySelector(".qq-chat-group-list");
    if (!listEl) return;

    let filtered = sessionsCache;

    if (currentFilter === "official") {
      filtered = filtered.filter((s) => s.isOfficial);
    } else if (currentFilter === "active" && currentSessionType === "group") {
      filtered = filtered.filter((s) => s.active);
    }

    if (searchQuery) {
      filtered = filtered.filter((s) => {
        return (
          (s.groupName || "").toLowerCase().includes(searchQuery) ||
          (s.groupId || "").toLowerCase().includes(searchQuery)
        );
      });
    }

    const emptyHint = currentSessionType === "private" ? "无匹配私聊用户" : "无匹配群聊";
    if (filtered.length === 0) {
      listEl.innerHTML = `<div class="qq-chat-empty-hint">${emptyHint}</div>`;
      return;
    }

    const activeId = getCurrentActiveId();
    listEl.innerHTML = "";
    filtered.forEach((s) => {
      const item = document.createElement("div");
      item.className = `qq-chat-group-item ${s.groupId === activeId ? "active" : ""}`;
      item.setAttribute("data-id", s.groupId);

      const color = getAvatarColor(s.groupId);
      const isPrivate = currentSessionType === "private";
      const initial = isPrivate ? getInitials(s.groupName, "私") : getInitials(s.groupName, "群");
      const snippet = s.lastMsg ? formatGroupSnippet(s.lastMsg.content) : "暂无消息";
      const timeStr = s.lastMsg ? formatTime(s.lastMsg.timestamp) : "";

      let badgesHtml = "";
      if (isPrivate) {
        if (s.senderSuffix) {
          badgesHtml += `<span class="qq-chat-id-tag" title="官Bot用户后四位ID: ${escapeHtml(s.senderSuffix)}">#${escapeHtml(s.senderSuffix)}</span>`;
        }
        badgesHtml += `<span class="qq-chat-badge qq-chat-badge-private">私聊</span>`;
        if (s.isOfficial) {
          badgesHtml += `<span class="qq-chat-badge qq-chat-badge-official">官Bot</span>`;
        }
      } else {
        if (s.isOfficial) {
          badgesHtml += `<span class="qq-chat-badge qq-chat-badge-official">官Bot</span>`;
        }
        badgesHtml += `<span class="qq-chat-badge ${s.active ? "qq-chat-badge-status-on" : "qq-chat-badge-status-off"}">${s.active ? "开启" : "关闭"}</span>`;
      }

      item.innerHTML = `
        <div class="qq-chat-group-avatar ${s.isOfficial ? "official" : ""}" style="background-color: ${color}">
          ${avatarHtml(s.avatarUrl, initial)}
        </div>
        <div class="qq-chat-group-content">
          <div class="qq-chat-group-title-row">
            <span class="qq-chat-group-name" title="${escapeHtml(s.groupName || s.groupId)}">${escapeHtml(s.groupName || s.groupId)}</span>
            <span class="qq-chat-group-time">${timeStr}</span>
          </div>
          <div class="qq-chat-group-desc-row">
            <span class="qq-chat-group-snippet" title="${snippet}">${snippet}</span>
            <div class="qq-chat-group-badges">
              ${badgesHtml}
            </div>
          </div>
        </div>
      `;

      listEl.appendChild(item);
    });
  }

  function resetMainHeader(view) {
    if (!view) view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;
    const avatarEl = view.querySelector(".qq-chat-header-avatar");
    const titleEl = view.querySelector(".qq-chat-header-title");
    const typeBadge = view.querySelector(".qq-chat-badge-type");
    const officialBadge = view.querySelector(".qq-chat-badge-official");
    const statusBadge = view.querySelector(".qq-chat-badge-status");
    const idEl = view.querySelector(".qq-chat-header-id");
    const copyBtn = view.querySelector(".qq-chat-copy-id-btn");
    const membersEl = view.querySelector(".qq-chat-header-members");
    const streamEl = view.querySelector(".qq-chat-messages-stream");

    if (avatarEl) avatarEl.style.display = "none";
    if (titleEl) titleEl.textContent = currentSessionType === "private" ? "请选择左侧私聊用户" : "请选择左侧群聊";
    if (typeBadge) typeBadge.style.display = "none";
    if (officialBadge) officialBadge.style.display = "none";
    if (statusBadge) statusBadge.style.display = "none";
    if (idEl) idEl.textContent = "";
    if (copyBtn) copyBtn.style.display = "none";
    if (membersEl) membersEl.textContent = "";
    if (streamEl) {
      streamEl.innerHTML = `<div class="qq-chat-empty-hint">${currentSessionType === "private" ? "请从左侧选择私聊用户开始查看消息" : "请从左侧选择群聊开始查看消息"}</div>`;
    }
  }

  function selectSession(sessionId, forceReload = false) {
    if (!sessionId) return;
    const isPrivate = currentSessionType === "private";
    const currentActive = isPrivate ? currentPrivateId : currentGroupId;
    if (currentActive === sessionId && !forceReload) return;
    cancelChatRead("messages");
    cancelChatRead("history");
    cancelChatRead("profile");
    closeResourcePicker();

    if (isPrivate) {
      currentPrivateId = sessionId;
      sessionStorage.setItem("qq_chat_current_private", sessionId);
    } else {
      currentGroupId = sessionId;
      sessionStorage.setItem("qq_chat_current_group", sessionId);
    }

    // Highlight sidebar
    document.querySelectorAll(".qq-chat-group-item").forEach((el) => {
      if (el.getAttribute("data-id") === sessionId) {
        el.classList.add("active");
      } else {
        el.classList.remove("active");
      }
    });

    const session = knownSessionsMap.get(sessionId) || sessionsCache.find((s) => s.groupId === sessionId);
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;

    const avatarEl = view.querySelector(".qq-chat-header-avatar");
    const titleEl = view.querySelector(".qq-chat-header-title");
    const typeBadge = view.querySelector(".qq-chat-badge-type");
    const officialBadge = view.querySelector(".qq-chat-badge-official");
    const statusBadge = view.querySelector(".qq-chat-badge-status");
    const idEl = view.querySelector(".qq-chat-header-id");
    const copyBtn = view.querySelector(".qq-chat-copy-id-btn");
    const membersEl = view.querySelector(".qq-chat-header-members");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const textarea = view.querySelector(".qq-chat-textarea");

    if (session) {
      avatarEl.style.display = "flex";
      avatarEl.innerHTML = avatarHtml(session.avatarUrl, isPrivate ? getInitials(session.groupName, "私") : getInitials(session.groupName, "群"));
      avatarEl.style.backgroundColor = getAvatarColor(session.groupId);
      titleEl.textContent = session.groupName || session.groupId;

      if (typeBadge) {
        typeBadge.style.display = isPrivate ? "inline-block" : "none";
        typeBadge.className = "qq-chat-badge qq-chat-badge-type qq-chat-badge-private";
        typeBadge.textContent = "私聊";
      }

      officialBadge.style.display = session.isOfficial ? "inline-block" : "none";

      if (isPrivate) {
        statusBadge.style.display = "none";
        membersEl.textContent = "";
      } else {
        statusBadge.style.display = "inline-block";
        statusBadge.className = `qq-chat-badge qq-chat-badge-status ${session.active ? "qq-chat-badge-status-on" : "qq-chat-badge-status-off"}`;
        statusBadge.textContent = session.active ? "Bot 已开启" : "Bot 已关闭";
        membersEl.textContent = session.memberNum > 0 ? `· ${session.memberNum} 人` : "";
      }

      idEl.textContent = session.groupId;
      copyBtn.style.display = "inline-block";
    } else {
      avatarEl.style.display = "flex";
      avatarEl.textContent = isPrivate ? getInitials(sessionId, "私") : getInitials(sessionId, "群");
      avatarEl.style.backgroundColor = getAvatarColor(sessionId);
      titleEl.textContent = sessionId;
      if (typeBadge) {
        typeBadge.style.display = isPrivate ? "inline-block" : "none";
        typeBadge.className = "qq-chat-badge qq-chat-badge-type qq-chat-badge-private";
        typeBadge.textContent = "私聊";
      }
      officialBadge.style.display = "none";
      statusBadge.style.display = "none";
      idEl.textContent = sessionId;
      copyBtn.style.display = "inline-block";
      membersEl.textContent = "";
    }

    if (sendBtn && textarea) {
      sendBtn.disabled = textarea.value.trim().length === 0 || isSending;
    }
    updateMarkdownMode();

    fetchMessages(sessionId, true);
    refreshSessionProfile(sessionId);
  }

  async function fetchMessages(sessionId, clearBefore = false) {
    if (!sessionId) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view || view.style.display === "none") return;
    if (!clearBefore && (chatReads.has("messages") || chatReads.has("history"))) return;
    const requestId = ++messagesRequestId;
    const requestedType = currentSessionType;

    const streamEl = view.querySelector(".qq-chat-messages-stream");
    if (clearBefore) {
      streamEl.innerHTML = '<div class="qq-chat-empty-hint">正在拉取消息记录...</div>';
      currentOldestTimestamp = 0;
      hasMoreHistory = false;
      lastRenderedSignature = "";
    }

    try {
      const url = `/sd-api/chat/messages?sessionId=${encodeURIComponent(sessionId)}&sessionType=${encodeURIComponent(currentSessionType)}&limit=${messagesLimit}`;
      const result = await fetchChatJson("messages", url, 8000, clearBefore);
      if (!result) {
        if (clearBefore && requestId === messagesRequestId && getCurrentActiveId() === sessionId && currentSessionType === requestedType && streamEl.querySelector(".qq-chat-empty-hint")) {
          streamEl.querySelector(".qq-chat-empty-hint").textContent = "消息暂时未加载，正在自动重试...";
        }
        return;
      }
      const { res, data } = result;
      if (!res.ok) return;
      if (getCurrentActiveId() !== sessionId) return;

      if (getCurrentActiveId() !== sessionId || requestedType !== currentSessionType || requestId !== messagesRequestId) return;

      const messages = data.messages || [];
      hasMoreHistory = !!data.hasMore;
      const totalMsgCount = data.total || messages.length;

      const signature = `${sessionId}:${messages.length}:${messages.length > 0 ? messages[messages.length - 1].timestamp + "_" + (messages[messages.length - 1].content || "") : ""}`;
      if (!clearBefore && signature === lastRenderedSignature) {
        return;
      }
      lastRenderedSignature = signature;

      if (messages.length === 0) {
        const emptyMsg = currentSessionType === "private"
          ? "该私聊暂无聊天记录，在私聊中发言或通过下方发送消息即可显示"
          : "该群暂无聊天记录，在群内发言或通过下方发送消息即可显示";
        streamEl.innerHTML = `<div class="qq-chat-empty-hint">${emptyMsg}</div>`;
        return;
      }

      currentOldestTimestamp = messages[0].timestamp;
      renderMessages(messages, streamEl, false, hasMoreHistory, totalMsgCount - messages.length);
    } catch (err) {
      console.warn("fetchMessages error:", err);
    }
  }

  async function loadEarlierMessages(sessionId) {
    if (!sessionId || isLoadingHistory || !currentOldestTimestamp) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;
    const requestedType = currentSessionType;

    const container = view.querySelector(".qq-chat-messages-container");
    const streamEl = view.querySelector(".qq-chat-messages-stream");
    const loadBtn = view.querySelector(".qq-chat-load-more-btn");
    const previousLabel = loadBtn?.textContent;

    isLoadingHistory = true;
    if (loadBtn) {
      loadBtn.disabled = true;
      loadBtn.textContent = "正在加载更早的消息...";
    }

    try {
      const url = `/sd-api/chat/messages?sessionId=${encodeURIComponent(sessionId)}&sessionType=${encodeURIComponent(currentSessionType)}&limit=${messagesLimit}&before=${currentOldestTimestamp}`;
      const result = await fetchChatJson("history", url, 10000);
      if (!result) return;
      const { res, data } = result;
      if (!res.ok) return;
      if (getCurrentActiveId() !== sessionId) return;

      if (getCurrentActiveId() !== sessionId || currentSessionType !== requestedType) return;
      const earlier = data.messages || [];
      hasMoreHistory = !!data.hasMore;

      if (earlier.length > 0) {
        currentOldestTimestamp = earlier[0].timestamp;
        const prevScrollHeight = container.scrollHeight;
        const prevScrollTop = container.scrollTop;

        // Prepend earlier messages
        prependEarlierMessages(earlier, streamEl, hasMoreHistory, (data.total || 0) - earlier.length);

        // Keep scroll position stable
        const newScrollHeight = container.scrollHeight;
        container.scrollTop = prevScrollTop + (newScrollHeight - prevScrollHeight);
      } else {
        hasMoreHistory = false;
        const bar = streamEl.querySelector(".qq-chat-load-more-bar");
        if (bar) bar.remove();
      }
    } catch (err) {
      console.warn("loadEarlierMessages error:", err);
      showToast("加载历史消息失败", "error");
    } finally {
      isLoadingHistory = false;
      if (loadBtn?.isConnected && getCurrentActiveId() === sessionId && currentSessionType === requestedType) {
        loadBtn.disabled = false;
        loadBtn.textContent = previousLabel;
      }
    }
  }

  function buildMessageRow(msg) {
    const row = document.createElement("div");
    row.className = `qq-chat-msg-row ${msg.isBot ? "qq-chat-msg-right" : "qq-chat-msg-left"}`;
    if (msg.rawId != null) row.dataset.rawId = String(msg.rawId);

    const isPrivate = currentSessionType === "private";
    const avatarColor = msg.isBot ? "" : getAvatarColor(msg.senderId);
    const avatarInitial = msg.isBot ? "海" : (getInitials(msg.senderName, isPrivate ? "私" : "用"));
    const formattedContent = parseContentToHtml(msg.content);
    const timeStr = formatTime(msg.timestamp);

    if (msg.isBot) {
      row.innerHTML = `
        <div class="qq-chat-msg-content-wrap">
          <div class="qq-chat-msg-meta">
            <span class="qq-chat-msg-time">${timeStr}</span>
            <span class="qq-chat-bot-tag">BOT</span>
            <span class="qq-chat-msg-sender">${escapeHtml(msg.senderName || "海豹核心")}</span>
          </div>
          <div class="qq-chat-msg-bubble qq-chat-bubble-bot">${formattedContent}</div>
        </div>
        <div class="qq-chat-msg-avatar qq-chat-avatar-bot" title="${escapeHtml(msg.senderName || "海豹核心")}">
          ${avatarHtml(msg.senderAvatar, avatarInitial)}
        </div>
      `;
    } else {
      const suffixBadge = msg.senderSuffix
        ? `<span class="qq-chat-id-tag" title="官Bot用户后四位ID: ${msg.senderSuffix}">#${msg.senderSuffix}</span>`
        : "";

      row.innerHTML = `
        <div class="qq-chat-msg-avatar" data-sender-id="${escapeHtml(msg.senderId)}" data-sender-name="${escapeHtml(msg.senderName)}" style="background-color: ${avatarColor}" title="${escapeHtml(msg.senderDisplay || msg.senderName)}${isPrivate ? "" : " · 双击快捷 @"}">
          ${avatarHtml(msg.senderAvatar, avatarInitial)}
        </div>
        <div class="qq-chat-msg-content-wrap">
          <div class="qq-chat-msg-meta">
            <span class="qq-chat-msg-sender">${escapeHtml(msg.senderName || (isPrivate ? "私聊用户" : "用户"))}</span>
            ${suffixBadge}
            <span class="qq-chat-msg-time">${timeStr}</span>
          </div>
          <div class="qq-chat-msg-bubble">${formattedContent}</div>
        </div>
      `;
    }
    return row;
  }

  function renderMessages(messages, streamEl, isIncremental = false, hasMore = false, remaining = 0) {
    streamEl.innerHTML = "";

    if (hasMore) {
      const loadMoreBar = document.createElement("div");
      loadMoreBar.className = "qq-chat-load-more-bar";
      loadMoreBar.innerHTML = `<button type="button" class="qq-chat-load-more-btn">↑ 查看更早的历史记录${remaining > 0 ? ` (还剩 ${remaining} 条)` : ""}</button>`;
      loadMoreBar.querySelector("button").addEventListener("click", () => {
        loadEarlierMessages(getCurrentActiveId());
      });
      streamEl.appendChild(loadMoreBar);
    }

    let lastTime = 0;
    messages.forEach((msg) => {
      if (msg.timestamp - lastTime > 300) {
        const timeDivider = document.createElement("div");
        timeDivider.className = "qq-chat-time-divider";
        timeDivider.innerHTML = `<span>${formatFullTime(msg.timestamp)}</span>`;
        streamEl.appendChild(timeDivider);
      }
      lastTime = msg.timestamp;
      streamEl.appendChild(buildMessageRow(msg));
    });

    if (autoScroll) {
      scrollToBottom();
    }
  }

  function prependEarlierMessages(earlier, streamEl, hasMore, remaining) {
    const oldBar = streamEl.querySelector(".qq-chat-load-more-bar");
    if (oldBar) oldBar.remove();

    const fragment = document.createDocumentFragment();

    if (hasMore) {
      const loadMoreBar = document.createElement("div");
      loadMoreBar.className = "qq-chat-load-more-bar";
      loadMoreBar.innerHTML = `<button type="button" class="qq-chat-load-more-btn">↑ 查看更早的历史记录${remaining > 0 ? ` (还剩 ${remaining} 条)` : ""}</button>`;
      loadMoreBar.querySelector("button").addEventListener("click", () => {
        loadEarlierMessages(getCurrentActiveId());
      });
      fragment.appendChild(loadMoreBar);
    }

    let lastTime = 0;
    earlier.forEach((msg) => {
      if (msg.timestamp - lastTime > 300) {
        const timeDivider = document.createElement("div");
        timeDivider.className = "qq-chat-time-divider";
        timeDivider.innerHTML = `<span>${formatFullTime(msg.timestamp)}</span>`;
        fragment.appendChild(timeDivider);
      }
      lastTime = msg.timestamp;
      fragment.appendChild(buildMessageRow(msg));
    });

    streamEl.insertBefore(fragment, streamEl.firstChild);
  }

  function scrollToBottom() {
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;
    const container = view.querySelector(".qq-chat-messages-container");
    if (container) {
      window.requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    }
  }

  async function sendMessage(resourcePath = "") {
    const activeId = getCurrentActiveId();
    if (!activeId || isSending) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;

    const textarea = view.querySelector(".qq-chat-textarea");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const text = textarea.value.trim();
    const originalDraft = textarea.value;
    const sendingType = currentSessionType;
    const markdown = !resourcePath && !view.querySelector(".qq-chat-markdown-mode").disabled && view.querySelector(".qq-chat-markdown-mode").checked;

    if (!text && !resourcePath) return;

    isSending = true;
    sendBtn.disabled = true;
    const imageSendBtn = view.querySelector(".qq-chat-resource-send");
    imageSendBtn.disabled = true;
    sendBtn.querySelector("span").textContent = "发送中...";

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 15000);

    try {
      const res = await fetch("/sd-api/chat/send", {
        method: "POST",
        headers: getHeaders(),
        body: JSON.stringify({
          targetId: activeId,
          groupId: activeId,
          sessionType: sendingType,
          message: resourcePath ? "" : text,
          resourcePath,
          markdown,
        }),
        signal: controller.signal,
      });
      clearTimeout(timeoutId);

      if (!res.ok) {
        if (res.status === 502) {
          showToast("服务暂不可用 (502)：机器人端点可能离线或网络异常，请在控制台检查连接", "error");
          return;
        }
        if (res.status === 504) {
          showToast("网关超时 (504)：消息发送响应超时，请稍后重试", "error");
          return;
        }
        const err = await res.json().catch(() => ({}));
        showToast(err.error || `发送失败 (${res.status})`, "error");
        return;
      }

      // Success: clear input and refresh messages
      if (resourcePath) closeResourcePicker();
      if (!resourcePath && getCurrentActiveId() === activeId && currentSessionType === sendingType && textarea.value === originalDraft) {
        textarea.value = "";
        view.querySelector(".qq-chat-char-counter").textContent = "0/1000";
      }
      showToast("消息已发送", "success");

      if (getCurrentActiveId() === activeId && currentSessionType === sendingType) {
        await fetchMessages(activeId, false);
        scrollToBottom();
      }
    } catch (err) {
      clearTimeout(timeoutId);
      if (err.name === "AbortError") {
        showToast("发送请求超时 (15s)，请检查网络与机器人连接", "error");
      } else {
        showToast("发送异常，请检查网络与连接", "error");
      }
    } finally {
      isSending = false;
      sendBtn.disabled = textarea.value.trim().length === 0 || !getCurrentActiveId();
      imageSendBtn.disabled = !resourceSelection;
      sendBtn.querySelector("span").textContent = "发送";
      textarea.focus();
    }
  }

  async function clearSessionMessages(sessionId) {
    try {
      const res = await fetch("/sd-api/chat/clear", {
        method: "POST",
        headers: getHeaders(),
        body: JSON.stringify({
          targetId: sessionId,
          groupId: sessionId,
          sessionType: currentSessionType,
        }),
      });
      if (res.ok) {
        showToast("已清空本地记录", "success");
        fetchMessages(sessionId, true);
        fetchSessions();
      }
    } catch (err) {
      showToast("清空失败", "error");
    }
  }

  // --------------------------------------------------------------------------
  // Route Switching Controller & View Mounting
  // --------------------------------------------------------------------------
  function ensureChatViewMounted() {
    const isChat = window.location.hash.includes(CHAT_ROUTE);
    if (!isChat) return;

    const mainContainer = document.querySelector(".main-container");
    let chatView = document.getElementById("sealdice-qq-chat-view");
    if (!chatView) {
      chatView = getOrCreateChatView();
    }

    if (mainContainer) {
      mainContainer.classList.add("is-qq-chat-active");
      if (chatView.parentNode !== mainContainer) {
        mainContainer.appendChild(chatView);
      }
      Array.from(mainContainer.children).forEach((child) => {
        if (child !== chatView) {
          if (child.style.display !== "none") {
            child.__qq_prev_display = child.style.display;
          }
          child.style.display = "none";
        }
      });
    } else {
      const app = document.getElementById("app") || document.body;
      if (chatView.parentNode !== app) {
        app.appendChild(chatView);
      }
    }

    if (chatView.style.display !== "flex") {
      chatView.style.display = "flex";
    }
  }

  function handleRouteChange() {
    hookVueRouter();
    injectNavTab();
    updateNavActiveState();

    const isChat = window.location.hash.includes(CHAT_ROUTE);

    if (isChat) {
      ensureChatViewMounted();
      if (!isChatRouteActive) {
        isChatRouteActive = true;
        fetchSessions(false);
        startPolling();
      }
    } else {
      if (isChatRouteActive) {
        isChatRouteActive = false;
        const chatView = document.getElementById("sealdice-qq-chat-view");
        if (chatView) chatView.style.display = "none";
        const mainContainer = document.querySelector(".main-container");
        if (mainContainer) {
          mainContainer.classList.remove("is-qq-chat-active");
          Array.from(mainContainer.children).forEach((child) => {
            if (child !== chatView && child.__qq_prev_display !== undefined) {
              child.style.display = child.__qq_prev_display;
              delete child.__qq_prev_display;
            }
          });
        }
        stopPolling();
        closeResourcePicker();
      }
    }
  }

  function startPolling() {
    stopPolling(false);
    if (document.hidden) return;
    pollTimer = window.setInterval(() => {
      const isChat = window.location.hash.includes(CHAT_ROUTE);
      const view = document.getElementById("sealdice-qq-chat-view");
      if (isChat && view && view.style.display !== "none") {
        const activeId = getCurrentActiveId();
        if (activeId) {
          fetchMessages(activeId, false);
          if (++profilePollCount % 12 === 0) refreshSessionProfile(activeId);
        }
      } else if (!isChat) {
        stopPolling();
      }
    }, 2500);
  }

  function stopPolling(cancelReads = true) {
    if (pollTimer) {
      window.clearInterval(pollTimer);
      pollTimer = null;
    }
    if (cancelReads) {
      for (const key of Array.from(chatReads.keys())) cancelChatRead(key);
    }
  }

  // --------------------------------------------------------------------------
  // Initialization & Mutation Observer
  // --------------------------------------------------------------------------
  window.addEventListener("hashchange", handleRouteChange);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      stopPolling();
    } else if (window.location.hash.includes(CHAT_ROUTE)) {
      startPolling();
      if (!sessionsCache.length) fetchSessions(false);
      const activeId = getCurrentActiveId();
      if (activeId) fetchMessages(activeId, false);
    }
  });

  const observer = new MutationObserver(() => {
    hookVueRouter();
    injectNavTab();
    if (window.location.hash.includes(CHAT_ROUTE)) {
      ensureChatViewMounted();
    }
  });

  const app = document.getElementById("app") || document.body;
  observer.observe(app, { childList: true, subtree: true });

  window.setTimeout(handleRouteChange, 80);
})();
