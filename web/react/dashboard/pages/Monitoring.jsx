import { useEffect, useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Input, Select, Textarea } from "../../ui/input.jsx";
import { Label } from "../../ui/label.jsx";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../ui/table.jsx";
import { cn } from "../../lib/utils.js";

const MONITORING_REFRESH_INTERVAL = 2000;

export function Monitoring({ prefix, publicView = false, apiPath = "" }) {
  const [nodes, setNodes] = useState([]);
  const [mode, setMode] = useState("charts");
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editingNode, setEditingNode] = useState(null);
  const [nodeForm, setNodeForm] = useState(defaultNodeForm());
  const [history, setHistory] = useState({});
  const [loading, setLoading] = useState(true);

  async function load() {
    try {
      const res = await fetch(apiPath || `${prefix}/api/node-metrics`, { cache: "no-store" });
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
            const sampleTime = Number(node.SampledAt || sampledAt);
            if (items.at(-1)?.sampleTime === sampleTime) return;
            items.push({
              t: sampleTime,
              sampleTime,
              netRx: node.NetRxBytes,
              netTx: node.NetTxBytes,
            });
            next[node.ID] = items.slice(-36);
          });
          return next;
        });
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = setInterval(load, MONITORING_REFRESH_INTERVAL);
    return () => clearInterval(timer);
  }, []);

  function openCreateNode() {
    if (publicView) return;
    setEditingNode(null);
    setNodeForm(defaultNodeForm());
    setOpen(true);
  }

  function openEditNode(node) {
    if (publicView) return;
    setEditingNode(node);
    setNodeForm({
      code: "",
      name: node.Name || "",
      tag: node.Tag || "",
      startAt: node.StartAtDate || "",
      endAt: node.EndAtDate || "",
      billingCycle: node.BillingCycle || "",
    });
    setOpen(true);
  }

  async function submitNode() {
    if (publicView) return;
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
          ? nodeFormParams(nodeForm)
          : { code: nodeForm.code.trim(), ...nodeFormParams(nodeForm) },
      );
      const url = editingNode ? `${prefix}/api/nodes/${editingNode.ID}/update` : `${prefix}/api/nodes`;
      const res = await fetch(url, { method: "POST", body });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`${editingNode ? "保存" : "添加"}失败: ${payload.error || ""}`);
        return;
      }
      if (payload.warn) alert(payload.warn);
      setNodeForm(defaultNodeForm());
      setEditingNode(null);
      setOpen(false);
      load();
    } catch (err) {
      alert(`请求失败: ${err.message}`);
    } finally {
      setSaving(false);
    }
  }

  async function renewNode(node) {
    if (publicView) return;
    if (!confirm(`确认已为「${node.Name}」续期？`)) return;
    const res = await fetch(`${prefix}/api/nodes/${node.ID}/renew`, { method: "POST" });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`续期失败: ${payload.error || ""}`);
      return;
    }
    load();
  }

  async function deleteNode(node) {
    if (publicView) return;
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
            {!publicView && <Button onClick={openCreateNode}>+ 新增节点</Button>}
          </div>
        }
      />
      {mode === "charts" ? (
        <MonitoringCharts nodes={nodes} history={history} loading={loading} publicView={publicView} onEdit={openEditNode} onRenew={renewNode} onDelete={deleteNode} />
      ) : (
        <MonitoringTable nodes={nodes} history={history} loading={loading} publicView={publicView} onEdit={openEditNode} onRenew={renewNode} onDelete={deleteNode} />
      )}
      {!publicView && (
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
      )}
    </>
  );
}

function defaultNodeForm() {
  const startAt = dateInput(new Date());
  return {
    code: "",
    name: "",
    tag: "",
    startAt,
    endAt: dateInput(addMonths(new Date(), 1)),
    billingCycle: "monthly",
  };
}

function nodeFormParams(form) {
  return {
    name: form.name.trim(),
    tag: form.tag.trim(),
    start_at: form.startAt,
    end_at: form.endAt,
    billing_cycle: form.billingCycle,
  };
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
      <Label>标签 / 用途
        <Input value={form.tag} onChange={(e) => onChange({ ...form, tag: e.target.value })} placeholder="例如：荷兰中转、流媒体、备用节点" />
      </Label>
      <div className="grid gap-3 sm:grid-cols-2">
        <Label>开始时间
          <Input value={form.startAt} onChange={(e) => onChange({ ...form, startAt: e.target.value })} type="date" />
        </Label>
        <Label>结束时间
          <Input value={form.endAt} onChange={(e) => onChange({ ...form, endAt: e.target.value })} type="date" />
        </Label>
      </div>
      <Label>续费周期
        <Select value={form.billingCycle} onChange={(e) => onChange({ ...form, billingCycle: e.target.value })}>
          <option value="">不自动续期</option>
          <option value="monthly">月</option>
          <option value="quarterly">季</option>
          <option value="half_year">半年</option>
          <option value="yearly">年</option>
          <option value="two_years">2 年</option>
          <option value="three_years">3 年</option>
        </Select>
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

function MonitoringCharts({ nodes, history, loading, publicView, onEdit, onRenew, onDelete }) {
  if (nodes.length === 0) {
    return <div className="py-10 text-center text-sm text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</div>;
  }
  return (
    <div className="grid items-start gap-4 md:grid-cols-[repeat(auto-fill,minmax(360px,560px))]">
      {nodes.map((node) => (
        <NodeMetricCard key={node.ID} node={node} history={history[node.ID] || []} publicView={publicView} onEdit={onEdit} onRenew={onRenew} onDelete={onDelete} />
      ))}
    </div>
  );
}

function NodeMetricCard({ node, history, publicView, onEdit, onRenew, onDelete }) {
  const rates = networkRates(history);
  const tags = splitTags(node.Tag);
  const statusLabel = node.Pending ? "采集中" : node.Online ? "在线" : "离线";
  return (
    <div className="w-full max-w-xl rounded-xl border bg-card p-4 shadow-sm">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h4 className="truncate font-medium">{node.Name}</h4>
          {!publicView && <p className="truncate font-mono text-xs text-muted-foreground">{node.Address}</p>}
        </div>
        <Badge active={node.Online}>{statusLabel}</Badge>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <MetricBlock label="CPU" sub={`${node.CPUCores || "-"} 核`} value={node.Online ? `${node.CPUPercent.toFixed(1)}%` : "-"} percent={node.CPUPercent} tone="text-chart-1" />
        <MetricBlock label="内存" value={node.Mem || "-"} percent={node.MemPercent} tone="text-chart-2" />
        <MetricBlock label="硬盘" value={node.Disk || "-"} percent={node.DiskPercent} tone="text-chart-3" />
        <MetricBlock label="负载" value={node.Online ? formatLoad(node.Load1) : "-"} percent={loadPercent(node)} tone="text-chart-4" />
      </div>
      <div className="mt-4 grid gap-3 border-t pt-4 sm:grid-cols-2 lg:grid-cols-4">
        <MiniMetric label="上行" value={rates.upRate} />
        <MiniMetric label="下行" value={rates.downRate} />
        <MiniMetric label="出站累计" value={formatBytes(node.NetTxBytes)} />
        <MiniMetric label="入站累计" value={formatBytes(node.NetRxBytes)} />
      </div>
      <div className="mt-4 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        {node.StartAtDate && <span>开始 {node.StartAtDate}</span>}
        {node.EndAtDate && <span>续费 {node.EndAtDate}</span>}
        {node.BillingCycleLabel && node.BillingCycleLabel !== "未设置" && <span>周期 {node.BillingCycleLabel}</span>}
      </div>
      <div className="mt-4 flex items-end justify-between gap-3">
        <div className="flex flex-wrap gap-2">
          {tags.length > 0 ? tags.map((tag) => <TagBadge key={tag}>{tag}</TagBadge>) : (
            !publicView && <Button size="sm" variant="secondary" onClick={() => onEdit(node)}>添加标签</Button>
          )}
        </div>
        {!publicView && (
          <div className="flex flex-wrap justify-end gap-1">
            <Button size="sm" variant="secondary" onClick={() => onRenew(node)} disabled={!node.EndAtDate || !node.BillingCycle}>续期</Button>
            <Button size="sm" variant="outline" onClick={() => onEdit(node)}>编辑</Button>
            <Button size="sm" variant="destructive" onClick={() => onDelete(node)}>删除</Button>
          </div>
        )}
      </div>
    </div>
  );
}

function MetricBlock({ label, sub, value, percent, tone }) {
  return (
    <div className={tone}>
      <div className="mb-1 flex items-start justify-between gap-3 text-sm">
        <span className="flex min-w-0 items-baseline gap-2">
          <span className="text-muted-foreground">{label}</span>
          {sub && <span className="text-xs text-muted-foreground">{sub}</span>}
        </span>
        <strong className="tabular-nums text-foreground">{value}</strong>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <span className="block h-full rounded-full bg-current" style={{ width: `${clampPercent(percent)}%` }} />
      </div>
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

function TagBadge({ children }) {
  return <span className="rounded-md bg-secondary px-2.5 py-1 text-xs font-medium text-secondary-foreground">{children}</span>;
}

function MonitoringTable({ nodes, history, loading, publicView, onEdit, onRenew, onDelete }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>节点</TableHead>
          {!publicView && <TableHead>地址</TableHead>}
          <TableHead>状态</TableHead>
          <TableHead>CPU</TableHead>
          <TableHead>负载</TableHead>
          <TableHead>内存</TableHead>
          <TableHead>硬盘</TableHead>
          <TableHead>上行</TableHead>
          <TableHead>下行</TableHead>
          <TableHead>标签</TableHead>
          <TableHead>续费</TableHead>
          {!publicView && <TableHead>操作</TableHead>}
        </TableRow>
      </TableHeader>
      <TableBody>
        {nodes.length === 0 && (
          <TableRow><TableCell colSpan={publicView ? "10" : "12"} className="text-muted-foreground">{loading ? "正在读取节点指标。" : "还没有受控节点。"}</TableCell></TableRow>
        )}
        {nodes.map((node) => {
          const rates = networkRates(history[node.ID] || []);
          return (
            <TableRow key={node.ID}>
              <TableCell>{node.Name}</TableCell>
              {!publicView && <TableCell className="font-mono text-xs text-muted-foreground">{node.Address}</TableCell>}
              <TableCell><Badge active={node.Online}>{node.Pending ? "采集中" : node.Online ? "在线" : "离线"}</Badge></TableCell>
              <TableCell>{node.CPU || "-"}</TableCell>
              <TableCell>{node.Online ? formatLoad(node.Load1) : "-"}</TableCell>
              <TableCell>{node.Mem || "-"}</TableCell>
              <TableCell>{node.Disk || "-"}</TableCell>
              <TableCell>{rates.upRate}</TableCell>
              <TableCell>{rates.downRate}</TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {splitTags(node.Tag).map((tag) => <TagBadge key={tag}>{tag}</TagBadge>)}
                </div>
              </TableCell>
              <TableCell>{node.EndAtDate ? `${node.EndAtDate} / ${node.BillingCycleLabel}` : "-"}</TableCell>
              {!publicView && (
                <TableCell>
                  <div className="flex gap-1">
                    <Button size="sm" variant="secondary" onClick={() => onRenew(node)} disabled={!node.EndAtDate || !node.BillingCycle}>续期</Button>
                    <Button size="sm" variant="outline" onClick={() => onEdit(node)}>编辑</Button>
                    <Button size="sm" variant="destructive" onClick={() => onDelete(node)}>删除</Button>
                  </div>
                </TableCell>
              )}
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

function splitTags(value) {
  return String(value || "")
    .split(/[,，、\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function dateInput(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function addMonths(date, months) {
  const next = new Date(date);
  next.setMonth(next.getMonth() + months);
  return next;
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
