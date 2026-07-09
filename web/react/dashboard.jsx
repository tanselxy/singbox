import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Link, Navigate, NavLink, Route, Routes } from "react-router";

const VIEW_META = {
  overview: { label: "总览", eyebrow: "Overview", title: "服务总览" },
  clients: { label: "客户管理", eyebrow: "Clients", title: "客户管理" },
  monitoring: { label: "实时监控", eyebrow: "Monitoring", title: "实时监控" },
  system: { label: "系统安全", eyebrow: "Security", title: "系统安全" },
  logs: { label: "运行日志", eyebrow: "Logs", title: "运行日志" },
};

function routePath(view) {
  return view === "overview" ? "/dashboard" : `/dashboard/${view}`;
}

function Badge({ active, children }) {
  return <span className={`badge ${active ? "ok" : "down"}`}>{children}</span>;
}

function PageHead({ meta, action }) {
  return (
    <div className="page-head">
      <div>
        <p className="eyebrow">{meta.eyebrow}</p>
        <h1>{meta.title}</h1>
      </div>
      {action}
    </div>
  );
}

function Shell({ children }) {
  return (
    <main className="console-shell">
      <aside className="console-sidebar">
        <div className="sidebar-title">控制台</div>
        {Object.entries(VIEW_META).map(([key, item]) => (
          <NavLink
            key={key}
            className={({ isActive }) => `side-link ${isActive ? "active" : ""}`}
            end={key === "overview"}
            to={routePath(key)}
          >
            {item.label}
          </NavLink>
        ))}
      </aside>
      <section className="console-content">{children}</section>
    </main>
  );
}

function Overview({ data, onServiceAction, prefix }) {
  const summary = data.Summary || {};
  return (
    <>
      <PageHead
        meta={VIEW_META.overview}
        action={
          <div className="head-actions">
            <button onClick={() => onServiceAction("restart")}>重启服务</button>
            <button className="warn" onClick={() => onServiceAction("stop")}>停止</button>
            <button onClick={() => onServiceAction("start")}>启动</button>
          </div>
        }
      />

      <section className="card service-card">
        <div>
          <div className="muted">服务器</div>
          <div className="mono metric-main">{data.ServerIP || "-"}</div>
        </div>
        <div>
          <div className="muted">sing-box 状态</div>
          <Badge active={data.Active}>{data.Active ? "运行中" : "已停止"}</Badge>
        </div>
        <div>
          <div className="muted">面板版本</div>
          <div className="mono">{data.Version || "-"}</div>
        </div>
      </section>

      <section className="metric-grid">
        <Metric label="客户总数" value={summary.TotalClients || 0} />
        <Metric label="启用客户" value={summary.EnabledClients || 0} />
        <Metric label="有限额客户" value={summary.LimitedClients || 0} />
        <Metric label="已到期客户" value={summary.ExpiredClients || 0} alert={(summary.ExpiredClients || 0) > 0} />
      </section>

      <section className="card">
        <div className="node-head">
          <h3>工作区</h3>
        </div>
        <div className="quick-grid">
          <QuickLink title="客户管理" desc="创建客户、停用订阅、清零流量" to="/dashboard/clients" />
          <QuickLink title="实时监控" desc="查看被控端 CPU、内存、硬盘状态" to="/dashboard/monitoring" />
          <QuickLink title="系统安全" desc="版本升级、BBR、fail2ban、SSH 端口" to="/dashboard/system" />
        </div>
      </section>

      <section className="card">
        <div className="node-head">
          <h3>外部工具</h3>
        </div>
        <div className="quick-grid">
          <a className="quick-link" href={`${prefix}/nodes`}>
            <strong>节点管理</strong>
            <span className="muted">添加受控节点、复制本机接入码</span>
          </a>
          <a className="quick-link" href={`${prefix}/tools`}>
            <strong>节点转换</strong>
            <span className="muted">把节点链接转换成 Clash 片段</span>
          </a>
        </div>
      </section>
    </>
  );
}

function Metric({ label, value, alert }) {
  return (
    <div className="metric-card">
      <div className="muted">{label}</div>
      <strong className={alert ? "alert" : ""}>{value}</strong>
    </div>
  );
}

function QuickLink({ title, desc, to }) {
  return (
    <Link className="quick-link" to={to}>
      <strong>{title}</strong>
      <span className="muted">{desc}</span>
    </Link>
  );
}

function Clients({ data, refresh, prefix }) {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ name: "", quota: "", quotaUnit: "GB", devices: "", expires: "" });
  const clients = data.Clients || [];

  async function createClient() {
    if (!form.name.trim()) {
      alert("请填写客户名");
      return;
    }
    const body = new URLSearchParams({
      name: form.name.trim(),
      quota: form.quota.trim(),
      quota_unit: form.quotaUnit,
      device_limit: form.devices.trim(),
      expires_at: form.expires,
    });
    const res = await fetch(`${prefix}/api/clients`, { method: "POST", body });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`创建失败: ${payload.error || ""}`);
      return;
    }
    setForm({ name: "", quota: "", quotaUnit: "GB", devices: "", expires: "" });
    setOpen(false);
    refresh();
  }

  async function clientAction(client, action) {
    if (action === "delete" && !confirm("确认删除该客户？其订阅将立即失效。")) return;
    const res = await fetch(`${prefix}/api/clients/${client.ID}/${action}`, { method: "POST" });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`操作失败: ${payload.error || ""}`);
      return;
    }
    refresh();
  }

  return (
    <>
      <PageHead meta={VIEW_META.clients} action={<button onClick={() => setOpen((v) => !v)}>+ 新增客户</button>} />
      <section className="card">
        {open && (
          <div className="add-form">
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} type="text" placeholder="客户名（唯一）" />
            <span className="field">
              <input value={form.quota} onChange={(e) => setForm({ ...form, quota: e.target.value })} type="number" min="0" step="1" placeholder="流量配额" />
              <select value={form.quotaUnit} onChange={(e) => setForm({ ...form, quotaUnit: e.target.value })}>
                <option value="GB">GB</option>
                <option value="MB">MB</option>
              </select>
            </span>
            <input value={form.devices} onChange={(e) => setForm({ ...form, devices: e.target.value })} type="number" min="0" step="1" placeholder="设备数（0=不限）" />
            <input value={form.expires} onChange={(e) => setForm({ ...form, expires: e.target.value })} type="date" title="到期日期（留空=永久）" />
            <button onClick={createClient}>确认创建</button>
            <button className="link" onClick={() => setOpen(false)}>取消</button>
          </div>
        )}
        <p className="muted small">流量/到期到点会自动停用该客户；设备数目前仅记录，暂不自动限制。</p>
        <div className="table-wrap">
          <table className="clients">
            <thead>
              <tr><th>名称</th><th>状态</th><th>已用</th><th>配额</th><th>设备</th><th>到期</th><th>操作</th></tr>
            </thead>
            <tbody>
              {clients.length === 0 && <tr><td colSpan="7" className="muted">还没有客户，点右上角新增。</td></tr>}
              {clients.map((client) => (
                <tr key={client.ID}>
                  <td><a href={`${prefix}/client/${client.ID}`}>{client.Name}</a></td>
                  <td><Badge active={client.Enabled}>{client.Enabled ? "启用" : "停用"}</Badge></td>
                  <td className={client.OverQuota ? "alert" : ""}>{client.Used}</td>
                  <td>{client.Quota}</td>
                  <td>{client.Devices}</td>
                  <td className={client.Expired ? "alert" : ""}>{client.Expiry}</td>
                  <td className="row-actions">
                    <button onClick={() => clientAction(client, client.Enabled ? "disable" : "enable")}>{client.Enabled ? "停用" : "启用"}</button>
                    <button onClick={() => clientAction(client, "reset-traffic")}>清零流量</button>
                    <button className="warn" onClick={() => clientAction(client, "delete")}>删除</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}

function Monitoring({ prefix }) {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [updatedAt, setUpdatedAt] = useState("");

  async function load() {
    try {
      const res = await fetch(`${prefix}/api/node-metrics`);
      const payload = await res.json();
      if (payload.ok) {
        setNodes(payload.nodes || []);
        setUpdatedAt(new Date().toLocaleTimeString());
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = setInterval(load, 10000);
    return () => clearInterval(timer);
  }, []);

  return (
    <>
      <PageHead
        meta={VIEW_META.monitoring}
        action={<button onClick={load}>{loading ? "加载中" : "刷新"}</button>}
      />
      <section className="card">
        <div className="node-head">
          <h3>被控端资源</h3>
          <span className="muted small">{updatedAt ? `更新于 ${updatedAt}` : "等待采集"}</span>
        </div>
        <div className="table-wrap">
          <table className="clients">
            <thead>
              <tr><th>节点</th><th>地址</th><th>状态</th><th>CPU</th><th>内存</th><th>硬盘</th></tr>
            </thead>
            <tbody>
              {nodes.length === 0 && <tr><td colSpan="6" className="muted">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</td></tr>}
              {nodes.map((node) => (
                <tr key={node.ID}>
                  <td>{node.Name}</td>
                  <td className="mono small">{node.Address}</td>
                  <td><Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge></td>
                  <td>{node.CPU || "-"}</td>
                  <td>{node.Mem || "-"}</td>
                  <td>{node.Disk || "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}

function System({ data, refresh, prefix }) {
  const [latest, setLatest] = useState("");
  const [upgradeLabel, setUpgradeLabel] = useState("");

  async function systemAction(action) {
    let body;
    if (action === "ssh-port") {
      const port = document.getElementById("ssh-port").value.trim();
      if (!port) {
        alert("请填写新的 SSH 端口");
        return;
      }
      if (!confirm(`即将把 SSH 端口改为 ${port}。请勿关闭当前会话，改完先用新端口测试！确定继续？`)) return;
      body = new URLSearchParams({ port });
    }
    const res = await fetch(`${prefix}/api/system/${action}`, { method: "POST", body });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`操作失败: ${payload.error || ""}`);
      return;
    }
    alert("操作成功");
    refresh();
  }

  async function checkUpdate() {
    const res = await fetch(`${prefix}/api/version`);
    const payload = await res.json();
    if (payload.upgradable) {
      setLatest(`有新版本 ${payload.latest}`);
      setUpgradeLabel(`升级到 ${payload.latest}`);
    } else {
      setLatest(payload.latest ? `已是最新（${payload.latest}）` : "无法获取最新版本");
      setUpgradeLabel("");
    }
  }

  async function upgrade() {
    if (!confirm("确认升级？升级期间面板会短暂重启，请稍候刷新页面。")) return;
    setUpgradeLabel("升级中...");
    try {
      const res = await fetch(`${prefix}/api/upgrade`, { method: "POST" });
      const payload = await res.json();
      if (payload.ok) alert("已下载新版本，面板正在重启，约 10 秒后请刷新页面。");
      else {
        alert(`升级失败: ${payload.error || ""}`);
        setUpgradeLabel("重试升级");
      }
    } catch {
      alert("面板正在重启，约 10 秒后请刷新页面。");
    }
  }

  const system = data.System || {};
  return (
    <>
      <PageHead meta={VIEW_META.system} />
      <section className="card">
        <div className="sys-row">
          <div>
            <div className="muted">面板版本</div>
            <span className="mono">{data.Version}</span>
            <span className="small muted"> {latest}</span>
          </div>
          <span className="field compact-field">
            <button onClick={checkUpdate}>检查更新</button>
            {upgradeLabel && <button onClick={upgrade}>{upgradeLabel}</button>}
          </span>
        </div>
        <div className="sys-row">
          <div>
            <div className="muted">BBR + TCP 优化</div>
            <Badge active={system.BBR}>{system.BBR ? "已开启" : "未开启"}</Badge>
          </div>
          <button onClick={() => systemAction("bbr")} disabled={system.BBR}>开启 BBR</button>
        </div>
        <div className="sys-row">
          <div>
            <div className="muted">fail2ban（SSH 防爆破）</div>
            <Badge active={system.Fail2ban}>{system.Fail2ban ? "运行中" : "未安装"}</Badge>
          </div>
          <button onClick={() => systemAction("fail2ban")} disabled={system.Fail2ban}>安装并启用</button>
        </div>
        <div className="sys-row">
          <div>
            <div className="muted">SSH 端口（当前 {system.SSHPort}）</div>
            <span className="small muted">改端口有风险，务必保持当前 SSH 会话不要关，改完用新端口测试能连上再退出。</span>
          </div>
          <span className="field compact-field">
            <input id="ssh-port" type="number" min="1" max="65535" placeholder="新端口" />
            <button className="warn" onClick={() => systemAction("ssh-port")}>修改</button>
          </span>
        </div>
      </section>
    </>
  );
}

function Logs({ prefix }) {
  const [logs, setLogs] = useState("点击上方按钮加载。");
  const [loaded, setLoaded] = useState(false);

  async function loadLogs() {
    setLogs("加载中...");
    const res = await fetch(`${prefix}/api/logs`);
    setLogs(await res.text());
    setLoaded(true);
  }

  return (
    <>
      <PageHead meta={VIEW_META.logs} action={<button onClick={loadLogs}>加载最近日志</button>} />
      <section className="card log-panel">
        <pre className={`logs ${loaded ? "" : "muted"}`}>{logs}</pre>
      </section>
    </>
  );
}

function DashboardRoutes({ data, error, loadDashboard, prefix, serviceAction }) {
  let content;
  if (error) content = <section className="card"><p className="alert">加载失败：{error}</p></section>;
  else if (!data) content = <section className="card"><p className="muted">正在加载控制台数据。</p></section>;
  else {
    content = (
      <Routes>
        <Route index element={<Overview data={data} onServiceAction={serviceAction} prefix={prefix} />} />
        <Route path="clients" element={<Clients data={data} refresh={loadDashboard} prefix={prefix} />} />
        <Route path="monitoring" element={<Monitoring prefix={prefix} />} />
        <Route path="system" element={<System data={data} refresh={loadDashboard} prefix={prefix} />} />
        <Route path="logs" element={<Logs prefix={prefix} />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    );
  }

  return <Shell>{content}</Shell>;
}

function App({ root }) {
  const prefix = root.dataset.prefix || "";
  const [data, setData] = useState(null);
  const [error, setError] = useState("");

  async function loadDashboard() {
    setError("");
    try {
      const res = await fetch(`${prefix}/api/dashboard`);
      const payload = await res.json();
      if (!payload.ok) {
        setError(payload.error || "读取控制台失败");
        return;
      }
      setData(payload.dashboard);
    } catch (err) {
      setError(err.message);
    }
  }

  async function serviceAction(action) {
    const res = await fetch(`${prefix}/api/service/${action}`, { method: "POST" });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`操作失败: ${payload.error || "未知错误"}`);
      return;
    }
    setData((current) => ({ ...current, Active: payload.active }));
  }

  useEffect(() => {
    loadDashboard();
  }, []);

  return (
    <BrowserRouter basename={prefix}>
      <Routes>
        <Route
          path="/dashboard/*"
          element={
            <DashboardRoutes
              data={data}
              error={error}
              loadDashboard={loadDashboard}
              prefix={prefix}
              serviceAction={serviceAction}
            />
          }
        />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

const root = document.getElementById("dashboard-root");
if (root) {
  createRoot(root).render(<App root={root} />);
}
