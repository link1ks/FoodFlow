import { useEffect, useState } from "react";
import { ImageReview } from "../ImageRecognition";
import { AdviceView, type AdviceResult } from "../NutritionAdvice";
import { api } from "../api";
import {
  ErrorLine,
  Load,
  Title,
  statusLabel,
  useBase,
  useData,
  type Job,
} from "../app/shared";
import { useUI } from "../store";
import { Button, Card, Notice } from "../ui";
export function Jobs({
  kind,
  activeOnly = false,
}: {
  kind?: string;
  activeOnly?: boolean;
}) {
  const { p } = useBase();
  const jobs = useData<{ id: string; kind: string; status: string }[]>(
    "jobs",
    p + "/jobs",
  );
  const [hidden, setHidden] = useState<string[]>([]);
  const visible = jobs.data?.filter(
    (j) =>
      !hidden.includes(j.id) &&
      (!kind || j.kind === kind) &&
      (!activeOnly ||
        ["queued", "running", "awaiting_confirmation", "failed"].includes(
          j.status,
        )),
  );
  return (
    <>
      <Title
        title={
          kind === "image"
            ? "识别结果"
            : kind === "advice"
              ? "搭配建议记录"
              : kind === "plan"
                ? "菜单生成结果"
                : "生成记录"
        }
        subtitle="离开页面后，仍可在这里继续查看和确认"
      />
      {
        <Load loading={jobs.isLoading} error={jobs.error}>
          {visible?.length ? (
            <div className="space-y-3">
              {visible.map((job) => (
                <JobCard
                  key={job.id}
                  id={job.id}
                  onHidden={() =>
                    setHidden((ids) =>
                      ids.includes(job.id) ? ids : [...ids, job.id],
                    )
                  }
                />
              ))}
            </div>
          ) : (
            <Card>
              暂无相关任务。已取消的任务已收起。
              <Button
                className="mt-3 block"
                onClick={() => useUI.getState().setPage("inventory")}
              >
                选食材，获取建议
              </Button>
            </Card>
          )}
        </Load>
      }
    </>
  );
}

export function JobCard({
  id,
  onHidden,
}: {
  id: string;
  onHidden: () => void;
}) {
  const { token, p, q } = useBase();
  const household = useData<{ role: string }>("household", p);
  const d = useData<Job>("job", p + "/jobs/" + id);
  const [err, setErr] = useState("");
  async function cancel() {
    try {
      await api(p + "/jobs/" + id + "/cancel", token, "POST", {});
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  async function retry() {
    try {
      await api(p + "/jobs/" + id + "/retry", token, "POST", {});
      q.invalidateQueries();
    } catch (e) {
      setErr(String(e));
    }
  }
  async function adopt(recipeID: string) {
    setErr("");
    try {
      await api(p + "/plans", token, "POST", {
        meals: [
          {
            day: new Date().toLocaleDateString("en-CA"),
            meal: "dinner",
            servings: d.data?.result?.servings || 2,
            recipe_id: recipeID,
          },
        ],
      });
      await q.invalidateQueries();
      useUI.getState().setPage("week");
    } catch (e) {
      setErr(String(e));
    }
  }
  const j = d.data;
  useEffect(() => {
    if (j?.status !== "cancelled") return;
    const timer = setTimeout(onHidden, 450);
    return () => clearTimeout(timer);
  }, [j?.status, onHidden]);
  if (d.error)
    return (
      <Card>
        <ErrorLine error={d.error} />
        <Button onClick={() => d.refetch()}>重试任务读取</Button>
      </Card>
    );
  if (!j) return <Card>加载任务…</Card>;
  return (
    <Card
      className={j.status === "cancelled" ? "cancelled-task-exit" : undefined}
    >
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">
          {j.kind === "image"
            ? "图片识别"
            : j.kind === "advice"
              ? "食材营养建议"
              : "菜单规划"}{" "}
          · {statusLabel(j.status)}
        </h2>
        <span className="text-xs text-slate-500">尝试 {j.attempts} 次</span>
      </div>
      <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100">
        <div
          className="h-full bg-emerald-600"
          style={{ width: j.progress + "%" }}
        />
      </div>
      {j.kind !== "advice" && j.result?.mode === "demo" && (
        <Notice>演示模式：未调用真实模型，使用确定性菜谱选择。</Notice>
      )}
      {j.kind === "advice" && j.status === "succeeded" && j.result?.summary && (
        <AdviceView
          result={j.result as AdviceResult}
          onChoose={
            household.data && household.data.role !== "viewer"
              ? adopt
              : undefined
          }
        />
      )}
      {j.kind === "image" &&
        j.status === "awaiting_confirmation" &&
        j.result?.name && (
          <ImageReview
            token={token}
            household={p.slice("/households/".length)}
            job={id}
            name={j.result.name}
            category={j.result.category || ""}
          />
        )}
      {j.error && (
        <div className="mt-3">
          <Notice tone="error">{j.error}</Notice>
        </div>
      )}
      {j.status === "awaiting_confirmation" && j.result?.plan_id && (
        <div className="mt-3">
          <Notice tone="success">方案已生成。到“安排菜单”查看并确认。</Notice>
          <Button
            size="sm"
            className="mt-2"
            onClick={() => useUI.getState().setPage("week")}
          >
            查看方案
          </Button>
        </div>
      )}
      {(["queued", "running"].includes(j.status) ||
        (j.kind === "image" && j.status === "awaiting_confirmation")) && (
        <Button size="sm" variant="outline" className="mt-3" onClick={cancel}>
          {j.kind === "image" && j.status === "awaiting_confirmation"
            ? "拒绝识别结果"
            : "取消任务"}
        </Button>
      )}
      {j.status === "failed" && (
        <Button size="sm" variant="outline" className="mt-3" onClick={retry}>
          手动重试（可能再次调用模型）
        </Button>
      )}
      {err && <ErrorLine error={err} />}
    </Card>
  );
}
