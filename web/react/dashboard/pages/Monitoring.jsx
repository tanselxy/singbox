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
  const [saving, setSaving] = useState(false);
  const [editingNode, setEditingNode] = useState(null);
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
              load1: node.Load1,
              netRx: node.NetRxBytes,
              netTx: node.NetTxBytes,
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

  function openCreateNode() {
    setEditingNode(null);
    setNodeForm({ code: "", name: "" });
    setOpen(true);
  }

  function openEditNode(node) {
    setEditingNode(node);
    setNodeForm({ code: "", name: node.Name || "" });
    setOpen(true);
  }

  async function submitNode() {
    if (!editingNode && !nodeForm.code.trim()) {
      alert("请粘贴节点的接入码");
      return;
    }
    if (editingNode && !nodeForm.name.trim()) {
      alert("请填写节点名");
      return;
    }
    setSaving(true);
    try {
      const body = new URLSearchParams(
        editingNode
          ? { name: nodeForm.name.trim() }
          : { code: nodeForm.code.trim(), name: nodeForm.name.trim() },
      );
      const url = editingNode ? `${prefix}/api/nodes/${editingNode.ID}/update` : `${prefix}/api/nodes`;
      const res = await fetch(url, { method: "POST", body });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`${editingNode ? "保存" : "添加"}失败: ${payload.error || ""}`);
        return;
      }
      if (payload.warn) alert(payload.warn);
      setNodeForm({ code: "", name: "" });
      setEditingNode(null);
      setOpen(false);
      load();
    } catch (err) {
      alert(`请求失败: ${err.message}`);
    } finally {
      setSaving(false);
    }
  }

  async function deleteNode(node) {
    if (!confirm(`确认删除节点「${node.Name}」？`)) return;
    const res = await fetch(`${prefix}/api/nodes/${node.ID}/delete`, { method: "POST" });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`删除失败: ${payload.error || ""}`);
      return;
    }
    setHistory((current) => {
      const next = { ...current };
      delete next[node.ID];
      return next;
    });
    load();
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
            <Button onClick={openCreateNode}>+ 新增节点</Button>
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
            <MonitoringCharts nodes={nodes} history={history} loading={loading} onEdit={openEditNode} onDelete={deleteNode} />
          ) : (
            <MonitoringTable nodes={nodes} history={history} loading={loading} onEdit={openEditNode} onDelete={deleteNode} />
          )}
        </CardContent>
      </Card>
      <NodeDialog
        open={open}
        form={nodeForm}
        saving={saving}
        editing={Boolean(editingNode)}
        onChange={setNodeForm}
        onCancel={() => {
          setOpen(false);
          setEditingNode(null);
        }}
        onSubmit={submitNode}
      />
    </>
  );
}

function NodeDialog({ open, form, saving, editing, onChange, onCancel, onSubmit }) {
  return (
    <ModalFrame
      open={open}
      onCancel={onCancel}
      eyebrow="Node"
      title={editing ? "编辑节点" : "新增节点"}
      footer={
        <>
          <Button variant="outline" onClick={onCancel} disabled={saving}>取消</Button>
          <Button onClick={onSubmit} disabled={saving}>{saving ? "保存中..." : editing ? "保存修改" : "确认添加"}</Button>
        </>
      }
    >
      {!editing && (
        <Label>接入码
          <Textarea value={form.code} onChange={(e) => onChange({ ...form, code: e.target.value })} rows="4" placeholder="粘贴节点页复制的接入码" autoFocus />
        </Label>
      )}
      <Label>节点名
        <Input value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} placeholder={editing ? "节点显示名称" : "可留空，默认使用节点 IP"} autoFocus={editing} />
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
    CPUCores: Number(node.CPUCores || 0),
    Load1: Number(node.Load1 || 0),
    Load5: Number(node.Load5 || 0),
    Load15: Number(node.Load15 || 0),
    NetRxBytes: Number(node.NetRxBytes || 0),
    NetTxBytes: Number(node.NetTxBytes || 0),
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

function MonitoringCharts({ nodes, history, loading, onEdit, onDelete }) {
  if (nodes.length === 0) {
    return <div className="py-10 text-center text-sm text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</div>;
  }
  return (
    <div className="grid gap-4 xl:grid-cols-2">
      {nodes.map((node) => (
        <NodeMetricCard key={node.ID} node={node} history={history[node.ID] || []} onEdit={onEdit} onDelete={onDelete} />
      ))}
    </div>
  );
}

function NodeMetricCard({ node, history, onEdit, onDelete }) {
  const rates = networkRates(history);
  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h4 className="truncate font-medium">{node.Name}</h4>
          <p className="truncate font-mono text-xs text-muted-foreground">{node.Address}</p>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-2">
          <Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge>
          <div className="flex gap-1">
            <Button size="sm" variant="outline" onClick={() => onEdit(node)}>编辑</Button>
            <Button size="sm" variant="destructive" onClick={() => onDelete(node)}>删除</Button>
          </div>
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <MetricBlock label="CPU" sub={`${node.CPUCores || "-"} 核`} value={`${node.CPUPercent.toFixed(1)}%`} percent={node.CPUPercent} samples={history.map((i) => i.cpu)} tone="text-chart-1" />
        <MetricBlock label="内存" value={node.Mem || "-"} percent={node.MemPercent} samples={history.map((i) => i.mem)} tone="text-chart-2" />
        <MetricBlock label="硬盘" value={node.Disk || "-"} percent={node.DiskPercent} samples={history.map((i) => i.disk)} tone="text-chart-3" />
        <MetricBlock label="负载" sub={`5m ${formatLoad(node.Load5)} · 15m ${formatLoad(node.Load15)}`} value={formatLoad(node.Load1)} percent={loadPercent(node)} samples={history.map((i) => loadPercent({ CPUCores: node.CPUCores, Load1: i.load1 }))} tone="text-chart-4" />
      </div>
      <div className="mt-4 grid gap-3 border-t pt-4 sm:grid-cols-2 lg:grid-cols-4">
        <MiniMetric label="上行" value={rates.upRate} />
        <MiniMetric label="下行" value={rates.downRate} />
        <MiniMetric label="出站累计" value={formatBytes(node.NetTxBytes)} />
        <MiniMetric label="入站累计" value={formatBytes(node.NetRxBytes)} />
      </div>
    </div>
  );
}

function MetricBlock({ label, sub, value, percent, samples, tone }) {
  return (
    <div className={tone}>
      <div className="mb-1 flex items-start justify-between gap-3 text-sm">
        <span>
          <span className="block text-muted-foreground">{label}</span>
          {sub && <span className="text-xs text-muted-foreground">{sub}</span>}
        </span>
        <strong className="tabular-nums text-foreground">{value}</strong>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <span className="block h-full rounded-full bg-current" style={{ width: `${clampPercent(percent)}%` }} />
      </div>
      <Sparkline samples={samples} />
    </div>
  );
}

function MiniMetric({ label, value }) {
  return (
    <div className="rounded-lg border bg-background px-3 py-2">
      <span className="text-xs text-muted-foreground">{label}</span>
      <strong className="mt-1 block text-sm tabular-nums">{value}</strong>
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

function MonitoringTable({ nodes, history, loading, onEdit, onDelete }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>节点</TableHead>
          <TableHead>地址</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>CPU</TableHead>
          <TableHead>负载</TableHead>
          <TableHead>内存</TableHead>
          <TableHead>硬盘</TableHead>
          <TableHead>上行</TableHead>
          <TableHead>下行</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {nodes.length === 0 && (
          <TableRow><TableCell colSpan="10" className="text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</TableCell></TableRow>
        )}
        {nodes.map((node) => {
          const rates = networkRates(history[node.ID] || []);
          return (
            <TableRow key={node.ID}>
              <TableCell>{node.Name}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">{node.Address}</TableCell>
              <TableCell><Badge active={node.Online}>{node.Online ? "在线" : "离线"}</Badge></TableCell>
              <TableCell>{node.CPU || "-"}</TableCell>
              <TableCell>{formatLoad(node.Load1)}</TableCell>
              <TableCell>{node.Mem || "-"}</TableCell>
              <TableCell>{node.Disk || "-"}</TableCell>
              <TableCell>{rates.upRate}</TableCell>
              <TableCell>{rates.downRate}</TableCell>
              <TableCell>
                <div className="flex gap-1">
                  <Button size="sm" variant="outline" onClick={() => onEdit(node)}>编辑</Button>
                  <Button size="sm" variant="destructive" onClick={() => onDelete(node)}>删除</Button>
                </div>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

function networkRates(history) {
  if (history.length < 2) return { upRate: "采样中", downRate: "采样中" };
  const prev = history[history.length - 2];
  const curr = history[history.length - 1];
  const seconds = Math.max(1, (curr.t - prev.t) / 1000);
  const up = Math.max(0, (curr.netTx || 0) - (prev.netTx || 0)) / seconds;
  const down = Math.max(0, (curr.netRx || 0) - (prev.netRx || 0)) / seconds;
  return { upRate: `${formatBytes(up)}/s`, downRate: `${formatBytes(down)}/s` };
}

function loadPercent(node) {
  const cores = Math.max(1, Number(node.CPUCores || 1));
  return clampPercent((Number(node.Load1 || 0) / cores) * 100);
}

function formatLoad(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number.toFixed(2) : "-";
}

function formatBytes(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let size = number;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size >= 10 || index === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[index]}`;
}
