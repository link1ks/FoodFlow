import { useState } from "react";
import { ChefHat, LockKeyhole, Users, ArrowRight } from "lucide-react";
import { api } from "../api";
import { useBase, useData, type Household, ErrorLine } from "../app/shared";
import { useUI } from "../store";
import { Button, Card, Field } from "../ui";

export function KitchenSpaces({ current }: { current: Household }) {
  const { token, q } = useBase();
  const spaces = useData<Household[]>("households", "/households");
  const [name, setName] = useState(""),
    [servings, setServings] = useState("2"),
    [code, setCode] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState<unknown>();
  function choose(id: string) {
    useUI.getState().setHousehold(id);
    useUI.getState().setPage("today");
  }
  async function submit(kind: "create" | "join") {
    setBusy(true);
    setError(undefined);
    try {
      const result =
        kind === "create"
          ? await api<{ id: string }>("/households", token, "POST", {
              name: name.trim(),
              servings: Number(servings),
              preferences: "",
            })
          : await api<{ household_id: string }>(
              "/invites/accept",
              token,
              "POST",
              { code: code.trim() },
            );
      await q.invalidateQueries({ queryKey: ["households"] });
      choose("id" in result ? result.id : result.household_id);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  const families = spaces.data?.filter((h) => h.kind === "family") || [];
  return (
    <section className="mb-5 space-y-4" aria-label="个人厨房与家庭共享">
      <Card className="ff-personal-space">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="ff-eyebrow">
              <LockKeyhole size={14} /> JUST FOR YOU · 个人空间
            </p>
            <h2 className="mt-2 flex items-center gap-2 text-xl font-bold">
              <ChefHat size={22} /> 我的个人厨房
            </h2>
            <p className="mt-3 text-sm leading-6 text-slate-600">
              一个人也能好好吃饭。食材、菜单和记录仅自己可见。
            </p>
          </div>
          {current.kind === "personal" ? (
            <span className="rounded-full bg-emerald-800 px-4 py-2 text-xs font-semibold text-white">
              当前使用
            </span>
          ) : (
            <Button
              onClick={() => {
                const personal = q.getQueryData<Household>([
                  "personal-kitchen",
                ]);
                if (personal) choose(personal.id);
              }}
            >
              使用个人厨房 <ArrowRight size={16} />
            </Button>
          )}
        </div>
        {(spaces.data?.filter((h) => h.kind === "personal").length || 0) >
          1 && (
          <div className="mt-4 flex flex-wrap gap-2">
            {spaces.data
              ?.filter((h) => h.kind === "personal")
              .map((h) => (
                <Button
                  key={h.id}
                  variant="outline"
                  onClick={() => choose(h.id)}
                >
                  {h.name} · 个人
                </Button>
              ))}
          </div>
        )}
      </Card>
      <Card>
        <p className="ff-eyebrow">
          <Users size={15} /> TOGETHER, IF YOU LIKE · 可选共享
        </p>
        <h2 className="mt-2 text-xl font-bold">家庭共享</h2>
        <p className="mt-3 text-sm leading-6 text-slate-600">
          想和家人一起管理冰箱时，再创建或加入家庭。个人厨房与家庭的数据分别保存。
        </p>
        {spaces.error && (
          <div className="mt-3">
            <ErrorLine error={spaces.error} />
            <Button
              variant="outline"
              size="sm"
              onClick={() => spaces.refetch()}
            >
              重试加载
            </Button>
          </div>
        )}
        {families.length > 0 && (
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            {families.map((h) => (
              <button
                key={h.id}
                className="ff-family-choice"
                aria-label={`使用家庭：${h.name}`}
                aria-pressed={current.id === h.id}
                onClick={() => choose(h.id)}
              >
                <span className="font-semibold break-all">{h.name}</span>
                <span className="mt-1 block text-xs text-slate-500">
                  {h.servings} 人 ·{" "}
                  {h.role === "owner"
                    ? "家庭管理员"
                    : h.role === "editor"
                      ? "可编辑"
                      : "只读"}
                  {current.id === h.id ? " · 当前使用" : ""}
                </span>
              </button>
            ))}
          </div>
        )}
        {!families.length && !spaces.isPending && (
          <p className="mt-4 text-sm text-slate-500">
            暂未加入家庭，个人厨房的所有功能已经可以使用。
          </p>
        )}
        <details className="ff-family-setup mt-5">
          <summary>创建或加入家庭</summary>
          <div className="mt-4 grid gap-5 sm:grid-cols-2">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                submit("create");
              }}
              className="space-y-3"
            >
              <h3 className="font-semibold">创建共享家庭</h3>
              <Field
                label="家庭名称"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                maxLength={80}
              />
              <Field
                label="家庭默认用餐人数"
                type="number"
                min={1}
                max={20}
                value={servings}
                onChange={(e) => setServings(e.target.value)}
                required
              />
              <Button disabled={busy || !name.trim()} type="submit">
                创建家庭
              </Button>
            </form>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                submit("join");
              }}
              className="space-y-3"
            >
              <h3 className="font-semibold">接受家人的邀请</h3>
              <Field
                label="家庭邀请码"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
              />
              <p className="text-xs leading-6 text-slate-500">
                邀请码需由家人生成，并发送给你当前账号绑定的邮箱。
              </p>
              <Button
                disabled={busy || !code.trim()}
                variant="outline"
                type="submit"
              >
                加入家庭
              </Button>
            </form>
          </div>
        </details>
        {error !== undefined && (
          <div className="mt-4">
            <ErrorLine error={error} />
          </div>
        )}
      </Card>
    </section>
  );
}
