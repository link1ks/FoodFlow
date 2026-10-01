import { ErrorLine, useBase, useData } from "../app/shared";
import { Button, Card } from "../ui";
import { Quantity } from "../Quantity";

export function StockHistory() {
  const { p } = useBase();
  const rows = useData<
    {
      id: string;
      ingredient: string;
      unit: string;
      quantity: string;
      reason: string;
      created_at: string;
    }[]
  >("ledger", p + "/ledger");
  const reasons: Record<string, string> = {
    purchase: "采购入库",
    manual: "手工入库",
    consume: "烹饪消耗",
    waste: "报损",
    correction: "余量校准",
  };
  if (rows.isLoading) return <Card>正在读取库存流水…</Card>;
  if (rows.error)
    return (
      <Card>
        <ErrorLine error={rows.error} />
        <Button onClick={() => rows.refetch()}>重试</Button>
      </Card>
    );
  return (
    <>
      <p className="mb-3 text-xs text-slate-500">
        最近 100 条库存变动，名称与单位按历史快照展示。
      </p>
      {rows.data?.length ? (
        <div className="space-y-3">
          {rows.data.map((row) => (
            <Card key={row.id}>
              <div className="flex justify-between gap-3">
                <div>
                  <h2 className="font-semibold">
                    {row.ingredient || "历史食材（缺少快照）"}
                  </h2>
                  <p className="text-sm text-slate-500">
                    {reasons[row.reason] || row.reason} ·{" "}
                    {new Date(row.created_at).toLocaleString()}
                  </p>
                </div>
                <div>
                  <Quantity value={row.quantity} unit={row.unit || undefined} />
                  {!row.unit && (
                    <p className="text-xs text-slate-500">历史单位未知</p>
                  )}
                </div>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <Card>暂无库存变动记录。</Card>
      )}
    </>
  );
}
