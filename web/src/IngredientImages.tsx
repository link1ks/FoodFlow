import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { IngredientPhoto } from "./CatalogArt";
import { catalogPhoto } from "./catalogPhotos";
import { Button } from "./ui";

export function IngredientImageManager({
  id,
  name,
  category,
  hasImage,
  version,
  token,
  household,
  canEdit,
}: {
  id: string;
  name: string;
  category: string;
  hasImage: boolean;
  version: number;
  token: string;
  household: string;
  canEdit: boolean;
}) {
  const query = useQueryClient();
  const [preview, setPreview] = useState({ scope: "", url: "" }),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const path = "/households/" + household + "/ingredients/" + id + "/image";
  const scope = [path, version, hasImage, token].join("|");
  const shown = preview.scope === scope ? preview.url : "";
  useEffect(() => {
    setPreview({ scope, url: "" });
    setError("");
    if (!hasImage) return;
    let url = "";
    const controller = new AbortController();
    fetch("/api" + path + "?v=" + version, {
      headers: { Authorization: "Bearer " + token },
      signal: controller.signal,
    })
      .then((response) => {
        if (!response.ok) throw Error("image unavailable");
        return response.blob();
      })
      .then((blob) => {
        if (!controller.signal.aborted) {
          url = URL.createObjectURL(blob);
          setPreview({ scope, url });
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) setError("图片暂时无法显示");
      });
    return () => {
      controller.abort();
      if (url) URL.revokeObjectURL(url);
    };
  }, [hasImage, version, path, token]);
  async function upload(file: File | undefined) {
    if (!file) return;
    setError("");
    setBusy(true);
    try {
      if (file.size > 5 * 1024 * 1024) throw Error("图片不能超过 5 MiB");
      const body = new FormData();
      body.append("image", file);
      const response = await fetch("/api" + path, {
        method: "POST",
        headers: { Authorization: "Bearer " + token },
        body,
      });
      const value = await response
        .json()
        .catch(() => ({ error: "响应无法解析" }));
      if (!response.ok) throw Error(value.error || "上传失败");
      await query.invalidateQueries({
        queryKey: ["inventory", "/households/" + household + "/inventory"],
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    setError("");
    setBusy(true);
    try {
      await api(path, token, "DELETE");
      await query.invalidateQueries({
        queryKey: ["inventory", "/households/" + household + "/inventory"],
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="ff-stock-image shrink-0">
      <div className="ff-stock-image-frame relative overflow-hidden rounded-xl bg-emerald-50">
        <IngredientPhoto
          name={name}
          category={category}
          photo={shown}
          className="h-full w-full object-cover"
        />
        <span className="absolute bottom-1 left-1 rounded-md bg-white/90 px-1.5 py-0.5 text-[10px] text-slate-700">
          {shown
            ? "家庭照片"
            : hasImage && !error
              ? "照片加载中"
              : catalogPhoto(name)
                ? "参考照片"
                : "示意插画"}
        </span>
      </div>
      {canEdit && (
        <div className="mt-1 space-y-1">
          <label className="block cursor-pointer text-center text-xs text-emerald-700">
            {busy ? "处理中…" : hasImage ? "更换图片" : "上传实拍照片"}
            <input
              type="file"
              aria-label={`为${name}上传图片`}
              accept="image/jpeg,image/png,image/webp"
              disabled={busy}
              className="sr-only"
              onChange={(e) => {
                void upload(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
          {hasImage && (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-full text-xs"
              disabled={busy}
              onClick={remove}
            >
              移除
            </Button>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="mt-1 text-xs text-rose-700">
          {error}，已回退到参考图片
        </p>
      )}
    </div>
  );
}
