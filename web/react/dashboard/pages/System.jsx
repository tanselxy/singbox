import { useState } from "react";
import { PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";

export function System({ data, prefix }) {
  const [latest, setLatest] = useState("");
  const [upgradeLabel, setUpgradeLabel] = useState("");
  const [copied, setCopied] = useState(false);

  async function checkUpdate() {
    const res = await fetch(`${prefix}/api/version`);
    const payload = await res.json();
    if (payload.upgradable) {
      setLatest(`有新版本 ${payload.latest}`);
      setUpgradeLabel(`升级到 ${payload.latest}`);
    } else {
      setLatest(payload.latest ? `已是最新（${payload.latest}）` : "无法获取最新版本");
      setUpgradeLabel("");
    }
  }

  async function upgrade() {
    if (!confirm("确认升级？升级期间面板会短暂重启，请稍候刷新页面。")) return;
    setUpgradeLabel("升级中...");
    try {
      const res = await fetch(`${prefix}/api/upgrade`, { method: "POST" });
      const payload = await res.json();
      if (payload.ok) alert("已下载新版本，面板正在重启，约 10 秒后请刷新页面。");
      else {
        alert(`升级失败: ${payload.error || ""}`);
        setUpgradeLabel("重试升级");
      }
    } catch {
      alert("面板正在重启，约 10 秒后请刷新页面。");
    }
  }

  async function copyAccessCode() {
    if (!data.AccessCode) return;
    try {
      await navigator.clipboard.writeText(data.AccessCode);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      prompt("复制接入码", data.AccessCode);
    }
  }

  return (
    <>
      <PageHead meta={VIEW_META.system} />
      <div className="space-y-4">
        <Card>
          <CardContent className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <div className="text-sm text-muted-foreground">面板版本</div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="font-mono">{data.Version}</span>
                <span className="text-sm text-muted-foreground">{latest}</span>
              </div>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={checkUpdate}>检查更新</Button>
              {upgradeLabel && <Button onClick={upgrade}>{upgradeLabel}</Button>}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>本机接入码</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-background px-4 py-3">
              <div>
                <div className="text-sm font-medium">被控端接入</div>
                <div className="mt-1 font-mono text-xs text-muted-foreground">{data.SelfAgentURL || "-"}</div>
              </div>
              {data.Managed && <span className="rounded-md bg-destructive/10 px-2 py-1 text-xs font-medium text-destructive">已被主控接管</span>}
            </div>
            <div>
              <div className="mb-2 text-sm text-muted-foreground">把这串码复制到主控面板的“实时监控 / 新增节点”，这台机器就会被纳入监控。</div>
              <div className="flex gap-2">
                <input
                  className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  readOnly
                  value={data.AccessCode || ""}
                />
                <Button onClick={copyAccessCode}>{copied ? "已复制" : "复制"}</Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
