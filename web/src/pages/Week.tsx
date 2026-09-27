import { Sparkles } from "lucide-react";
import { useState } from "react";
import { PageSwitch } from "../GettingStarted";
import { MultiMealPlanner } from "../MultiMealPlanner";
import { Quantity } from "../Quantity";
import { api } from "../api";
import {
  ErrorLine,
  Load,
  Title,
  useBase,
  useData,
  type Meal,
  type Plan,
  type Recipe,
} from "../app/shared";
import { useUI } from "../store";
import { Button, Card, Field, Notice } from "../ui";
export function Week() {
  const [mode, setMode] = useState("single");
  const { token, p, q } = useBase();
  const plans = useData<{ id: string; status: string; revision: number }[]>(
    "plans",
    p + "/plans",
  );
  const recipes = useData<Recipe[]>("recipes", "/recipes");
  const [day, setDay] = useState(new Date().toLocaleDateString("en-CA")),
    [meal, setMeal] = useState("dinner"),
    [servings, setServings] = useState("2"),
    [max, setMax] = useState("30"),
    [preference, setPreference] = useState(""),
    [excluded, setExcluded] = useState(""),
    [recipe, setRecipe] = useState(""),
    [err, setErr] = useState("");
  async function generate() {
    setErr("");
    try {
      await api(p + "/jobs/plan", token, "POST", {
        day,
        meal,
        servings: Number(servings),
        max_minutes: Number(max),
        preference,
        excluded_ingredients: excluded
          .split(/[，,、]/)
          .map((x) => x.trim())
          .filter(Boolean),
      });
      q.invalidateQueries();
      useUI.getState().setPage("jobs");
    } catch (e) {
      setErr(String(e));
    }
  }
  async function manual() {
    setErr("");
    try {
      await api(p + "/plans", token, "POST", {
        meals: [{ day, meal, servings: Number(servings), recipe_id: recipe }],
      });
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  return (
    <>
      <Title
        title="安排菜单"
        subtitle="告诉我们几个人吃、能做多久；确认喜欢的方案后，缺少的食材会加入采购清单"
      />
      <PageSwitch
        value={mode}
        onChange={setMode}
        options={[
          {
            id: "single",
            title: "安排一顿饭",
            description: "AI 帮我选菜，或自己挑一道菜",
          },
          {
            id: "multi",
            title: "连续几餐 · 清冰箱",
            description: "优先用掉临期食材，安排接下来几餐",
          },
        ]}
      />
      {mode === "multi" ? (
        <MultiMealPlanner token={token} root={p} />
      ) : (
        <Card className="mb-5">
          <h2 className="mb-4 font-semibold">这顿饭，怎么安排？</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Field
              label="日期"
              type="date"
              value={day}
              onChange={(e) => setDay(e.target.value)}
            />
            <label className="text-sm">
              餐次
              <select
                className="mt-1 w-full rounded-xl border p-2.5"
                value={meal}
                onChange={(e) => setMeal(e.target.value)}
              >
                <option value="breakfast">早餐</option>
                <option value="lunch">午餐</option>
                <option value="dinner">晚餐</option>
              </select>
            </label>
            <Field
              label="人数"
              type="number"
              min="1"
              max="20"
              value={servings}
              onChange={(e) => setServings(e.target.value)}
            />
            <Field
              label="最长烹饪时间（分钟）"
              type="number"
              min="1"
              max="240"
              value={max}
              onChange={(e) => setMax(e.target.value)}
            />
          </div>
          <div className="mt-3">
            <Field
              label="其他偏好"
              value={preference}
              onChange={(e) => setPreference(e.target.value)}
            />
            <div className="mt-3">
              <Field
                label="这顿饭不能吃什么（用逗号隔开）"
                value={excluded}
                onChange={(e) => setExcluded(e.target.value)}
              />
            </div>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button onClick={generate}>
              <Sparkles size={16} /> 帮我推荐菜单
            </Button>
            <select
              className="rounded-xl border p-2 text-sm"
              value={recipe}
              onChange={(e) => setRecipe(e.target.value)}
            >
              <option value="">选择菜谱手工规划</option>
              {recipes.data?.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.title} · {r.minutes} 分钟
                </option>
              ))}
            </select>
            <Button variant="outline" disabled={!recipe} onClick={manual}>
              用这道菜安排
            </Button>
          </div>
          <p className="mt-3 text-xs text-slate-500">
            生成后先看方案，确认满意再加入菜单。做饭完成后才扣库存。
          </p>
        </Card>
      )}
      {err && (
        <div className="mb-4">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
      <h2 className="mb-3 text-lg font-semibold">最近方案</h2>
      <Load loading={plans.isLoading} error={plans.error}>
        {plans.data?.length ? (
          <div className="space-y-4">
            {plans.data.map((x) => (
              <PlanCard key={x.id} id={x.id} />
            ))}
          </div>
        ) : (
          <Card>还没有菜单方案。</Card>
        )}
      </Load>
    </>
  );
}

export function PlanCard({ id }: { id: string }) {
  const { token, p, q } = useBase();
  const d = useData<Plan>("plan", p + "/plans/" + id);
  const recipes = useData<Recipe[]>("recipes", "/recipes");
  const [err, setErr] = useState(""),
    [busy, setBusy] = useState(false);
  async function act(action: "confirm" | "reject") {
    if (!d.data) return;
    setBusy(true);
    setErr("");
    try {
      await api(
        p + "/plans/" + id + "/" + action,
        token,
        "POST",
        action === "confirm"
          ? { revision: d.data.revision, accept_uncertain: false }
          : {},
        undefined,
      );
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  }
  async function change(
    m: Meal,
    recipeId: string,
    servings: number,
    cancel = false,
    additional?: string[],
  ) {
    try {
      await api(p + "/plans/" + id + "/meals/" + m.id, token, "PATCH", {
        recipe_id: recipeId,
        servings,
        cancel,
        ...(additional ? { additional_recipe_ids: additional } : {}),
      });
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  if (d.isLoading) return <Card>加载方案…</Card>;
  if (d.error || !d.data) return <ErrorLine error={d.error} />;
  const v = d.data;
  return (
    <Card>
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">
          {v.meals[0]?.day || "空菜单"} ·{" "}
          {v.status === "draft"
            ? "待确认"
            : v.status === "confirmed"
              ? "已确认"
              : v.status === "consumed"
                ? "已消耗"
                : v.status}
        </h3>
        <span className="text-xs text-slate-500">版本 {v.revision}</span>
      </div>
      <div className="mt-3 space-y-3">
        {v.meals.map((m) => (
          <div key={m.id} className="rounded-xl bg-slate-50 p-3 text-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <b>
                {m.meal === "breakfast"
                  ? "早餐"
                  : m.meal === "lunch"
                    ? "午餐"
                    : "晚餐"}{" "}
                · {m.title}
                {m.additional_recipes.map((r) => " + " + r.title).join("")}
              </b>
              <span>
                {m.servings} 人 ·{" "}
                {m.minutes +
                  m.additional_recipes.reduce((n, r) => n + r.minutes, 0)}{" "}
                分钟
              </span>
            </div>
            {v.status === "draft" && (
              <div className="mt-2 flex flex-wrap gap-2">
                <select
                  aria-label="替换菜谱"
                  value={m.recipe_id}
                  onChange={(e) => change(m, e.target.value, m.servings)}
                  className="rounded-lg border p-1 text-xs"
                >
                  {recipes.data
                    ?.filter(
                      (r) =>
                        !m.additional_recipes.some(
                          (extra) => extra.id === r.id,
                        ),
                    )
                    .map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.title}
                      </option>
                    ))}
                </select>
                <input
                  aria-label="调整人数"
                  className="w-16 rounded-lg border p-1 text-xs"
                  type="number"
                  min="1"
                  max="20"
                  defaultValue={m.servings}
                  onBlur={(e) => {
                    const n = Number(e.target.value);
                    if (n !== m.servings) change(m, m.recipe_id, n);
                  }}
                />
                {m.additional_recipes.map((extra) => (
                  <span key={extra.id} className="flex items-center gap-1">
                    <select
                      aria-label={"替换 " + extra.title}
                      value={extra.id}
                      onChange={(e) =>
                        change(
                          m,
                          m.recipe_id,
                          m.servings,
                          false,
                          m.additional_recipes.map((r) =>
                            r.id === extra.id ? e.target.value : r.id,
                          ),
                        )
                      }
                      className="rounded-lg border p-1 text-xs"
                    >
                      {recipes.data
                        ?.filter(
                          (r) =>
                            r.id === extra.id ||
                            (r.id !== m.recipe_id &&
                              !m.additional_recipes.some(
                                (other) => other.id === r.id,
                              )),
                        )
                        .map((r) => (
                          <option key={r.id} value={r.id}>
                            {r.title}
                          </option>
                        ))}
                    </select>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        change(
                          m,
                          m.recipe_id,
                          m.servings,
                          false,
                          m.additional_recipes
                            .filter((r) => r.id !== extra.id)
                            .map((r) => r.id),
                        )
                      }
                    >
                      移除
                    </Button>
                  </span>
                ))}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => change(m, m.recipe_id, m.servings, true)}
                >
                  取消此餐
                </Button>
              </div>
            )}
          </div>
        ))}
      </div>
      <h4 className="mt-4 text-sm font-semibold">食材需求与采购缺口</h4>
      <div className="mt-2 grid gap-1">
        {v.ingredients.map((x, i) => (
          <div
            key={i}
            className="flex justify-between border-b border-slate-100 py-1 text-sm"
          >
            <span>
              {x.name}
              {x.conversion_needs_confirmation && (
                <span className="ml-1 text-amber-700">单位需确认</span>
              )}
            </span>
            <span>
              需 <Quantity value={x.needed} unit={x.unit} /> · 缺{" "}
              <Quantity value={x.shortage} unit={x.unit} />
            </span>
          </div>
        ))}
      </div>
      <p className="mt-2 text-xs text-slate-500">
        库存以当前数据库重新计算：
        {new Date(v.inventory_rechecked_at).toLocaleString()}
      </p>
      {v.status === "draft" && (
        <div className="mt-4 flex gap-2">
          <Button disabled={busy} onClick={() => act("confirm")}>
            确认菜单并生成采购
          </Button>
          <Button
            disabled={busy}
            variant="outline"
            onClick={() => act("reject")}
          >
            拒绝方案
          </Button>
        </div>
      )}
      {v.status === "confirmed" && (
        <p className="mt-3 text-xs text-slate-500">
          到「我的厨房」查看今日菜单，做好后点击烹饪完成。
        </p>
      )}
      {err && (
        <div className="mt-3">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
    </Card>
  );
}
