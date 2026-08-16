(function () {
  const GROUP_ROUTE = "/misc/group";

  function showConfirmDialog() {
    const overlay = document.createElement("div");
    overlay.className = "project-confirm-overlay";
    overlay.innerHTML = `
      <section class="project-confirm" role="dialog" aria-modal="true" aria-labelledby="project-confirm-title">
        <h3 id="project-confirm-title">关闭所有群聊？</h3>
        <p>骰子会在所有已记录群组中停止响应，但不会退出群聊或删除群资料。</p>
        <p class="project-confirm-error" aria-live="polite"></p>
        <footer>
          <button type="button" class="project-confirm-cancel">取消</button>
          <button type="button" class="project-confirm-submit">确认全部关闭</button>
        </footer>
      </section>`;

    const close = () => overlay.remove();
    overlay.addEventListener("click", (event) => {
      if (event.target === overlay) close();
    });
    overlay.querySelector(".project-confirm-cancel").addEventListener("click", close);
    overlay.querySelector(".project-confirm-submit").addEventListener("click", async (event) => {
      const button = event.currentTarget;
      const errorBox = overlay.querySelector(".project-confirm-error");
      const token = localStorage.getItem("t") || "";
      button.disabled = true;
      button.textContent = "正在关闭...";
      errorBox.textContent = "";
      try {
        const response = await fetch("/sd-api/group/disable_all", {
          method: "POST",
          headers: { "Content-Type": "application/json", "Authorization": token, "token": token },
          body: "{}"
        });
        if (!response.ok) {
          throw new Error(response.status === 403 ? "登录凭证已失效，请重新登录" : `请求失败（${response.status}）`);
        }
        const result = await response.json();
        sessionStorage.setItem(
          "project-disable-all-result",
          result.testMode
            ? "测试模式下未修改群聊状态"
            : `已关闭 ${result.changed || 0} 个群聊，${result.alreadyDisabled || 0} 个原本已关闭`
        );
        window.location.reload();
      } catch (error) {
        button.disabled = false;
        button.textContent = "确认全部关闭";
        errorBox.textContent = error.message || "关闭失败，请稍后重试";
      }
    });
    document.body.appendChild(overlay);
    overlay.querySelector(".project-confirm-cancel").focus();
  }

  function injectGroupControls() {
    if (!window.location.hash.includes(GROUP_ROUTE) || document.querySelector(".project-group-toolbar")) return;
    const heading = Array.from(document.querySelectorAll("h2")).find((node) => node.textContent.trim() === "群管理");
    if (!heading) return;

    const toolbar = document.createElement("div");
    toolbar.className = "project-group-toolbar";
    toolbar.innerHTML = '<button type="button" class="project-disable-all-groups" title="关闭所有群聊">&#10005; 一键关闭所有群聊</button>';
    toolbar.querySelector("button").addEventListener("click", showConfirmDialog);
    heading.insertAdjacentElement("afterend", toolbar);

    const result = sessionStorage.getItem("project-disable-all-result");
    if (result) {
      sessionStorage.removeItem("project-disable-all-result");
      const notice = document.createElement("div");
      notice.className = "project-group-notice";
      notice.textContent = result;
      toolbar.insertAdjacentElement("afterend", notice);
      window.setTimeout(() => notice.remove(), 5000);
    }
  }

  const app = document.getElementById("app");
  if (app) new MutationObserver(injectGroupControls).observe(app, { childList: true, subtree: true });
  window.addEventListener("hashchange", () => window.setTimeout(injectGroupControls, 0));
  injectGroupControls();
})();
