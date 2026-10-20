import { Jobs } from "./Jobs";
import { ModelNotice } from "../app/Capabilities";
import { Sparkles } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { MultiMealPlanner } from "../MultiMealPlanner";
import { Quantity } from "../Quantity";
import { RecipeDiscovery } from "../RecipeDiscovery";
import { discoverRecipes } from "../recipeSelection";
import { api } from "../api";
import {
  ErrorLine,
  Load,
  useBase,
  useData,
  type Meal,
  type Plan,
  type Recipe,
  type Job,
  type Household,
} from "../app/shared";
import { Button, Card, Field, Notice } from "../ui";

export function Week() {
  const { p } = useBase();
  return <MenuStudio key={p} />;
}
function MenuStudio() {
  const { token, p, q } = useBase();
  const household = useData<Household & { excluded_ingredients: string[] }>(
    "household",
    p,
  );
  const results = useData<Job[]>("jobs", p + "/jobs");
  const plans = useData<{ id: string; status: string; revision: number }[]>(
    "plans",
    p + "/plans",
  );
  const recipes = useData<Recipe[]>("recipes", "/recipes");
  const [day, setDay] = useState(new Date().toLocaleDateString("en-CA")),
    [meal, setMeal] = useState("");
  const [servings, setServings] = useState("1"),
    [max, setMax] = useState("30");
  const [preference, setPreference] = useState(""),
    [excluded, setExcluded] = useState("");
  const [selected, setSelected] = useState<string[]>([]),
    [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const lock = useRef(false),
    pending = useRef<{ body: string; key: string } | null>(null);
  const resultRef = useRef<HTMLDivElement>(null);
  const canEdit =
    household.data?.role === "owner" || household.data?.role === "editor";
  const exclusions = [
    ...(household.data?.excluded_ingredients || []),
    ...excluded
      .split(/[，,、]/)
      .map((x) => x.trim())
      .filter(Boolean),
  ];
  const valid =
    !!day &&
    !!meal &&
    Number.isInteger(Number(servings)) &&
    Number(servings) >= 1 &&
    Number(servings) <= 20 &&
    Number.isInteger(Number(max)) &&
    Number(max) >= 1 &&
    Number(max) <= 240;
  const safe = new Set(
    discoverRecipes(recipes.data || [], "", "全部", exclusions).map(
      (r) => r.id,
    ),
  );
  const conflict = selected.some((id) => !safe.has(id));
  useEffect(() => {
    if (household.data) setServings(String(household.data.servings));
  }, [household.data?.servings]);
  async function create(ai = false) {
    if (
      !valid ||
      !canEdit ||
      lock.current ||
      (!ai && (!selected.length || conflict))
    )
      return;
    lock.current = true;
    setBusy(true);
    setErr("");
    const body = ai
      ? {
          day,
          meal,
          servings: Number(servings),
          max_minutes: Number(max),
          preference,
          excluded_ingredients: exclusions,
        }
      : {
          meals: [
            {
              day,
              meal,
              servings: Number(servings),
              recipe_id: selected[0],
              additional_recipe_ids: selected.slice(1),
            },
          ],
        };
    const identity = JSON.stringify({ ai, body });
    if (pending.current?.body !== identity)
      pending.current = { body: identity, key: crypto.randomUUID() };
    try {
      await api(
        p + (ai ? "/jobs/plan" : "/plans"),
        token,
        "POST",
        body,
        pending.current.key,
      );
      pending.current = null;
      await q.invalidateQueries();
      if (!ai) setSelected([]);
      resultRef.current?.scrollIntoView({
        behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
          ? "auto"
          : "smooth",
        block: "start",
      });
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return (
    <div className="menu-studio">
      <header className="menu-hero">
        <span className="recipe-eyebrow">FOODFLOW / THE EVERYDAY TABLE</span>
        <h1>今天，吃点喜欢的。</h1>
        <p>
          没想法，就从几道家常好菜开始。想露一手，也可以慢慢逛，把喜欢的搭成一餐。
        </p>
      </header>
      <ErrorLine error={household.error} />
      {household.data?.role === "viewer" && (
        <Notice>当前为只读成员，可以浏览菜谱；安排菜单需要编辑权限。</Notice>
      )}
      <Load
        loading={recipes.isLoading || household.isLoading}
        error={recipes.error || household.error}
      >
        <RecipeDiscovery
          recipes={recipes.data || []}
          selected={selected}
          onSelect={setSelected}
          maxMinutes={Number(max)}
          excluded={exclusions}
          disabled={busy || !canEdit}
        />
      </Load>
      <section className="menu-arrangement" aria-label="用餐安排">
        <div className="menu-meals">
          <span className="mr-2 text-sm font-semibold text-emerald-950">
            安排哪一餐？
          </span>
          {[
            ["breakfast", "早餐"],
            ["lunch", "午餐"],
            ["dinner", "晚餐"],
          ].map(([id, label]) => (
            <button
              key={id}
              disabled={busy}
              aria-pressed={meal === id}
              onClick={() => setMeal(id)}
            >
              {label}
            </button>
          ))}
        </div>
        <details className="mt-4">
          <summary>
            {day} · {servings} 人吃 · 单道最多 {max} 分钟　调整
          </summary>
          <div className="menu-arrangement-grid">
            <Field
              label="日期"
              type="date"
              value={day}
              disabled={busy}
              onChange={(e) => setDay(e.target.value)}
            />
            <Field
              label="人数"
              type="number"
              min="1"
              max="20"
              value={servings}
              disabled={busy}
              onChange={(e) => setServings(e.target.value)}
            />
            <Field
              label="单道最多分钟"
              type="number"
              min="1"
              max="240"
              value={max}
              disabled={busy}
              onChange={(e) => setMax(e.target.value)}
            />
            <Field
              label="这餐避开的食材"
              placeholder="如：虾、鸡蛋"
              value={excluded}
              disabled={busy}
              onChange={(e) => setExcluded(e.target.value)}
            />
          </div>
        </details>
        <p className="mt-3 text-xs text-slate-500">
          {exclusions.length
            ? "已避开：" + exclusions.join("、")
            : "未设置忌口，可在「调整」中添加。"}{" "}
          搜索按食材名称筛选，请在详情中核对用料。
        </p>
      </section>
      {selected.length > 0 && (
        <div className="menu-submit">
          <Button
            disabled={busy || !valid || !canEdit || conflict}
            onClick={() => create()}
          >
            {busy ? "正在安排…" : "用这些菜安排一餐"}
          </Button>
          <p>
            {conflict
              ? "已选菜含避开的食材，请移除或换菜。"
              : !meal
                ? "选一下早餐、午餐或晚餐，就可以安排了。"
                : "先生成草案核对缺料，确认后加入菜单；做好后才扣库存。"}
          </p>
        </div>
      )}
      {err && <Notice tone="error">{err}</Notice>}
      <div ref={resultRef} id="menu-results" className="my-6">
        {(results.data?.some(
          (j) => j.kind === "plan" && j.status !== "cancelled",
        ) ||
          results.error) && (
          <details open className="mb-4 rounded-2xl border bg-white p-4">
            <summary className="cursor-pointer font-semibold">
              AI 菜单生成结果 · 继续处理
            </summary>
            <Jobs kind="plan" />
          </details>
        )}
        <Load loading={plans.isLoading} error={plans.error}>
          {plans.data?.some((x) => x.status === "draft") && (
            <>
              <h2 className="mb-3 text-xl font-semibold text-emerald-950">
                这一餐，核对一下
              </h2>
              <div className="space-y-4">
                {plans.data
                  .filter((x) => x.status === "draft")
                  .map((x) => (
                    <PlanCard key={x.id} id={x.id} />
                  ))}
              </div>
            </>
          )}
          {plans.data?.some(
            (x) => !["draft", "rejected"].includes(x.status),
          ) && (
            <details className="mt-5 rounded-2xl border bg-white p-4">
              <summary className="cursor-pointer font-semibold">
                已安排的菜单
              </summary>
              <div className="mt-4 space-y-4">
                {plans.data
                  .filter((x) => !["draft", "rejected"].includes(x.status))
                  .map((x) => (
                    <PlanCard key={x.id} id={x.id} />
                  ))}
              </div>
            </details>
          )}
        </Load>
      </div>
      <details className="my-5 rounded-2xl border border-stone-200 p-5">
        <summary className="cursor-pointer text-sm font-semibold text-slate-600">
          想按特别的要求搭配？让 AI 帮忙
        </summary>
        <div className="mt-4">
          <ModelNotice />
          <Field
            label="其他偏好"
            value={preference}
            onChange={(e) => setPreference(e.target.value)}
            placeholder="比如：想吃清淡一点"
          />
          <Button
            className="mt-4"
            disabled={!valid || busy || !canEdit}
            onClick={() => create(true)}
          >
            <Sparkles size={16} />
            帮我推荐菜单
          </Button>
          <p className="mt-2 text-xs text-slate-500">
            沿用上面的日期、餐次、人数与忌口；点击才会提交推荐请求。
          </p>
        </div>
      </details>
      <details className="my-5 text-sm text-slate-500">
        <summary className="cursor-pointer">需要安排接下来几餐？</summary>
        <div className="mt-4">
          <MultiMealPlanner token={token} root={p} />
        </div>
      </details>
    </div>
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
          到「今天」查看今日菜单，做好后点击烹饪完成。
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
