import { useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent } from "../../ui/card.jsx";
import { Input, Select } from "../../ui/input.jsx";
import { Label } from "../../ui/label.jsx";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../ui/table.jsx";

const EMPTY_CLIENT_FORM = { name: "", quota: "", quotaUnit: "GB", devices: "", expires: "" };

export function Clients({ data, refresh, prefix }) {
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editingClient, setEditingClient] = useState(null);
  const [devicesOpen, setDevicesOpen] = useState(false);
  const [devicesClient, setDevicesClient] = useState(null);
  const [devicesLoading, setDevicesLoading] = useState(false);
  const [devices, setDevices] = useState([]);
  const [form, setForm] = useState(EMPTY_CLIENT_FORM);
  const clients = data.Clients || [];

  function openCreate() {
    setEditingClient(null);
    setForm(EMPTY_CLIENT_FORM);
    setOpen(true);
  }

  function openEdit(client) {
    setEditingClient(client);
    setForm({
      name: client.Name || "",
      quota: client.QuotaValue || "",
      quotaUnit: client.QuotaUnit || "GB",
      devices: client.DeviceLimit ? String(client.DeviceLimit) : "",
      expires: client.ExpiresAtDate || "",
    });
    setOpen(true);
  }

  async function submitClient() {
    if (!form.name.trim()) {
      alert("请填写客户名");
      return;
    }
    setCreating(true);
    const body = new URLSearchParams({
      name: form.name.trim(),
      quota: form.quota.trim(),
      quota_unit: form.quotaUnit,
      device_limit: form.devices.trim(),
      expires_at: form.expires,
    });
    try {
      const url = editingClient ? `${prefix}/api/clients/${editingClient.ID}/update` : `${prefix}/api/clients`;
      const res = await fetch(url, { method: "POST", body });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`${editingClient ? "保存" : "创建"}失败: ${payload.error || ""}`);
        return;
      }
      setForm(EMPTY_CLIENT_FORM);
      setEditingClient(null);
      setOpen(false);
      refresh();
    } finally {
      setCreating(false);
    }
  }

  async function clientAction(client, action) {
    if (action === "delete" && !confirm("确认删除该客户？其订阅将立即失效。")) return;
    if (action === "reset-subscription" && !confirm("确认重置订阅？该客户旧订阅地址和所有旧协议链接会立即失效。")) return;
    const res = await fetch(`${prefix}/api/clients/${client.ID}/${action}`, { method: "POST" });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`操作失败: ${payload.error || ""}`);
      return;
    }
    refresh();
  }

  async function openDevices(client) {
    setDevicesClient(client);
    setDevices([]);
    setDevicesOpen(true);
    setDevicesLoading(true);
    try {
      const res = await fetch(`${prefix}/api/clients/${client.ID}/devices`);
      const payload = await res.json();
      if (!payload.ok) {
        alert(`读取设备失败: ${payload.error || ""}`);
        return;
      }
      setDevices(payload.devices || []);
    } finally {
      setDevicesLoading(false);
    }
  }

  async function kickDevice(device) {
    if (!devicesClient || !confirm(`确认踢下线 ${device.source_ip}？该 IP 下当前连接会被关闭。`)) return;
    const body = new URLSearchParams({ source_ip: device.source_ip });
    const res = await fetch(`${prefix}/api/clients/${devicesClient.ID}/devices/kick`, { method: "POST", body });
    const payload = await res.json();
    if (!payload.ok) {
      alert(`踢下线失败: ${payload.error || ""}`);
      return;
    }
    await openDevices(devicesClient);
  }

  return (
    <>
      <PageHead meta={VIEW_META.clients} action={<Button onClick={openCreate}>+ 新增客户</Button>} />
      <Card>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">流量/到期到点会自动停用该客户；设备数按不同来源 IP 进行连接限制。</p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>已用</TableHead>
                <TableHead>配额</TableHead>
                <TableHead>设备</TableHead>
                <TableHead>到期</TableHead>
                <TableHead>操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {clients.length === 0 && (
                <TableRow>
                  <TableCell colSpan="7" className="text-muted-foreground">还没有客户，点右上角新增。</TableCell>
                </TableRow>
              )}
              {clients.map((client) => (
                <TableRow key={client.ID}>
                  <TableCell>
                    <a className="font-medium text-primary hover:underline" href={`${prefix}/client/${client.ID}`}>{client.Name}</a>
                  </TableCell>
                  <TableCell><Badge active={client.Enabled}>{client.Enabled ? "启用" : "停用"}</Badge></TableCell>
                  <TableCell className={client.OverQuota ? "text-destructive" : ""}>{client.Used}</TableCell>
                  <TableCell>{client.Quota}</TableCell>
                  <TableCell>{client.Devices}</TableCell>
                  <TableCell className={client.Expired ? "text-destructive" : ""}>{client.Expiry}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1.5">
                      <Button variant="outline" size="sm" onClick={() => openEdit(client)}>编辑</Button>
                      <Button variant="outline" size="sm" onClick={() => clientAction(client, client.Enabled ? "disable" : "enable")}>
                        {client.Enabled ? "停用" : "启用"}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => clientAction(client, "reset-subscription")}>重置订阅</Button>
                      <Button variant="outline" size="sm" onClick={() => openDevices(client)}>设备</Button>
                      <Button variant="outline" size="sm" onClick={() => clientAction(client, "reset-traffic")}>清零流量</Button>
                      <Button variant="destructive" size="sm" onClick={() => clientAction(client, "delete")}>删除</Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <ClientCreateDialog
        open={open}
        form={form}
        creating={creating}
        editing={Boolean(editingClient)}
        onChange={setForm}
        onCancel={() => {
          setOpen(false);
          setEditingClient(null);
        }}
        onSubmit={submitClient}
      />
      <ClientDevicesDialog
        open={devicesOpen}
        client={devicesClient}
        loading={devicesLoading}
        devices={devices}
        onRefresh={() => devicesClient && openDevices(devicesClient)}
        onKick={kickDevice}
        onCancel={() => setDevicesOpen(false)}
      />
    </>
  );
}

function ClientCreateDialog({ open, form, creating, editing, onChange, onCancel, onSubmit }) {
  return (
    <ModalFrame
      open={open}
      onCancel={onCancel}
      eyebrow="Client"
      title={editing ? "编辑客户" : "新增客户"}
      footer={
        <>
          <Button variant="outline" onClick={onCancel} disabled={creating}>取消</Button>
          <Button onClick={onSubmit} disabled={creating}>{creating ? "保存中..." : editing ? "保存修改" : "确认创建"}</Button>
        </>
      }
    >
      <Label>客户名
        <Input value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} placeholder="例如 tansel" autoFocus />
      </Label>
      <Label>流量配额
        <div className="flex gap-2">
          <Input value={form.quota} onChange={(e) => onChange({ ...form, quota: e.target.value })} type="number" min="0" step="1" placeholder="留空不限" />
          <Select value={form.quotaUnit} onChange={(e) => onChange({ ...form, quotaUnit: e.target.value })} className="w-24">
            <option value="GB">GB</option>
            <option value="MB">MB</option>
          </Select>
        </div>
      </Label>
      <Label>设备数
        <Input value={form.devices} onChange={(e) => onChange({ ...form, devices: e.target.value })} type="number" min="0" step="1" placeholder="0 = 不限" />
      </Label>
      <Label>到期日期
        <Input value={form.expires} onChange={(e) => onChange({ ...form, expires: e.target.value })} type="date" title="到期日期（留空=永久）" />
      </Label>
    </ModalFrame>
  );
}

function ClientDevicesDialog({ open, client, loading, devices, onRefresh, onKick, onCancel }) {
  return (
    <ModalFrame
      open={open}
      onCancel={onCancel}
      eyebrow="Devices"
      title={client ? `${client.Name} 的设备` : "设备"}
      className="sm:max-w-4xl"
      footer={
        <>
          <Button variant="outline" onClick={onCancel}>关闭</Button>
          <Button onClick={onRefresh} disabled={loading}>{loading ? "加载中..." : "刷新"}</Button>
        </>
      }
    >
      <div className="max-h-[56vh] overflow-auto rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>来源 IP</TableHead>
              <TableHead>连接</TableHead>
              <TableHead>上传</TableHead>
              <TableHead>下载</TableHead>
              <TableHead>协议</TableHead>
              <TableHead>最近连接</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && (
              <TableRow>
                <TableCell colSpan="7" className="text-muted-foreground">正在加载设备...</TableCell>
              </TableRow>
            )}
            {!loading && devices.length === 0 && (
              <TableRow>
                <TableCell colSpan="7" className="text-muted-foreground">当前没有在线设备。</TableCell>
              </TableRow>
            )}
            {!loading && devices.map((device) => (
              <TableRow key={device.source_ip}>
                <TableCell className="font-medium tabular-nums">{device.source_ip}</TableCell>
                <TableCell>{device.connections}</TableCell>
                <TableCell>{device.upload}</TableCell>
                <TableCell>{device.download}</TableCell>
                <TableCell className="max-w-36 truncate">{(device.protocols || []).join("、") || "-"}</TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">{device.last_seen}</TableCell>
                <TableCell className="text-right">
                  <Button variant="destructive" size="sm" onClick={() => onKick(device)}>踢下线</Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </ModalFrame>
  );
}
