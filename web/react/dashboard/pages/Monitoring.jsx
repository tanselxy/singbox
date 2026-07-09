import { useEffect, useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

export function Monitoring({ prefix }) {
  const [nodes, setNodes] = useState([]);
  const [mode, setMode] = useState("charts");
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [nodeForm, setNodeForm] = useState({ code: "", name: "" });
  const [history, setHistory] = useState({});
  const [loading, setLoading] = useState(true);
  const [updatedAt, setUpdatedAt] = useState("");

  async function load() {
    try {
      const res = await fetch(`${prefix}/api/node-metrics`);
      const payload = await res.json();
      if (payload.ok) {
        const nextNodes = (payload.nodes || []).map(normalizeNodeMetrics);
        const sampledAt = Date.now();
        setNodes(nextNodes);
        setHistory((current) => {
          const next = { ...current };
          nextNodes.forEach((node) => {
            if (!node.Online) return;
            const items = next[node.ID] ? [...next[node.ID]] : [];
            items.push({
              t: sampledAt,
              cpu: node.CPUPercent,
              mem: node.MemPercent,
              disk: node.DiskPercent,
            });
            next[node.ID] = items.slice(-36);
          });
          return next;
        });
        setUpdatedAt(new Date(sampledAt).toLocaleTimeString());
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

  async function createNode() {
    if (!nodeForm.code.trim()) {
      alert("请粘贴节点的接入码");
      return;
    }
    setCreating(true);
    try {
      const body = new URLSearchParams({ code: nodeForm.code.trim(), name: nodeForm.name.trim() });
      const res = await fetch(`${prefix}/api/nodes`, { method: "POST", body });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`添加失败: ${payload.error || ""}`);
        return;
      }
      if (payload.warn) alert(payload.warn);
      setNodeForm({ code: "", name: "" });
      setOpen(false);
      load();
    } catch (err) {
      alert(`请求失败: ${err.message}`);
    } finally {
      setCreating(false);
    }
  }

  return (
    <>
      <PageHead
        meta={VIEW_META.monitoring}
        action={
          <div className="head-actions">
            <span className="segmented">
              <button className={mode === "charts" ? "active" : ""} onClick={() => setMode("charts")}>图表</button>
              <button className={mode === "table" ? "active" : ""} onClick={() => setMode("table")}>表格</button>
            </span>
            <button onClick={() => setOpen(true)}>+ 新增节点</button>
          </div>
        }
      />
      <section className="card">
        <div className="node-head">
          <h3>被控端资源</h3>
          <span className="muted small">{updatedAt ? `更新于 ${updatedAt}` : "等待采集"}</span>
        </div>
        {mode === "charts" ? (
          <MonitoringCharts nodes={nodes} history={history} loading={loading} />
        ) : (
          <MonitoringTable nodes={nodes} loading={loading} />
        )}
      </section>
      <NodeCreateDialog
        open={open}
        form={nodeForm}
        creating={creating}
        onChange={setNodeForm}
        onCancel={() => setOpen(false)}
        onSubmit={createNode}
      />
    </>
  );
}

function NodeCreateDialog({ open, form, creating, onChange, onCancel, onSubmit }) {
  return (
    <ModalFrame
      open={open}
      onCancel={onCancel}
      eyebrow="Node"
      title="新增节点"
      titleId="create-node-title"
      footer={
        <>
          <button className="secondary" onClick={onCancel} disabled={creating}>取消</button>
          <button onClick={onSubmit} disabled={creating}>{creating ? "添加中..." : "确认添加"}</button>
        </>
      }
    >
      <label className="form-field wide">接入码
        <textarea value={form.code} onChange={(e) => onChange({ ...form, code: e.target.value })} rows="4" placeholder="粘贴节点页复制的接入码" autoFocus />
      </label>
      <label className="form-field wide">节点名
        <input value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} type="text" placeholder="可留空，默认使用节点 IP" />
      </label>
    </ModalFrame>
  );
}

function normalizeNodeMetrics(node) {
  const cpu = clampPercent(node.CPUPercent ?? parsePercent(node.CPU));
  const mem = clampPercent(node.MemPercent);
  const disk = clampPercent(node.DiskPercent);
  return {
    ...node,
    CPUPercent: cpu,
    MemPercent: mem,
    DiskPercent: disk,
  };
}

function clampPercent(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) return 0;
  return Math.max(0, Math.min(100, number));
}

function parsePercent(value) {
  const number = Number(String(value || "").replace("%", ""));
  return Number.isFinite(number) ? number : 0;
}

function MonitoringCharts({ nodes, history, loading }) {
  if (nodes.length === 0) {
    return <div className="empty-state muted">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</div>;
  }
  return (
    <div className="monitor-grid">
      {nodes.map((node) => (
        <NodeMetricCard key={node.ID} node={node} history={history[node.ID] || []} />
      ))}
    </div>
  );
}

function NodeMetricCard({ node, history }) {
  return (
    <article className="monitor-card">
      <div className="monitor-card-head">
        <div>
          <h4>{node.Name}</h4>
          <p className="mono muted">{node.Address}</p>
        </div>
        <Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge>
      </div>
      <div className="monitor-metrics">
        <MetricBlock label="CPU" value={`${node.CPUPercent.toFixed(1)}%`} percent={node.CPUPercent} samples={history.map((item) => item.cpu)} tone="cpu" />
        <MetricBlock label="内存" value={node.Mem || "-"} percent={node.MemPercent} samples={history.map((item) => item.mem)} tone="mem" />
        <MetricBlock label="硬盘" value={node.Disk || "-"} percent={node.DiskPercent} samples={history.map((item) => item.disk)} tone="disk" />
      </div>
    </article>
  );
}

function MetricBlock({ label, value, percent, samples, tone }) {
  return (
    <div className={`metric-block ${tone}`}>
      <div className="metric-block-top">
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
      <div className="usage-bar" aria-label={`${label} ${percent.toFixed(1)}%`}>
        <span style={{ width: `${percent}%` }} />
      </div>
      <Sparkline samples={samples} />
    </div>
  );
}

function Sparkline({ samples }) {
  const values = samples.length > 1 ? samples : [0, ...samples];
  const points = values.map((value, index) => {
    const x = values.length === 1 ? 0 : (index / (values.length - 1)) * 100;
    const y = 42 - (clampPercent(value) / 100) * 38;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(" ");
  const fill = `0,44 ${points} 100,44`;
  return (
    <svg className="sparkline" viewBox="0 0 100 44" preserveAspectRatio="none" role="img" aria-label="最近采样趋势">
      <polygon points={fill} />
      <polyline points={points} />
    </svg>
  );
}

function MonitoringTable({ nodes, loading }) {
  return (
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
  );
}
