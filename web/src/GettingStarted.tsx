import {
  ArrowRight,
  Boxes,
  CalendarDays,
  ShoppingBasket,
  ChefHat,
  Leaf,
} from "lucide-react";
import { useUI } from "./store";

const steps = [
  {
    page: "inventory",
    icon: Boxes,
    title: "家里有什么？",
    action: "添加 / 查看食材",
    description: "记录冰箱里的食材，看看哪些需要先吃。",
  },
  {
    page: "week",
    icon: CalendarDays,
    title: "接下来吃什么？",
    action: "安排一顿饭",
    description: "选人数和时间，让 AI 建议菜单，也能自己选菜。",
  },
  {
    page: "shopping",
    icon: ShoppingBasket,
    title: "还需要买什么？",
    action: "查看采购清单",
    description: "确认菜单后自动列出缺少的食材，买好再确认入库。",
  },
  {
    page: "today",
    icon: ChefHat,
    title: "今天怎么做？",
    action: "查看今日菜单",
    description: "跟着备餐时间轴做饭，完成后自动更新库存。",
  },
] as const;

export function GettingStarted({ household }: { household: string }) {
  const setPage = useUI((s) => s.setPage);
  function go(page: string) {
    if (page === "today")
      document
        .getElementById("today-meals")
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    else setPage(page);
  }
  return (
    <section className="mb-7" aria-label="厨房使用指南">
      <div className="kitchen-welcome">
        <div>
          <p className="mb-2 flex items-center gap-2 text-xs font-semibold text-emerald-700">
            <Leaf size={16} aria-hidden /> {household} · 我的家庭厨房
          </p>
          <h1 className="text-2xl font-bold leading-tight text-emerald-950 sm:text-3xl">
            从冰箱到餐桌，今天也好好吃饭。
          </h1>
          <p className="mt-3 text-sm leading-6 text-emerald-900/75">
            第一次使用？先添加家里的食材，再安排想吃的菜。
          </p>
          <button
            className="mt-5 inline-flex items-center gap-2 rounded-xl bg-emerald-800 px-5 py-3 text-sm font-semibold text-white hover:bg-emerald-900 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-emerald-700"
            onClick={() => go("inventory")}
          >
            从添加食材开始 <ArrowRight size={16} />
          </button>
        </div>
        <div
          className="hidden shrink-0 items-center justify-center rounded-full border border-emerald-200/70 bg-white/60 p-8 text-6xl sm:flex"
          aria-hidden
        >
          🥬
        </div>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {steps.map((s, i) => (
          <button
            key={s.page}
            onClick={() => go(s.page)}
            className="journey-card group text-left"
          >
            <div className="mb-4 flex items-center justify-between">
              <span className="rounded-xl bg-emerald-50 p-2.5 text-emerald-800">
                <s.icon size={23} aria-hidden />
              </span>
              <span className="text-xs font-semibold text-slate-400">
                0{i + 1}
              </span>
            </div>
            <h2 className="text-base font-bold text-slate-900">{s.title}</h2>
            <p className="mt-2 text-sm leading-6 text-slate-500">
              {s.description}
            </p>
            <span className="mt-4 flex items-center justify-between text-sm font-semibold text-emerald-800">
              {s.action}
              <ArrowRight
                size={16}
                className="transition-transform group-hover:translate-x-1"
              />
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

export function PageSwitch({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { id: string; title: string; description: string }[];
}) {
  return (
    <div
      className={
        "ff-page-switch mb-6 grid grid-cols-2 gap-2 " +
        (options.length > 3 ? "lg:grid-cols-5" : "")
      }
      aria-label="选择使用方式"
    >
      {options.map((o) => (
        <button
          key={o.id}
          aria-pressed={value === o.id}
          onClick={() => onChange(o.id)}
          className={
            "min-w-0 rounded-2xl border p-3 text-left transition focus-visible:outline-2 focus-visible:outline-emerald-600 sm:p-4 " +
            (value === o.id
              ? "border-emerald-600 bg-emerald-50"
              : "border-transparent hover:border-emerald-300")
          }
        >
          <span
            className={
              "block text-sm font-semibold " +
              (value === o.id ? "text-emerald-900" : "text-slate-700")
            }
          >
            {o.title}
          </span>
          <span className="mt-1 block text-xs leading-5 text-slate-500">
            {o.description}
          </span>
        </button>
      ))}
    </div>
  );
}
