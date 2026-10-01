export type MonthlyItem = {
  ingredient: string;
  unit: string | null;
  inbound_milli: string;
  consumed_milli: string;
  wasted_milli: string;
  adjusted_milli: string;
  known_consumed_cost: string;
  known_wasted_cost: string;
  unknown_outbound_events: number;
};
export type MonthlyReport = {
  month: string;
  timezone: string;
  currency: "CNY";
  as_of: string;
  summary: {
    outbound_events: number;
    priced_outbound_events: number;
    unknown_outbound_events: number;
    estimated_outbound_events: number;
    known_consumed_cost: string;
    known_wasted_cost: string;
    purchase_records: number;
    recorded_purchase_cost: string;
  };
  item_count: number;
  items: MonthlyItem[];
  items_limit: number;
  cost_basis: "immutable_purchase_allocations";
  purchase_period_basis: "cost_recorded_at";
  historical_outbound_repriced: false;
};
export const REPORT_PAGE_SIZE = 40;
export function monthlyQuantity(milli: string, unit: string | null): string {
  const n = BigInt(milli),
    a = n < 0n ? -n : n;
  const fraction = (a % 1000n).toString().padStart(3, "0").replace(/0+$/, "");
  return `${n < 0n ? "-" : ""}${a / 1000n}${fraction ? "." + fraction : ""} ${unit ?? "原单位未知"}`;
}
export function reportLines(report: MonthlyReport, page: number): string[] {
  const pages = Math.max(1, Math.ceil(report.items.length / REPORT_PAGE_SIZE));
  if (!Number.isInteger(page) || page < 0 || page >= pages)
    throw new Error("导出页无效");
  const s = report.summary;
  const items = report.items.slice(
    page * REPORT_PAGE_SIZE,
    (page + 1) * REPORT_PAGE_SIZE,
  );
  return [
    `食光 FoodFlow · ${report.month} 家庭月报`,
    `家庭时区：${report.timezone} · 第 ${page + 1}/${pages} 页`,
    `数据截至：${report.as_of}`,
    `本月记录采购金额：¥${s.recorded_purchase_cost}（${s.purchase_records} 笔）`,
    `已知消耗成本：¥${s.known_consumed_cost} · 已知报损成本：¥${s.known_wasted_cost}`,
    `出库成本覆盖：${s.priced_outbound_events}/${s.outbound_events} 条 · 未知成本 ${s.unknown_outbound_events} 条`,
    `估算扣减：${s.estimated_outbound_events} 条（含调味品估算）`,
    "采购金额按成本记录日期；出库成本为当时不可变快照。",
    "未记录的成本保持未知；已知小计不等于完整支出或节省金额。",
    `月内明细：${report.items.length}/${report.item_count} 组；按历史名称和单位分组。`,
    ...(report.item_count > report.items.length
      ? ["明细超过接口展示上限，总计仍包含全部记录。"]
      : []),
    ...items.flatMap((x) => [
      `${x.ingredient} · ${x.unit ?? "原单位未知"}`,
      `入库 ${monthlyQuantity(x.inbound_milli, x.unit)} · 消耗 ${monthlyQuantity(x.consumed_milli, x.unit)} · 报损 ${monthlyQuantity(x.wasted_milli, x.unit)}`,
      `校准 ${monthlyQuantity(x.adjusted_milli, x.unit)} · 已知消耗 ¥${x.known_consumed_cost} · 已知报损 ¥${x.known_wasted_cost} · 未知成本 ${x.unknown_outbound_events} 条`,
    ]),
  ];
}
export async function exportMonthlyReport(report: MonthlyReport, page: number) {
  await document.fonts.ready;
  const canvas = document.createElement("canvas");
  canvas.width = 1000;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("当前浏览器无法生成图片");
  ctx.font = "26px sans-serif";
  const wrapped: string[] = [];
  for (const line of reportLines(report, page)) {
    let current = "";
    for (const character of line) {
      if (current && ctx.measureText(current + character).width > 900) {
        wrapped.push(current);
        current = "";
      }
      current += character;
    }
    wrapped.push(current);
  }
  canvas.height = 100 + wrapped.length * 42;
  ctx.fillStyle = "#f8fafc";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.font = "26px sans-serif";
  ctx.fillStyle = "#0f172a";
  wrapped.forEach((line, i) => ctx.fillText(line, 50, 60 + i * 42));
  const blob = await new Promise<Blob>((resolve, reject) =>
    canvas.toBlob(
      (value) =>
        value ? resolve(value) : reject(new Error("图片生成失败，请重试")),
      "image/png",
    ),
  );
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `foodflow-month-${report.month}-${page + 1}.png`;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
