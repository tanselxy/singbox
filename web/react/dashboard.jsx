import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { Shell } from "./dashboard/Shell.jsx";
import { Clients } from "./dashboard/pages/Clients.jsx";
import { Logs } from "./dashboard/pages/Logs.jsx";
import { Monitoring } from "./dashboard/pages/Monitoring.jsx";
import { Notifications } from "./dashboard/pages/Notifications.jsx";
import { Overview } from "./dashboard/pages/Overview.jsx";
import { System } from "./dashboard/pages/System.jsx";
import { Toolbox } from "./dashboard/pages/Toolbox.jsx";

function DashboardRoutes({ data, error, loadDashboard, prefix, serviceAction }) {
  let content;
  if (error) content = <div className="rounded-xl border bg-card p-6 text-sm text-destructive shadow-sm">加载失败：{error}</div>;
  else if (!data) content = <div className="rounded-xl border bg-card p-6 text-sm text-muted-foreground shadow-sm">正在加载总览数据。</div>;
  else {
    content = (
      <Routes>
        <Route index element={<Overview data={data} onServiceAction={serviceAction} />} />
        <Route path="clients" element={<Clients data={data} refresh={loadDashboard} prefix={prefix} />} />
        <Route path="monitoring" element={<Monitoring prefix={prefix} />} />
        <Route path="notifications" element={<Notifications prefix={prefix} />} />
        <Route path="system" element={<System data={data} prefix={prefix} />} />
        <Route path="toolbox" element={<Toolbox data={data} refresh={loadDashboard} prefix={prefix} />} />
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
        setError(payload.error || "读取面板失败");
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
