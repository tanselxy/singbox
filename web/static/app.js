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
      const name = document.getElementById("node-name").value.trim();
      const address = document.getElementById("node-address").value.trim();
      const token = document.getElementById("node-token").value.trim();
      if (!name || !address || !token) {
        alert("请填写名称、地址和令牌");
        return;
      }
      e.target.disabled = true;
      try {
        const body = new URLSearchParams({ name, address, token });
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
