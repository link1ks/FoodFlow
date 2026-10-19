import { useEffect, useState } from "react";
import type { NutritionProfile } from "./Nutrition";
import { catalogPhoto } from "./catalogPhotos";

export type CatalogItem = {
  id: string;
  name: string;
  category: string;
  default_unit: string;
  aliases: string[];
  nutrition?: NutritionProfile;
};
export type InventoryItem = {
  id: string;
  name: string;
  unit: string;
  quantity: string;
  has_image: boolean;
  image_version: number;
};

export function IngredientPhoto({
  name,
  photo = "",
  className,
}: {
  name: string;
  category: string;
  photo?: string;
  className: string;
}) {
  const reference = catalogPhoto(name);
  const [failed, setFailed] = useState<string[]>([]);
  const family = photo && !failed.includes(photo) ? photo : "";
  const builtin =
    reference && !failed.includes(reference.src) ? reference.src : "";
  const source = family || builtin;
  const kind = family ? "家庭照片" : "参考照片";
  if (!source) {
    return (
      <span
        className={`ff-ingredient-photo ff-photo-empty ${className}`}
        role="img"
        aria-label={`${name}暂无实拍照片`}
      >
        暂无实拍照片
      </span>
    );
  }
  return (
    <img
      src={source}
      alt={`${name}${kind}`}
      title={
        builtin && !family
          ? `${reference?.author} · ${reference?.license}`
          : undefined
      }
      data-photo-kind={kind}
      loading="lazy"
      decoding="async"
      width={320}
      height={240}
      className={`ff-ingredient-photo ${className}`}
      onError={() => {
        if (family || builtin) setFailed((current) => [...current, source]);
      }}
    />
  );
}
export function CatalogPicture({
  item,
  stock,
  token,
  household,
  className,
}: {
  item: CatalogItem;
  stock?: InventoryItem;
  token: string;
  household: string;
  className: string;
}) {
  const scope = [
    stock?.id,
    stock?.has_image,
    stock?.image_version,
    token,
    household,
  ].join("|");
  const [photo, setPhoto] = useState({ scope: "", url: "" });
  useEffect(() => {
    setPhoto({ scope, url: "" });
    if (!stock?.has_image) return;
    const controller = new AbortController();
    let objectURL = "";
    fetch(
      "/api/households/" +
        household +
        "/ingredients/" +
        stock.id +
        "/image?v=" +
        stock.image_version,
      {
        headers: { Authorization: "Bearer " + token },
        signal: controller.signal,
      },
    )
      .then((response) => {
        if (!response.ok) throw Error("image unavailable");
        return response.blob();
      })
      .then((blob) => {
        if (!controller.signal.aborted) {
          objectURL = URL.createObjectURL(blob);
          setPhoto({ scope, url: objectURL });
        }
      })
      .catch(() => {});
    return () => {
      controller.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [stock?.id, stock?.has_image, stock?.image_version, token, household]);
  return (
    <IngredientPhoto
      name={item.name}
      category={item.category}
      photo={photo.scope === scope ? photo.url : ""}
      className={className}
    />
  );
}
