import { useEffect, useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";
import { Input, Textarea } from "../../ui/input.jsx";
import { Label } from "../../ui/label.jsx";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../ui/table.jsx";
import { cn } from "../../lib/utils.js";

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
            items.push({ t: sampledAt, cpu: node.CPUPercent, mem: node.MemPercent, disk: node.DiskPercent });
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
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex rounded-md border p-0.5">
              <button
                className={cn("rounded px-3 py-1 text-sm cursor-pointer", mode === "charts" ? "bg-accent text-accent-foreground" : "text-muted-foreground")}
                onClick={() => setMode("charts")}
              >图表</button>
              <button
                className={cn("rounded px-3 py-1 text-sm cursor-pointer", mode === "table" ? "bg-accent text-accent-foreground" : "text-muted-foreground")}
                onClick={() => setMode("table")}
              >表格</button>
            </div>
            <Button onClick={() => setOpen(true)}>+ 新增节点</Button>
          </div>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>被控端资源</CardTitle>
          <span className="text-sm text-muted-foreground">{updatedAt ? `更新于 ${updatedAt}` : "等待采集"}</span>
        </CardHeader>
        <CardContent>
          {mode === "charts" ? (
            <MonitoringCharts nodes={nodes} history={history} loading={loading} />
          ) : (
            <MonitoringTable nodes={nodes} loading={loading} />
          )}
        </CardContent>
      </Card>
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
      footer={
        <>
          <Button variant="outline" onClick={onCancel} disabled={creating}>取消</Button>
          <Button onClick={onSubmit} disabled={creating}>{creating ? "添加中..." : "确认添加"}</Button>
        </>
      }
    >
      <Label>接入码
        <Textarea value={form.code} onChange={(e) => onChange({ ...form, code: e.target.value })} rows="4" placeholder="粘贴节点页复制的接入码" autoFocus />
      </Label>
      <Label>节点名
        <Input value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} placeholder="可留空，默认使用节点 IP" />
      </Label>
    </ModalFrame>
  );
}

function normalizeNodeMetrics(node) {
  return {
    ...node,
    CPUPercent: clampPercent(node.CPUPercent ?? parsePercent(node.CPU)),
    MemPercent: clampPercent(node.MemPercent),
    DiskPercent: clampPercent(node.DiskPercent),
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
    return <div className="py-10 text-center text-sm text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</div>;
  }
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {nodes.map((node) => (
        <NodeMetricCard key={node.ID} node={node} history={history[node.ID] || []} />
      ))}
    </div>
  );
}

function NodeMetricCard({ node, history }) {
  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h4 className="truncate font-medium">{node.Name}</h4>
          <p className="truncate font-mono text-xs text-muted-foreground">{node.Address}</p>
        </div>
        <Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge>
      </div>
      <div className="space-y-3">
        <MetricBlock label="CPU" value={`${node.CPUPercent.toFixed(1)}%`} percent={node.CPUPercent} samples={history.map((i) => i.cpu)} tone="text-chart-1" />
        <MetricBlock label="内存" value={node.Mem || "-"} percent={node.MemPercent} samples={history.map((i) => i.mem)} tone="text-chart-2" />
        <MetricBlock label="硬盘" value={node.Disk || "-"} percent={node.DiskPercent} samples={history.map((i) => i.disk)} tone="text-chart-3" />
      </div>
    </div>
  );
}

function MetricBlock({ label, value, percent, samples, tone }) {
  return (
    <div className={tone}>
      <div className="mb-1 flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <strong className="tabular-nums text-foreground">{value}</strong>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <span className="block h-full rounded-full bg-current" style={{ width: `${percent}%` }} />
      </div>
      <Sparkline samples={samples} />
    </div>
  );
}

function Sparkline({ samples }) {
  const values = samples.length > 1 ? samples : [0, ...samples];
  const points = values
    .map((value, index) => {
      const x = values.length === 1 ? 0 : (index / (values.length - 1)) * 100;
      const y = 42 - (clampPercent(value) / 100) * 38;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
  return (
    <svg className="mt-1.5 h-8 w-full text-current" viewBox="0 0 100 44" preserveAspectRatio="none" role="img" aria-label="最近采样趋势">
      <polygon points={`0,44 ${points} 100,44`} fill="currentColor" fillOpacity="0.12" />
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function MonitoringTable({ nodes, loading }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>节点</TableHead>
          <TableHead>地址</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>CPU</TableHead>
          <TableHead>内存</TableHead>
          <TableHead>硬盘</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {nodes.length === 0 && (
          <TableRow><TableCell colSpan="6" className="text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</TableCell></TableRow>
        )}
        {nodes.map((node) => (
          <TableRow key={node.ID}>
            <TableCell>{node.Name}</TableCell>
            <TableCell className="font-mono text-xs text-muted-foreground">{node.Address}</TableCell>
            <TableCell><Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge></TableCell>
            <TableCell>{node.CPU || "-"}</TableCell>
            <TableCell>{node.Mem || "-"}</TableCell>
            <TableCell>{node.Disk || "-"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
