import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { IngredientPhoto } from "./CatalogArt";
import { Button } from "./ui";
import { Camera, X } from "lucide-react";

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
      </div>
      {canEdit && (
        <div className="ff-photo-actions">
          <label className="ff-photo-upload">
            <Camera size={16} aria-hidden="true" />
            <span className="sr-only">
              {busy ? "处理中…" : hasImage ? "更换图片" : "上传实拍照片"}
            </span>
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
              className="h-8 w-8 p-0"
              aria-label={`移除${name}照片`}
              disabled={busy}
              onClick={remove}
            >
              <X size={15} aria-hidden="true" />
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
