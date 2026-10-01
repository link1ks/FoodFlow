import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { Card, Button, Notice } from "../ui";
import { ErrorLine, useBase } from "../app/shared";
import {
  exportMonthlyReport,
  monthlyQuantity,
  REPORT_PAGE_SIZE,
  type MonthlyReport as Report,
} from "../monthlyReport";

export function MonthlyReport() {
  const { p, token } = useBase();
  const [month, setMonth] = useState(""),
    [page, setPage] = useState(0),
    [error, setError] = useState(""),
    [exporting, setExporting] = useState(false);
  const query = useQuery({
    queryKey: ["monthly-report", p, month],
    queryFn: () =>
      api<Report>(
        p +
          "/monthly-report" +
          (month ? "?month=" + encodeURIComponent(month) : ""),
        token,
      ),
  });
  async function download() {
    if (!query.data) return;
    setExporting(true);
    setError("");
    try {
      await exportMonthlyReport(query.data, page);
    } catch (e) {
      setError(String(e));
    } finally {
      setExporting(false);
    }
  }
  const report = query.data,
    s = report?.summary;
  const pages = Math.max(
    1,
    Math.ceil((report?.items.length ?? 0) / REPORT_PAGE_SIZE),
  );
  return (
    <>
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <label className="text-sm">
          月报月份{" "}
          <input
            aria-label="月报月份"
            type="month"
            className="ml-2 rounded-lg border p-2"
            value={month}
            onChange={(e) => {
              setMonth(e.target.value);
              setPage(0);
            }}
          />
        </label>
        <Button variant="outline" onClick={() => query.refetch()}>
          刷新月报
        </Button>
        {pages > 1 && (
          <label className="text-sm">
            导出页{" "}
            <select
              aria-label="月报导出页"
              value={page}
              onChange={(e) => setPage(Number(e.target.value))}
              className="ml-2 rounded-lg border p-2"
            >
              {Array.from({ length: pages }, (_, i) => (
                <option key={i} value={i}>
                  第 {i + 1} / {pages} 页
                </option>
              ))}
            </select>
          </label>
        )}
        <Button
          disabled={!report || query.isFetching || exporting}
          onClick={download}
        >
          {exporting ? "生成图片中…" : "导出月报图片"}
        </Button>
      </div>
      {error && <Notice tone="error">{error}</Notice>}
      {query.isPending ? (
        <Card>正在整理月报…</Card>
      ) : query.isError ? (
        <Card>
          <ErrorLine error={query.error} />
        </Card>
      ) : (
        report &&
        s && (
          <>
            <p className="mb-3 text-sm text-slate-500">
              {report.month} · {report.timezone} · 数据截至{" "}
              {new Date(report.as_of).toLocaleString()}
            </p>
            <Card>
              <h2 className="font-semibold">家庭金额月报</h2>
              <div className="my-4 grid gap-3 sm:grid-cols-3">
                <p>
                  本月记录采购金额
                  <br />
                  <strong>¥{s.recorded_purchase_cost}</strong> ·{" "}
                  {s.purchase_records} 笔
                </p>
                <p>
                  已知消耗成本
                  <br />
                  <strong>¥{s.known_consumed_cost}</strong>
                </p>
                <p>
                  已知报损成本
                  <br />
                  <strong>¥{s.known_wasted_cost}</strong>
                </p>
              </div>
              <p className="text-sm">
                出库成本覆盖 {s.priced_outbound_events}/{s.outbound_events}{" "}
                条；未知成本 {s.unknown_outbound_events} 条；估算扣减{" "}
                {s.estimated_outbound_events} 条。
              </p>
              <p className="mt-3 text-xs text-slate-500">
                采购金额按成本记录日期统计。消耗和报损使用当时的成本快照；后来补记采购金额也不改写历史出库。未知成本保持未知，已知小计不等于完整支出或节省金额。调味品包含用量估算。
              </p>
            </Card>
            <p className="my-4 text-sm text-slate-500">
              明细 {report.items.length}/{report.item_count}{" "}
              组；不同单位分别展示。每张导出图片最多 {REPORT_PAGE_SIZE} 组
              {report.item_count > report.items.length
                ? "；明细超过展示上限，总计仍包含全部记录"
                : ""}
              。
            </p>
            <div className="grid gap-3 md:grid-cols-2">
              {report.items.map((x) => (
                <Card key={x.ingredient + ":" + x.unit}>
                  <h3 className="font-semibold">
                    {x.ingredient} · {x.unit ?? "原单位未知"}
                  </h3>
                  <p className="mt-2 text-sm">
                    入库 {monthlyQuantity(x.inbound_milli, x.unit)} · 消耗{" "}
                    {monthlyQuantity(x.consumed_milli, x.unit)} · 报损{" "}
                    {monthlyQuantity(x.wasted_milli, x.unit)}
                  </p>
                  <p className="mt-2 text-sm">
                    已知消耗 ¥{x.known_consumed_cost} · 已知报损 ¥
                    {x.known_wasted_cost} · 未知成本 {x.unknown_outbound_events}{" "}
                    条
                  </p>
                </Card>
              ))}
            </div>
          </>
        )
      )}
    </>
  );
}
