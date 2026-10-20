import { CondimentContainer } from "./CondimentContainer";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idem } from "./api";
import { Button, Card, Field, Notice } from "./ui";
import type { InventoryBatch } from "./BatchActions";
type Bottle = {
  id: string;
  batch_id: string;
  name: string;
  unit: string;
  capacity: string;
  quantity: string;
  threshold: number;
  usable: boolean;
};
type Option = { name: string; unit: string; basis: string[] };
type Write = (path: string, body: unknown) => Promise<void>;
const qty = (s: string) => s.replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1");
function BottleCard({
  b,
  root,
  busy,
  editable,
  write,
}: {
  b: Bottle;
  root: string;
  busy: boolean;
  editable: boolean;
  write: Write;
}) {
  const percent = Math.max(
    0,
    Math.min(100, Math.round((Number(b.quantity) / Number(b.capacity)) * 100)),
  );
  const [target, setTarget] = useState(percent),
    [threshold, setThreshold] = useState(String(b.threshold)),
    [edit, setEdit] = useState(false);
  useEffect(() => {
    setTarget(percent);
    setThreshold(String(b.threshold));
    setEdit(false);
  }, [percent, b.quantity, b.threshold]);
  const low = Number(b.quantity) * 100 < Number(b.capacity) * b.threshold;
  return (
    <div className="ff-pantry-bottle rounded-2xl border border-slate-200 bg-white p-4">
      <div className="ff-pantry-overview">
        <h3 className="font-semibold">{b.name}</h3>
        <CondimentContainer
          name={b.name}
          quantity={b.quantity}
          capacity={b.capacity}
          unit={b.unit}
          basis="满瓶容量"
        />
        <div className="flex-1">
          {low && (
            <p className="mt-2 text-sm text-amber-800">
              {percent === 0
                ? "估算已耗尽，请核对实物并更换新瓶。"
                : "余量偏低，建议查看采购清单补货。"}
            </p>
          )}
          {!b.usable && (
            <p className="mt-2 text-sm text-rose-700">
              批次已过期或报损，请更换或停用。
            </p>
          )}
          {editable && (
            <Button variant="outline" size="sm" onClick={() => setEdit(!edit)}>
              {edit ? "收起" : "校准 / 管理"}
            </Button>
          )}
        </div>
      </div>
      {edit && (
        <div className="mt-4 space-y-3 border-t border-slate-100 pt-3">
          <label className="block text-sm">
            目测剩余 {target}%
            <input
              aria-label={b.name + "余量校准"}
              className="mt-2 block w-full accent-emerald-700"
              type="range"
              min="0"
              max="100"
              value={target}
              onChange={(e) => setTarget(Number(e.target.value))}
            />
          </label>
          <Button
            size="sm"
            disabled={busy || target === percent}
            onClick={() =>
              write(root + "/pantry/" + b.id, {
                action: "calibrate",
                expected_quantity: b.quantity,
                percent: target,
              })
            }
          >
            确认校准为 {target}%
          </Button>
          <Field
            label="补货线（%）"
            type="number"
            min="1"
            max="50"
            value={threshold}
            onChange={(e) => setThreshold(e.target.value)}
          />
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() =>
                write(root + "/pantry/" + b.id, {
                  action: "threshold",
                  threshold: Number(threshold),
                })
              }
            >
              保存补货线
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() =>
                write(root + "/pantry/" + b.id, { action: "disable" })
              }
            >
              停用免称重
            </Button>
          </div>
          <p className="text-xs text-slate-500">
            校准同步更新该批次库存并留存调整记录。停用保留库存，购买的新瓶需单独启用。
          </p>
        </div>
      )}
    </div>
  );
}
export function VirtualPantry({
  token,
  root,
  items,
  batches,
  canEdit,
}: {
  token: string;
  root: string;
  items: { id: string; name: string; unit: string }[];
  batches: InventoryBatch[];
  canEdit: boolean;
}) {
  const q = useQueryClient(),
    [add, setAdd] = useState(false),
    [batch, setBatch] = useState(""),
    [capacity, setCapacity] = useState(""),
    [threshold, setThreshold] = useState("15"),
    [accept, setAccept] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const pending = useRef<{ identity: string; key: string } | null>(null);
  const data = useQuery({
    queryKey: ["pantry", root],
    queryFn: () =>
      api<{ items: Bottle[]; options: Option[] }>(root + "/pantry", token),
  });
  const eligible = batches.filter(
    (b) =>
      Number(b.quantity) > 0 &&
      b.condition === "normal" &&
      (!b.expires_at || new Date(b.expires_at).getTime() > Date.now()) &&
      !data.data?.items.some((p) => p.batch_id === b.id) &&
      items.some(
        (i) =>
          i.id === b.ingredient_id &&
          data.data?.options.some(
            (o) => o.name === i.name && o.unit === i.unit,
          ),
      ),
  );
  const selected = eligible.find((b) => b.id === batch),
    ingredient = items.find((i) => i.id === selected?.ingredient_id),
    option = data.data?.options.find((o) => o.name === ingredient?.name);
  async function write(path: string, body: unknown) {
    setBusy(true);
    setError("");
    const identity = path + JSON.stringify(body);
    if (pending.current?.identity !== identity)
      pending.current = { identity, key: idem() };
    try {
      await api(path, token, "POST", body, pending.current.key);
      pending.current = null;
      await q.invalidateQueries();
      if (path === root + "/pantry") {
        setAdd(false);
        setBatch("");
        setAccept(false);
      }
    } catch (e) {
      setError(String(e));
      await q.invalidateQueries();
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="mb-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="font-semibold">常用调味品</h2>
          <p className="mt-1 text-xs text-slate-500">
            油、盐、酱油，一眼看清余量。设置容量后，按配方估算用量。
          </p>
        </div>
        {canEdit && (
          <Button variant="outline" size="sm" onClick={() => setAdd(!add)}>
            {add ? "收起" : "启用已有批次"}
          </Button>
        )}
      </div>
      {data.isLoading && <p className="mt-3 text-sm">读取调味品…</p>}
      {data.error && (
        <Notice tone="error">
          读取失败{" "}
          <Button size="sm" onClick={() => data.refetch()}>
            重试
          </Button>
        </Notice>
      )}
      {error && (
        <div className="mt-3">
          <Notice tone="error">{error}</Notice>
        </div>
      )}

      {add && (
        <div
          id="pantry-setup"
          className="mt-4 space-y-3 rounded-xl bg-emerald-50 p-3"
        >
          <label className="block text-sm">
            选择正常未过期的批次
            <select
              className="mt-1 w-full rounded-xl border border-slate-300 bg-white p-2"
              value={batch}
              onChange={(e) => {
                setBatch(e.target.value);
                setAccept(false);
                setCapacity(
                  eligible.find((b) => b.id === e.target.value)?.quantity || "",
                );
              }}
            >
              <option value="">请选择</option>
              {eligible.map((b) => {
                const i = items.find((i) => i.id === b.ingredient_id);
                return (
                  <option key={b.id} value={b.id}>
                    {i?.name} · {qty(b.quantity)} {i?.unit} ·{" "}
                    {b.location || "未标位置"} · {b.id.slice(-6)}
                  </option>
                );
              })}
            </select>
          </label>
          {!eligible.length && (
            <p className="text-sm text-slate-600">
              没有可启用批次。请先录入食用油/酱油（ml）或食盐（g）。
            </p>
          )}
          <div className="grid gap-3 sm:grid-cols-2">
            <Field
              label={"满瓶容量 " + (ingredient?.unit || "")}
              value={capacity}
              onChange={(e) => setCapacity(e.target.value)}
            />
            <Field
              label="补货线（%）"
              type="number"
              min="1"
              max="50"
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
            />
          </div>
          {option && (
            <Notice>
              {option.basis.join("；")}
              。可用菜谱：四道项目示例菜的油、盐，以及土豆炖鸡的酱油。
            </Notice>
          )}
          <label className="flex items-start gap-2 text-sm">
            <input
              className="mt-1"
              type="checkbox"
              checked={accept}
              onChange={(e) => setAccept(e.target.checked)}
            />
            我确认启用配方估算，并会按实物校准余量；估算并非称重结果。
          </label>
          <Button
            disabled={busy || !batch || !accept}
            onClick={() =>
              write(root + "/pantry", {
                batch_id: batch,
                capacity,
                threshold: Number(threshold),
                accept_estimates: accept,
              })
            }
          >
            确认启用
          </Button>
        </div>
      )}
      <div className="ff-pantry-shelf">
        {data.data?.options
          .filter((o) => !data.data.items.some((b) => b.name === o.name))
          .map((o) => {
            const candidate = eligible.find((b) =>
              items.some(
                (i) =>
                  i.id === b.ingredient_id &&
                  i.name === o.name &&
                  i.unit === o.unit,
              ),
            );
            return (
              <div className="ff-pantry-placeholder" key={o.name}>
                <h3>{o.name}</h3>
                <CondimentContainer
                  name={o.name}
                  quantity="0"
                  capacity="0"
                  unit={o.unit}
                  basis="满瓶容量"
                  status={candidate ? "待设置容量" : "未添加"}
                />
                {canEdit &&
                  (candidate ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setAdd(true);
                        setBatch(candidate.id);
                        setCapacity(candidate.quantity);
                        setAccept(false);
                        requestAnimationFrame(() =>
                          document
                            .getElementById("pantry-setup")
                            ?.scrollIntoView({
                              behavior: "smooth",
                              block: "center",
                            }),
                        );
                      }}
                    >
                      设置{o.name}容量
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        document
                          .getElementById("inventory-add")
                          ?.scrollIntoView({ behavior: "smooth" })
                      }
                    >
                      添加{o.name}库存
                    </Button>
                  ))}
              </div>
            );
          })}
        {data.data?.items.map((b) => (
          <BottleCard
            key={b.id}
            b={b}
            root={root}
            busy={busy}
            editable={canEdit}
            write={write}
          />
        ))}
      </div>
    </Card>
  );
}
