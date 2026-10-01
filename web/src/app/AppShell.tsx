import {
  Boxes,
  BarChart3,
  CalendarDays,
  Home,
  LogOut,
  RefreshCw,
  ShoppingBasket,
} from "lucide-react";
import { useEffect, useState } from "react";
import { version } from "../../package.json";
import { AccountMerge, PhoneBinding } from "../AccountLifecycle";
import { Avatar } from "../Avatar";
import { NatureBackdrop } from "../NatureTheme";
import { api } from "../api";
import {
  ErrorLine,
  Title,
  useBase,
  useData,
  type Household,
} from "../app/shared";
import { Prices, Shopping, Today } from "../pages/Boards";
import { InventoryPage } from "../pages/InventoryPage";
import { Jobs } from "../pages/Jobs";
import {
  ExclusionSettings,
  ProfileSettings,
  Settings,
} from "../pages/Settings";
import { Week } from "../pages/Week";
import { Records } from "../pages/Records";
import { KitchenHelp } from "../KitchenHome";
import { useUI } from "../store";
import { Button, Card, Field, Notice } from "../ui";
export function HouseholdGate() {
  const { token, q } = useBase();
  const query = useData<Household[]>("households", "/households");
  const setHouse = useUI((s) => s.setHousehold),
    setToken = useUI((s) => s.setToken),
    house = useUI((s) => s.household);
  const [name, setName] = useState(""),
    [servings, setServings] = useState("2"),
    [code, setCode] = useState(""),
    [err, setErr] = useState("");
  const current = query.data?.find((x) => x.id === house);
  useEffect(() => {
    if (query.data?.length && !current) setHouse(query.data[0].id);
  }, [query.data, current, setHouse]);
  async function create() {
    try {
      const v = await api<{ id: string }>("/households", token, "POST", {
        name,
        servings: Number(servings),
        preferences: "",
      });
      await query.refetch();
      setHouse(v.id);
    } catch (e) {
      setErr(String(e));
    }
  }
  async function join() {
    try {
      const v = await api<{ household_id: string }>(
        "/invites/accept",
        token,
        "POST",
        { code },
      );
      await query.refetch();
      setHouse(v.household_id);
    } catch (e) {
      setErr(String(e));
    }
  }
  if (query.isLoading) return <div className="p-10">正在加载家庭…</div>;
  if (query.error)
    return (
      <main className="mx-auto max-w-md p-6">
        <Card>
          <ErrorLine error={query.error} />
          <Button
            className="mt-3"
            onClick={() => {
              setToken("");
              q.clear();
            }}
          >
            重新登录
          </Button>
        </Card>
      </main>
    );
  if (current) return <AppShell household={current} />;
  return (
    <main className="mx-auto max-w-lg px-4 py-12">
      <h1 className="mb-6 text-3xl font-bold">开始使用食光</h1>
      <Card>
        <h2 className="mb-4 text-xl font-semibold">创建家庭</h2>
        <div className="space-y-3">
          <Field
            label="家庭名称"
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
          <Button onClick={create}>创建</Button>
        </div>
      </Card>
      <Card className="mt-4">
        <h2 className="mb-4 text-xl font-semibold">接受邀请</h2>
        <Field
          label="邀请码"
          value={code}
          onChange={(e) => setCode(e.target.value)}
        />
        <Button className="mt-3" variant="outline" onClick={join}>
          加入家庭
        </Button>
      </Card>
      {err && (
        <div className="mt-4">
          <Notice tone="error">{err}</Notice>
        </div>
      )}
    </main>
  );
}

export const nav = [
  ["today", "今天", Home, "厨房待办"],
  ["inventory", "食材", Boxes, "库存 · 保鲜"],
  ["week", "菜单", CalendarDays, "安排 · 做饭"],
  ["shopping", "采购", ShoppingBasket, "清单 · 菜价"],
  ["records", "记录", BarChart3, "营养 · 用量"],
] as const;

export function AppShell({ household }: { household: Household }) {
  const page = useUI((s) => s.page),
    setPage = useUI((s) => s.setPage),
    setToken = useUI((s) => s.setToken);
  const { token, p, q } = useBase();
  const [connected, setConnected] = useState(true);
  const me = useData<{ name: string; email: string; phone: string }>(
    "me",
    "/me",
  );
  const [logoutError, setLogoutError] = useState("");
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [page]);
  useEffect(() => {
    let stop = false,
      controller: AbortController;
    async function connect() {
      while (!stop) {
        controller = new AbortController();
        try {
          const r = await fetch("/api" + p + "/events", {
            headers: { Authorization: "Bearer " + token },
            signal: controller.signal,
          });
          if (!r.ok) throw Error("stream failed");
          setConnected(true);
          const reader = r.body?.getReader();
          if (!reader) throw Error("stream unavailable");
          while (!stop) {
            const v = await reader.read();
            if (v.done) break;
            if (/event: (job|sync)/.test(new TextDecoder().decode(v.value)))
              q.invalidateQueries();
          }
        } catch {
          if (!stop) setConnected(false);
        }
        await new Promise((resolve) => setTimeout(resolve, 3000));
        q.invalidateQueries();
      }
    }
    connect();
    return () => {
      stop = true;
      controller?.abort();
    };
  }, [token, p, q]);
  async function logout() {
    setLogoutError("");
    try {
      await api("/logout", token, "POST");
      setToken("");
      q.clear();
    } catch (e) {
      setLogoutError(String(e));
    }
  }
  return (
    <div className="foodflow-shell min-h-screen pb-24">
      <NatureBackdrop />
      <header className="sticky top-0 z-20 border-b border-slate-100 bg-white/95 shadow-sm">
        <div className="flex min-h-16 flex-wrap items-center gap-x-5 px-4 sm:px-6 xl:flex-nowrap xl:px-8">
          <button
            onClick={() => setPage("today")}
            aria-label="食光首页"
            className="group order-1 flex shrink-0 items-center gap-2 py-3 text-left focus-visible:outline-2 focus-visible:outline-emerald-600"
          >
            <span
              className="text-2xl transition-transform group-hover:-rotate-12"
              aria-hidden="true"
            >
              🍲
            </span>
            <span className="text-xl font-extrabold tracking-tight text-emerald-800">
              食光
              <span className="ml-1.5 text-xs font-medium tracking-normal text-slate-400">
                FoodFlow
              </span>
            </span>
          </button>
          <nav
            aria-label="主导航"
            className="fixed inset-x-0 bottom-0 z-40 flex justify-around border-t border-slate-200 bg-white pb-[env(safe-area-inset-bottom)] sm:static sm:order-3 sm:w-full sm:justify-start sm:border-0 xl:order-2 xl:w-auto xl:gap-2"
          >
            {nav.map(([id, label, Icon, hint]) => (
              <button
                key={id}
                aria-current={
                  page === id ||
                  (id === "shopping" && page === "prices") ||
                  (id === "records" && page === "insights")
                    ? "page"
                    : undefined
                }
                onClick={() => setPage(id)}
                className={
                  "group relative flex h-16 flex-col justify-center shrink-0 items-center gap-1 whitespace-nowrap sm:flex-row sm:gap-2 px-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-emerald-600 xl:h-16 " +
                  (page === id ||
                  (id === "shopping" && page === "prices") ||
                  (id === "records" && page === "insights")
                    ? "font-semibold text-emerald-700"
                    : "text-slate-700 hover:text-emerald-700")
                }
              >
                <Icon
                  size={16}
                  className="transition-transform group-hover:-translate-y-0.5"
                />
                <span className="text-left">
                  <span className="block">{label}</span>
                  <span className="mt-0.5 hidden sm:block text-[10px] font-normal text-slate-500">
                    {hint}
                  </span>
                </span>
                {(page === id ||
                  (id === "shopping" && page === "prices") ||
                  (id === "records" && page === "insights")) && (
                  <span className="absolute inset-x-2 bottom-0 h-[3px] rounded-t-full bg-emerald-600" />
                )}
              </button>
            ))}
          </nav>
          <div className="order-2 ml-auto flex shrink-0 items-center gap-2 xl:order-3 xl:gap-4">
            {!connected && (
              <span role="status" className="text-xs text-amber-700">
                重连中
              </span>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => q.invalidateQueries()}
              aria-label="刷新"
            >
              <RefreshCw size={17} />
            </Button>
            <button
              onClick={() => setPage("profile")}
              aria-label="打开个人主页"
              aria-current={page === "profile" ? "page" : undefined}
              className={
                "group flex items-center gap-2 rounded-full py-1 pl-1 pr-2 transition focus-visible:outline-2 focus-visible:outline-emerald-600 " +
                (page === "profile" ? "bg-emerald-50" : "hover:bg-slate-50")
              }
            >
              <span className="transition-transform group-hover:scale-105">
                <Avatar name={me.data?.name} />
              </span>
              <span className="hidden text-left sm:block">
                <span className="block max-w-24 truncate text-xs font-semibold text-slate-700">
                  {me.data?.name || "我的账号"}
                </span>
                <span className="block text-[11px] text-slate-400">
                  个人中心
                </span>
              </span>
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto min-w-0 max-w-6xl px-4 py-6">
        {page === "today" ? (
          <>
            <div id="today-meals">
              <Today name={household.name} />
            </div>
          </>
        ) : page === "inventory" ? (
          <InventoryPage />
        ) : page === "week" ? (
          <Week />
        ) : page === "shopping" ? (
          <Shopping />
        ) : page === "prices" ? (
          <Prices />
        ) : page === "jobs" ? (
          <Jobs />
        ) : page === "records" || page === "insights" ? (
          <Records />
        ) : page === "profile" ? (
          <div className="mx-auto max-w-3xl">
            <Title
              title="个人主页"
              subtitle="管理个人资料、登录方式与提醒偏好"
            />
            <Card>
              <div className="flex items-center gap-4">
                <Avatar name={me.data?.name} large />
                <div className="min-w-0">
                  <h2 className="truncate text-xl font-semibold">
                    {me.data?.name || "我的账号"}
                  </h2>
                  <p className="mt-1 break-all text-sm text-slate-500">
                    {me.data?.email || me.data?.phone || "个人账号"}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    {household.name}
                  </p>
                </div>
              </div>
              {me.error && (
                <div className="mt-3">
                  <ErrorLine error={me.error} />
                </div>
              )}
            </Card>
            <div className="my-4 flex flex-wrap gap-3">
              <Button variant="outline" onClick={() => setPage("settings")}>
                家庭成员与偏好
              </Button>
              <Button variant="outline" onClick={() => setPage("help")}>
                使用帮助
              </Button>
            </div>
            <p className="mb-4 text-xs text-slate-500">FoodFlow v{version}</p>
            <ProfileSettings />
            <PhoneBinding token={token} />
            <AccountMerge token={token} />
            <div className="mt-5">
              <Button variant="outline" onClick={logout}>
                <LogOut size={16} className="mr-2" />
                退出登录
              </Button>
              {logoutError && (
                <div className="mt-3">
                  <Notice tone="error">{logoutError}</Notice>
                </div>
              )}
            </div>
          </div>
        ) : page === "help" ? (
          <KitchenHelp name={household.name} />
        ) : (
          <>
            <Settings />
            <ExclusionSettings />
          </>
        )}
      </main>
    </div>
  );
}
