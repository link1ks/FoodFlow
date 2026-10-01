import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { useBase, Title, ErrorLine } from "../app/shared";
import { Capabilities } from "../app/Capabilities";
import { useData } from "../app/shared";
import { Card, Button } from "../ui";
type Row = {
  ingredient: string;
  category: string;
  unit: string;
  inbound_milli: string;
  consumed_milli: string;
  wasted_milli: string;
  adjusted_milli: string;
};
type Summary = {
  month: string;
  timezone: string;
  items: Row[];
  last_received_at: string | null;
};
// Exact integer quantities remain strings; rendering never passes through float.
function quantity(milli: string) {
  const n = BigInt(milli),
    negative = n < 0n,
    a = negative ? -n : n;
  const fraction = (a % 1000n).toString().padStart(3, "0").replace(/0+$/, "");
  return `${negative ? "-" : ""}${a / 1000n}${fraction ? "." + fraction : ""}`;
}
export function KitchenInsights() {
  const { p, token } = useBase();
  const capabilities = useData<Capabilities>("capabilities", "/capabilities");
  const [month, setMonth] = useState("");
  const query = useQuery({
    queryKey: ["kitchen-insights", p, month],
    queryFn: () =>
      api<Summary>(`${p}/insights${month ? `?month=${month}` : ""}`, token),
    enabled: capabilities.data?.insights_enabled === true,
    refetchInterval: 10000,
    retry: 1,
  });
  if (capabilities.isLoading) return <Card>正在读取统计能力…</Card>;
  if (capabilities.error)
    return (
      <Card>
        <ErrorLine error={capabilities.error} />
        <Button onClick={() => capabilities.refetch()}>重试</Button>
      </Card>
    );
  if (!capabilities.data?.insights_enabled)
    return (
      <Card>
        <h2 className="font-semibold">食材用量尚未启用</h2>
        <p className="mt-2 text-sm text-slate-500">
          当前环境暂不提供数量汇总。库存、采购和营养记录仍可使用。
        </p>
      </Card>
    );
  return (
    <>
      <Title
        title="食材用量"
        subtitle="这个月买了多少、吃了多少、丢掉多少，一眼看清"
      />
      <div className="mb-4 flex items-center gap-3">
        <label className="text-sm text-slate-600">
          月份{" "}
          <input
            aria-label="统计月份"
            className="ml-2 rounded-lg border bg-white p-2"
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
          />
        </label>
        <Button onClick={() => query.refetch()}>刷新</Button>
      </div>
      {query.isPending ? (
        <Card>正在整理厨房记录…</Card>
      ) : query.isError ? (
        <Card>
          <ErrorLine error={query.error} />
          <Button onClick={() => query.refetch()}>重试</Button>
        </Card>
      ) : (
        <>
          <p className="mb-4 text-sm text-slate-500">
            {query.data.month} · {query.data.timezone} ·{" "}
            {query.data.last_received_at
              ? `最近同步 ${new Date(query.data.last_received_at).toLocaleString()}`
              : "尚无同步记录"}
            。新记录可能稍后显示。
          </p>
          {query.data.items.length === 0 ? (
            <Card>
              本月暂无用量记录。添加食材或完成做饭后，会自动汇总在这里。
            </Card>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {query.data.items.map((row) => (
                <Card key={row.ingredient + row.unit}>
                  <div className="mb-4 flex justify-between">
                    <h2 className="font-semibold">{row.ingredient}</h2>
                    <span className="text-sm text-slate-500">
                      {row.category} · {row.unit}
                    </span>
                  </div>
                  <div className="grid grid-cols-3 gap-3">
                    {[
                      ["买入", row.inbound_milli],
                      ["吃掉", row.consumed_milli],
                      ["报损", row.wasted_milli],
                    ].map(([label, value]) => (
                      <div key={label} className="rounded-xl bg-emerald-50 p-3">
                        <p className="text-xs text-slate-500">{label}</p>
                        <p className="mt-1 font-semibold">
                          {quantity(value)} <small>{row.unit}</small>
                        </p>
                      </div>
                    ))}
                  </div>
                  {row.adjusted_milli !== "0" && (
                    <p className="mt-3 text-sm text-slate-500">
                      余量校准：{quantity(row.adjusted_milli)} {row.unit}
                    </p>
                  )}
                </Card>
              ))}
            </div>
          )}
          <p className="mt-5 text-xs text-slate-500">
            按食材和原单位分别统计，来源为有历史快照的库存流水。
          </p>
        </>
      )}
    </>
  );
}
