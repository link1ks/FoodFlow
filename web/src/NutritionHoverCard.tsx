import { useEffect, useRef, useState, type ReactNode } from "react";
import { Info, Leaf, X } from "lucide-react";
import { Card } from "./ui";
import type { NutritionProfile } from "./Nutrition";

const nutrients = [
  ["energy_kcal", "能量", "kcal"],
  ["protein_g", "蛋白质", "g"],
  ["fat_g", "脂肪", "g"],
  ["carbs_g", "碳水", "g"],
  ["fiber_g", "膳食纤维", "g"],
  ["sodium_mg", "钠", "mg"],
];

export function NutritionHoverCard({
  name,
  profile,
  className,
  children,
}: {
  name: string;
  profile?: NutritionProfile;
  className: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const pinned = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  function clearTimer() {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  }
  function close() {
    pinned.current = false;
    clearTimer();
    setOpen(false);
  }
  useEffect(() => () => clearTimer(), []);
  const reference = profile?.status === "reference";
  return (
    <Card
      className={className}
      onPointerEnter={(e) => {
        if (e.pointerType === "mouse" && !pinned.current) {
          clearTimer();
          timer.current = setTimeout(() => setOpen(true), 3000);
        }
      }}
      onPointerLeave={(e) => {
        if (e.pointerType === "mouse") {
          clearTimer();
          if (!pinned.current) close();
        }
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.stopPropagation();
          close();
        }
      }}
    >
      {children}
      <button
        type="button"
        className="ff-nutrition-trigger"
        aria-label={`查看${name}营养价值`}
        aria-expanded={open}
        onClick={() => {
          clearTimer();
          pinned.current = !open;
          setOpen(!open);
        }}
      >
        <Info size={16} aria-hidden="true" />
      </button>
      {open && (
        <section className="ff-nutrition-glass" aria-label={`${name}营养价值`}>
          <div className="ff-nutrition-heading">
            <span>
              <Leaf size={15} aria-hidden="true" /> 食物小档案
            </span>
            <button type="button" aria-label="关闭营养信息" onClick={close}>
              <X size={17} aria-hidden="true" />
            </button>
          </div>
          <h4>{name}</h4>
          <p className="ff-nutrition-basis">
            {reference ? "每 100 克 · 参考营养" : "暂无已核验数值"}
          </p>
          {reference ? (
            <div className="ff-nutrition-grid">
              {nutrients.map(([key, label, unit]) => (
                <div key={key}>
                  <span>{label}</span>
                  <strong>
                    {profile.nutrients[key] == null
                      ? "—"
                      : Number(profile.nutrients[key])}
                    <small>
                      {profile.nutrients[key] == null ? "未提供" : unit}
                    </small>
                  </strong>
                </div>
              ))}
            </div>
          ) : (
            <p className="ff-nutrition-empty">请参考具体食材或包装营养标签。</p>
          )}
          <p className="ff-nutrition-note">
            {reference
              ? "参考值不等于当前库存总量；品种、生熟及品牌会影响数值。"
              : profile?.note || "暂未匹配参考资料，不估算营养数值。"}
          </p>
          {reference && (
            <a
              href={profile.source_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              {profile.source} ↗
            </a>
          )}
        </section>
      )}
    </Card>
  );
}
