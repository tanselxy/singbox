import { useState } from "react";
import { Badge, ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../ui/table.jsx";

const PAGE_SIZE = 25;

export function Toolbox({ data, refresh, prefix }) {
  const system = data.System || {};
  const fail2banInstalled = Boolean(system.Fail2banInstalled || system.Fail2ban);
  const [runningAction, setRunningAction] = useState("");
  const [actionError, setActionError] = useState("");
  const [uninstallOpen, setUninstallOpen] = useState(false);
  const [bansOpen, setBansOpen] = useState(false);
  const [bans, setBans] = useState({ loading: false, error: "", entries: [], page: 1, total: 0 });

  async function runSystemAction(action) {
    setRunningAction(action);
    setActionError("");
    try {
      const res = await fetch(`${prefix}/api/system/${action}`, { method: "POST" });
      const payload = await res.json();
      if (!payload.ok) {
        setActionError(payload.error || "操作失败");
        return false;
      }
      await refresh();
      return true;
    } catch (error) {
      setActionError(error.message || "操作失败");
      return false;
    } finally {
      setRunningAction("");
    }
  }

  async function loadBans(page = 1) {
    setBans((current) => ({ ...current, loading: true, error: "" }));
    try {
      const res = await fetch(`${prefix}/api/system/fail2ban/bans?page=${page}`, { cache: "no-store" });
      const payload = await res.json();
      if (!payload.ok) throw new Error(payload.error || "读取封禁记录失败");
      setBans({
        loading: false,
        error: "",
        entries: payload.bans || [],
        page: payload.page || page,
        total: payload.total || 0,
      });
    } catch (error) {
      setBans((current) => ({ ...current, loading: false, error: error.message || "读取封禁记录失败" }));
    }
  }

  function openBans() {
    setBansOpen(true);
    loadBans(1);
  }

  async function uninstallFail2ban() {
    const completed = await runSystemAction("fail2ban-uninstall");
    if (completed) setUninstallOpen(false);
  }

  const pageCount = Math.max(1, Math.ceil(bans.total / PAGE_SIZE));

  return (
    <>
      <PageHead meta={VIEW_META.toolbox} />
      <Card>
        <CardContent className="py-0">
          <div className="flex flex-wrap items-center justify-between gap-4 border-b py-4">
            <div>
              <div className="text-sm text-muted-foreground">BBR + TCP 优化</div>
              <div className="mt-1"><Badge active={system.BBR}>{system.BBR ? "已开启" : "未开启"}</Badge></div>
            </div>
            <Button variant="outline" onClick={() => runSystemAction("bbr")} disabled={system.BBR || Boolean(runningAction)}>
              {runningAction === "bbr" ? "开启中..." : "开启 BBR"}
            </Button>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 py-4">
            <div>
              <div className="text-sm text-muted-foreground">fail2ban（SSH 防爆破）</div>
              <div className="mt-1">
                <Badge active={system.Fail2ban}>
                  {system.Fail2ban ? "运行中" : fail2banInstalled ? "已安装，未运行" : "未安装"}
                </Badge>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              {system.Fail2ban && <Button variant="outline" onClick={openBans} disabled={Boolean(runningAction)}>查看封禁</Button>}
              {!system.Fail2ban && (
                <Button variant="outline" onClick={() => runSystemAction("fail2ban")} disabled={Boolean(runningAction)}>
                  {runningAction === "fail2ban" ? "处理中..." : fail2banInstalled ? "启用" : "安装并启用"}
                </Button>
              )}
              {fail2banInstalled && (
                <Button variant="destructive" onClick={() => setUninstallOpen(true)} disabled={Boolean(runningAction)}>
                  卸载
                </Button>
              )}
            </div>
          </div>
          {actionError && <div className="pb-4 text-sm text-destructive">操作失败：{actionError}</div>}
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

      <ModalFrame
        open={bansOpen}
        onCancel={() => setBansOpen(false)}
        eyebrow="Fail2ban"
        title="SSH 当前封禁"
        className="sm:max-w-3xl"
        footer={
          <>
            <Button variant="outline" onClick={() => setBansOpen(false)}>关闭</Button>
            <Button onClick={() => loadBans(bans.page)} disabled={bans.loading}>{bans.loading ? "刷新中..." : "刷新"}</Button>
          </>
        }
      >
        <p className="text-sm text-muted-foreground">仅展示 sshd jail 中仍处于封禁状态的 IP。</p>
        <div className="max-h-[55vh] overflow-auto rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>IP 地址</TableHead>
                <TableHead>Jail</TableHead>
                <TableHead>状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bans.loading && (
                <TableRow><TableCell colSpan="3" className="text-muted-foreground">正在读取封禁记录...</TableCell></TableRow>
              )}
              {!bans.loading && bans.error && (
                <TableRow><TableCell colSpan="3" className="text-destructive">读取失败：{bans.error}</TableCell></TableRow>
              )}
              {!bans.loading && !bans.error && bans.entries.length === 0 && (
                <TableRow><TableCell colSpan="3" className="text-muted-foreground">当前没有被封禁的 IP。</TableCell></TableRow>
              )}
              {!bans.loading && !bans.error && bans.entries.map((ban) => (
                <TableRow key={ban.ip}>
                  <TableCell className="font-mono">{ban.ip}</TableCell>
                  <TableCell>{ban.jail}</TableCell>
                  <TableCell>{ban.status}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span className="text-sm text-muted-foreground">共 {bans.total} 条，第 {bans.page} / {pageCount} 页</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => loadBans(bans.page - 1)} disabled={bans.loading || bans.page <= 1}>上一页</Button>
            <Button variant="outline" size="sm" onClick={() => loadBans(bans.page + 1)} disabled={bans.loading || bans.page >= pageCount}>下一页</Button>
          </div>
        </div>
      </ModalFrame>

      <ModalFrame
        open={uninstallOpen}
        onCancel={() => !runningAction && setUninstallOpen(false)}
        eyebrow="Fail2ban"
        title="卸载 fail2ban"
        footer={
          <>
            <Button variant="outline" onClick={() => setUninstallOpen(false)} disabled={Boolean(runningAction)}>取消</Button>
            <Button variant="destructive" onClick={uninstallFail2ban} disabled={Boolean(runningAction)}>
              {runningAction === "fail2ban-uninstall" ? "卸载中..." : "确认卸载"}
            </Button>
          </>
        }
      >
        <p className="text-sm leading-6 text-muted-foreground">这会停止并卸载 fail2ban。由面板创建的 SSH 防爆破配置会被移除，手工修改过的配置会保留。</p>
      </ModalFrame>
    </>
  );
}
