import { useEffect, useState } from "react";
import { PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";
import { Input } from "../../ui/input.jsx";
import { Label } from "../../ui/label.jsx";

const DEFAULT_CONFIG = {
  telegram_enabled: false,
  telegram_bot_token: "",
  telegram_chat_id: "",
  notify_node_offline: true,
  notify_cpu: true,
  cpu_threshold: 85,
  notify_memory: true,
  memory_threshold: 85,
  notify_disk: true,
  disk_threshold: 90,
  cooldown_minutes: 30,
};

export function Notifications({ prefix }) {
  const [config, setConfig] = useState(DEFAULT_CONFIG);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const res = await fetch(`${prefix}/api/notifications`);
      const payload = await res.json();
      if (payload.ok) setConfig({ ...DEFAULT_CONFIG, ...payload.config });
      else alert(`读取失败: ${payload.error || ""}`);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  function update(patch) {
    setConfig((current) => ({ ...current, ...patch }));
  }

  async function save({ quiet = false } = {}) {
    setSaving(true);
    try {
      const res = await fetch(`${prefix}/api/notifications`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`保存失败: ${payload.error || ""}`);
        return;
      }
      if (!quiet) alert("已保存");
      return true;
    } finally {
      setSaving(false);
    }
  }

  async function testTelegram() {
    setTesting(true);
    try {
      const saved = await save({ quiet: true });
      if (!saved) return;
      const res = await fetch(`${prefix}/api/notifications/test`, { method: "POST" });
      const payload = await res.json();
      if (!payload.ok) {
        alert(`发送失败: ${payload.error || ""}`);
        return;
      }
      alert("测试通知已发送");
    } finally {
      setTesting(false);
    }
  }

  return (
    <>
      <PageHead
        meta={VIEW_META.notifications}
        action={<Button onClick={save} disabled={saving || loading}>{saving ? "保存中..." : "保存配置"}</Button>}
      />
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card>
          <CardHeader>
            <CardTitle>Telegram</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <ToggleRow
              label="启用 TG 通知"
              checked={config.telegram_enabled}
              onChange={(checked) => update({ telegram_enabled: checked })}
            />
            <Label>Bot Token
              <Input
                value={config.telegram_bot_token}
                onChange={(e) => update({ telegram_bot_token: e.target.value })}
                type="password"
                placeholder="123456:ABC-DEF..."
                autoComplete="off"
              />
            </Label>
            <Label>Chat ID
              <Input
                value={config.telegram_chat_id}
                onChange={(e) => update({ telegram_chat_id: e.target.value })}
                placeholder="-1001234567890"
              />
            </Label>
            <Button variant="outline" onClick={testTelegram} disabled={testing || loading}>
              {testing ? "发送中..." : "发送测试通知"}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>通知规则</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <ToggleRow
              label="节点离线"
              checked={config.notify_node_offline}
              onChange={(checked) => update({ notify_node_offline: checked })}
            />
            <ThresholdRow
              label="CPU"
              enabled={config.notify_cpu}
              value={config.cpu_threshold}
              onEnabled={(checked) => update({ notify_cpu: checked })}
              onValue={(value) => update({ cpu_threshold: value })}
            />
            <ThresholdRow
              label="内存"
              enabled={config.notify_memory}
              value={config.memory_threshold}
              onEnabled={(checked) => update({ notify_memory: checked })}
              onValue={(value) => update({ memory_threshold: value })}
            />
            <ThresholdRow
              label="硬盘"
              enabled={config.notify_disk}
              value={config.disk_threshold}
              onEnabled={(checked) => update({ notify_disk: checked })}
              onValue={(value) => update({ disk_threshold: value })}
            />
            <Label>通知冷却（分钟）
              <Input
                value={config.cooldown_minutes}
                onChange={(e) => update({ cooldown_minutes: Number(e.target.value) })}
                type="number"
                min="1"
                max="1440"
              />
            </Label>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

function ToggleRow({ label, checked, onChange }) {
  return (
    <label className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2 text-sm font-medium">
      <span>{label}</span>
      <input
        className="h-4 w-4 accent-primary"
        type="checkbox"
        checked={Boolean(checked)}
        onChange={(e) => onChange(e.target.checked)}
      />
    </label>
  );
}

function ThresholdRow({ label, enabled, value, onEnabled, onValue }) {
  return (
    <div className="rounded-lg border bg-background p-3">
      <ToggleRow label={`${label} 超阈值`} checked={enabled} onChange={onEnabled} />
      <div className="mt-3 flex items-center gap-2">
        <Input
          value={value}
          onChange={(e) => onValue(Number(e.target.value))}
          type="number"
          min="1"
          max="100"
          disabled={!enabled}
        />
        <span className="text-sm text-muted-foreground">%</span>
      </div>
    </div>
  );
}
