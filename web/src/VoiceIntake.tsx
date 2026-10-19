import { useEffect, useRef, useState } from "react";
import { Mic, Square, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { api, idem } from "./api";
import { useData } from "./app/shared";
import type { Capabilities } from "./app/Capabilities";
import type { CatalogItem } from "./CatalogArt";
import { Button, Card, Field, Notice } from "./ui";

export type VoiceResult = {
  decision?: "draft" | "clarify" | "noop";
  transcript: string;
  warning: string;
  items: {
    catalog_id: string;
    name: string;
    unit: string;
    quantity: string;
    heard: string;
    warning: string;
  }[];
};
type ReviewItem = VoiceResult["items"][number] & {
  location: string;
  bought_on: string;
  expires_on: string;
};
const maxBytes = 5 * 1024 * 1024;

export function VoiceUpload({
  token,
  household,
  onQueued,
}: {
  token: string;
  household: string;
  onQueued: () => void;
}) {
  const capability = useData<Capabilities>("capabilities", "/capabilities");
  const [consent, setConsent] = useState(false),
    [recording, setRecording] = useState(false),
    [busy, setBusy] = useState(false);
  const [file, setFile] = useState<File | null>(null),
    [error, setError] = useState(""),
    [seconds, setSeconds] = useState(0);
  const recorder = useRef<MediaRecorder | null>(null),
    stream = useRef<MediaStream | null>(null);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null),
    alive = useRef(true);
  const uploadKey = useRef<string | null>(null),
    captureGeneration = useRef(0);
  const enabled = !!capability.data?.speech_enabled;
  function stop() {
    if (recorder.current?.state === "recording") recorder.current.stop();
    stream.current?.getTracks().forEach((track) => track.stop());
    if (timer.current) {
      clearInterval(timer.current);
      timer.current = null;
    }
  }
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      captureGeneration.current++;
      stop();
    };
  }, []);
  async function record() {
    setError("");
    if (
      !navigator.mediaDevices?.getUserMedia ||
      typeof MediaRecorder === "undefined"
    ) {
      setError("当前浏览器无法录音，请更换支持录音的浏览器，或使用食材目录。");
      return;
    }
    const mime = [
      "audio/webm;codecs=opus",
      "audio/webm",
      "audio/ogg;codecs=opus",
      "audio/ogg",
    ].find((type) => MediaRecorder.isTypeSupported(type));
    if (!mime) {
      setError("当前浏览器的录音格式暂不支持，请更换浏览器，或使用食材目录。");
      return;
    }
    setBusy(true);
    const generation = ++captureGeneration.current;
    try {
      const media = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (!alive.current || generation !== captureGeneration.current) {
        media.getTracks().forEach((track) => track.stop());
        return;
      }
      stream.current = media;
      const capture = new MediaRecorder(media, { mimeType: mime });
      recorder.current = capture;
      const chunks: Blob[] = [];
      let size = 0,
        elapsed = 0,
        failed = false;
      capture.ondataavailable = (event) => {
        if (event.data.size) {
          chunks.push(event.data);
          size += event.data.size;
          if (size > maxBytes) stop();
        }
      };
      capture.onerror = () => {
        failed = true;
        if (alive.current) setError("录音失败，请检查麦克风后重新录音。");
        stop();
      };
      capture.onstop = () => {
        if (timer.current) {
          clearInterval(timer.current);
          timer.current = null;
        }
        media.getTracks().forEach((track) => track.stop());
        if (!alive.current) return;
        setRecording(false);
        if (failed) {
          setFile(null);
          return;
        }
        if (!size || size > maxBytes) {
          setFile(null);
          setError("录音为空或超过 5 MiB，请缩短后重新录音。");
          return;
        }
        const type = capture.mimeType.split(";")[0];
        setFile(
          new File(
            chunks,
            "ingredients." + (type.includes("ogg") ? "ogg" : "webm"),
            { type },
          ),
        );
        uploadKey.current = null;
      };
      setFile(null);
      setSeconds(0);
      setRecording(true);
      capture.start(1000);
      timer.current = setInterval(() => {
        elapsed++;
        setSeconds(elapsed);
        if (elapsed >= 60) stop();
      }, 1000);
    } catch {
      stream.current?.getTracks().forEach((track) => track.stop());
      if (alive.current) {
        setRecording(false);
        setError("无法使用麦克风，请检查浏览器授权，或使用食材目录。");
      }
    } finally {
      if (alive.current) setBusy(false);
    }
  }
  async function upload() {
    if (!file || !consent) return;
    setBusy(true);
    setError("");
    if (!uploadKey.current) uploadKey.current = idem();
    try {
      const body = new FormData();
      body.append("audio", file);
      body.append("consent", "true");
      const response = await fetch(`/api/households/${household}/jobs/voice`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Idempotency-Key": uploadKey.current,
        },
        body,
      });
      const data = await response.json();
      if (!response.ok)
        throw new Error(
          typeof data.error === "string" ? data.error : "录音提交失败",
        );
      if (!alive.current) return;
      setFile(null);
      uploadKey.current = null;
      onQueued();
    } catch (e) {
      if (alive.current)
        setError(e instanceof Error ? e.message : "录音提交失败，请重试");
    } finally {
      if (alive.current) setBusy(false);
    }
  }
  return (
    <Card>
      <h2 className="flex items-center gap-2 font-semibold">
        <Mic size={20} /> 说一说，添加食材
      </h2>
      <p className="mt-2 text-sm text-slate-600">
        例如：“西红柿半斤、鸡蛋六个”。说清数量和单位，识别后核对再入库。
      </p>
      {capability.error ? (
        <Notice tone="error">
          无法读取语音能力，请稍后重试。
          <Button size="sm" onClick={() => capability.refetch()}>
            重试
          </Button>
        </Notice>
      ) : capability.isLoading ? (
        <p className="mt-3 text-sm">正在读取语音能力…</p>
      ) : !enabled ? (
        <div className="mt-3">
          <Notice>
            语音识别尚未启用，请联系管理员检查识别服务和 AI
            额度开关。可继续使用食材目录添加。
          </Notice>
        </div>
      ) : (
        <div className="mt-3 space-y-3">
          <label className="flex items-start gap-2 text-sm">
            <input
              className="mt-1"
              type="checkbox"
              checked={consent}
              disabled={busy || recording}
              onChange={(e) => setConsent(e.target.checked)}
            />
            我同意将本次录音发送至百炼语音模型识别。识别结果需核对后确认入库。
          </label>
          <div className="flex flex-wrap gap-2">
            {recording ? (
              <Button variant="outline" onClick={stop}>
                <Square size={16} /> 停止录音
              </Button>
            ) : (
              <Button disabled={!consent || busy} onClick={record}>
                <Mic size={16} /> 开始录音
              </Button>
            )}
          </div>
          <p role="status" className="text-xs text-slate-500">
            {recording
              ? `录音中 · ${seconds} 秒，最长 60 秒`
              : file
                ? `录音已完成，可以提交识别。`
                : "点击开始录音，最长 60 秒，最大 5 MiB。"}
          </p>
          {file && !recording && (
            <Button disabled={!consent || busy} onClick={upload}>
              {busy ? "提交中…" : "识别这段录音"}
            </Button>
          )}
        </div>
      )}
      {error && (
        <div className="mt-3">
          <Notice tone="error">{error}</Notice>
        </div>
      )}
    </Card>
  );
}

export function VoiceReview({
  token,
  household,
  job,
  result,
  canEdit,
}: {
  token: string;
  household: string;
  job: string;
  result: VoiceResult;
  canEdit: boolean;
}) {
  const query = useQueryClient();
  canEdit = canEdit && result.decision !== "noop";
  const catalog = useData<CatalogItem[]>(
    "ingredient-catalog",
    "/ingredient-catalog",
  );
  const [items, setItems] = useState<ReviewItem[]>(() =>
    result.items.map((item) => ({
      ...item,
      location: "",
      bought_on: "",
      expires_on: "",
    })),
  );
  const [confirmed, setConfirmed] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const last = useRef<{ signature: string; key: string } | null>(null);
  function update(index: number, patch: Partial<ReviewItem>) {
    setConfirmed(false);
    setItems((current) =>
      current.map((item, i) => (i === index ? { ...item, ...patch } : item)),
    );
  }
  const complete =
    items.length > 0 &&
    items.every(
      (item) =>
        item.catalog_id &&
        /^\d{1,15}(\.\d{1,3})?$/.test(item.quantity) &&
        Number(item.quantity) > 0,
    );
  async function submit() {
    setBusy(true);
    setError("");
    const body = {
      confirm: true,
      items: items.map(
        ({ catalog_id, quantity, location, bought_on, expires_on }) => ({
          catalog_id,
          quantity,
          location,
          bought_on,
          expires_on,
        }),
      ),
    };
    const signature = JSON.stringify(body);
    if (last.current?.signature !== signature)
      last.current = { signature, key: idem() };
    try {
      await api(
        `/households/${household}/jobs/${job}/voice/confirm`,
        token,
        "POST",
        body,
        last.current.key,
      );
      await query.invalidateQueries();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="mt-4 space-y-3" aria-label="语音食材草稿">
      <div className="rounded-xl bg-slate-50 p-3">
        <h3 className="text-sm font-semibold">识别原文</h3>
        <p className="mt-1 whitespace-pre-wrap break-words text-sm">
          {result.transcript}
        </p>
      </div>
      <Notice>
        {result.warning}{" "}
        {result.decision !== "noop" &&
          "请对照原文检查是否遗漏，并补充或移除草稿。"}
      </Notice>
      {catalog.error && (
        <Notice tone="error">
          食材目录读取失败。
          <Button size="sm" onClick={() => catalog.refetch()}>
            重试目录
          </Button>
        </Notice>
      )}
      {items.map((item, index) => (
        <fieldset
          key={index}
          disabled={busy || !canEdit}
          className="min-w-0 space-y-3 rounded-xl border border-slate-200 p-3"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-semibold">食材 {index + 1}</span>
            <Button
              size="sm"
              variant="ghost"
              aria-label={`移除语音食材 ${index + 1}`}
              onClick={() => {
                setItems((current) => current.filter((_, i) => i !== index));
                setConfirmed(false);
              }}
            >
              <X size={15} />
              移除
            </Button>
          </div>
          <label className="block text-sm">
            食材名称
            <select
              aria-label={`语音食材 ${index + 1} 名称`}
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white p-2"
              value={item.catalog_id}
              onChange={(e) => {
                const chosen = catalog.data?.find(
                  (entry) => entry.id === e.target.value,
                );
                update(index, {
                  catalog_id: e.target.value,
                  name: chosen?.name || "",
                  unit: chosen?.default_unit || "",
                  quantity: "",
                });
              }}
            >
              <option value="">请选择目录食材</option>
              {catalog.data?.map((entry) => (
                <option key={entry.id} value={entry.id}>
                  {entry.name} · {entry.default_unit}
                </option>
              ))}
            </select>
          </label>
          {item.warning && (
            <p className="text-sm text-amber-800">{item.warning}</p>
          )}
          <Field
            label={`语音食材 ${index + 1} 数量（${item.unit || "先选食材"}）`}
            type="number"
            min="0.001"
            step="0.001"
            value={item.quantity}
            onChange={(e) => update(index, { quantity: e.target.value })}
          />
          <details>
            <summary className="cursor-pointer text-sm text-slate-600">
              补充位置和日期（选填）
            </summary>
            <div className="mt-3 space-y-3">
              <Field
                label={`语音食材 ${index + 1} 存放位置`}
                maxLength={100}
                value={item.location}
                onChange={(e) => update(index, { location: e.target.value })}
              />
              <Field
                label={`语音食材 ${index + 1} 购买日期`}
                type="date"
                value={item.bought_on}
                onChange={(e) => update(index, { bought_on: e.target.value })}
              />
              <Field
                label={`语音食材 ${index + 1} 有效期`}
                type="date"
                value={item.expires_on}
                onChange={(e) => update(index, { expires_on: e.target.value })}
              />
            </div>
          </details>
        </fieldset>
      ))}
      {canEdit && (
        <>
          <Button
            size="sm"
            variant="outline"
            disabled={busy || items.length >= 20}
            onClick={() => {
              setItems((current) => [
                ...current,
                {
                  catalog_id: "",
                  name: "",
                  unit: "",
                  quantity: "",
                  heard: "",
                  warning: "",
                  location: "",
                  bought_on: "",
                  expires_on: "",
                },
              ]);
              setConfirmed(false);
            }}
          >
            补充一项食材
          </Button>
          <label className="flex items-start gap-2 text-sm">
            <input
              className="mt-1"
              type="checkbox"
              checked={confirmed}
              disabled={busy || !complete}
              onChange={(e) => setConfirmed(e.target.checked)}
            />
            我已核对食材与实际数量，确认这些是本次新增库存。
          </label>
          <Button disabled={busy || !confirmed || !complete} onClick={submit}>
            {busy ? "入库中…" : "确认语音草稿并入库"}
          </Button>
        </>
      )}
      {error && <Notice tone="error">{error}</Notice>}
    </section>
  );
}
