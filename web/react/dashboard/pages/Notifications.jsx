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
  offline_repeat_hours: 0,
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
            <SettingToggle
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
          <CardContent className="pt-0">
            <div className="divide-y">
            <RuleRow
              id="notify-node-offline"
              label="节点离线"
              description="节点首次离线时立即通知"
              checked={config.notify_node_offline}
              onChange={(checked) => update({ notify_node_offline: checked })}
              control={
                <CompactNumber
                  label="重复提醒间隔"
                  value={config.offline_repeat_hours}
                  onChange={(value) => update({ offline_repeat_hours: value })}
                  disabled={!config.notify_node_offline}
                  min="0"
                  max="720"
                  unit="小时"
                />
              }
            />
            <RuleRow
              id="notify-cpu"
              label="CPU 使用率"
              description="使用率达到设定值时通知"
              enabled={config.notify_cpu}
              value={config.cpu_threshold}
              onEnabled={(checked) => update({ notify_cpu: checked })}
              onValue={(value) => update({ cpu_threshold: value })}
            />
            <RuleRow
              id="notify-memory"
              label="内存使用率"
              description="使用率达到设定值时通知"
              enabled={config.notify_memory}
              value={config.memory_threshold}
              onEnabled={(checked) => update({ notify_memory: checked })}
              onValue={(value) => update({ memory_threshold: value })}
            />
            <RuleRow
              id="notify-disk"
              label="硬盘使用率"
              description="使用率达到设定值时通知"
              enabled={config.notify_disk}
              value={config.disk_threshold}
              onEnabled={(checked) => update({ notify_disk: checked })}
              onValue={(value) => update({ disk_threshold: value })}
            />
            </div>
            <div className="grid gap-4 border-t pt-5 sm:grid-cols-2">
              <CompactField
                label="指标通知冷却"
                description="同一指标再次提醒前的间隔"
                value={config.cooldown_minutes}
                onChange={(value) => update({ cooldown_minutes: value })}
                min="1"
                max="1440"
                unit="分钟"
              />
              <div className="self-end pb-1 text-sm text-muted-foreground">阈值规则会分别记录冷却时间。</div>
            </div>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

function SettingToggle({ label, checked, onChange }) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm font-medium">
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

function RuleRow({ id, label, description, checked, onChange, enabled, value, onEnabled, onValue, control }) {
  const active = checked ?? enabled;
  const updateActive = onChange || onEnabled;
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 py-4">
      <div className="flex min-w-0 items-start gap-3">
        <input
          id={id}
          className="mt-1 h-4 w-4 shrink-0 accent-primary"
          type="checkbox"
          checked={Boolean(active)}
          onChange={(event) => updateActive(event.target.checked)}
        />
        <div className="min-w-0">
          <label htmlFor={id} className="cursor-pointer text-sm font-medium">{label}</label>
          <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{description}</p>
        </div>
      </div>
      {control || (
        <CompactNumber
          label={`${label} 阈值`}
          value={value}
          onChange={onValue}
          disabled={!active}
          min="1"
          max="100"
          unit="%"
        />
      )}
    </div>
  );
}

function CompactNumber({ label, value, onChange, disabled, min, max, unit }) {
  return (
    <div className="flex items-center gap-2">
      <Input
        aria-label={label}
        className="h-8 w-20 text-right tabular-nums"
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        type="number"
        min={min}
        max={max}
        disabled={disabled}
      />
      <span className="w-8 text-xs text-muted-foreground">{unit}</span>
    </div>
  );
}

function CompactField({ label, description, value, onChange, min, max, unit }) {
  return (
    <Label className="gap-2">
      <span>{label}</span>
      <span className="text-xs font-normal text-muted-foreground">{description}</span>
      <CompactNumber label={label} value={value} onChange={onChange} min={min} max={max} unit={unit} />
    </Label>
  );
}
