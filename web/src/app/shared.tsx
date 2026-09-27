import { QueryClient, useQuery, useQueryClient } from "@tanstack/react-query";
import React from "react";
import { api } from "../api";
import { type InventoryBatch } from "../BatchActions";
import { type NutritionProfile } from "../Nutrition";
import { type AdviceResult } from "../NutritionAdvice";
import { useUI } from "../store";
import { Notice } from "../ui";
export type Household = {
  id: string;
  name: string;
  role: string;
  servings: number;
};

export type Ingredient = {
  id: string;
  name: string;
  category: string;
  unit: string;
  quantity: string;
  low: string;
  low_stock: boolean;
  nutrition?: NutritionProfile;
  has_image: boolean;
  image_version: number;
};

export type Batch = InventoryBatch;

export type Inventory = { items: Ingredient[]; batches: Batch[] };

export type Recipe = {
  id: string;
  title: string;
  minutes: number;
  servings: number;
  tags: string[];
  source: string;
  ingredients: { name: string; quantity: string; unit: string }[];
};

export type Meal = {
  id: string;
  day: string;
  meal: string;
  servings: number;
  recipe_id: string;
  title: string;
  minutes: number;
  additional_recipes: { id: string; title: string; minutes: number }[];
};

export type Demand = {
  name: string;
  unit: string;
  needed: string;
  available: string;
  shortage: string;
  conversion_needs_confirmation: boolean;
};

export type Plan = {
  id: string;
  status: string;
  revision: number;
  meals: Meal[];
  ingredients: Demand[];
  inventory_rechecked_at: string;
};

export type Job = {
  id: string;
  kind: string;
  status: string;
  progress: number;
  attempts: number;
  error: string;
  result?: {
    plan_id?: string;
    mode?: string;
    name?: string;
    category?: string;
  } & Partial<AdviceResult>;
};

export const qc = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: true, staleTime: 5000 },
  },
});

export function useBase() {
  const token = useUI((s) => s.token),
    house = useUI((s) => s.household);
  return { token, house, p: `/households/${house}`, q: useQueryClient() };
}

export function useData<T>(key: string, path: string, enabled = true) {
  const { token } = useBase();
  return useQuery({
    queryKey: [key, path],
    queryFn: () => api<T>(path, token),
    enabled: !!token && enabled,
  });
}

export function ErrorLine({ error }: { error: unknown }) {
  return error ? (
    <Notice tone="error">
      {error instanceof Error ? error.message : String(error)}
    </Notice>
  ) : null;
}

export function Load({
  children,
  loading,
  error,
}: {
  children: React.ReactNode;
  loading: boolean;
  error: unknown;
}) {
  if (loading)
    return <div className="py-12 text-center text-slate-500">正在加载…</div>;
  if (error) return <ErrorLine error={error} />;
  return <>{children}</>;
}

export function Title({
  title,
  subtitle,
}: {
  title: string;
  subtitle: string;
}) {
  return (
    <div className="mb-5">
      <h1 className="text-2xl font-bold text-slate-900 md:text-3xl">{title}</h1>
      <p className="mt-1 text-sm text-slate-500">{subtitle}</p>
    </div>
  );
}

export const statusLabel = (s: string) =>
  (
    ({
      queued: "排队中",
      running: "运行中",
      awaiting_confirmation: "等待确认",
      succeeded: "已完成",
      failed: "失败",
      cancelled: "已取消",
    }) as Record<string, string>
  )[s] || s;
