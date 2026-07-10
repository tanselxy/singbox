import { useEffect, useState } from "react";
import { ModalFrame, PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";
import { Input } from "../../ui/input.jsx";
import { Label } from "../../ui/label.jsx";

const UPDATE_CHECK_INTERVAL = 10 * 60 * 1000;
const UPDATE_RECOVERY_TIMEOUT = 2 * 60 * 1000;

export function System({ data, prefix }) {
  const [update, setUpdate] = useState({ phase: "checking", latest: "", upgradable: false });
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false);
  const [upgradePhase, setUpgradePhase] = useState("idle");
  const [upgradeError, setUpgradeError] = useState("");
  const [copied, setCopied] = useState(false);
  const [accountForm, setAccountForm] = useState({ username: data.Username || "admin", currentPassword: "", newPassword: "", confirmPassword: "" });
  const [accountSaving, setAccountSaving] = useState(false);
  const [accountError, setAccountError] = useState("");

  useEffect(() => {
    let active = true;

    async function checkUpdate() {
      if (active) setUpdate((current) => ({ ...current, phase: "checking" }));
      try {
        const res = await fetch(`${prefix}/api/version`, { cache: "no-store" });
        const payload = await res.json();
        if (!res.ok || !payload) throw new Error(payload?.error || "检测失败");
        if (active) {
          setUpdate({
            phase: "ready",
            latest: payload.latest || "",
            upgradable: Boolean(payload.upgradable),
          });
        }
      } catch {
        if (active) setUpdate({ phase: "unavailable", latest: "", upgradable: false });
      }
    }

    checkUpdate();
    const timer = window.setInterval(checkUpdate, UPDATE_CHECK_INTERVAL);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [prefix]);

  useEffect(() => {
    setAccountForm((current) => ({ ...current, username: data.Username || "admin" }));
  }, [data.Username]);

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

  async function saveAccount() {
    if (!accountForm.currentPassword) {
      setAccountError("请输入当前密码");
      return;
    }
    if (accountForm.newPassword !== accountForm.confirmPassword) {
      setAccountError("两次输入的新密码不一致");
      return;
    }
    setAccountSaving(true);
    setAccountError("");
    try {
      const body = new URLSearchParams({
        username: accountForm.username.trim(),
        current_password: accountForm.currentPassword,
        new_password: accountForm.newPassword,
        confirm_password: accountForm.confirmPassword,
      });
      const res = await fetch(`${prefix}/api/account`, { method: "POST", body });
      const payload = await res.json();
      if (!payload.ok) {
        setAccountError(payload.error || "保存失败");
        return;
      }
      window.location.assign(`${prefix}/login`);
    } catch (error) {
      setAccountError(error.message || "保存失败");
    } finally {
      setAccountSaving(false);
    }
  }

  function openUpdateDialog() {
    setUpgradePhase("idle");
    setUpgradeError("");
    setUpdateDialogOpen(true);
  }

  function closeUpdateDialog() {
    if (upgradePhase === "upgrading" || upgradePhase === "waiting") return;
    setUpdateDialogOpen(false);
  }

  function waitForUpgrade(target) {
    const deadline = Date.now() + UPDATE_RECOVERY_TIMEOUT;

    async function poll() {
      try {
        const res = await fetch(`${prefix}/api/version`, { cache: "no-store" });
        const payload = await res.json();
        if (res.ok && payload.current === target) {
          window.location.reload();
          return;
        }
      } catch {
        // The panel is expected to be briefly unavailable while it restarts.
      }

      if (Date.now() < deadline) {
        window.setTimeout(poll, 2000);
      } else {
        setUpgradePhase("failed");
        setUpgradeError("面板恢复超时，请稍后重新打开页面确认版本。");
      }
    }

    window.setTimeout(poll, 2500);
  }

  async function upgrade() {
    setUpgradePhase("upgrading");
    setUpgradeError("");
    try {
      const res = await fetch(`${prefix}/api/upgrade`, { method: "POST" });
      const payload = await res.json();
      if (!payload.ok) {
        setUpgradePhase("failed");
        setUpgradeError(payload.error || "升级失败");
        return;
      }
      setUpgradePhase("waiting");
      waitForUpgrade(update.latest);
    } catch {
      // The process can restart before the browser finishes receiving the body.
      setUpgradePhase("waiting");
      waitForUpgrade(update.latest);
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
              <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="font-mono">{data.Version}</span>
                <UpdateStatus update={update} />
              </div>
            </div>
            {update.upgradable && <Button onClick={openUpdateDialog}>更新到 {update.latest}</Button>}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>账户安全</CardTitle>
          </CardHeader>
          <CardContent className="max-w-xl space-y-4">
            <Label>用户名
              <Input value={accountForm.username} onChange={(event) => setAccountForm((current) => ({ ...current, username: event.target.value }))} autoComplete="username" />
            </Label>
            <Label>当前密码
              <Input type="password" value={accountForm.currentPassword} onChange={(event) => setAccountForm((current) => ({ ...current, currentPassword: event.target.value }))} autoComplete="current-password" />
            </Label>
            <div className="grid gap-4 sm:grid-cols-2">
              <Label>新密码
                <Input type="password" value={accountForm.newPassword} onChange={(event) => setAccountForm((current) => ({ ...current, newPassword: event.target.value }))} placeholder="留空则不修改" autoComplete="new-password" />
              </Label>
              <Label>确认新密码
                <Input type="password" value={accountForm.confirmPassword} onChange={(event) => setAccountForm((current) => ({ ...current, confirmPassword: event.target.value }))} autoComplete="new-password" />
              </Label>
            </div>
            <p className="text-sm text-muted-foreground">保存后会退出所有已登录会话，需要使用新的账户信息重新登录。</p>
            {accountError && <p className="text-sm text-destructive">{accountError}</p>}
            <Button onClick={saveAccount} disabled={accountSaving}>{accountSaving ? "保存中..." : "保存账户设置"}</Button>
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

      <ModalFrame
        open={updateDialogOpen}
        onCancel={closeUpdateDialog}
        eyebrow="Update"
        title={upgradePhase === "waiting" ? "正在更新面板" : `更新到 ${update.latest || "新版本"}`}
        footer={
          upgradePhase === "waiting" || upgradePhase === "upgrading" ? null : (
            <>
              <Button variant="outline" onClick={closeUpdateDialog}>取消</Button>
              <Button onClick={upgrade}>{upgradePhase === "failed" ? "重试更新" : "确认更新"}</Button>
            </>
          )
        }
      >
        {upgradePhase === "idle" && (
          <p className="text-sm leading-6 text-muted-foreground">升级会替换面板与 sing-box 二进制文件，并短暂重启服务。当前页面会在更新完成后自动刷新。</p>
        )}
        {upgradePhase === "upgrading" && <p className="text-sm text-muted-foreground">正在下载并替换程序文件，请勿关闭页面。</p>}
        {upgradePhase === "waiting" && <p className="text-sm text-muted-foreground">服务正在重启。检测到新版本恢复后，页面会自动刷新。</p>}
        {upgradePhase === "failed" && <p className="text-sm text-destructive">升级失败：{upgradeError}</p>}
      </ModalFrame>
    </>
  );
}

function UpdateStatus({ update }) {
  if (update.phase === "checking") return <span className="text-sm text-muted-foreground">正在自动检测更新...</span>;
  if (update.phase === "unavailable") return <span className="text-sm text-muted-foreground">暂时无法检测更新</span>;
  if (update.upgradable) return <span className="text-sm text-muted-foreground">发现新版本 {update.latest}</span>;
  return <span className="text-sm text-muted-foreground">已是最新{update.latest ? `（${update.latest}）` : ""}</span>;
}
