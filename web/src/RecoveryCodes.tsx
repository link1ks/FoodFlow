import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { Button, Card, Field, Notice } from "./ui";
import type { paths } from "./generated/api";
type Status =
  paths["/me/recovery-codes"]["get"]["responses"][200]["content"]["application/json"];
type Issued =
  paths["/me/recovery-codes"]["post"]["responses"][200]["content"]["application/json"];
export function RecoveryCodes({ token }: { token: string }) {
  const status = useQuery({
    queryKey: ["recovery-codes", token],
    queryFn: () => api<Status>("/me/recovery-codes", token),
  });
  const [password, setPassword] = useState(""),
    [confirm, setConfirm] = useState(false),
    [issued, setIssued] = useState<Issued | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function generate(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setIssued(null);
    try {
      const result = await api<Issued>("/me/recovery-codes", token, "POST", {
        password,
        confirm,
      });
      setIssued(result);
      setPassword("");
      setConfirm(false);
      await status.refetch();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="mt-4">
      <h2 className="font-semibold">账号恢复码</h2>
      <p className="mt-2 text-sm text-slate-600">
        忘记密码时，用账号和预先保存的恢复码找回，无需短信或付费邮件。恢复码仅显示一次，请保存在可信的密码管理器或纸上。
      </p>
      {status.data && (
        <p className="mt-2 text-sm">
          有效恢复码：{status.data.remaining} 个
          {status.data.expires_at
            ? `，有效至 ${new Date(status.data.expires_at).toLocaleDateString()}`
            : "，请先生成并保存"}
        </p>
      )}
      <form className="mt-4 space-y-3" onSubmit={generate}>
        <Field
          label="生成恢复码的当前密码"
          type="password"
          autoComplete="current-password"
          required
          minLength={8}
          maxLength={72}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <label className="flex items-start gap-2 text-sm">
          <input
            type="checkbox"
            className="mt-1"
            checked={confirm}
            onChange={(e) => setConfirm(e.target.checked)}
          />
          确认重新生成，旧恢复码全部失效。
        </label>
        <Button disabled={busy || !confirm}>
          {busy ? "生成中…" : "生成新的恢复码"}
        </Button>
      </form>
      {issued && (
        <div className="mt-4 space-y-3" data-testid="issued-recovery-codes">
          <Notice>
            请立即保存下面五个恢复码。重置密码后，所有设备和剩余恢复码都会失效，需要重新生成。
          </Notice>
          <ul className="space-y-2 text-sm">
            {issued.codes.map((code) => (
              <li key={code}>
                <code className="break-all select-all rounded-lg bg-slate-50 p-1 font-mono">
                  {code}
                </code>
              </li>
            ))}
          </ul>
          <Button
            type="button"
            variant="outline"
            onClick={() => setIssued(null)}
          >
            我已妥善保存，隐藏恢复码
          </Button>
        </div>
      )}
      {(error || status.error) && (
        <Notice tone="error">{error || String(status.error)}</Notice>
      )}
    </Card>
  );
}
export function CodePasswordRecovery({
  onBack,
  onSaved,
}: {
  onBack: () => void;
  onSaved: () => void;
}) {
  const [account, setAccount] = useState(""),
    [code, setCode] = useState(""),
    [password, setPassword] = useState(""),
    [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/auth/recovery/reset", "", "POST", {
        account,
        code,
        password,
        confirm,
      });
      setCode("");
      setPassword("");
      onSaved();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="space-y-4" onSubmit={submit}>
      <p className="text-sm text-slate-600">
        使用个人主页中预先生成并保存的恢复码。没有恢复码且未绑定可用手机号时，无法在此自助重置。
      </p>
      <Field
        label="找回账号的邮箱或手机号"
        autoComplete="username"
        required
        maxLength={254}
        value={account}
        onChange={(e) => setAccount(e.target.value)}
      />
      <Field
        label="已保存的恢复码"
        autoComplete="off"
        required
        maxLength={40}
        value={code}
        onChange={(e) => setCode(e.target.value)}
      />
      <Field
        label="新密码"
        type="password"
        autoComplete="new-password"
        required
        minLength={8}
        maxLength={72}
        value={password}
        onChange={(e) => setPassword(e.target.value)}
      />
      <label className="flex items-start gap-2 text-sm">
        <input
          type="checkbox"
          className="mt-1"
          checked={confirm}
          onChange={(e) => setConfirm(e.target.checked)}
        />
        确认重置密码，所有设备和剩余恢复码将失效。
      </label>
      {error && <Notice tone="error">{error}</Notice>}
      <Button className="w-full" disabled={busy || !confirm || !code.trim()}>
        {busy ? "重置中…" : "确认用恢复码重置"}
      </Button>
      <Button type="button" variant="ghost" onClick={onBack}>
        返回登录
      </Button>
    </form>
  );
}
