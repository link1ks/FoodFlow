import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idem } from "./api";
import { formatQuantity } from "./Quantity";
import { Button, Field, Notice } from "./ui";

import type { components } from "./generated/api";
type Basis = components["schemas"]["NutritionBasis"];
export function BatchNutrition({
  batch,
  name,
  unit,
  token,
  household,
}: {
  batch: string;
  name: string;
  unit: string;
  token: string;
  household: string;
}) {
  const q = useQueryClient(),
    [open, setOpen] = useState(false),
    [quantity, setQuantity] = useState(""),
    [grams, setGrams] = useState(""),
    [classification, setClassification] = useState("catalog"),
    [confirm, setConfirm] = useState(false),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [saved, setSaved] = useState(false);
  const path = `/households/${household}/batches/${batch}/nutrition-basis`,
    basis = useQuery({
      queryKey: ["nutrition-basis", household, batch],
      queryFn: () => api<Basis>(path, token),
      enabled: open,
    });
  const pending = useRef<{ body: string; key: string } | undefined>(undefined);
  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved(false);
    const body = {
        quantity,
        edible_grams: grams,
        classification,
        expected_revision: basis.data?.revision,
        confirm,
      },
      encoded = JSON.stringify(body);
    if (pending.current?.body !== encoded)
      pending.current = { body: encoded, key: idem() };
    try {
      await api(path, token, "POST", body, pending.current!.key);
      setConfirm(false);
      setSaved(true);
      await q.invalidateQueries();
    } catch (e) {
      setError(String(e));
      await basis.refetch();
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="mt-2">
      <button
        type="button"
        className="text-xs text-slate-500 hover:text-emerald-800"
        aria-expanded={open}
        aria-label={name + "营养换算"}
        onClick={() => setOpen(!open)}
      >
        营养换算 · 可食重量
      </button>
      {open && (
        <div className="mt-2 rounded-xl bg-slate-50 p-3">
          {basis.isPending ? (
            <p>加载中…</p>
          ) : basis.error ? (
            <Notice tone="error">
              读取失败 <button onClick={() => basis.refetch()}>重试</button>
            </Notice>
          ) : (
            <>
              <p className="mb-2 text-xs text-slate-500">
                称量本批有代表性的原料，去除壳、骨等不可食部分后记录净重。换算仅用于此批之后的营养估算；库存单位、余量与旧餐次保持原记录。蔬菜分类可保留目录值或明确标记未知。
              </p>
              {basis.data?.recorded && (
                <p className="mb-2">
                  当前依据：{formatQuantity(basis.data.quantity!)} {unit} 对应{" "}
                  {formatQuantity(basis.data.edible_grams!)} g 可食部 · 版本{" "}
                  {basis.data.revision}
                </p>
              )}
              <form className="space-y-3" onSubmit={save}>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field
                    label={"称量样本数量（" + unit + "）"}
                    type="number"
                    min="0.001"
                    step="0.001"
                    required
                    value={quantity}
                    onChange={(e) => setQuantity(e.target.value)}
                  />
                  <Field
                    label="样本可食净重（g）"
                    type="number"
                    min="0.001"
                    step="0.001"
                    required
                    value={grams}
                    onChange={(e) => setGrams(e.target.value)}
                  />
                </div>
                <label className="block">
                  本批蔬菜分类
                  <select
                    className="mt-1 block w-full rounded-lg border border-slate-200 bg-white p-2"
                    value={classification}
                    onChange={(e) => setClassification(e.target.value)}
                  >
                    <option value="catalog">保留目录分类</option>
                    <option value="dark">确认深色蔬菜</option>
                    <option value="other">确认非深色蔬菜</option>
                    <option value="unknown">品种不明，保持未知</option>
                  </select>
                </label>
                <label className="flex items-start gap-2">
                  <input
                    type="checkbox"
                    className="mt-1"
                    checked={confirm}
                    onChange={(e) => setConfirm(e.target.checked)}
                  />
                  <span>确认以上是本批称重与分类依据，仅影响今后记录。</span>
                </label>
                <Button
                  size="sm"
                  disabled={busy || !confirm || !quantity || !grams}
                >
                  {busy ? "保存中…" : "确认营养依据"}
                </Button>
              </form>
            </>
          )}
          {saved && (
            <Notice tone="success">已保存，历史营养记录未重算。</Notice>
          )}
          {error && <Notice tone="error">{error}</Notice>}
        </div>
      )}
    </div>
  );
}
