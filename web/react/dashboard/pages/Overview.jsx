import { Badge, Metric, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent } from "../../ui/card.jsx";

export function Overview({ data, onServiceAction }) {
  const summary = data.Summary || {};
  return (
    <>
      <PageHead
        meta={VIEW_META.overview}
        action={
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => onServiceAction("restart")}>重启服务</Button>
            <Button variant="destructive" onClick={() => onServiceAction("stop")}>停止</Button>
            <Button onClick={() => onServiceAction("start")}>启动</Button>
          </div>
        }
      />

      <Card>
        <CardContent className="flex flex-wrap items-center gap-x-12 gap-y-4">
          <div>
            <div className="text-sm text-muted-foreground">服务器</div>
            <div className="mt-1 font-mono text-lg">{data.ServerIP || "-"}</div>
          </div>
          <div>
            <div className="text-sm text-muted-foreground">sing-box 状态</div>
            <div className="mt-1"><Badge active={data.Active}>{data.Active ? "运行中" : "已停止"}</Badge></div>
          </div>
          <div>
            <div className="text-sm text-muted-foreground">面板版本</div>
            <div className="mt-1 font-mono">{data.Version || "-"}</div>
          </div>
        </CardContent>
      </Card>

      <div className="mt-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Metric label="客户总数" value={summary.TotalClients || 0} />
        <Metric label="启用客户" value={summary.EnabledClients || 0} />
        <Metric label="有限额客户" value={summary.LimitedClients || 0} />
        <Metric label="已到期客户" value={summary.ExpiredClients || 0} alert={(summary.ExpiredClients || 0) > 0} />
      </div>
    </>
  );
}
