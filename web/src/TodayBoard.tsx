import * as Dialog from "@radix-ui/react-dialog";
import { useUI } from "./store";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idem } from "./api";
import { Button, Card, Notice } from "./ui";
import { KitchenPipeline } from "./KitchenPipeline";
import { ChefHat, ArrowRight, Utensils } from "lucide-react";

type Meal = {
  plan_id: string;
  meal_id: string;
  meal: string;
  servings: number;
  title: string;
  minutes: number;
  status: string;
};
type Reminder = { name: string; expires_on: string; expiry_kind: string };
const mealName: Record<string, string> = {
  breakfast: "早餐",
  lunch: "午餐",
  dinner: "晚餐",
};

export function TodayBoard({
  token,
  household,
  showReminders = true,
}: {
  token: string;
  household: string;
  showReminders?: boolean;
}) {
  const q = useQueryClient();
  const day = new Date().toLocaleDateString("en-CA");
  const root = "/households/" + household;
  const meals = useQuery({
    queryKey: ["today", root, day],
    queryFn: () => api<Meal[]>(root + "/today?day=" + day, token),
  });
  const reminders = useQuery({
    queryKey: ["reminders", root],
    queryFn: () => api<Reminder[]>(root + "/reminders", token),
  });
  const [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [review, setReview] = useState<Meal | null>(null);
  async function complete(meal: Meal) {
    setBusy(meal.meal_id);
    setError("");
    try {
      await api(
        root +
          "/plans/" +
          meal.plan_id +
          "/meals/" +
          meal.meal_id +
          "/complete",
        token,
        "POST",
        {},
        idem(),
      );
      await q.invalidateQueries();
      setReview(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy("");
    }
  }
  return (
    <>
      <div className="mb-5">
        <h2 className="text-xl font-bold">今天的厨房待办</h2>
        <p className="text-sm text-slate-500">
          看看今天吃什么，以及哪些食材需要优先处理。
        </p>
      </div>
      <Dialog.Root
        open={!!review}
        onOpenChange={(open) => {
          if (!open && !busy) setReview(null);
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 z-40 bg-slate-900/35" />
          <Dialog.Content
            className="fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-2xl border bg-white p-5 shadow-xl"
            onEscapeKeyDown={(e) => {
              if (busy) e.preventDefault();
            }}
            onPointerDownOutside={(e) => {
              if (busy) e.preventDefault();
            }}
          >
            {review && (
              <>
                <Dialog.Title className="font-semibold">
                  确认完成 {review.title}？
                </Dialog.Title>
                <Dialog.Description className="my-3 text-sm text-slate-500">
                  按这餐 {review.servings}{" "}
                  人的配方扣减库存并记录完成，提交时会再次检查实际余量。
                </Dialog.Description>
                {error && <Notice tone="error">{error}</Notice>}
                <div className="flex gap-2">
                  <Button disabled={!!busy} onClick={() => complete(review)}>
                    确认完成并扣库
                  </Button>
                  <Button
                    disabled={!!busy}
                    variant="outline"
                    onClick={() => setReview(null)}
                  >
                    继续做饭
                  </Button>
                </div>
              </>
            )}
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
      <div
        className={showReminders ? "grid gap-4 lg:grid-cols-2" : "grid gap-4"}
      >
        <Card>
          <h2 className="mb-4 flex items-center gap-2 font-semibold text-emerald-950">
            <Utensils size={18} aria-hidden="true" />
            今日菜单
          </h2>
          {meals.isLoading && <p className="text-sm text-slate-500">加载中…</p>}
          {meals.error && (
            <Notice tone="error">
              加载失败{" "}
              <Button size="sm" onClick={() => meals.refetch()}>
                重试
              </Button>
            </Notice>
          )}
          {meals.data?.length === 0 && (
            <div className="ff-meal-empty">
              <span className="ff-empty-icon" aria-hidden="true">
                <ChefHat size={32} strokeWidth={1.4} />
              </span>
              <div className="min-w-0">
                <p className="text-lg font-semibold text-emerald-950">
                  今天还没安排吃什么
                </p>
                <p className="mt-2 text-sm leading-6 text-slate-500">
                  先选一份菜单并确认，就可以在这里跟着步骤做饭。
                </p>
                <Button
                  className="mt-4"
                  onClick={() => useUI.getState().setPage("week")}
                >
                  去安排今天的菜单 <ArrowRight size={16} aria-hidden="true" />
                </Button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {meals.data?.map((meal) => (
              <div
                key={meal.meal_id}
                className="rounded-xl bg-emerald-50 p-3 text-sm"
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <b>
                      {mealName[meal.meal] || meal.meal} · {meal.title}
                    </b>
                    <div className="text-xs text-slate-600">
                      {meal.servings} 人 · {meal.minutes} 分钟
                    </div>
                  </div>
                  {meal.status === "consumed" ? (
                    <span className="text-emerald-800">已完成</span>
                  ) : (
                    <Button
                      size="sm"
                      disabled={!!busy}
                      onClick={() => setReview(meal)}
                    >
                      {busy === meal.meal_id ? "记录中…" : "烹饪完成"}
                    </Button>
                  )}
                </div>
                <KitchenPipeline
                  token={token}
                  root={root}
                  meal={meal.meal_id}
                  consumed={meal.status === "consumed"}
                />
              </div>
            ))}
          </div>
          {error && (
            <div className="mt-3">
              <Notice tone="error">{error}</Notice>
            </div>
          )}
        </Card>
        {showReminders && (
          <Card>
            <h2 className="mb-3 font-semibold">这些食材，记得先吃</h2>
            {reminders.isLoading && <p className="text-sm">加载中…</p>}
            {reminders.error && (
              <Notice tone="error">
                提醒加载失败{" "}
                <Button size="sm" onClick={() => reminders.refetch()}>
                  重试
                </Button>
              </Notice>
            )}
            {reminders.data?.length === 0 && (
              <p className="text-sm text-slate-500">目前没有临期提醒。</p>
            )}
            {reminders.data?.map((v, i) => (
              <div key={i} className="mb-2 rounded-xl bg-amber-50 p-3 text-sm">
                {v.name} · {v.expires_on} 到期
                {v.expiry_kind === "estimate" ? "（估计）" : ""}
              </div>
            ))}
          </Card>
        )}
      </div>
    </>
  );
}
