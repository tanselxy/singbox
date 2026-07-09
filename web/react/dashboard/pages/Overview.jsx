import { Badge, Metric, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

export function Overview({ data, onServiceAction }) {
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
    </>
  );
}
