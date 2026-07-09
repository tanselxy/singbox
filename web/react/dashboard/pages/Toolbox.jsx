import { Badge, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";

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

function ToggleRow({ label, active, activeText, inactiveText, actionText, onAction }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 border-b py-4 last:border-0">
      <div>
        <div className="text-sm text-muted-foreground">{label}</div>
        <div className="mt-1"><Badge active={active}>{active ? activeText : inactiveText}</Badge></div>
      </div>
      <Button variant="outline" onClick={onAction} disabled={active}>{actionText}</Button>
    </div>
  );
}

export function Toolbox({ data, refresh, prefix }) {
  const system = data.System || {};
  return (
    <>
      <PageHead meta={VIEW_META.toolbox} />
      <Card>
        <CardContent className="py-0">
          <ToggleRow
            label="BBR + TCP 优化"
            active={system.BBR}
            activeText="已开启"
            inactiveText="未开启"
            actionText="开启 BBR"
            onAction={() => runSystemAction(prefix, "bbr", refresh)}
          />
          <ToggleRow
            label="fail2ban（SSH 防爆破）"
            active={system.Fail2ban}
            activeText="运行中"
            inactiveText="未安装"
            actionText="安装并启用"
            onAction={() => runSystemAction(prefix, "fail2ban", refresh)}
          />
        </CardContent>
      </Card>

      <div className="mt-6">
        <Card>
          <CardHeader><CardTitle>配置工具</CardTitle></CardHeader>
          <CardContent>
            <a
              className="flex flex-col gap-1 rounded-lg border p-4 transition-colors hover:bg-accent"
              href={`${prefix}/tools`}
            >
              <strong className="text-sm">节点转换</strong>
              <span className="text-sm text-muted-foreground">把节点链接转换成 Clash 代理片段</span>
            </a>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
