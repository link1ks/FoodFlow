import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { Button, Field, Notice } from "./ui";

export function SMSCode({
  phone,
  purpose,
  token = "",
  label = "短信验证码",
  onChallenge,
  code,
  onCode,
}: {
  phone: string;
  purpose: "login" | "register" | "bind" | "reset" | "change_old" | "merge";
  token?: string;
  label?: string;
  onChallenge: (id: string) => void;
  code: string;
  onCode: (v: string) => void;
}) {
  const capability = useQuery({
    queryKey: ["sms-capabilities"],
    queryFn: () => api<{ sms_enabled: boolean }>("/auth/capabilities", ""),
  });
  const [until, setUntil] = useState(0),
    [now, setNow] = useState(Date.now()),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const seconds = Math.max(0, Math.ceil((until - now) / 1000));
  useEffect(() => {
    if (!until) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [until]);
  async function send() {
    setBusy(true);
    setError("");
    try {
      const authenticated = ["bind", "change_old", "merge"].includes(purpose);
      const r = await api<{ challenge_id: string; retry_after: number }>(
        authenticated ? "/me/phone/code" : "/auth/sms/code",
        token,
        "POST",
        { phone, purpose },
      );
      onChallenge(r.challenge_id);
      setUntil(Date.now() + r.retry_after * 1000);
      setNow(Date.now());
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="space-y-2">
      <div className="flex items-end gap-2">
        <div className="min-w-0 flex-1">
          <Field
            label={label}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            value={code}
            onChange={(e) => onCode(e.target.value)}
            required
          />
        </div>
        <Button
          type="button"
          variant="outline"
          disabled={
            busy ||
            seconds > 0 ||
            !capability.data?.sms_enabled ||
            !phone.trim()
          }
          onClick={send}
        >
          {busy ? "发送中…" : seconds ? seconds + "秒后重发" : "获取验证码"}
        </Button>
      </div>
      {capability.isLoading && (
        <p className="text-xs text-slate-500">读取短信服务状态…</p>
      )}
      {capability.data && !capability.data.sms_enabled && (
        <Notice>
          {purpose === "register" || purpose === "login"
            ? "短信服务尚未配置。已有账号可使用密码登录；新用户可使用邮箱注册。"
            : "短信服务尚未配置，此操作暂不可用。请保留现有登录方式。"}
        </Notice>
      )}
      {capability.error && (
        <Notice tone="error">
          短信状态读取失败{" "}
          <button type="button" onClick={() => capability.refetch()}>
            重试
          </button>
        </Notice>
      )}
      {error && <Notice tone="error">{error}</Notice>}
    </div>
  );
}
