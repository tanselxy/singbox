import { useEffect, useState } from "react";
import { Badge, Metric, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent } from "../../ui/card.jsx";

const SERVICE_ACTIONS = {
  restart: { label: "重启服务", running: "重启中...", done: "服务已重启", status: "正在重启服务" },
  stop: { label: "停止", running: "停止中...", done: "服务已停止", status: "正在停止服务" },
  start: { label: "启动", running: "启动中...", done: "服务已启动", status: "正在启动服务" },
};

export function Overview({ data, prefix, onServiceAction }) {
  const summary = data.Summary || {};
  const [serviceState, setServiceState] = useState({
    active: Boolean(data.Active),
    action: "",
    phase: "",
    message: "",
  });
  const isRunning = serviceState.phase === "running";

  useEffect(() => {
    if (!isRunning) {
      setServiceState((current) => ({ ...current, active: Boolean(data.Active) }));
    }
  }, [data.Active, isRunning]);

  useEffect(() => {
    if (!isRunning) return undefined;
    let cancelled = false;
    async function pollStatus() {
      try {
        const res = await fetch(`${prefix}/api/status`);
        const payload = await res.json();
        if (!cancelled) {
          setServiceState((current) => ({ ...current, active: Boolean(payload.active) }));
        }
      } catch {
        // The operation result request below will surface the real error.
      }
    }
    pollStatus();
    const timer = window.setInterval(pollStatus, 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [isRunning, prefix]);

  async function runServiceAction(action) {
    const meta = SERVICE_ACTIONS[action];
    setServiceState((current) => ({
      ...current,
      action,
      phase: "running",
      message: meta.status,
    }));
    try {
      const payload = await onServiceAction(action);
      const active = Boolean(payload.active);
      setServiceState({ active, action, phase: "done", message: `${meta.done}，当前${active ? "运行中" : "已停止"}` });
      window.setTimeout(() => {
        setServiceState((current) => (current.action === action && current.phase === "done" ? { ...current, action: "", phase: "", message: "" } : current));
      }, 3000);
    } catch (err) {
      setServiceState((current) => ({
        ...current,
        action,
        phase: "error",
        message: `操作失败：${err.message}`,
      }));
    }
  }

  return (
    <>
      <PageHead
        meta={VIEW_META.overview}
        action={
          <div className="flex flex-wrap gap-2">
            <ServiceButton action="restart" variant="outline" state={serviceState} disabled={isRunning} onClick={runServiceAction} />
            <ServiceButton action="stop" variant="destructive" state={serviceState} disabled={isRunning} onClick={runServiceAction} />
            <ServiceButton action="start" state={serviceState} disabled={isRunning} onClick={runServiceAction} />
          </div>
        }
      />

      <Card>
        <CardContent className="flex flex-wrap items-center gap-x-12 gap-y-4">
          <div>
            <div className="text-sm text-muted-foreground">服务器</div>
            <div className="mt-1 font-mono text-lg">{data.ServerIP || "-"}</div>
          </div>
          <div>
            <div className="text-sm text-muted-foreground">sing-box 状态</div>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <Badge active={serviceState.active}>{serviceState.active ? "运行中" : "已停止"}</Badge>
              {serviceState.message && (
                <span className={serviceState.phase === "error" ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>
                  {serviceState.message}
                </span>
              )}
            </div>
          </div>
          <div>
            <div className="text-sm text-muted-foreground">面板版本</div>
            <div className="mt-1 font-mono">{data.Version || "-"}</div>
          </div>
        </CardContent>
      </Card>

      <div className="mt-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Metric label="客户总数" value={summary.TotalClients || 0} />
        <Metric label="启用客户" value={summary.EnabledClients || 0} />
        <Metric label="有限额客户" value={summary.LimitedClients || 0} />
        <Metric label="已到期客户" value={summary.ExpiredClients || 0} alert={(summary.ExpiredClients || 0) > 0} />
      </div>
    </>
  );
}

function ServiceButton({ action, variant, state, disabled, onClick }) {
  const meta = SERVICE_ACTIONS[action];
  const running = state.phase === "running" && state.action === action;
  return (
    <Button variant={variant} onClick={() => onClick(action)} disabled={disabled}>
      {running ? meta.running : meta.label}
    </Button>
  );
}
