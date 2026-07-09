import { Badge, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

async function runSystemAction(prefix, action, refresh) {
  const res = await fetch(`${prefix}/api/system/${action}`, { method: "POST" });
  const payload = await res.json();
  if (!payload.ok) {
    alert(`操作失败: ${payload.error || ""}`);
    return;
  }
  alert("操作成功");
  refresh();
}

export function Toolbox({ data, refresh, prefix }) {
  const system = data.System || {};
  return (
    <>
      <PageHead meta={VIEW_META.toolbox} />
      <section className="card">
        <div className="sys-row">
          <div>
            <div className="muted">BBR + TCP 优化</div>
            <Badge active={system.BBR}>{system.BBR ? "已开启" : "未开启"}</Badge>
          </div>
          <button onClick={() => runSystemAction(prefix, "bbr", refresh)} disabled={system.BBR}>开启 BBR</button>
        </div>
        <div className="sys-row">
          <div>
            <div className="muted">fail2ban（SSH 防爆破）</div>
            <Badge active={system.Fail2ban}>{system.Fail2ban ? "运行中" : "未安装"}</Badge>
          </div>
          <button onClick={() => runSystemAction(prefix, "fail2ban", refresh)} disabled={system.Fail2ban}>安装并启用</button>
        </div>
      </section>

      <section className="card">
        <div className="node-head">
          <h3>配置工具</h3>
        </div>
        <div className="quick-grid">
          <a className="quick-link" href={`${prefix}/tools`}>
            <strong>节点转换</strong>
            <span className="muted">把节点链接转换成 Clash 代理片段</span>
          </a>
        </div>
      </section>
    </>
  );
}
