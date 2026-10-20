import { useEffect, useId, useState } from "react";
import { Quantity } from "./Quantity";

const styles: Record<string, { color: string; jar?: boolean }> = {
  食用油: { color: "#dba928" },
  花生油: { color: "#dba928" },
  菜籽油: { color: "#dba928" },
  橄榄油: { color: "#a4a340" },
  香油: { color: "#b87b24" },
  酱油: { color: "#6f3926" },
  生抽: { color: "#6f3926" },
  老抽: { color: "#492c24" },
  醋: { color: "#8c4930" },
  陈醋: { color: "#8c4930" },
  米醋: { color: "#ceae69" },
  料酒: { color: "#cba75c" },
  蚝油: { color: "#633b27" },
  食盐: { color: "#d9dfd6", jar: true },
  白糖: { color: "#e0dac6", jar: true },
  白砂糖: { color: "#e0dac6", jar: true },
  胡椒粉: { color: "#aa9175", jar: true },
  黑胡椒: { color: "#74604d", jar: true },
  淀粉: { color: "#ded6c1", jar: true },
  鸡精: { color: "#dbc47a", jar: true },
  味精: { color: "#deded0", jar: true },
};
export const condimentStyle = (name: string) => styles[name];

// This is an amount illustration, not a measurement of physical liquid volume.
export function CondimentContainer({
  name,
  quantity,
  capacity,
  unit,
  basis,
  status,
}: {
  name: string;
  quantity: string;
  capacity: string;
  unit: string;
  basis: string;
  status?: string;
}) {
  const id = useId();
  const style = condimentStyle(name) || { color: "#82986d" };
  const percent =
    Number(capacity) > 0
      ? Math.max(
          0,
          Math.min(
            100,
            Math.round((Number(quantity) / Number(capacity)) * 100),
          ),
        )
      : null;
  const [shownPercent, setShownPercent] = useState(0);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setShownPercent(percent ?? 0));
    return () => cancelAnimationFrame(frame);
  }, [percent]);
  const path = style.jar
    ? "M35 36 H105 V140 Q105 150 95 150 H45 Q35 150 35 140 Z"
    : "M54 24 H86 V43 Q86 49 100 60 V140 Q100 150 90 150 H50 Q40 150 40 140 V60 Q54 49 54 43 Z";
  return (
    <div
      className="ff-condiment"
      role="img"
      aria-label={
        status
          ? `${name}容器示意，${status}`
          : `${name}容器示意，剩余 ${quantity} ${unit}，${basis} ${capacity} ${unit}${percent === null ? "，比例未知" : `，${percent}%`}`
      }
    >
      <svg viewBox="0 0 140 170" aria-hidden="true">
        <defs>
          <clipPath id={id}>
            <path d={path} />
          </clipPath>
        </defs>
        <ellipse cx="70" cy="156" rx="40" ry="6" fill="#665128" opacity=".1" />
        <path d={path} fill="#fffdf3" stroke="#9eaa96" strokeWidth="2" />
        {percent !== null && (
          <g clipPath={`url(#${id})`}>
            <rect
              x="30"
              className="ff-container-fill"
              y="38"
              style={{
                transform: `translateY(${112 * (1 - shownPercent / 100)}px)`,
              }}
              width="80"
              height="112"
              fill={style.color}
              opacity=".85"
            />
            <path
              d="M48 50 V135"
              stroke="white"
              strokeWidth="5"
              opacity=".5"
              strokeLinecap="round"
            />
          </g>
        )}
        <rect
          x={style.jar ? 32 : 51}
          y={style.jar ? 25 : 15}
          width={style.jar ? 76 : 38}
          height="14"
          rx="4"
          fill="#66745b"
        />
        <rect
          x="44"
          y="82"
          width="52"
          height="32"
          rx="9"
          fill="#fffdf1"
          opacity=".94"
        />
        <text
          x="70"
          y="103"
          textAnchor="middle"
          fill="#394b32"
          fontSize="16"
          fontWeight="700"
        >
          {percent === null ? "—" : `${percent}%`}
        </text>
      </svg>
      <span>{status || <Quantity value={quantity} unit={unit} />}</span>
      <small>
        {status ? (
          "容量待设置"
        ) : (
          <>
            {basis} <Quantity value={capacity} unit={unit} />
          </>
        )}
      </small>
    </div>
  );
}
