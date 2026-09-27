import { useEffect, useState } from "react";
import { MemberRoleControl } from "../MemberRoleControl";
import { api } from "../api";
import { Title, qc, useBase, useData, type Household } from "../app/shared";
import { useUI } from "../store";
import { Button, Card, Field, Notice } from "../ui";
export function Settings() {
  const { token, house, p, q } = useBase();
  const d = useData<{
    name: string;
    servings: number;
    preferences: string;
    excluded_ingredients: string[];
    role: string;
  }>("household", p);
  const members = useData<
    { id: string; name: string; email: string; role: string }[]
  >("members", p + "/members");
  const houses = useData<Household[]>("households", "/households");
  const [name, setName] = useState(""),
    [servings, setServings] = useState("2"),
    [preferences, setPreferences] = useState(""),
    [email, setEmail] = useState(""),
    [role, setRole] = useState("editor"),
    [code, setCode] = useState(""),
    [err, setErr] = useState("");
  useEffect(() => {
    if (d.data) {
      setName(d.data.name);
      setServings(String(d.data.servings));
      setPreferences(d.data.preferences);
    }
  }, [d.data]);
  async function save() {
    try {
      await api(p, token, "PATCH", {
        name,
        servings: Number(servings),
        preferences,
        excluded_ingredients: d.data?.excluded_ingredients || [],
      });
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  async function invite() {
    try {
      const v = await api<{ code: string }>(p + "/invites", token, "POST", {
        email,
        role,
      });
      setCode(v.code);
    } catch (e) {
      setErr(String(e));
    }
  }
  return (
    <>
      <Title title="家庭设置" subtitle="维护家庭偏好、成员角色和邀请" />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <h2 className="mb-3 font-semibold">当前家庭</h2>
          <label className="text-sm">
            切换家庭
            <select
              className="mt-1 w-full rounded-xl border p-2"
              value={house}
              onChange={(e) => useUI.getState().setHousehold(e.target.value)}
            >
              {houses.data?.map((h) => (
                <option key={h.id} value={h.id}>
                  {h.name}
                </option>
              ))}
            </select>
          </label>
          <div className="mt-3 space-y-3">
            <Field
              label="名称"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <Field
              label="默认用餐人数"
              type="number"
              min="1"
              max="20"
              value={servings}
              onChange={(e) => setServings(e.target.value)}
            />
            <Field
              label="饮食偏好与忌口"
              value={preferences}
              onChange={(e) => setPreferences(e.target.value)}
            />
            {d.data?.role === "owner" && (
              <Button onClick={save}>保存设置</Button>
            )}
          </div>
        </Card>
        <Card>
          <h2 className="mb-3 font-semibold">成员</h2>
          {members.data?.map((m) => (
            <div
              key={m.id}
              className="flex justify-between border-b py-2 text-sm"
            >
              <span>
                {m.name} <span className="text-slate-500">{m.email}</span>
              </span>
              <MemberRoleControl
                member={m}
                canEdit={d.data?.role === "owner"}
                token={token}
                household={house}
              />
            </div>
          ))}
          {d.data?.role === "owner" && (
            <div className="mt-4 space-y-3">
              <Field
                label="邀请邮箱"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <select
                className="w-full rounded-xl border p-2"
                value={role}
                onChange={(e) => setRole(e.target.value)}
              >
                <option value="editor">可编辑</option>
                <option value="viewer">只读</option>
              </select>
              <Button onClick={invite}>生成邀请码</Button>
              {code && (
                <Notice>
                  请将邀请码安全发给 {email}：
                  <b className="break-all">{code}</b>
                </Notice>
              )}
            </div>
          )}
        </Card>
      </div>
      {err && (
        <div className="mt-3">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
    </>
  );
}

export function ExclusionSettings() {
  const { token, p, q } = useBase();
  const d = useData<{
    name: string;
    servings: number;
    preferences: string;
    excluded_ingredients: string[];
    role: string;
  }>("household", p);
  const [value, setValue] = useState(""),
    [err, setErr] = useState(""),
    [saved, setSaved] = useState(false);
  useEffect(() => {
    setValue(d.data?.excluded_ingredients?.join("、") || "");
  }, [d.data]);
  async function save() {
    if (!d.data) return;
    setErr("");
    setSaved(false);
    try {
      await api(p, token, "PATCH", {
        name: d.data.name,
        servings: d.data.servings,
        preferences: d.data.preferences,
        excluded_ingredients: value
          .split(/[，,、]/)
          .map((x) => x.trim())
          .filter(Boolean),
      });
      setSaved(true);
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  return (
    <Card className="mt-4">
      <h2 className="mb-3 font-semibold">明确忌口食材</h2>
      <p className="mb-3 text-sm text-slate-500">
        按菜谱食材名称匹配，规划和菜单确认时强制排除。多个名称用逗号分隔。
      </p>
      <Field
        label="例如：鸡蛋、花生"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        disabled={d.data?.role !== "owner"}
      />
      {d.data?.role === "owner" && (
        <Button className="mt-3" onClick={save}>
          保存忌口
        </Button>
      )}
      {saved && (
        <div className="mt-2">
          <Notice tone="success">已保存</Notice>
        </div>
      )}
      {err && (
        <div className="mt-2">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
    </Card>
  );
}

export function ProfileSettings() {
  const { token } = useBase();
  const d = useData<{ name: string; timezone: string; remind_days: number }>(
    "me",
    "/me",
  );
  const [name, setName] = useState(""),
    [timezone, setTimezone] = useState("Asia/Shanghai"),
    [days, setDays] = useState("3"),
    [err, setErr] = useState(""),
    [saved, setSaved] = useState(false);
  useEffect(() => {
    if (d.data) {
      setName(d.data.name);
      setTimezone(d.data.timezone);
      setDays(String(d.data.remind_days));
    }
  }, [d.data]);
  async function save() {
    setErr("");
    try {
      await api("/me", token, "PATCH", {
        name,
        timezone,
        remind_days: Number(days),
      });
      await qc.invalidateQueries({ queryKey: ["me", "/me"] });
      setSaved(true);
    } catch (e) {
      setErr(String(e));
    }
  }
  return (
    <Card className="mt-4">
      <h2 className="mb-3 font-semibold">个人提醒偏好</h2>
      <div className="grid gap-3 sm:grid-cols-3">
        <Field
          label="称呼"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Field
          label="时区（IANA）"
          value={timezone}
          onChange={(e) => setTimezone(e.target.value)}
        />
        <Field
          label="提前提醒天数"
          type="number"
          min="0"
          max="30"
          value={days}
          onChange={(e) => setDays(e.target.value)}
        />
      </div>
      <Button className="mt-3" onClick={save}>
        保存提醒偏好
      </Button>
      {saved && (
        <div className="mt-2">
          <Notice tone="success">已保存</Notice>
        </div>
      )}
      {err && (
        <div className="mt-2">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
    </Card>
  );
}
