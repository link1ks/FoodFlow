import { useEffect, useState } from "react";
import { GettingStarted } from "./GettingStarted";
import { TodayBoard } from "./TodayBoard";
import { freshness } from "./BatchActions";
import { useData, Inventory, Job, ErrorLine, Title } from "./app/shared";
import { Button, Card } from "./ui";
import { useUI } from "./store";

export function KitchenHelp({ name }: { name: string }) {
  return <GettingStarted household={name} />;
}

export function KitchenHome({
  name,
  token,
  household,
}: {
  name: string;
  token: string;
  household: string;
}) {
  const root = "/households/" + household;
  const inventory = useData<Inventory>("inventory", root + "/inventory");
  const jobs = useData<Job[]>("jobs", root + "/jobs");
  const shopping = useData<
    { id: string; checked: boolean; stocked: boolean }[]
  >("shopping", root + "/shopping");
  const key = "ff_guide_dismissed_" + household;
  const [dismissed, setDismissed] = useState(
    () => localStorage.getItem(key) === "yes",
  );
  useEffect(() => {
    if (
      inventory.data?.items.length ||
      jobs.data?.length ||
      shopping.data?.length
    ) {
      localStorage.setItem(key, "yes");
      setDismissed(true);
    }
  }, [key, inventory.data, jobs.data, shopping.data]);
  const setPage = useUI((s) => s.setPage);
  const tasks =
    jobs.data?.filter((j) =>
      ["queued", "running", "awaiting_confirmation", "failed"].includes(
        j.status,
      ),
    ) || [];
  const urgent =
    inventory.data?.batches.filter(
      (b) => Number(b.quantity) > 0 && freshness(b).urgent,
    ) || [];
  const expired = urgent.filter((b) =>
    ["已过期", "已变质"].includes(freshness(b).label),
  );
  const expiring = urgent.filter((b) => !expired.includes(b));
  const pending = shopping.data?.filter((i) => !i.stocked) || [];
  const newKitchen =
    inventory.data?.items.length === 0 &&
    jobs.data?.length === 0 &&
    shopping.data?.length === 0;
  return (
    <>
      <Title
        title={`${name} · 今天`}
        subtitle="先处理需要确认的事，再看看今天吃什么"
      />
      {inventory.isLoading || jobs.isLoading || shopping.isLoading ? (
        <p className="mb-4 text-sm text-slate-500">正在整理厨房待办…</p>
      ) : null}
      {newKitchen && !dismissed && (
        <div className="mb-5">
          <Button
            className="mb-3"
            variant="outline"
            onClick={() => {
              localStorage.setItem(key, "yes");
              setDismissed(true);
            }}
          >
            收起指南，查看今日待办
          </Button>
          <GettingStarted household={name} />
        </div>
      )}
      <div className="mb-4">
        <Button variant="ghost" size="sm" onClick={() => setPage("help")}>
          使用帮助
        </Button>
      </div>
      {jobs.error ? (
        <Card>
          <ErrorLine error={jobs.error} />
          <Button onClick={() => jobs.refetch()}>重试待确认事项</Button>
        </Card>
      ) : (
        tasks.length > 0 && (
          <Card className="mb-4">
            <h2 className="mb-3 font-semibold">需要你确认与继续处理</h2>
            <div className="flex flex-wrap gap-2">
              {tasks.map((j) => (
                <Button
                  key={j.id}
                  variant="outline"
                  onClick={() => {
                    setPage(j.kind === "plan" ? "week" : "inventory");
                  }}
                >
                  {j.kind === "image"
                    ? "图片识别"
                    : j.kind === "advice"
                      ? "搭配建议"
                      : "菜单规划"}{" "}
                  ·{" "}
                  {j.status === "awaiting_confirmation"
                    ? "待确认"
                    : j.status === "failed"
                      ? "失败，查看原因"
                      : "处理中"}
                </Button>
              ))}
            </div>
            <p className="mt-2 text-xs text-slate-500">
              查看近期生成任务；全部生成记录可在“记录”中找回。
            </p>
          </Card>
        )
      )}
      {inventory.error ? (
        <Card>
          <ErrorLine error={inventory.error} />
          <Button onClick={() => inventory.refetch()}>重试食材提醒</Button>
        </Card>
      ) : (
        urgent.length > 0 && (
          <Card className="mb-4">
            {[
              ["已过期或变质 · 核对后处理", expired],
              ["临期食材 · 优先安排", expiring],
            ].map(
              ([label, batches]) =>
                (batches as typeof urgent).length > 0 && (
                  <div key={label as string} className="mb-3">
                    <h2 className="font-semibold">{label as string}</h2>
                    <div className="my-2 flex flex-wrap gap-2">
                      {(batches as typeof urgent).map((b) => (
                        <span
                          key={b.id}
                          className="rounded-xl bg-amber-50 p-2 text-sm"
                        >
                          {
                            inventory.data?.items.find(
                              (i) => i.id === b.ingredient_id,
                            )?.name
                          }{" "}
                          · {freshness(b).label}
                          {b.expiry_kind === "estimate" ? "（估计）" : ""}
                        </span>
                      ))}
                    </div>
                  </div>
                ),
            )}
            <Button variant="outline" onClick={() => setPage("inventory")}>
              查看食材与处理
            </Button>
          </Card>
        )
      )}
      <TodayBoard token={token} household={household} showReminders={false} />
      {shopping.error ? (
        <Card>
          <ErrorLine error={shopping.error} />
          <Button onClick={() => shopping.refetch()}>重试采购待办</Button>
        </Card>
      ) : (
        pending.length > 0 && (
          <Card className="mt-4">
            <h2 className="font-semibold">还需要买什么</h2>
            <p className="my-3 text-sm text-slate-500">
              {pending.filter((i) => !i.checked).length} 项待买 ·{" "}
              {pending.filter((i) => i.checked).length} 项已买待入库
            </p>
            <Button onClick={() => setPage("shopping")}>处理采购清单</Button>
          </Card>
        )
      )}
    </>
  );
}
