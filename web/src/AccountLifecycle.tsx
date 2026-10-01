import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { useUI } from "./store";
import { SMSCode } from "./SMSCode";
import { Button, Card, Field, Notice } from "./ui";

import type { components } from "./generated/api";
type Account = components["schemas"]["Account"];
function Confirm({
  checked,
  onChange,
  children,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  children: React.ReactNode;
}) {
  return (
    <label className="flex items-start gap-2 text-sm">
      <input
        type="checkbox"
        className="mt-1 shrink-0"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>{children}</span>
    </label>
  );
}
function useReauthenticate() {
  const q = useQueryClient(),
    setToken = useUI((s) => s.setToken),
    setHousehold = useUI((s) => s.setHousehold),
    setPage = useUI((s) => s.setPage);
  return () => {
    q.clear();
    setHousehold("");
    setPage("today");
    setToken("");
  };
}

export function PasswordRecovery({
  onBack,
  onSaved,
}: {
  onBack: () => void;
  onSaved: () => void;
}) {
  const [phone, setPhone] = useState(""),
    [password, setPassword] = useState(""),
    [code, setCode] = useState(""),
    [challenge, setChallenge] = useState(""),
    [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/auth/password/reset", "", "POST", {
        phone,
        password,
        code,
        challenge_id: challenge,
        confirm,
      });
      onSaved();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <h2 className="mb-5 text-2xl font-bold">找回密码</h2>
      <form className="space-y-4" onSubmit={submit}>
        <p className="text-sm text-slate-600">
          使用账号已验证的手机号找回。仅有邮箱的账号暂不支持自助找回。
        </p>
        <Field
          label="已验证手机号"
          type="tel"
          required
          value={phone}
          onChange={(e) => {
            setPhone(e.target.value);
            setCode("");
            setChallenge("");
          }}
        />
        <Field
          label="新密码"
          type="password"
          minLength={8}
          autoComplete="new-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <SMSCode
          key={phone}
          phone={phone}
          purpose="reset"
          code={code}
          onCode={setCode}
          onChallenge={setChallenge}
        />
        <Confirm checked={confirm} onChange={setConfirm}>
          确认重置密码，所有设备需重新登录。
        </Confirm>
        {error && <Notice tone="error">{error}</Notice>}
        <Button className="w-full" disabled={busy || !challenge || !confirm}>
          {busy ? "重置中…" : "确认重置密码"}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>
          返回登录
        </Button>
      </form>
    </>
  );
}

export function PhoneBinding({ token }: { token: string }) {
  const q = useQueryClient(),
    logout = useReauthenticate();
  const me = useQuery({
    queryKey: ["account-phone"],
    queryFn: () => api<Account>("/me", token),
  });
  const [replace, setReplace] = useState(false),
    [phone, setPhone] = useState(""),
    [password, setPassword] = useState(""),
    [code, setCode] = useState(""),
    [challenge, setChallenge] = useState(""),
    [oldCode, setOldCode] = useState(""),
    [oldChallenge, setOldChallenge] = useState(""),
    [confirm, setConfirm] = useState(false),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [saved, setSaved] = useState(false);
  const replacing = Boolean(me.data?.phone_verified && replace);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/me/phone", token, "POST", {
        phone,
        password,
        code,
        challenge_id: challenge,
        ...(replacing
          ? {
              old_code: oldCode,
              old_challenge_id: oldChallenge,
              confirm_replace: confirm,
            }
          : {}),
      });
      if (replacing) {
        logout();
        return;
      }
      setSaved(true);
      setPassword("");
      setCode("");
      setChallenge("");
      await q.invalidateQueries();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="mt-4">
      <h2 className="font-semibold">手机号绑定</h2>
      {me.isLoading ? (
        <p>加载中…</p>
      ) : me.error ? (
        <Notice tone="error">
          账号读取失败 <Button onClick={() => me.refetch()}>重试</Button>
        </Notice>
      ) : me.data?.phone_verified && !replace ? (
        <div className="mt-3 space-y-3">
          <p className="text-sm">
            已验证绑定 {me.data.phone}，可以使用手机号或已绑定邮箱及原密码登录。
          </p>
          <Button variant="outline" onClick={() => setReplace(true)}>
            更换手机号
          </Button>
        </div>
      ) : (
        <form className="mt-3 space-y-3" onSubmit={submit}>
          <p className="text-sm text-slate-600">
            {replacing
              ? "更换需要验证原号码和新号码。完成后所有设备重新登录，密码不变。"
              : "验证手机号后添加到当前账号，不会合并其他账号。"}
          </p>
          <Field
            label={
              replacing ? "新手机号（中国大陆 +86）" : "手机号（中国大陆 +86）"
            }
            type="tel"
            required
            value={phone}
            onChange={(e) => {
              setPhone(e.target.value);
              setCode("");
              setChallenge("");
            }}
          />
          <Field
            label="当前账号密码"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {replacing && (
            <>
              <p className="text-sm">原号码：{me.data?.phone}</p>
              <SMSCode
                phone={me.data?.phone || ""}
                purpose="change_old"
                token={token}
                label="原号码验证码"
                code={oldCode}
                onCode={setOldCode}
                onChallenge={setOldChallenge}
              />
            </>
          )}
          <SMSCode
            key={phone}
            phone={phone}
            purpose="bind"
            token={token}
            label={replacing ? "新号码验证码" : "短信验证码"}
            onChallenge={setChallenge}
            code={code}
            onCode={setCode}
          />
          {replacing && (
            <Confirm checked={confirm} onChange={setConfirm}>
              确认更换手机号并退出所有设备。
            </Confirm>
          )}
          <Button
            disabled={
              busy ||
              !challenge ||
              (replacing &&
                (!oldChallenge || !confirm || phone === me.data?.phone))
            }
          >
            {busy ? "验证中…" : replacing ? "确认更换手机号" : "验证并绑定"}
          </Button>
          {replacing && (
            <Button
              type="button"
              variant="ghost"
              onClick={() => setReplace(false)}
            >
              取消更换
            </Button>
          )}
        </form>
      )}
      {saved && <Notice tone="success">绑定成功，密码保持不变。</Notice>}
      {error && <Notice tone="error">{error}</Notice>}
    </Card>
  );
}

export function AccountMerge({ token }: { token: string }) {
  const me = useQuery({
      queryKey: ["account-phone"],
      queryFn: () => api<Account>("/me", token),
    }),
    logout = useReauthenticate();
  const [open, setOpen] = useState(false),
    [account, setAccount] = useState(""),
    [password, setPassword] = useState(""),
    [sourcePassword, setSourcePassword] = useState(""),
    [code, setCode] = useState(""),
    [challenge, setChallenge] = useState(""),
    [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const phoneOnly = Boolean(me.data?.phone_verified && !me.data.email),
    emailOnly = Boolean(me.data?.email && !me.data.phone),
    phone = phoneOnly ? me.data?.phone || "" : account;
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/me/merge", token, "POST", {
        source_account: account,
        password,
        source_password: sourcePassword,
        code,
        challenge_id: challenge,
        confirm,
      });
      logout();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="mt-4">
      <h2 className="font-semibold">合并已有账号</h2>
      <p className="mt-3 text-sm text-slate-600">
        可将一个邮箱账号与一个已验证手机号账号合并。保留当前账号的称呼和密码，汇总两边的家庭权限，保留库存与历史记录。来源账号正在生成的任务会取消。
      </p>
      {me.error ? (
        <Notice tone="error">
          账号读取失败 <Button onClick={() => me.refetch()}>重试</Button>
        </Notice>
      ) : me.isLoading ? (
        <p>加载中…</p>
      ) : !phoneOnly && !emailOnly ? (
        <p className="mt-3 text-sm">
          当前账号已有两种登录方式或尚未验证手机号，暂不支持这类合并。
        </p>
      ) : !open ? (
        <Button
          className="mt-3"
          variant="outline"
          onClick={() => setOpen(true)}
        >
          开始合并账号
        </Button>
      ) : (
        <form className="mt-3 space-y-3" onSubmit={submit}>
          <Field
            label={phoneOnly ? "来源邮箱账号" : "来源已验证手机号账号"}
            type={phoneOnly ? "email" : "tel"}
            required
            value={account}
            onChange={(e) => {
              setAccount(e.target.value);
              setChallenge("");
              setCode("");
            }}
          />
          <Field
            label="当前账号密码"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <Field
            label="来源账号密码"
            type="password"
            autoComplete="off"
            required
            value={sourcePassword}
            onChange={(e) => setSourcePassword(e.target.value)}
          />
          <SMSCode
            key={phone}
            phone={phone}
            purpose="merge"
            token={token}
            label="合并手机号验证码"
            code={code}
            onCode={setCode}
            onChallenge={setChallenge}
          />
          <Confirm checked={confirm} onChange={setConfirm}>
            确认合并并退出两个账号的所有设备，重新使用当前账号密码登录。
          </Confirm>
          {error && <Notice tone="error">{error}</Notice>}
          <Button disabled={busy || !challenge || !confirm}>
            {busy ? "合并中…" : "确认合并账号"}
          </Button>
          <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
            取消
          </Button>
        </form>
      )}
    </Card>
  );
}
