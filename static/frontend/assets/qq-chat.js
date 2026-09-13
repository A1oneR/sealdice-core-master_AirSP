(function () {
  const CHAT_ROUTE = "/qq-chat";
  let pollTimer = null;
  let currentGroupId = sessionStorage.getItem("qq_chat_current_group") || "";
  let groupsCache = [];
  let currentFilter = "all";
  let searchQuery = "";
  let autoScroll = true;
  let isSending = false;
  let groupPage = 1;
  let groupPageSize = 20;
  let groupTotalPages = 1;
  let groupTotal = 0;
  let onlyWithMsgs = true;
  let messagesLimit = 35;
  let currentOldestTimestamp = 0;
  let hasMoreHistory = false;
  let isLoadingHistory = false;
  const knownGroupsMap = new Map();
  let lastRenderedSignature = "";
  let isChatRouteActive = false;

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

  function getInitials(name) {
    if (!name) return "群";
    const clean = name.replace(/^[\[\(（【]/, "").trim();
    return clean ? clean.slice(0, 1) : "群";
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

    // 1. [CQ:image, ...]
    let text = String(content).replace(/\[CQ:image(?:,([^\]]*))?\]/gi, (match, paramStr) => {
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
        <div class="qq-chat-search-wrap">
          <input type="text" class="qq-chat-search-input" placeholder="搜索群聊名称 / ID..." />
          <button type="button" class="qq-chat-icon-btn qq-chat-refresh-groups-btn" title="刷新群列表">&#x21bb;</button>
        </div>
        <div class="qq-chat-scope-tabs">
          <button type="button" class="qq-chat-scope-tab active" data-scope="active">仅看有发言群</button>
          <button type="button" class="qq-chat-scope-tab" data-scope="all">全部群聊</button>
        </div>
        <div class="qq-chat-filter-tabs">
          <button type="button" class="qq-chat-filter-tab active" data-filter="all">全部</button>
          <button type="button" class="qq-chat-filter-tab" data-filter="official">官Bot</button>
          <button type="button" class="qq-chat-filter-tab" data-filter="active">已开启</button>
        </div>
        <div class="qq-chat-group-list">
          <div class="qq-chat-empty-hint">正在加载群聊...</div>
        </div>
        <div class="qq-chat-pagination-bar">
          <button type="button" class="qq-chat-page-btn qq-chat-page-prev" disabled>上一页</button>
          <span class="qq-chat-page-indicator">1 / 1 (0群)</span>
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
                <span class="qq-chat-header-title">请选择左侧群聊</span>
                <span class="qq-chat-badge qq-chat-badge-official" style="display:none;">官Bot</span>
                <span class="qq-chat-badge qq-chat-badge-status" style="display:none;"></span>
              </div>
              <div class="qq-chat-header-subtitle">
                <span class="qq-chat-header-id"></span>
                <button type="button" class="qq-chat-copy-id-btn" style="display:none;" title="复制群ID">复制</button>
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
            <div class="qq-chat-empty-hint">请从左侧选择群聊开始查看消息</div>
          </div>
        </div>

        <footer class="qq-chat-footer">
          <div class="qq-chat-input-toolbar">
            <span class="qq-chat-toolbar-tip">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>
              以骰娘身份发送消息 · 纯文本发送无任何额外标识
            </span>
            <span class="qq-chat-char-counter">0/1000</span>
          </div>
          <div class="qq-chat-input-box">
            <textarea class="qq-chat-textarea" placeholder="输入要发送到群聊的信息... (Enter 发送, Ctrl+Enter 换行)" rows="2" maxlength="1000"></textarea>
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

  function bindChatViewEvents(view) {
    const searchInput = view.querySelector(".qq-chat-search-input");
    searchInput.addEventListener("input", (e) => {
      searchQuery = e.target.value.trim().toLowerCase();
      renderGroupList();
    });

    const refreshGroupsBtn = view.querySelector(".qq-chat-refresh-groups-btn");
    refreshGroupsBtn.addEventListener("click", () => {
      fetchGroups(true);
    });

    const groupListEl = view.querySelector(".qq-chat-group-list");
    if (groupListEl) {
      groupListEl.addEventListener("click", (e) => {
        const item = e.target.closest(".qq-chat-group-item");
        if (!item) return;
        const gid = item.getAttribute("data-id");
        if (gid) {
          selectGroup(gid, true);
        }
      });
    }

    const filterTabs = view.querySelectorAll(".qq-chat-filter-tab");
    filterTabs.forEach((tab) => {
      tab.addEventListener("click", () => {
        filterTabs.forEach((t) => t.classList.remove("active"));
        tab.classList.add("active");
        currentFilter = tab.getAttribute("data-filter");
        renderGroupList();
      });
    });

    const copyBtn = view.querySelector(".qq-chat-copy-id-btn");
    copyBtn.addEventListener("click", () => {
      const idText = view.querySelector(".qq-chat-header-id").textContent;
      if (idText && navigator.clipboard) {
        navigator.clipboard.writeText(idText).then(() => showToast("已复制群号", "success"));
      }
    });

    const autoScrollChk = view.querySelector("#qq-chat-autoscroll-chk");
    autoScrollChk.addEventListener("change", (e) => {
      autoScroll = e.target.checked;
      if (autoScroll) scrollToBottom();
    });

    const messagesContainer = view.querySelector(".qq-chat-messages-container");
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
      if (currentGroupId) fetchMessages(currentGroupId, false);
    });

    const clearMsgBtn = view.querySelector(".qq-chat-btn-clear");
    clearMsgBtn.addEventListener("click", () => {
      if (!currentGroupId) return;
      if (confirm("确定要清空此群在面板中的聊天记录吗？（不影响群内消息）")) {
        clearGroupMessages(currentGroupId);
      }
    });

    const textarea = view.querySelector(".qq-chat-textarea");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const counter = view.querySelector(".qq-chat-char-counter");

    textarea.addEventListener("input", () => {
      const len = textarea.value.trim().length;
      counter.textContent = `${len}/1000`;
      sendBtn.disabled = len === 0 || isSending || !currentGroupId;
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
        groupPage = 1;
        fetchGroups(false);
      });
    });

    const prevBtn = view.querySelector(".qq-chat-page-prev");
    if (prevBtn) {
      prevBtn.addEventListener("click", () => {
        if (groupPage > 1) {
          groupPage--;
          fetchGroups(false);
        }
      });
    }

    const nextBtn = view.querySelector(".qq-chat-page-next");
    if (nextBtn) {
      nextBtn.addEventListener("click", () => {
        if (groupPage < groupTotalPages) {
          groupPage++;
          fetchGroups(false);
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
  async function fetchGroups(showNotice = false) {
    try {
      const url = `/sd-api/chat/groups?page=${groupPage}&pageSize=${groupPageSize}&hasMessagesOnly=${onlyWithMsgs}`;
      const res = await fetch(url, {
        headers: getHeaders(),
      });
      if (!res.ok) {
        if (res.status === 403) showToast("登录已失效，请重新登录", "error");
        return;
      }
      const data = await res.json();
      groupsCache = data.items || [];
      groupTotal = data.total || groupsCache.length;
      groupPage = data.page || 1;
      groupTotalPages = data.totalPages || 1;

      groupsCache.forEach((g) => knownGroupsMap.set(g.groupId, g));

      renderGroupList();
      renderPaginationBar();
      if (showNotice) showToast("群列表已刷新", "success");

      // Auto select only on initial load when no group has been selected yet
      if (!currentGroupId && groupsCache.length > 0) {
        const stored = sessionStorage.getItem("qq_chat_current_group");
        const foundStored = stored && groupsCache.find((g) => g.groupId === stored);
        const target = foundStored ? stored : groupsCache[0].groupId;
        selectGroup(target, false);
      }
    } catch (err) {
      console.warn("fetchGroups error:", err);
    }
  }

  function renderPaginationBar() {
    const barEl = document.querySelector(".qq-chat-pagination-bar");
    if (!barEl) return;
    const prevBtn = barEl.querySelector(".qq-chat-page-prev");
    const nextBtn = barEl.querySelector(".qq-chat-page-next");
    const indicator = barEl.querySelector(".qq-chat-page-indicator");

    if (indicator) {
      indicator.textContent = `${groupPage} / ${groupTotalPages} (共 ${groupTotal} 群)`;
    }
    if (prevBtn) {
      prevBtn.disabled = groupPage <= 1;
    }
    if (nextBtn) {
      nextBtn.disabled = groupPage >= groupTotalPages;
    }
  }

  function renderGroupList() {
    const listEl = document.querySelector(".qq-chat-group-list");
    if (!listEl) return;

    let filtered = groupsCache;

    if (currentFilter === "official") {
      filtered = filtered.filter((g) => g.isOfficial);
    } else if (currentFilter === "active") {
      filtered = filtered.filter((g) => g.active);
    }

    if (searchQuery) {
      filtered = filtered.filter((g) => {
        return (
          (g.groupName || "").toLowerCase().includes(searchQuery) ||
          (g.groupId || "").toLowerCase().includes(searchQuery)
        );
      });
    }

    if (filtered.length === 0) {
      listEl.innerHTML = '<div class="qq-chat-empty-hint">无匹配群聊</div>';
      return;
    }

    listEl.innerHTML = "";
    filtered.forEach((g) => {
      const item = document.createElement("div");
      item.className = `qq-chat-group-item ${g.groupId === currentGroupId ? "active" : ""}`;
      item.setAttribute("data-id", g.groupId);

      const color = getAvatarColor(g.groupId);
      const initial = getInitials(g.groupName);
      const snippet = g.lastMsg ? formatGroupSnippet(g.lastMsg.content) : "暂无消息";
      const timeStr = g.lastMsg ? formatTime(g.lastMsg.timestamp) : "";

      item.innerHTML = `
        <div class="qq-chat-group-avatar ${g.isOfficial ? "official" : ""}" style="background-color: ${color}">
          ${initial}
        </div>
        <div class="qq-chat-group-content">
          <div class="qq-chat-group-title-row">
            <span class="qq-chat-group-name" title="${escapeHtml(g.groupName)}">${escapeHtml(g.groupName)}</span>
            <span class="qq-chat-group-time">${timeStr}</span>
          </div>
          <div class="qq-chat-group-desc-row">
            <span class="qq-chat-group-snippet" title="${snippet}">${snippet}</span>
            <div class="qq-chat-group-badges">
              ${g.isOfficial ? '<span class="qq-chat-badge qq-chat-badge-official">官Bot</span>' : ""}
              <span class="qq-chat-badge ${g.active ? "qq-chat-badge-status-on" : "qq-chat-badge-status-off"}">
                ${g.active ? "开启" : "关闭"}
              </span>
            </div>
          </div>
        </div>
      `;

      listEl.appendChild(item);
    });
  }

  function selectGroup(groupId, forceReload = false) {
    if (!groupId) return;
    if (currentGroupId === groupId && !forceReload) return;
    currentGroupId = groupId;
    sessionStorage.setItem("qq_chat_current_group", groupId);

    // Highlight sidebar
    document.querySelectorAll(".qq-chat-group-item").forEach((el) => {
      if (el.getAttribute("data-id") === groupId) {
        el.classList.add("active");
      } else {
        el.classList.remove("active");
      }
    });

    // Update Header
    const group = knownGroupsMap.get(groupId) || groupsCache.find((g) => g.groupId === groupId);
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;

    const avatarEl = view.querySelector(".qq-chat-header-avatar");
    const titleEl = view.querySelector(".qq-chat-header-title");
    const officialBadge = view.querySelector(".qq-chat-badge-official");
    const statusBadge = view.querySelector(".qq-chat-badge-status");
    const idEl = view.querySelector(".qq-chat-header-id");
    const copyBtn = view.querySelector(".qq-chat-copy-id-btn");
    const membersEl = view.querySelector(".qq-chat-header-members");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const textarea = view.querySelector(".qq-chat-textarea");

    if (group) {
      avatarEl.style.display = "flex";
      avatarEl.textContent = getInitials(group.groupName);
      avatarEl.style.backgroundColor = getAvatarColor(group.groupId);
      titleEl.textContent = group.groupName || group.groupId;
      officialBadge.style.display = group.isOfficial ? "inline-block" : "none";

      statusBadge.style.display = "inline-block";
      statusBadge.className = `qq-chat-badge ${group.active ? "qq-chat-badge-status-on" : "qq-chat-badge-status-off"}`;
      statusBadge.textContent = group.active ? "Bot 已开启" : "Bot 已关闭";

      idEl.textContent = group.groupId;
      copyBtn.style.display = "inline-block";
      membersEl.textContent = group.memberNum > 0 ? `· ${group.memberNum} 人` : "";
    } else {
      avatarEl.style.display = "flex";
      avatarEl.textContent = getInitials(groupId);
      avatarEl.style.backgroundColor = getAvatarColor(groupId);
      titleEl.textContent = groupId;
      officialBadge.style.display = "none";
      statusBadge.style.display = "none";
      idEl.textContent = groupId;
      copyBtn.style.display = "inline-block";
      membersEl.textContent = "";
    }

    if (sendBtn && textarea) {
      sendBtn.disabled = textarea.value.trim().length === 0 || isSending;
    }

    fetchMessages(groupId, true);
  }

  async function fetchMessages(groupId, clearBefore = false) {
    if (!groupId) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view || view.style.display === "none") return;

    const streamEl = view.querySelector(".qq-chat-messages-stream");
    if (clearBefore) {
      streamEl.innerHTML = '<div class="qq-chat-empty-hint">正在拉取消息记录...</div>';
      currentOldestTimestamp = 0;
      hasMoreHistory = false;
      lastRenderedSignature = "";
    }

    try {
      const url = `/sd-api/chat/messages?groupId=${encodeURIComponent(groupId)}&limit=${messagesLimit}`;
      const res = await fetch(url, { headers: getHeaders() });
      if (!res.ok) return;
      if (currentGroupId !== groupId) return;

      const data = await res.json();
      if (currentGroupId !== groupId) return;

      const messages = data.messages || [];
      hasMoreHistory = !!data.hasMore;
      const totalMsgCount = data.total || messages.length;

      const signature = `${groupId}:${messages.length}:${messages.length > 0 ? messages[messages.length - 1].timestamp + "_" + (messages[messages.length - 1].content || "") : ""}`;
      if (!clearBefore && signature === lastRenderedSignature) {
        return;
      }
      lastRenderedSignature = signature;

      if (messages.length === 0) {
        streamEl.innerHTML = '<div class="qq-chat-empty-hint">该群暂无聊天记录，在群内发言或通过下方发送消息即可显示</div>';
        return;
      }

      currentOldestTimestamp = messages[0].timestamp;
      renderMessages(messages, streamEl, false, hasMoreHistory, totalMsgCount - messages.length);
    } catch (err) {
      console.warn("fetchMessages error:", err);
    }
  }

  async function loadEarlierMessages(groupId) {
    if (!groupId || isLoadingHistory || !currentOldestTimestamp) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;

    const container = view.querySelector(".qq-chat-messages-container");
    const streamEl = view.querySelector(".qq-chat-messages-stream");
    const loadBtn = view.querySelector(".qq-chat-load-more-btn");

    isLoadingHistory = true;
    if (loadBtn) {
      loadBtn.disabled = true;
      loadBtn.textContent = "正在加载更早的消息...";
    }

    try {
      const url = `/sd-api/chat/messages?groupId=${encodeURIComponent(groupId)}&limit=${messagesLimit}&before=${currentOldestTimestamp}`;
      const res = await fetch(url, { headers: getHeaders() });
      if (!res.ok) return;
      if (currentGroupId !== groupId) return;

      const data = await res.json();
      if (currentGroupId !== groupId) return;
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
    }
  }

  function buildMessageRow(msg) {
    const row = document.createElement("div");
    row.className = `qq-chat-msg-row ${msg.isBot ? "qq-chat-msg-right" : "qq-chat-msg-left"}`;

    const avatarColor = msg.isBot ? "" : getAvatarColor(msg.senderId);
    const avatarInitial = msg.isBot ? "海" : getInitials(msg.senderName);
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
          ${avatarInitial}
        </div>
      `;
    } else {
      const suffixBadge = msg.senderSuffix
        ? `<span class="qq-chat-id-tag" title="官Bot用户后四位ID: ${msg.senderSuffix}">#${msg.senderSuffix}</span>`
        : "";

      row.innerHTML = `
        <div class="qq-chat-msg-avatar" style="background-color: ${avatarColor}" title="${escapeHtml(msg.senderDisplay || msg.senderName)}">
          ${avatarInitial}
        </div>
        <div class="qq-chat-msg-content-wrap">
          <div class="qq-chat-msg-meta">
            <span class="qq-chat-msg-sender">${escapeHtml(msg.senderName || "用户")}</span>
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
        loadEarlierMessages(currentGroupId);
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
        loadEarlierMessages(currentGroupId);
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

  async function sendMessage() {
    if (!currentGroupId || isSending) return;
    const view = document.getElementById("sealdice-qq-chat-view");
    if (!view) return;

    const textarea = view.querySelector(".qq-chat-textarea");
    const sendBtn = view.querySelector(".qq-chat-send-btn");
    const text = textarea.value.trim();

    if (!text) return;

    isSending = true;
    sendBtn.disabled = true;
    sendBtn.querySelector("span").textContent = "发送中...";

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 15000);

    try {
      const res = await fetch("/sd-api/chat/send", {
        method: "POST",
        headers: getHeaders(),
        body: JSON.stringify({
          groupId: currentGroupId,
          message: text,
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
      textarea.value = "";
      view.querySelector(".qq-chat-char-counter").textContent = "0/1000";
      showToast("消息已发送", "success");

      await fetchMessages(currentGroupId, false);
      scrollToBottom();
    } catch (err) {
      clearTimeout(timeoutId);
      if (err.name === "AbortError") {
        showToast("发送请求超时 (15s)，请检查网络与机器人连接", "error");
      } else {
        showToast("发送异常，请检查网络与连接", "error");
      }
    } finally {
      isSending = false;
      sendBtn.disabled = textarea.value.trim().length === 0;
      sendBtn.querySelector("span").textContent = "发送";
      textarea.focus();
    }
  }

  async function clearGroupMessages(groupId) {
    try {
      const res = await fetch("/sd-api/chat/clear", {
        method: "POST",
        headers: getHeaders(),
        body: JSON.stringify({ groupId }),
      });
      if (res.ok) {
        showToast("已清空本地记录", "success");
        fetchMessages(groupId, true);
        fetchGroups();
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
        fetchGroups(false);
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
      }
    }
  }

  function startPolling() {
    stopPolling();
    pollTimer = window.setInterval(() => {
      const isChat = window.location.hash.includes(CHAT_ROUTE);
      const view = document.getElementById("sealdice-qq-chat-view");
      if (isChat && view && view.style.display !== "none") {
        if (currentGroupId) {
          fetchMessages(currentGroupId, false);
        }
      } else if (!isChat) {
        stopPolling();
      }
    }, 2500);
  }

  function stopPolling() {
    if (pollTimer) {
      window.clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  // --------------------------------------------------------------------------
  // Initialization & Mutation Observer
  // --------------------------------------------------------------------------
  window.addEventListener("hashchange", handleRouteChange);

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
