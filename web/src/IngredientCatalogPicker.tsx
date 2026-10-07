import { NutritionDetails } from "./Nutrition";
import { useMemo, useRef, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Check, Leaf, Search, X } from "lucide-react";
import { api, idem } from "./api";
import {
  CatalogPicture,
  type CatalogItem,
  type InventoryItem,
} from "./CatalogArt";
import { buildIngredientTrie, searchIngredients } from "./ingredientTrie";
import { Button, Card, Field, Notice } from "./ui";

type Dimension = "mass" | "volume" | "count";
const dimension = (unit: string): Dimension =>
  unit === "g" || unit === "kg"
    ? "mass"
    : unit === "ml" || unit === "l"
      ? "volume"
      : "count";
const dimensionLabel: Record<Dimension, string> = {
  mass: "重量",
  volume: "容量",
  count: "个数",
};

export function IngredientCatalogPicker({
  token,
  household,
  inventory,
}: {
  token: string;
  household: string;
  inventory: InventoryItem[];
}) {
  const queryClient = useQueryClient();
  const catalog = useQuery({
    queryKey: ["ingredient-catalog"],
    queryFn: () => api<CatalogItem[]>("/ingredient-catalog", token),
    staleTime: 60 * 60 * 1000,
  });
  const [open, setOpen] = useState(false),
    [category, setCategory] = useState(""),
    [search, setSearch] = useState(""),
    [unitFilter, setUnitFilter] = useState("all"),
    [stockFilter, setStockFilter] = useState(false);
  const [selected, setSelected] = useState<CatalogItem | null>(null),
    [showDetail, setShowDetail] = useState(false);
  const [low, setLow] = useState(""),
    [quantity, setQuantity] = useState(""),
    [location, setLocation] = useState(""),
    [boughtOn, setBoughtOn] = useState(""),
    [expiresOn, setExpiresOn] = useState("");
  const [added, setAdded] = useState<string[]>([]),
    [keepAdding, setKeepAdding] = useState(true);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const last = useRef<{ signature: string; key: string } | null>(null);
  const stockByName = useMemo(
    () => new Map(inventory.map((item) => [item.name + "|" + item.unit, item])),
    [inventory],
  );
  const categories = useMemo(
    () =>
      [...new Set(catalog.data?.map((item) => item.category) || [])].sort(
        (a, b) => a.localeCompare(b, "zh-CN"),
      ),
    [catalog.data],
  );
  const trie = useMemo(
    () => buildIngredientTrie(catalog.data || []),
    [catalog.data],
  );
  const matches = useMemo(() => {
    return searchIngredients(catalog.data || [], trie, search).filter(
      (item) =>
        (!category || item.category === category) &&
        (unitFilter === "all" || dimension(item.default_unit) === unitFilter) &&
        (!stockFilter ||
          Number(
            stockByName.get(item.name + "|" + item.default_unit)?.quantity,
          ) > 0),
    );
  }, [
    catalog.data,
    trie,
    category,
    unitFilter,
    stockFilter,
    search,
    stockByName,
  ]);
  function choose(item: CatalogItem) {
    if (selected?.id !== item.id) {
      setQuantity("");
      setExpiresOn("");
      setBoughtOn("");
      setLocation("");
      setLow("");
    }
    setSelected(item);
    setShowDetail(true);
    setError("");
  }
  async function add() {
    if (!selected) return;
    setBusy(true);
    setError("");
    try {
      const body = {
        catalog_id: selected.id,
        quantity,
        low,
        location,
        bought_on: boughtOn,
        expires_on: expiresOn,
      };
      const signature = JSON.stringify(body);
      if (last.current?.signature !== signature)
        last.current = { signature, key: idem() };
      await api(
        "/households/" + household + "/catalog-stock",
        token,
        "POST",
        body,
        last.current.key,
      );
      await queryClient.invalidateQueries({
        queryKey: ["inventory", "/households/" + household + "/inventory"],
      });
      setAdded((current) => [
        ...current,
        selected.name + " · " + quantity + " " + selected.default_unit,
      ]);
      last.current = null;
      setSelected(null);
      setShowDetail(false);
      setSearch("");
      setCategory("");
      setUnitFilter("all");
      setStockFilter(false);
      setLow("");
      setQuantity("");
      setLocation("");
      setBoughtOn("");
      setExpiresOn("");
      if (!keepAdding) setOpen(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="ff-catalog-launch">
      <span className="ff-catalog-launch-icon">
        <Leaf size={22} aria-hidden="true" />
      </span>
      <h2 className="font-semibold">选食材，记入冰箱</h2>
      <p className="mt-1 text-sm text-slate-500">
        按图片选食材，确认数量即可入库，日期可稍后补充。
      </p>
      <Button
        className="mt-4 w-full"
        onClick={() => {
          setAdded([]);
          setOpen(true);
        }}
      >
        ＋ 添加食材
      </Button>
      <Dialog.Root
        open={open}
        onOpenChange={(value) => {
          setOpen(value);
          if (!value) {
            setSelected(null);
            setShowDetail(false);
            setSearch("");
            setCategory("");
            setUnitFilter("all");
            setStockFilter(false);
            setError("");
          }
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="ff-catalog-overlay fixed inset-0 z-40 bg-slate-950/55" />
          <Dialog.Content className="ff-catalog-dialog fixed inset-0 z-50 flex h-[100dvh] w-screen flex-col overflow-hidden bg-white shadow-2xl outline-none md:inset-auto md:left-1/2 md:top-1/2 md:h-[min(90vh,860px)] md:w-[min(94vw,1120px)] md:-translate-x-1/2 md:-translate-y-1/2 md:rounded-2xl">
            <div className="ff-catalog-heading flex items-center justify-between border-b border-slate-200 px-4 py-3 md:px-6">
              <div>
                <Dialog.Title className="text-xl font-bold">
                  选择食材
                </Dialog.Title>
                <Dialog.Description className="text-xs text-slate-500">
                  把新鲜带回家 · 选好食材，再确认入库数量。
                </Dialog.Description>
              </div>
              <Dialog.Close asChild>
                <button
                  type="button"
                  aria-label="关闭食材选择"
                  className="rounded-lg p-2 hover:bg-slate-100"
                >
                  <X size={20} />
                </button>
              </Dialog.Close>
            </div>
            {added.length > 0 && (
              <div
                role="status"
                className="border-b bg-emerald-50 p-3 text-sm text-emerald-800"
              >
                本次已添加：{added.join("、")}
              </div>
            )}
            <div className="flex min-h-0 flex-1 flex-col md:flex-row">
              <div
                className={
                  "flex min-h-0 min-w-0 flex-1 flex-col " +
                  (showDetail ? "hidden md:flex" : "")
                }
              >
                <div className="ff-catalog-filters space-y-3 border-b border-slate-100 p-4 md:px-6">
                  <label className="relative block">
                    <Search
                      size={18}
                      className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
                    />
                    <input
                      aria-label="搜索食材名称或别名"
                      autoComplete="off"
                      className="w-full rounded-xl border border-slate-300 py-2.5 pl-10 pr-3 outline-none focus:border-emerald-600"
                      placeholder="输入一个字搜索，如“番”"
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                    />
                  </label>
                  <div
                    className="flex gap-2 overflow-x-auto pb-1"
                    role="group"
                    aria-label="食材分类过滤"
                  >
                    <button
                      type="button"
                      onClick={() => setCategory("")}
                      aria-pressed={!category}
                      className={
                        "shrink-0 rounded-full px-3 py-1.5 text-sm " +
                        (!category
                          ? "bg-emerald-700 text-white"
                          : "bg-slate-100 text-slate-700")
                      }
                    >
                      全部
                    </button>
                    {categories.map((value) => (
                      <button
                        type="button"
                        key={value}
                        aria-pressed={category === value}
                        onClick={() => setCategory(value)}
                        className={
                          "shrink-0 rounded-full px-3 py-1.5 text-sm " +
                          (category === value
                            ? "bg-emerald-700 text-white"
                            : "bg-slate-100 text-slate-700")
                        }
                      >
                        {value}
                      </button>
                    ))}
                  </div>
                  <div className="flex items-center gap-3">
                    <label className="text-xs text-slate-600">
                      计量类型{" "}
                      <select
                        aria-label="计量类型过滤"
                        className="ml-1 rounded-lg border border-slate-300 px-2 py-1.5 text-sm"
                        value={unitFilter}
                        onChange={(e) => setUnitFilter(e.target.value)}
                      >
                        <option value="all">全部</option>
                        <option value="mass">重量</option>
                        <option value="volume">容量</option>
                        <option value="count">个数</option>
                      </select>
                    </label>
                    <label className="flex items-center gap-1.5 text-xs text-slate-600">
                      <input
                        type="checkbox"
                        checked={stockFilter}
                        onChange={(e) => setStockFilter(e.target.checked)}
                      />
                      只看有库存
                    </label>
                  </div>
                </div>
                <div className="ff-catalog-results min-h-0 flex-1 overflow-y-auto p-4 md:px-6">
                  <p className="mb-3 text-xs text-slate-500">
                    显示 {matches.length} / {catalog.data?.length || 0} 种食材 ·
                    实拍参考照片，可为家庭食材上传自己的照片 ·{" "}
                    <a
                      href="/ingredient-photos/credits.html"
                      target="_blank"
                      rel="noreferrer"
                      className="underline"
                    >
                      照片来源与授权
                    </a>
                  </p>
                  {catalog.isLoading ? (
                    <p className="py-10 text-center text-slate-500">
                      正在加载食材目录…
                    </p>
                  ) : catalog.error ? (
                    <div>
                      <Notice tone="error">食材目录加载失败</Notice>
                      <Button
                        className="mt-3"
                        onClick={() => catalog.refetch()}
                      >
                        重试
                      </Button>
                    </div>
                  ) : matches.length === 0 ? (
                    <p className="py-10 text-center text-slate-500">
                      没有符合条件的食材，试试其他关键词或筛选。
                    </p>
                  ) : (
                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                      {matches.map((item) => {
                        const stocked = stockByName.get(
                          item.name + "|" + item.default_unit,
                        );
                        return (
                          <button
                            type="button"
                            key={item.id}
                            onClick={() => choose(item)}
                            aria-label={`选择${item.name}`}
                            aria-pressed={selected?.id === item.id}
                            className={
                              "ff-catalog-card overflow-hidden rounded-xl border bg-white text-left shadow-sm transition hover:shadow-md focus-visible:outline-2 focus-visible:outline-emerald-600 " +
                              (selected?.id === item.id
                                ? "border-emerald-600 ring-2 ring-emerald-200"
                                : "border-slate-200")
                            }
                          >
                            <div className="relative overflow-hidden">
                              <CatalogPicture
                                item={item}
                                stock={stocked}
                                token={token}
                                household={household}
                                className="aspect-[4/3] w-full object-cover"
                              />
                              {selected?.id === item.id && (
                                <span className="ff-catalog-selected-mark">
                                  <Check size={16} aria-hidden="true" />
                                </span>
                              )}
                              {stocked && Number(stocked.quantity) > 0 && (
                                <span className="ff-catalog-stock-badge">
                                  家中有库存
                                </span>
                              )}
                            </div>
                            <div className="p-3">
                              <div className="flex items-start justify-between gap-1">
                                <b className="text-sm">{item.name}</b>
                                <span className="shrink-0 text-xs text-slate-500">
                                  {item.default_unit}
                                </span>
                              </div>
                              <p className="mt-1 text-xs text-slate-500">
                                {item.category} ·{" "}
                                {dimensionLabel[dimension(item.default_unit)]}
                              </p>
                              {item.aliases.length > 0 && (
                                <p className="mt-1 truncate text-xs text-slate-400">
                                  又称 {item.aliases.join("、")}
                                </p>
                              )}
                              {stocked && (
                                <p className="mt-1 text-xs text-emerald-700">
                                  家中 {stocked.quantity} {stocked.unit}
                                </p>
                              )}
                            </div>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
              <div
                className={
                  "ff-catalog-detail min-h-0 overflow-y-auto border-l border-slate-200 bg-slate-50 p-4 md:w-80 md:shrink-0 md:p-5 " +
                  (showDetail ? "block" : "hidden md:block")
                }
              >
                {selected ? (
                  <>
                    <button
                      type="button"
                      className="mb-3 flex items-center gap-1 text-sm text-emerald-700 md:hidden"
                      onClick={() => setShowDetail(false)}
                    >
                      <ArrowLeft size={16} />
                      返回食材目录
                    </button>
                    <CatalogPicture
                      item={selected}
                      stock={stockByName.get(
                        selected.name + "|" + selected.default_unit,
                      )}
                      token={token}
                      household={household}
                      className="h-40 w-full rounded-xl object-cover"
                    />
                    <h3 className="mt-3 text-xl font-semibold">
                      {selected.name}
                    </h3>
                    <p className="mt-1 text-sm text-slate-600">
                      {selected.category} ·{" "}
                      {dimensionLabel[dimension(selected.default_unit)]} ·
                      默认单位 {selected.default_unit}
                    </p>
                    {selected.aliases.length > 0 && (
                      <p className="mt-1 text-xs text-slate-500">
                        别名：{selected.aliases.join("、")}
                      </p>
                    )}
                    {stockByName.get(
                      selected.name + "|" + selected.default_unit,
                    ) && (
                      <p className="mt-2 text-sm text-emerald-700">
                        家中现有{" "}
                        {
                          stockByName.get(
                            selected.name + "|" + selected.default_unit,
                          )?.quantity
                        }{" "}
                        {selected.default_unit}
                      </p>
                    )}
                    <NutritionDetails profile={selected.nutrition} />
                    <div className="mt-4 space-y-3">
                      <Field
                        label={`本次入库数量（${selected.default_unit}）`}
                        type="number"
                        min="0.001"
                        step="0.001"
                        value={quantity}
                        onChange={(e) => setQuantity(e.target.value)}
                      />
                      <div className="flex gap-2">
                        {(dimension(selected.default_unit) === "count"
                          ? ["1", "2", "5"]
                          : ["kg", "l"].includes(selected.default_unit)
                            ? ["0.1", "0.25", "0.5"]
                            : ["100", "250", "500"]
                        ).map((v) => (
                          <Button
                            key={v}
                            size="sm"
                            variant="outline"
                            onClick={() => setQuantity(v)}
                          >
                            {v} {selected.default_unit}
                          </Button>
                        ))}
                      </div>
                      <details>
                        <summary className="cursor-pointer text-sm text-slate-600">
                          补充日期、位置与补货线（选填）
                        </summary>
                        <div className="mt-3 space-y-3">
                          <Field
                            label="存放位置（可留空）"
                            value={location}
                            onChange={(e) => setLocation(e.target.value)}
                          />
                          <Field
                            label="购买日期（可留空）"
                            type="date"
                            value={boughtOn}
                            onChange={(e) => setBoughtOn(e.target.value)}
                          />
                          <Field
                            label="有效期（可留空，用户确认）"
                            type="date"
                            value={expiresOn}
                            onChange={(e) => setExpiresOn(e.target.value)}
                          />
                          <Field
                            label="低库存线（可留空）"
                            value={low}
                            onChange={(e) => setLow(e.target.value)}
                          />
                          <p className="text-xs text-slate-500">
                            不填写有效期时记为未知，不会自动估算。
                          </p>
                        </div>
                      </details>
                      <label className="flex gap-2 text-sm">
                        <input
                          type="checkbox"
                          checked={keepAdding}
                          onChange={(e) => setKeepAdding(e.target.checked)}
                        />
                        入库后继续添加下一种
                      </label>
                      <Button
                        className="w-full"
                        disabled={!quantity || busy}
                        onClick={add}
                      >
                        {busy ? "入库中…" : "确认入库"}
                      </Button>
                      {error && <Notice tone="error">{error}</Notice>}
                    </div>
                  </>
                ) : (
                  <div className="flex h-full items-center justify-center text-center text-sm text-slate-500">
                    选择一张食材卡片，查看属性并填写入库数量。
                  </div>
                )}
              </div>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </Card>
  );
}
