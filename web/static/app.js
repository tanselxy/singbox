// Dashboard interactions: copy links, service actions, log loading.
(function () {
  "use strict";
  const prefix = document.body.dataset.prefix || "";

  function toast(btn, text) {
    const old = btn.textContent;
    btn.textContent = text;
    setTimeout(() => (btn.textContent = old), 1500);
  }

  // Copy node link to clipboard.
  document.querySelectorAll(".copy").forEach((btn) => {
    btn.addEventListener("click", () => {
      navigator.clipboard.writeText(btn.dataset.url).then(() => toast(btn, "已复制"));
    });
  });

  // Service actions (restart/stop/start).
  const stateEl = document.getElementById("svc-state");
  document.querySelectorAll("[data-action]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const action = btn.dataset.action;
      btn.disabled = true;
      try {
        const res = await fetch(prefix + "/api/service/" + action, { method: "POST" });
        const data = await res.json();
        if (data.ok) {
          setState(data.active);
        } else {
          alert("操作失败: " + (data.error || "未知错误"));
        }
      } catch (e) {
        alert("请求失败: " + e.message);
      } finally {
        btn.disabled = false;
      }
    });
  });

  function setState(active) {
    if (!stateEl) return;
    stateEl.textContent = active ? "运行中" : "已停止";
    stateEl.className = "badge " + (active ? "ok" : "down");
  }

  // Load recent service logs.
  const loadBtn = document.getElementById("load-logs");
  if (loadBtn) {
    loadBtn.addEventListener("click", async () => {
      const pre = document.getElementById("logs");
      pre.textContent = "加载中…";
      try {
        const res = await fetch(prefix + "/api/logs");
        pre.textContent = await res.text();
        pre.classList.remove("muted");
      } catch (e) {
        pre.textContent = "加载失败: " + e.message;
      }
    });
  }
})();
