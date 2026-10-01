import { useState } from "react";
import { SMSCode } from "./SMSCode";
import { PasswordRecovery } from "./AccountLifecycle";
import { api } from "./api";
import { useUI } from "./store";
import { Button, Card, Field, Notice } from "./ui";
import { CookingPot, Leaf } from "lucide-react";
import { IngredientPhoto } from "./CatalogArt";
import { NatureBackdrop } from "./NatureTheme";

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
    <main className="foodflow-shell ff-auth">
      <NatureBackdrop />
      <div className="ff-auth-layout">
        <div className="ff-auth-story">
          <div className="mb-7 flex items-center gap-3">
            <span className="ff-brand-mark" aria-hidden="true">
              <CookingPot size={24} strokeWidth={1.6} />
            </span>
            <h1 className="text-2xl font-bold text-emerald-900">
              食光{" "}
              <span className="ml-1 text-sm font-normal text-slate-500">
                FoodFlow
              </span>
            </h1>
          </div>
          <p className="text-3xl font-bold leading-snug text-emerald-950 sm:text-4xl">
            好好吃饭，
            <br className="hidden sm:block" />
            少一点浪费。
          </p>
          <p className="mt-4 max-w-sm text-sm leading-7 text-slate-600">
            让家里的食材和每一餐有序流动。
            <br />
            从冰箱里的一份新鲜，到餐桌上的一顿用心。
          </p>
          <div className="ff-harvest-photos" aria-hidden="true">
            <div className="ff-harvest-circle">
              <IngredientPhoto
                name="西兰花"
                category="蔬菜"
                className="h-full w-full object-cover"
              />
            </div>
            <div className="ff-harvest-small">
              <IngredientPhoto
                name="番茄"
                category="蔬菜"
                className="h-full w-full object-cover"
              />
            </div>
            <span className="ff-harvest-caption">
              <Leaf size={14} /> 珍惜食材 · 按需备餐
            </span>
          </div>
          <a
            className="mt-5 hidden text-xs text-slate-500 underline decoration-slate-300 underline-offset-4 sm:inline-block"
            href="/ingredient-photos/credits.html"
            target="_blank"
            rel="noreferrer"
          >
            照片来源与授权
          </a>
        </div>
        <Card className="ff-auth-card">
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
                <p className="mb-2 text-xs font-medium tracking-widest text-emerald-700">
                  我的家庭厨房
                </p>
                <h2 className="mb-6 text-2xl font-bold text-emerald-950">
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
                    className="ff-auth-link mt-3 w-full text-sm text-emerald-700"
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
                  className="ff-auth-link mt-2 w-full text-sm text-emerald-700"
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
                    className="ff-auth-link mt-2 w-full text-sm text-slate-500"
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
