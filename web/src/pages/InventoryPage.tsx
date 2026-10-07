import { useEffect, useState } from "react";
import {
  ArrowDown,
  Leaf,
  Package,
  Search,
  ShoppingBasket,
  SlidersHorizontal,
} from "lucide-react";
import { IngredientPhoto } from "../CatalogArt";
import { BatchActions, freshness } from "../BatchActions";
import { Jobs } from "./Jobs";
import { ModelNotice } from "../app/Capabilities";
import { ImageUpload } from "../ImageRecognition";
import { IngredientCatalogPicker } from "../IngredientCatalogPicker";
import { IngredientDeleteControl } from "../IngredientDeleteControl";
import { IngredientImageManager } from "../IngredientImages";
import { NutritionDetails } from "../Nutrition";
import { Quantity } from "../Quantity";
import { RecipeExplorer } from "../RecipeExplorer";
import { VirtualPantry } from "../VirtualPantry";
import { api, idem } from "../api";
import {
  Load,
  ErrorLine,
  useBase,
  useData,
  type Household,
  type Inventory,
  type Job,
} from "../app/shared";
import { Button, Card, Field, Notice } from "../ui";
export function InventoryPage() {
  const { token, p, q } = useBase();
  const d = useData<Inventory>("inventory", p + "/inventory");
  const results = useData<Job[]>("jobs", p + "/jobs");
  const relatedResults =
    results.data?.filter(
      (j) => ["image", "advice"].includes(j.kind) && j.status !== "cancelled",
    ) || [];
  const household = useData<Household>("household", p);
  const [selected, setSelected] = useState(""),
    [selectedIDs, setSelectedIDs] = useState<string[]>([]),
    [readyCount, setReadyCount] = useState(0);
  const [qty, setQty] = useState(""),
    [expiry, setExpiry] = useState(""),
    [err, setErr] = useState(""),
    [busy, setBusy] = useState(false);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [onlyStocked, setOnlyStocked] = useState(false);
  useEffect(() => {
    setSelected("");
    setSelectedIDs([]);
    setSearch("");
    setCategory("");
    setOnlyStocked(false);
  }, [p]);
  async function stock() {
    setBusy(true);
    setErr("");
    try {
      await api(
        p + "/stock",
        token,
        "POST",
        {
          ingredient_id: selected,
          quantity: qty,
          reason: "manual",
          expires_on: expiry,
          expiry_kind: expiry ? "user" : "unknown",
          source: "手工录入",
        },
        idem(),
      );
      setQty("");
      setExpiry("");
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  }
  const canEdit = household.data?.role !== "viewer" && !!household.data;
  const urgent = (d.data?.batches || []).filter(
    (b) => Number(b.quantity) > 0 && freshness(b).urgent,
  );
  const items = d.data?.items || [];
  const categories = [
    ...new Set(items.map((i) => i.category || "未分类")),
  ].sort((a, b) => a.localeCompare(b, "zh-CN"));
  const visibleItems = items.filter(
    (i) =>
      i.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()) &&
      (!category || (i.category || "未分类") === category) &&
      (!onlyStocked || Number(i.quantity) > 0),
  );
  return (
    <div className="ff-inventory-page">
      <section className="ff-inventory-hero" aria-label="食材库存概览">
        <div className="ff-inventory-hero-copy">
          <p className="ff-eyebrow">
            <Leaf size={15} aria-hidden="true" /> 我的家庭冰箱
          </p>
          <h1>食材库存</h1>
          <p className="ff-inventory-intro">新鲜有数，每一餐都有着落。</p>
          <p className="mt-2 text-sm leading-6 text-slate-600">
            看看家里还有什么，选几样食材，做顿喜欢的饭。
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            {canEdit && (
              <Button
                variant="default"
                onClick={() =>
                  document
                    .getElementById("inventory-add")
                    ?.scrollIntoView({ behavior: "smooth" })
                }
              >
                <ShoppingBasket size={16} aria-hidden="true" /> 添加新食材
              </Button>
            )}
            <Button
              variant="outline"
              onClick={() =>
                document
                  .getElementById("inventory-list")
                  ?.scrollIntoView({ behavior: "smooth" })
              }
            >
              查看我的库存 <ArrowDown size={15} aria-hidden="true" />
            </Button>
          </div>
        </div>
        <div className="ff-inventory-still-life" aria-hidden="true">
          <IngredientPhoto
            name="西兰花"
            category="蔬菜"
            className="ff-inventory-photo ff-inventory-photo-green"
          />
          <IngredientPhoto
            name="番茄"
            category="蔬菜"
            className="ff-inventory-photo ff-inventory-photo-red"
          />
          <span>好食材 · 好日常</span>
        </div>
      </section>
      <div className="ff-inventory-stats" aria-label="库存统计" role="region">
        <div>
          <Package aria-hidden="true" size={20} />
          <span>
            家中有库存{" "}
            <b>
              {d.data
                ? items.filter((i) => Number(i.quantity) > 0).length
                : "—"}
              <small> 种</small>
            </b>
          </span>
        </div>
        <div>
          <Leaf aria-hidden="true" size={20} />
          <span>
            优先处理{" "}
            <b>
              {d.data ? urgent.length : "—"}
              <small> 批</small>
            </b>
          </span>
        </div>
        <div>
          <ShoppingBasket aria-hidden="true" size={20} />
          <span>
            本次已选{" "}
            <b>
              {selectedIDs.length}
              <small> 种</small>
            </b>
          </span>
        </div>
      </div>
      {urgent.length > 0 && (
        <Card className="mb-4 border-rose-200 bg-rose-50/80">
          <h2 className="font-semibold text-rose-900">
            优先处理 · {urgent.length} 批
          </h2>
          <div className="mt-2 flex flex-wrap gap-2">
            {urgent.map((b) => (
              <span
                key={b.id}
                className="rounded-full bg-white px-3 py-1 text-xs text-rose-800"
              >
                {d.data?.items.find((i) => i.id === b.ingredient_id)?.name} ·{" "}
                {freshness(b).label}
              </span>
            ))}
          </div>
        </Card>
      )}
      <div className="ff-inventory-layout">
        <div id="inventory-list" className="min-w-0">
          <div className="ff-inventory-toolbar">
            <div className="mb-4 flex items-center justify-between gap-3">
              <h2 className="text-lg font-semibold">
                我的食材{" "}
                <span className="ml-1 text-sm font-normal text-slate-500">
                  {items.length} 种
                </span>
              </h2>
              <span className="text-xs text-slate-500">
                勾选食材，查看可做的菜
              </span>
            </div>
            <div className="ff-inventory-filters">
              <label className="ff-inventory-search">
                <Search size={17} aria-hidden="true" />
                <input
                  aria-label="搜索库存食材"
                  placeholder="找找冰箱里的食材…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </label>
              <label className="ff-inventory-category">
                <SlidersHorizontal size={16} aria-hidden="true" />
                <select
                  aria-label="库存分类"
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                >
                  <option value="">全部分类</option>
                  {categories.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </label>
              <button
                type="button"
                className="ff-stock-filter"
                aria-pressed={onlyStocked}
                onClick={() => setOnlyStocked(!onlyStocked)}
              >
                只看有库存
              </button>
            </div>
            {(search || category || onlyStocked) && (
              <div className="mt-3 flex items-center justify-between text-xs text-slate-600">
                <span role="status">找到 {visibleItems.length} 种食材</span>
                <button
                  type="button"
                  className="text-emerald-800 underline underline-offset-4"
                  onClick={() => {
                    setSearch("");
                    setCategory("");
                    setOnlyStocked(false);
                  }}
                >
                  清除筛选
                </button>
              </div>
            )}
          </div>
          <Load loading={d.isLoading} error={d.error}>
            {d.data?.items.length ? (
              visibleItems.length ? (
                <div className="ff-inventory-grid">
                  {visibleItems.map((i) => (
                    <Card
                      key={i.id}
                      className={
                        "ff-stock-card " +
                        (selectedIDs.includes(i.id)
                          ? "ff-stock-card-selected"
                          : "")
                      }
                    >
                      <div className="flex gap-4">
                        <IngredientImageManager
                          id={i.id}
                          name={i.name}
                          category={i.category}
                          hasImage={i.has_image}
                          version={i.image_version}
                          token={token}
                          household={p.slice("/households/".length)}
                          canEdit={canEdit}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-start justify-between gap-2">
                            <div>
                              <h3 className="text-lg font-semibold">
                                {i.name}
                              </h3>
                              <p className="text-xs text-slate-500">
                                {i.category || "未分类"}
                                {Number(i.low) > 0 && (
                                  <>
                                    {" "}
                                    · 补货线{" "}
                                    <Quantity value={i.low} unit={i.unit} />
                                  </>
                                )}
                              </p>
                            </div>
                            <div className="ff-stock-quantity text-right">
                              <b>
                                <Quantity value={i.quantity} unit={i.unit} />
                              </b>
                              {i.low_stock && (
                                <p className="text-xs text-amber-700">
                                  库存偏低
                                </p>
                              )}
                            </div>
                          </div>
                          <label className="ff-stock-select mt-3 flex items-center gap-2 text-sm text-emerald-800">
                            <input
                              type="checkbox"
                              aria-label={`选作本次食材：${i.name}`}
                              checked={selectedIDs.includes(i.id)}
                              disabled={Number(i.quantity) <= 0}
                              onChange={(e) =>
                                setSelectedIDs((current) =>
                                  e.target.checked
                                    ? [...current, i.id]
                                    : current.filter((id) => id !== i.id),
                                )
                              }
                            />
                            选作本次食材
                          </label>
                        </div>
                      </div>
                      <NutritionDetails profile={i.nutrition} />
                      <div className="mt-3 space-y-1">
                        {d.data.batches
                          .filter(
                            (b) =>
                              b.ingredient_id === i.id &&
                              Number(b.quantity) > 0,
                          )
                          .map((b) => (
                            <BatchActions
                              key={b.id}
                              batch={b}
                              name={i.name}
                              unit={i.unit}
                              token={token}
                              household={p.slice("/households/".length)}
                              canEdit={canEdit}
                            />
                          ))}
                      </div>
                      {canEdit && (
                        <div className="mt-3 flex items-center justify-between gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setSelected(i.id);
                              setQty("");
                              setExpiry("");
                              document
                                .getElementById("inventory-add")
                                ?.scrollIntoView({ behavior: "smooth" });
                            }}
                          >
                            再买一批
                          </Button>
                          <IngredientDeleteControl
                            id={i.id}
                            name={i.name}
                            quantity={i.quantity}
                            unit={i.unit}
                            token={token}
                            household={p.slice("/households/".length)}
                            onArchived={() => {
                              setSelectedIDs((current) =>
                                current.filter((id) => id !== i.id),
                              );
                              if (selected === i.id) setSelected("");
                              q.invalidateQueries();
                            }}
                          />
                        </div>
                      )}
                    </Card>
                  ))}
                </div>
              ) : (
                <Card className="ff-inventory-empty">
                  <Search size={30} aria-hidden="true" />
                  <h3>没有找到符合条件的食材</h3>
                  <p>试试其他关键词，或者清除筛选查看全部。</p>
                  <Button
                    variant="outline"
                    onClick={() => {
                      setSearch("");
                      setCategory("");
                      setOnlyStocked(false);
                    }}
                  >
                    查看全部食材
                  </Button>
                </Card>
              )
            ) : (
              <Card className="ff-inventory-empty">
                <ShoppingBasket size={36} aria-hidden="true" />
                <h3>给冰箱添一份新鲜</h3>
                <p>库存还是空的，先添加一种食材。</p>
                {canEdit && (
                  <Button
                    variant="outline"
                    onClick={() =>
                      document
                        .getElementById("inventory-add")
                        ?.scrollIntoView({ behavior: "smooth" })
                    }
                  >
                    开始添加
                  </Button>
                )}
              </Card>
            )}
          </Load>
        </div>
        <aside
          id="inventory-add"
          className="ff-inventory-add space-y-4"
          aria-label="添加与补充食材"
        >
          <div>
            <p className="ff-eyebrow">把新鲜带回家</p>
            <h2 className="mt-2 text-lg font-semibold text-slate-800">
              往冰箱里添点什么？
            </h2>
          </div>
          {canEdit && (
            <IngredientCatalogPicker
              token={token}
              household={p.slice("/households/".length)}
              inventory={d.data?.items || []}
            />
          )}
          {canEdit && (
            <ImageUpload
              token={token}
              household={p.slice("/households/".length)}
              onQueued={() => {
                q.invalidateQueries();
                document
                  .getElementById("inventory-results")
                  ?.scrollIntoView({ behavior: "smooth" });
              }}
            />
          )}
          {selected && canEdit && (
            <Card>
              <h2 className="mb-3 font-semibold">
                补充 {d.data?.items.find((i) => i.id === selected)?.name}
              </h2>
              <div className="space-y-3">
                <Field
                  label="数量"
                  value={qty}
                  onChange={(e) => setQty(e.target.value)}
                />
                <Field
                  label="有效期（可留空）"
                  type="date"
                  value={expiry}
                  onChange={(e) => setExpiry(e.target.value)}
                />
                <Button onClick={stock} disabled={busy || !qty}>
                  确认入库
                </Button>
              </div>
            </Card>
          )}
          {err && <Notice tone="error">{err}</Notice>}
        </aside>
      </div>
      <div id="inventory-results" className="my-5">
        {(relatedResults.length > 0 || results.error) && (
          <details
            open={relatedResults.some((j) =>
              ["queued", "running", "awaiting_confirmation", "failed"].includes(
                j.status,
              ),
            )}
            className="rounded-2xl border bg-white p-4"
          >
            <summary className="cursor-pointer font-semibold">
              识别结果与搭配建议 · 继续处理
            </summary>
            <div className="mt-4">
              <ErrorLine error={results.error} />
              {relatedResults.some((j) => j.kind === "image") && (
                <Jobs kind="image" />
              )}
              {relatedResults.some((j) => j.kind === "advice") && (
                <Jobs kind="advice" />
              )}
            </div>
          </details>
        )}
      </div>
      <details className="my-5 rounded-2xl border border-slate-200 bg-white p-4">
        <summary className="cursor-pointer font-semibold">
          调味品免称重{" "}
          <span className="ml-2 text-xs font-normal text-slate-500">
            油盐不用每次称，按配方估算余量
          </span>
        </summary>
        <div className="mt-4">
          <VirtualPantry
            token={token}
            root={p}
            items={d.data?.items || []}
            batches={d.data?.batches || []}
            canEdit={canEdit}
          />
        </div>
      </details>
      <div id="recipe-explorer">
        {selectedIDs.length > 0 && <ModelNotice />}
        {selectedIDs.length > 0 ? (
          <RecipeExplorer
            token={token}
            household={p.slice("/households/".length)}
            items={d.data?.items || []}
            selected={selectedIDs}
            defaultServings={household.data?.servings || 2}
            canEdit={canEdit}
            onReadyCount={setReadyCount}
          />
        ) : (
          <Card className="mt-5 border-dashed">
            <h2 className="font-semibold">想用这些食材做顿饭？</h2>
            <p className="mt-2 text-sm text-slate-500">
              勾选上方食材卡片中的「选作本次食材」，这里就会显示可做的菜，以及
              AI 搭配建议入口。
            </p>
          </Card>
        )}
      </div>
      {selectedIDs.length > 0 && (
        <div className="fixed bottom-[calc(5rem+env(safe-area-inset-bottom))] sm:bottom-5 left-3 right-3 z-30 mx-auto flex max-w-xl items-center justify-between gap-3 rounded-2xl bg-emerald-900 px-4 py-3 text-white shadow-2xl">
          <span className="text-sm">
            已选 {selectedIDs.length} 种食材，可做 {readyCount} 道菜
          </span>
          <Button
            size="sm"
            className="shrink-0 bg-white text-emerald-900 hover:bg-emerald-50"
            onClick={() =>
              document
                .getElementById("recipe-explorer")
                ?.scrollIntoView({ behavior: "smooth" })
            }
          >
            立即规划 →
          </Button>
        </div>
      )}
    </div>
  );
}
