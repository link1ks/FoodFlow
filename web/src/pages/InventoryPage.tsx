import { useEffect, useState } from "react";
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
  Title,
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
  useEffect(() => {
    setSelected("");
    setSelectedIDs([]);
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
  return (
    <>
      <Title
        title="食材库存"
        subtitle="先录入家里的食材，再勾选它们，查看能做的菜和 AI 搭配建议"
      />
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
      <div className="grid gap-4 lg:grid-cols-[1fr_320px]">
        <div className="order-2 lg:order-1">
          <Load loading={d.isLoading} error={d.error}>
            {d.data?.items.length ? (
              <div className="space-y-3">
                {d.data.items.map((i) => (
                  <Card key={i.id}>
                    <div className="flex gap-3">
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
                            <h2 className="font-semibold">{i.name}</h2>
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
                          <div className="text-right">
                            <b>
                              <Quantity value={i.quantity} unit={i.unit} />
                            </b>
                            {i.low_stock && (
                              <p className="text-xs text-amber-700">库存偏低</p>
                            )}
                          </div>
                        </div>
                        <label className="mt-2 flex items-center gap-2 text-sm text-emerald-800">
                          <input
                            type="checkbox"
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
                            b.ingredient_id === i.id && Number(b.quantity) > 0,
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
              <Card>库存还是空的，先添加一种食材。</Card>
            )}
          </Load>
        </div>
        <div id="inventory-add" className="order-1 space-y-4 lg:order-2">
          <h2 className="font-semibold text-slate-800">往冰箱里添点什么？</h2>
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
        </div>
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
    </>
  );
}
