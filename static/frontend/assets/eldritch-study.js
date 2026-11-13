(function () {
  const root = document.documentElement;
  root.classList.add("eldritch-study");

  const atmosphere = document.createElement("div");
  atmosphere.className = "eldritch-atmosphere";
  atmosphere.setAttribute("aria-hidden", "true");
  atmosphere.innerHTML = '<div class="eldritch-sigil"></div><div class="eldritch-glow"></div><div class="eldritch-dust"></div>';
  document.body.appendChild(atmosphere);

  const dust = atmosphere.querySelector(".eldritch-dust");
  for (let i = 0; i < 28; i += 1) {
    const mote = document.createElement("i");
    mote.style.setProperty("--x", `${(i * 37) % 101}%`);
    mote.style.setProperty("--y", `${(i * 61) % 97}%`);
    mote.style.setProperty("--delay", `${-(i % 13)}s`);
    mote.style.setProperty("--duration", `${12 + (i % 9)}s`);
    mote.style.setProperty("--size", `${1 + (i % 3)}px`);
    dust.appendChild(mote);
  }

  let frame = 0;
  window.addEventListener("pointermove", (event) => {
    if (frame) return;
    frame = window.requestAnimationFrame(() => {
      root.style.setProperty("--study-x", `${(event.clientX / window.innerWidth - 0.5) * 10}px`);
      root.style.setProperty("--study-y", `${(event.clientY / window.innerHeight - 0.5) * 6}px`);
      frame = 0;
    });
  }, { passive: true });

  const GROUP_ROUTE = "/misc/group";

  function createGroupConfirmDialog(onConfirm) {
    const overlay = document.createElement("div");
    overlay.className = "eldritch-confirm-overlay";
    overlay.innerHTML = `
      <section class="eldritch-confirm" role="dialog" aria-modal="true" aria-labelledby="eldritch-confirm-title">
        <div class="eldritch-confirm-mark" aria-hidden="true">!</div>
        <div>
          <h3 id="eldritch-confirm-title">关闭所有群聊？</h3>
          <p>这会让骰子在所有已记录群组中停止响应消息。</p>
          <p class="eldritch-confirm-safe">不会退出群聊，也不会删除群资料；之后仍可逐群重新开启。</p>
        </div>
        <footer>
          <button type="button" class="eldritch-dialog-cancel">取消</button>
          <button type="button" class="eldritch-dialog-danger">确认全部关闭</button>
        </footer>
      </section>`;

    const close = () => overlay.remove();
    overlay.addEventListener("click", (event) => {
      if (event.target === overlay) close();
    });
    overlay.querySelector(".eldritch-dialog-cancel").addEventListener("click", close);
    overlay.querySelector(".eldritch-dialog-danger").addEventListener("click", async (event) => {
      const button = event.currentTarget;
      button.disabled = true;
      button.textContent = "正在关闭...";
      await onConfirm(overlay, button);
    });
    document.body.appendChild(overlay);
    overlay.querySelector(".eldritch-dialog-cancel").focus();
  }

  async function disableAllGroups(overlay, button) {
    const token = localStorage.getItem("t") || "";
    try {
      const response = await fetch("/sd-api/group/disable_all", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": token,
          "token": token
        },
        body: "{}"
      });
      if (!response.ok) {
        throw new Error(response.status === 403 ? "登录凭证已失效，请重新登录" : `请求失败（${response.status}）`);
      }

      const result = await response.json();
      const message = result.testMode
        ? "测试模式下未修改群聊状态"
        : `已关闭 ${result.changed || 0} 个群聊，${result.alreadyDisabled || 0} 个原本已关闭`;
      sessionStorage.setItem("eldritch-disable-all-result", message);
      button.textContent = "关闭完成";
      overlay.classList.add("is-success");
      window.setTimeout(() => window.location.reload(), 700);
    } catch (error) {
      button.disabled = false;
      button.textContent = "确认全部关闭";
      const oldError = overlay.querySelector(".eldritch-confirm-error");
      if (oldError) oldError.remove();
      const errorText = document.createElement("p");
      errorText.className = "eldritch-confirm-error";
      errorText.textContent = error.message || "关闭失败，请稍后重试";
      overlay.querySelector(".eldritch-confirm > div:nth-child(2)").appendChild(errorText);
    }
  }

  function injectGroupControls() {
    if (!window.location.hash.includes(GROUP_ROUTE)) return;
    if (document.querySelector(".eldritch-group-toolbar")) return;

    const heading = Array.from(document.querySelectorAll("h2"))
      .find((node) => node.textContent.trim() === "群管理");
    if (!heading || !heading.parentNode) return;

    const toolbar = document.createElement("div");
    toolbar.className = "eldritch-group-toolbar";
    toolbar.innerHTML = `
      <span class="eldritch-group-toolbar-note">群组总控</span>
      <button type="button" class="eldritch-disable-all-groups">
        <span aria-hidden="true">&#10005;</span>
        一键关闭所有群聊
      </button>`;
    toolbar.querySelector("button").addEventListener("click", () => {
      createGroupConfirmDialog(disableAllGroups);
    });
    heading.insertAdjacentElement("afterend", toolbar);

    const result = sessionStorage.getItem("eldritch-disable-all-result");
    if (result) {
      sessionStorage.removeItem("eldritch-disable-all-result");
      const notice = document.createElement("div");
      notice.className = "eldritch-group-notice";
      notice.textContent = result;
      toolbar.insertAdjacentElement("afterend", notice);
      window.setTimeout(() => notice.classList.add("is-leaving"), 4200);
      window.setTimeout(() => notice.remove(), 4800);
    }
  }

  const groupObserver = new MutationObserver(injectGroupControls);
  groupObserver.observe(document.getElementById("app"), { childList: true, subtree: true });
  window.addEventListener("hashchange", () => window.setTimeout(injectGroupControls, 0));
  injectGroupControls();
})();
