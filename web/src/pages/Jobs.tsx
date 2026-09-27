import { useEffect, useState } from "react";
import { PageSwitch } from "../GettingStarted";
import { ImageReview } from "../ImageRecognition";
import { AdviceView, type AdviceResult } from "../NutritionAdvice";
import { NutritionRadar } from "../NutritionRadar";
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
export function Jobs() {
  const { p, token } = useBase();
  const [view, setView] = useState("results");
  const jobs = useData<{ id: string }[]>("jobs", p + "/jobs");
  const [hidden, setHidden] = useState<string[]>([]);
  const visible = jobs.data?.filter((j) => !hidden.includes(j.id));
  return (
    <>
      <Title
        title="营养与建议"
        subtitle="查看 AI 为你生成的方案，或回顾家里最近吃得怎么样"
      />
      <PageSwitch
        value={view}
        onChange={setView}
        options={[
          {
            id: "results",
            title: "AI 建议与识别结果",
            description: "菜单生成、食材搭配建议和拍照识别都在这里",
          },
          {
            id: "radar",
            title: "家庭营养记录",
            description: "根据已完成的餐次，查看近7天和30天营养估算",
          },
        ]}
      />
      {view === "radar" ? (
        <NutritionRadar root={p} token={token} />
      ) : (
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
              暂无规划记录。已取消的任务已收起，可从食材库存发起新规划。
              <Button
                className="mt-3 block"
                onClick={() => useUI.getState().setPage("inventory")}
              >
                选食材，获取建议
              </Button>
            </Card>
          )}
        </Load>
      )}
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
  const j = d.data;
  useEffect(() => {
    if (j?.status !== "cancelled") return;
    const timer = setTimeout(onHidden, 450);
    return () => clearTimeout(timer);
  }, [j?.status, onHidden]);
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
        <AdviceView result={j.result as AdviceResult} />
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
