// Panel interactions: copy links, service actions, client management, logs.
(function () {
  "use strict";
  const prefix = document.body.dataset.prefix || "";

  function toast(btn, text) {
    const old = btn.textContent;
    btn.textContent = text;
    setTimeout(() => (btn.textContent = old), 1500);
  }

  async function postJSON(path) {
    const res = await fetch(prefix + path, { method: "POST" });
    return res.json();
  }

  // Copy to clipboard.
  document.querySelectorAll(".copy").forEach((btn) => {
    btn.addEventListener("click", () => {
      navigator.clipboard.writeText(btn.dataset.url).then(() => toast(btn, "已复制"));
    });
  });

  // sing-box service actions.
  const stateEl = document.getElementById("svc-state");
  document.querySelectorAll("[data-action]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      try {
        const data = await postJSON("/api/service/" + btn.dataset.action);
        if (data.ok) setState(data.active);
        else alert("操作失败: " + (data.error || "未知错误"));
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

  // Add-client form toggle.
  const addBtn = document.getElementById("add-client");
  const addForm = document.getElementById("add-form");
  if (addBtn && addForm) {
    addBtn.addEventListener("click", () => (addForm.hidden = !addForm.hidden));
    document.getElementById("cancel-add").addEventListener("click", () => (addForm.hidden = true));
    document.getElementById("create-client").addEventListener("click", async (e) => {
      e.target.disabled = true;
      const name = document.getElementById("new-name").value.trim();
      const quota = document.getElementById("new-quota").value.trim();
      const quotaUnit = document.getElementById("new-quota-unit").value;
      const devices = document.getElementById("new-devices").value.trim();
      const expires = document.getElementById("new-expires").value; // yyyy-mm-dd or ""
      if (!name) {
        alert("请填写客户名");
        e.target.disabled = false;
        return;
      }
      try {
        const body = new URLSearchParams({
          name,
          quota,
          quota_unit: quotaUnit,
          device_limit: devices,
          expires_at: expires,
        });
        const res = await fetch(prefix + "/api/clients", { method: "POST", body });
        const data = await res.json();
        if (data.ok) location.reload();
        else alert("创建失败: " + (data.error || ""));
      } catch (err) {
        alert("请求失败: " + err.message);
      } finally {
        e.target.disabled = false;
      }
    });
  }

  // Per-client actions (enable/disable/delete/reset-traffic).
  document.querySelectorAll("[data-client-action]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const action = btn.dataset.clientAction;
      const id = btn.dataset.id;
      if (action === "delete" && !confirm("确认删除该客户？其订阅将立即失效。")) return;
      btn.disabled = true;
      try {
        const data = await postJSON("/api/clients/" + id + "/" + action);
        if (data.ok) location.reload();
        else alert("操作失败: " + (data.error || ""));
      } catch (e) {
        alert("请求失败: " + e.message);
      } finally {
        btn.disabled = false;
      }
    });
  });

  // System optimization / security actions.
  document.querySelectorAll("[data-system]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const action = btn.dataset.system;
      let body;
      if (action === "ssh-port") {
        const port = document.getElementById("ssh-port").value.trim();
        if (!port) {
          alert("请填写新的 SSH 端口");
          return;
        }
        if (!confirm("即将把 SSH 端口改为 " + port + "。请勿关闭当前会话，改完先用新端口测试！确定继续？")) return;
        body = new URLSearchParams({ port });
      }
      btn.disabled = true;
      try {
        const res = await fetch(prefix + "/api/system/" + action, { method: "POST", body });
        const data = await res.json();
        if (data.ok) {
          alert("操作成功");
          location.reload();
        } else {
          alert("操作失败: " + (data.error || ""));
          btn.disabled = false;
        }
      } catch (e) {
        alert("请求失败: " + e.message);
        btn.disabled = false;
      }
    });
  });

  // Add-node form (master side).
  const addNodeBtn = document.getElementById("add-node");
  const nodeForm = document.getElementById("node-form");
  if (addNodeBtn && nodeForm) {
    addNodeBtn.addEventListener("click", () => (nodeForm.hidden = !nodeForm.hidden));
    document.getElementById("cancel-node").addEventListener("click", () => (nodeForm.hidden = true));
    document.getElementById("create-node").addEventListener("click", async (e) => {
      const code = document.getElementById("node-code").value.trim();
      const name = document.getElementById("node-name").value.trim();
      if (!code) {
        alert("请粘贴节点的接入码");
        return;
      }
      e.target.disabled = true;
      try {
        const body = new URLSearchParams({ code, name });
        const res = await fetch(prefix + "/api/nodes", { method: "POST", body });
        const data = await res.json();
        if (data.ok) {
          if (data.warn) alert(data.warn);
          location.reload();
        } else {
          alert("添加失败: " + (data.error || ""));
          e.target.disabled = false;
        }
      } catch (err) {
        alert("请求失败: " + err.message);
        e.target.disabled = false;
      }
    });
  }

  // Delete node.
  document.querySelectorAll("[data-node-action]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      if (!confirm("确认删除该节点？订阅将不再包含它。")) return;
      btn.disabled = true;
      try {
        const res = await fetch(prefix + "/api/nodes/" + btn.dataset.id + "/delete", { method: "POST" });
        const data = await res.json();
        if (data.ok) location.reload();
        else {
          alert("删除失败: " + (data.error || ""));
          btn.disabled = false;
        }
      } catch (e) {
        alert("请求失败: " + e.message);
        btn.disabled = false;
      }
    });
  });

  // Version check + in-panel upgrade.
  const checkBtn = document.getElementById("check-update");
  const upgradeBtn = document.getElementById("do-upgrade");
  if (checkBtn) {
    checkBtn.addEventListener("click", async () => {
      checkBtn.disabled = true;
      checkBtn.textContent = "检查中…";
      try {
        const res = await fetch(prefix + "/api/version");
        const d = await res.json();
        const latestEl = document.getElementById("ver-latest");
        if (d.upgradable) {
          latestEl.textContent = "有新版本 " + d.latest;
          upgradeBtn.hidden = false;
          upgradeBtn.textContent = "升级到 " + d.latest;
        } else {
          latestEl.textContent = d.latest ? "已是最新（" + d.latest + "）" : "无法获取最新版本";
        }
      } catch (e) {
        alert("检查失败: " + e.message);
      } finally {
        checkBtn.disabled = false;
        checkBtn.textContent = "检查更新";
      }
    });
  }
  if (upgradeBtn) {
    upgradeBtn.addEventListener("click", async () => {
      if (!confirm("确认升级？升级期间面板会短暂重启，请稍候刷新页面。")) return;
      upgradeBtn.disabled = true;
      upgradeBtn.textContent = "升级中…";
      try {
        const res = await fetch(prefix + "/api/upgrade", { method: "POST" });
        const d = await res.json();
        if (d.ok) {
          alert("已下载新版本，面板正在重启，约 10 秒后请刷新页面。");
        } else {
          alert("升级失败: " + (d.error || ""));
          upgradeBtn.disabled = false;
          upgradeBtn.textContent = "重试升级";
        }
      } catch (e) {
        // The panel may restart before responding; treat as in-progress.
        alert("面板正在重启，约 10 秒后请刷新页面。");
      }
    });
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
