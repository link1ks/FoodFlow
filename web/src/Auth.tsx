import { useState } from "react";
import { SMSCode } from "./SMSCode";
import { PasswordRecovery } from "./AccountLifecycle";
import { api } from "./api";
import { useUI } from "./store";
import { Button, Card, Field, Notice } from "./ui";

export function Auth() {
  const setToken = useUI((s) => s.setToken);
  const [register, setRegister] = useState(false),
    [sms, setSMS] = useState(false),
    [account, setAccount] = useState(""),
    [password, setPassword] = useState(""),
    [name, setName] = useState(""),
    [code, setCode] = useState(""),
    [challenge, setChallenge] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [recover, setRecover] = useState(false),
    [resetSaved, setResetSaved] = useState(false);
  const needCode = sms || (register && !account.includes("@"));
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const r = await api<{ token: string }>(
        register ? "/register" : sms ? "/auth/sms/login" : "/login",
        "",
        "POST",
        sms
          ? { phone: account, code, challenge_id: challenge }
          : {
              account,
              password,
              ...(register ? { name, code, challenge_id: challenge } : {}),
            },
      );
      setToken(r.token);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="min-h-screen bg-gradient-to-br from-emerald-50 via-white to-amber-50 px-4 py-16">
      <div className="mx-auto max-w-md">
        <div className="mb-8 text-center">
          <div className="text-5xl">🍲</div>
          <h1 className="mt-3 text-4xl font-bold text-emerald-900">
            FoodFlow 食光
          </h1>
          <p className="mt-2 text-slate-600">让家里的食材和每一餐有序流动</p>
        </div>
        <Card className="p-6">
          <>
            {recover ? (
              <PasswordRecovery
                onBack={() => setRecover(false)}
                onSaved={() => {
                  setRecover(false);
                  setResetSaved(true);
                  setSMS(false);
                  setRegister(false);
                }}
              />
            ) : (
              <>
                {resetSaved && (
                  <Notice tone="success">密码已重置，请使用新密码登录。</Notice>
                )}
                <h2 className="mb-5 text-2xl font-bold">
                  {register ? "创建账号" : "欢迎回来"}
                </h2>
                <form onSubmit={submit} className="space-y-4">
                  {register && (
                    <Field
                      label="称呼"
                      required
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  )}
                  <Field
                    label={sms ? "手机号" : "手机号或邮箱"}
                    type={sms ? "tel" : "text"}
                    autoComplete="username"
                    required
                    maxLength={254}
                    value={account}
                    onChange={(e) => {
                      setAccount(e.target.value);
                      setChallenge("");
                      setCode("");
                    }}
                  />
                  {!sms && (
                    <Field
                      label="密码"
                      type="password"
                      autoComplete={
                        register ? "new-password" : "current-password"
                      }
                      minLength={8}
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                    />
                  )}
                  <p className="text-xs text-slate-500">
                    绑定到同一账号的手机号与邮箱，共用同一个密码。
                  </p>
                  {needCode && (
                    <SMSCode
                      key={(sms ? "login" : "register") + account}
                      phone={account}
                      purpose={sms ? "login" : "register"}
                      onChallenge={setChallenge}
                      code={code}
                      onCode={setCode}
                    />
                  )}
                  {error && <Notice tone="error">{error}</Notice>}
                  <Button
                    className="w-full"
                    disabled={busy || (needCode && !challenge)}
                  >
                    {busy ? "请稍候…" : register ? "注册" : "登录"}
                  </Button>
                </form>
                {!register && (
                  <button
                    className="mt-4 w-full text-sm text-emerald-700"
                    onClick={() => {
                      setSMS(!sms);
                      setChallenge("");
                      setCode("");
                      setError("");
                    }}
                  >
                    {sms ? "使用密码登录" : "使用短信验证码登录"}
                  </button>
                )}
                <button
                  className="mt-4 w-full text-sm text-emerald-700"
                  onClick={() => {
                    setRegister(!register);
                    setSMS(false);
                    setChallenge("");
                    setCode("");
                    setError("");
                  }}
                >
                  {register ? "已有账号？登录" : "还没有账号？注册"}
                </button>
                {!register && (
                  <button
                    className="mt-4 w-full text-sm text-emerald-700"
                    onClick={() => {
                      setRecover(true);
                      setError("");
                    }}
                  >
                    忘记密码
                  </button>
                )}
              </>
            )}
          </>
        </Card>
      </div>
    </main>
  );
}
