import { useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

const EMPTY_CLIENT_FORM = { name: "", quota: "", quotaUnit: "GB", devices: "", expires: "" };

export function Clients({ data, refresh, prefix }) {
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editingClient, setEditingClient] = useState(null);
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
      <PageHead meta={VIEW_META.clients} action={<button onClick={openCreate}>+ 新增客户</button>} />
      <section className="card">
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
                    <button onClick={() => openEdit(client)}>编辑</button>
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
      titleId="create-client-title"
      footer={
        <>
          <button className="secondary" onClick={onCancel} disabled={creating}>取消</button>
          <button onClick={onSubmit} disabled={creating}>{creating ? "保存中..." : editing ? "保存修改" : "确认创建"}</button>
        </>
      }
    >
      <label className="form-field wide">客户名
        <input value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} type="text" placeholder="例如 tansel" autoFocus />
      </label>
      <label className="form-field">流量配额
        <span className="input-group">
          <input value={form.quota} onChange={(e) => onChange({ ...form, quota: e.target.value })} type="number" min="0" step="1" placeholder="留空不限" />
          <select value={form.quotaUnit} onChange={(e) => onChange({ ...form, quotaUnit: e.target.value })}>
            <option value="GB">GB</option>
            <option value="MB">MB</option>
          </select>
        </span>
      </label>
      <label className="form-field">设备数
        <input value={form.devices} onChange={(e) => onChange({ ...form, devices: e.target.value })} type="number" min="0" step="1" placeholder="0 = 不限" />
      </label>
      <label className="form-field wide">到期日期
        <input value={form.expires} onChange={(e) => onChange({ ...form, expires: e.target.value })} type="date" title="到期日期（留空=永久）" />
      </label>
    </ModalFrame>
  );
}
