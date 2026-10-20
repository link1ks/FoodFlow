import type { components } from "./generated/api";
import { freshness } from "./BatchActions";
import { Quantity } from "./Quantity";

type Batch = components["schemas"]["InventoryBatch"];
export const isUndatedStock = (b: Batch) =>
  Number(b.quantity) > 0 && b.condition === "normal" && !b.expires_at;

export function StockCardSummary({
  name,
  unit,
  batches,
}: {
  name: string;
  unit: string;
  batches: Batch[];
}) {
  const active = batches.filter((b) => Number(b.quantity) > 0);
  return (
    <div className="ff-stock-summary" aria-label={`${name}期限与用量`}>
      {active.length === 0 ? (
        <p className="ff-stock-summary-empty">已用完</p>
      ) : (
        active.map((b, index) => {
          const remaining = Number(b.quantity);
          const stocked = Number(b.stocked_quantity || 0);
          const percent =
            stocked > 0
              ? Math.min(
                  100,
                  Math.max(0, Math.round((remaining / stocked) * 100)),
                )
              : null;
          const life = freshness(b);
          const hasTimeline =
            Number.isFinite(Date.parse(b.expires_at)) &&
            Number.isFinite(Date.parse(b.bought_on)) &&
            Date.parse(b.expires_at) > Date.parse(b.bought_on);
          const expired =
            !!b.expires_at && Date.parse(b.expires_at) <= Date.now();
          return (
            <div key={b.id} className="ff-stock-batch-summary">
              <div className="ff-stock-summary-line">
                <span>
                  {active.length > 1 ? `第 ${index + 1} 批 · ` : ""}到期
                </span>
                <strong>
                  {b.expires_on
                    ? `${b.expiry_kind === "estimate" ? "预计 " : ""}${b.expires_on}`
                    : "未设期限"}
                </strong>
              </div>
              <div
                className={`ff-stock-expiry-track ff-expiry-${life.tone}`}
                role={hasTimeline ? "meter" : undefined}
                aria-label={`${name}第${index + 1}批有效期剩余比例`}
                aria-valuemin={hasTimeline ? 0 : undefined}
                aria-valuemax={hasTimeline ? 100 : undefined}
                aria-valuenow={hasTimeline ? life.pct : undefined}
                aria-valuetext={hasTimeline ? life.label : undefined}
              >
                {hasTimeline ? (
                  <span style={{ width: `${life.pct}%` }} />
                ) : (
                  <span className="ff-expiry-unknown" />
                )}
              </div>
              <p className="ff-expiry-caption">
                {!b.expires_at
                  ? "未设期限 · 优先处理"
                  : !hasTimeline
                    ? `${life.label} · 无起始日期`
                    : life.label}
              </p>
              <div className="ff-stock-summary-line">
                <span>
                  {b.condition === "spoiled"
                    ? "变质待处理"
                    : expired
                      ? "过期待处理"
                      : "剩余"}
                </span>
                <strong>
                  <Quantity value={b.quantity} unit={unit} />
                  {percent !== null && <small> · {percent}%</small>}
                </strong>
              </div>
              {percent !== null && (
                <div
                  className="ff-stock-amount-track"
                  role="meter"
                  aria-label={`${name}第${index + 1}批剩余比例`}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={percent}
                  aria-valuetext={`剩余 ${b.quantity} ${unit}，累计入库及调增 ${b.stocked_quantity} ${unit}`}
                >
                  <span style={{ width: `${percent}%` }} />
                </div>
              )}
            </div>
          );
        })
      )}
    </div>
  );
}
