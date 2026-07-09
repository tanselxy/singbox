import { useState } from "react";
import { PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

export function System({ data, prefix }) {
  const [latest, setLatest] = useState("");
  const [upgradeLabel, setUpgradeLabel] = useState("");

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

  return (
    <>
      <PageHead meta={VIEW_META.system} />
      <section className="card">
        <div className="sys-row">
          <div>
            <div className="muted">面板版本</div>
            <span className="mono">{data.Version}</span>
            <span className="small muted"> {latest}</span>
          </div>
          <span className="field compact-field">
            <button onClick={checkUpdate}>检查更新</button>
            {upgradeLabel && <button onClick={upgrade}>{upgradeLabel}</button>}
          </span>
        </div>
      </section>
    </>
  );
}
