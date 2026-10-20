import { useEffect, useRef, useState } from "react";
import {
  ArrowRight,
  Check,
  Clock3,
  Plus,
  Search,
  Shuffle,
  UtensilsCrossed,
  X,
} from "lucide-react";
import type { Recipe } from "./app/shared";
import { Button, Notice } from "./ui";
import {
  discoverRecipes,
  recipeCategories,
  suggestRecipes,
} from "./recipeSelection";
import photos from "./recipePhotos.json";
import "./RecipeDiscovery.css";

export function RecipePhoto({
  recipe,
  eager = false,
}: {
  recipe: Pick<Recipe, "id" | "title">;
  eager?: boolean;
}) {
  const [failed, setFailed] = useState(false);
  const photo = photos.find((p) => p.id === recipe.id);
  return photo && !failed ? (
    <img
      src={photo.src}
      alt={`${recipe.title} · AI 生成参考图`}
      loading={eager ? "eager" : "lazy"}
      width="480"
      height="360"
      onError={() => setFailed(true)}
    />
  ) : (
    <div className="recipe-photo-fallback">
      <UtensilsCrossed size={36} />
      <span>暂无菜品参考图</span>
    </div>
  );
}

export function RecipeDiscovery({
  recipes,
  selected,
  onSelect,
  maxMinutes,
  excluded,
  disabled,
}: {
  recipes: Recipe[];
  selected: string[];
  onSelect: (ids: string[]) => void;
  maxMinutes: number;
  excluded: string[];
  disabled: boolean;
}) {
  const [view, setView] = useState<"picks" | "browse">("picks");
  const [search, setSearch] = useState(""),
    [category, setCategory] = useState("全部");
  const [offset, setOffset] = useState(0),
    [detail, setDetail] = useState<string | null>(null);
  const [replacing, setReplacing] = useState<string | null>(null);
  const top = useRef<HTMLDivElement>(null);
  const tray = useRef<HTMLElement>(null);
  const details = useRef<HTMLElement>(null);
  function reveal(element: HTMLElement | null) {
    element?.scrollIntoView({ block: "center", behavior: "auto" });
  }
  useEffect(() => {
    if (detail) reveal(details.current);
  }, [detail]);
  useEffect(() => {
    if (replacing) reveal(top.current);
  }, [replacing]);
  const allowed = discoverRecipes(recipes, "", "全部", excluded);
  const visible =
    view === "picks"
      ? suggestRecipes(allowed, maxMinutes, offset)
      : discoverRecipes(recipes, search, category, excluded);
  const active = recipes.find((r) => r.id === detail);
  function choose(id: string) {
    if (replacing) {
      onSelect(selected.map((x) => (x === replacing ? id : x)));
      setReplacing(null);
      reveal(tray.current);
    } else onSelect([...selected, id]);
  }
  return (
    <div className="recipe-discovery">
      <div ref={top} className="recipe-discovery-nav">
        <div className="recipe-view-switch" role="group" aria-label="选菜方式">
          <button
            aria-pressed={view === "picks"}
            onClick={() => setView("picks")}
          >
            给我点灵感
          </button>
          <button
            aria-pressed={view === "browse"}
            onClick={() => setView("browse")}
          >
            自己逛菜谱 <span>{recipes.length}</span>
          </button>
        </div>
        {view === "picks" && (
          <Button
            variant="ghost"
            disabled={allowed.length <= 3}
            onClick={() => setOffset((n) => n + 1)}
          >
            <Shuffle size={16} />
            换一组
          </Button>
        )}
      </div>
      {selected.length > 0 && (
        <div className="recipe-selection-dock" role="status">
          <span>
            <Check size={15} />
            这一餐已选 {selected.length} 道
          </span>
          <button onClick={() => reveal(tray.current)}>
            查看这一餐 <ArrowRight size={15} />
          </button>
        </div>
      )}
      {view === "picks" ? (
        <div className="recipe-section-heading">
          <div>
            <span className="recipe-eyebrow">A LITTLE INSPIRATION</span>
            <h2>从这几道开始，怎么样？</h2>
          </div>
          <p>按单道耗时挑选 · 搭配由你决定</p>
        </div>
      ) : (
        <div className="recipe-browser-controls">
          <label className="recipe-search">
            <Search size={18} />
            <input
              aria-label="搜索菜名或食材"
              placeholder="想吃什么？搜菜名或食材"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </label>
          <div className="recipe-categories" role="group" aria-label="菜谱分类">
            {recipeCategories.map((c) => (
              <button
                key={c}
                aria-pressed={category === c}
                onClick={() => setCategory(c)}
              >
                {c}
              </button>
            ))}
          </div>
          <p className="text-sm text-slate-500">
            找到 {visible.length} 道 · 浏览全部耗时，加入前可查看详情
          </p>
        </div>
      )}
      {replacing && (
        <Notice>
          正在替换「{recipes.find((r) => r.id === replacing)?.title}
          」，其他菜会保留。
          <Button size="sm" variant="ghost" onClick={() => setReplacing(null)}>
            取消换菜
          </Button>
        </Notice>
      )}
      <div
        className={"recipe-grid " + (view === "picks" ? "recipe-picks" : "")}
      >
        {visible.map((r, index) => (
          <article className="recipe-tile" key={r.id}>
            <button
              className="recipe-photo-button"
              aria-label={`查看${r.title}`}
              onClick={() => setDetail(detail === r.id ? null : r.id)}
            >
              <RecipePhoto recipe={r} eager={index < 3} />
              <span className="recipe-time">
                <Clock3 size={13} />约 {r.minutes} 分钟
              </span>
            </button>
            <div className="recipe-tile-body">
              <p className="recipe-tags">{r.tags.slice(0, 2).join(" / ")}</p>
              <h3>{r.title}</h3>
              <p className="recipe-ingredients">
                {r.ingredients
                  .filter((i) => i.name !== "饮用水")
                  .map((i) => i.name)
                  .join(" · ")}
              </p>
              <div className="recipe-tile-actions">
                <button
                  className="recipe-detail-link"
                  onClick={() => setDetail(detail === r.id ? null : r.id)}
                >
                  做法与用量 <ArrowRight size={14} />
                </button>
                <button
                  className="recipe-add"
                  aria-label={`${selected.includes(r.id) ? "已选" : replacing ? "换成" : "加入"}${r.title}`}
                  disabled={
                    disabled ||
                    selected.includes(r.id) ||
                    (!replacing && selected.length >= 6)
                  }
                  onClick={() => choose(r.id)}
                >
                  {selected.includes(r.id) ? (
                    <Check size={18} />
                  ) : (
                    <Plus size={18} />
                  )}
                </button>
              </div>
            </div>
          </article>
        ))}
      </div>
      {!visible.length && (
        <div className="recipe-empty">
          <UtensilsCrossed size={32} />
          <h3>这次还没找到合适的菜</h3>
          <p>
            {view === "picks"
              ? "试试增加单道时间，或去菜谱里逛逛。"
              : "试试其他关键词或分类。忌口筛选仍会保留。"}
          </p>
          <Button
            variant="outline"
            onClick={() => {
              setView("browse");
              setSearch("");
              setCategory("全部");
            }}
          >
            浏览菜谱
          </Button>
        </div>
      )}
      {active && (
        <section
          ref={details}
          className="recipe-detail"
          aria-label="所选菜谱详情"
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="recipe-eyebrow">COOK SOMETHING GOOD</p>
              <h3>
                {active.title} · {active.servings} 人份
              </h3>
            </div>
            <Button
              variant="ghost"
              aria-label="关闭菜谱详情"
              onClick={() => setDetail(null)}
            >
              <X size={18} />
            </Button>
          </div>
          <p className="mt-3 text-sm">
            {active.ingredients
              .map((i) => `${i.name} ${i.quantity} ${i.unit}`)
              .join(" · ")}
          </p>
          <ol className="mt-4 list-decimal space-y-2 pl-5 text-sm">
            {active.steps.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ol>
          <p className="mt-4 text-xs text-slate-500">
            {active.source}
            。这是基准份量，安排时按人数换算；调味用量另按常备调味品设置估算。计时结束仍需检查熟透情况。
          </p>
        </section>
      )}
      {selected.length > 0 && (
        <section ref={tray} className="recipe-tray" aria-label="这一餐已选菜">
          <div>
            <p className="recipe-eyebrow">YOUR LITTLE MENU</p>
            <h2>这一餐，就这样搭</h2>
            <p className="text-sm text-slate-500">
              已选 {selected.length} 道，最多 6 道 · 用量按整餐人数计算
            </p>
          </div>
          <ul>
            {selected.map((id) => {
              const r = recipes.find((x) => x.id === id);
              return r ? (
                <li key={id}>
                  <RecipePhoto recipe={r} />
                  <span>{r.title}</span>
                  <button
                    disabled={disabled}
                    onClick={() => {
                      setReplacing(id);
                      setView("browse");
                      setSearch("");
                      setCategory("全部");
                    }}
                    aria-label={`替换${r.title}`}
                  >
                    换菜
                  </button>
                  <button
                    disabled={disabled}
                    aria-label={`移除${r.title}`}
                    onClick={() => {
                      onSelect(selected.filter((x) => x !== id));
                      if (replacing === id) setReplacing(null);
                    }}
                  >
                    <X size={16} />
                  </button>
                </li>
              ) : null;
            })}
          </ul>
        </section>
      )}
      <p className="recipe-photo-caption">
        菜品图片为 AI 生成参考图；食材是否足够，会在菜单草案中核对。
      </p>
    </div>
  );
}
