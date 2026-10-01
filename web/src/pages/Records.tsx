import { useState } from "react";
import { PageSwitch } from "../GettingStarted";
import { NutritionRadar } from "../NutritionRadar";
import { Title, useBase } from "../app/shared";
import { KitchenInsights } from "./KitchenInsights";
import { StockHistory } from "./StockHistory";
import { Jobs } from "./Jobs";
import { MonthlyReport } from "./MonthlyReport";

export function Records() {
  const [view, setView] = useState("nutrition");
  const { p, token } = useBase();
  return (
    <>
      <Title title="厨房记录" subtitle="回顾家庭营养、食材用量和生成记录" />
      <PageSwitch
        value={view}
        onChange={setView}
        options={[
          {
            id: "monthly",
            title: "金额月报",
            description: "查看成本覆盖、已知报损金额并导出图片",
          },
          {
            id: "nutrition",
            title: "营养记录",
            description: "已完成餐次的营养估算与数据覆盖率",
          },
          {
            id: "usage",
            title: "食材用量",
            description: "按原单位查看买入、消耗与报损数量",
          },
          {
            id: "history",
            title: "库存流水",
            description: "追溯入库、扣减、报损与校准",
          },
          {
            id: "activity",
            title: "生成记录",
            description: "找回菜单、搭配建议和识别任务",
          },
        ]}
      />
      {view === "monthly" ? (
        <MonthlyReport />
      ) : view === "nutrition" ? (
        <NutritionRadar root={p} token={token} />
      ) : view === "usage" ? (
        <KitchenInsights />
      ) : view === "history" ? (
        <StockHistory />
      ) : (
        <Jobs />
      )}
    </>
  );
}
