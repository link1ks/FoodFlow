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

const icons: Record<string, string> = {
  番茄: "🍅",
  黄瓜: "🥒",
  土豆: "🥔",
  胡萝卜: "🥕",
  白萝卜: "🥕",
  洋葱: "🧅",
  西兰花: "🥦",
  花菜: "🥦",
  生菜: "🥬",
  菠菜: "🥬",
  小白菜: "🥬",
  大白菜: "🥬",
  卷心菜: "🥬",
  油麦菜: "🥬",
  芹菜: "🌿",
  茄子: "🍆",
  青椒: "🫑",
  红椒: "🫑",
  南瓜: "🎃",
  冬瓜: "🍈",
  丝瓜: "🥒",
  玉米: "🌽",
  豌豆: "🫛",
  四季豆: "🫛",
  莲藕: "🪷",
  韭菜: "🌱",
  香菜: "🌿",
  苹果: "🍎",
  香蕉: "🍌",
  橙子: "🍊",
  梨: "🍐",
  葡萄: "🍇",
  草莓: "🍓",
  蓝莓: "🫐",
  西瓜: "🍉",
  柠檬: "🍋",
  牛肉: "🥩",
  猪肉: "🥩",
  鸡肉: "🍗",
  鸡胸肉: "🍗",
  鸡腿: "🍗",
  排骨: "🍖",
  羊肉: "🥩",
  鸭肉: "🍗",
  虾: "🦐",
  鱼肉: "🐟",
  三文鱼: "🐟",
  带鱼: "🐟",
  贝类: "🦪",
  鸡蛋: "🥚",
  鸭蛋: "🥚",
  牛奶: "🥛",
  酸奶: "🥛",
  奶酪: "🧀",
  豆腐: "🫘",
  豆干: "🫘",
  豆皮: "🫘",
  豆浆: "🥛",
  黄豆: "🫘",
  大米: "🍚",
  面粉: "🌾",
  面条: "🍜",
  米粉: "🍜",
  燕麦: "🌾",
  面包: "🍞",
  馒头: "🥟",
  红薯: "🍠",
  香菇: "🍄",
  口蘑: "🍄",
  金针菇: "🍄",
  木耳: "🍄",
  杏鲍菇: "🍄",
  蒜: "🧄",
  姜: "🫚",
  葱: "🌱",
  食盐: "🧂",
  白糖: "🧂",
  食用油: "🫒",
  酱油: "🫙",
  醋: "🫙",
  黑胡椒: "🧂",
  蜂蜜: "🍯",
  蚝油: "🫙",
  饮用水: "💧",
  椰奶: "🥥",
  咖啡: "☕",
};
const fallback: Record<string, string> = {
  蔬菜: "🥬",
  水果: "🍎",
  肉禽: "🥩",
  水产: "🐟",
  蛋奶: "🥚",
  豆制品: "🫘",
  主食谷物: "🌾",
  菌菇: "🍄",
  调味香料: "🧂",
  饮品: "🥛",
};
const colors: Record<string, [string, string]> = {
  蔬菜: ["#e9f7e8", "#c8eacb"],
  水果: ["#fff0e3", "#ffdab0"],
  肉禽: ["#fff0e9", "#f7d3c2"],
  水产: ["#e8f5fb", "#c4e6f4"],
  蛋奶: ["#fff9e7", "#f5e8b6"],
  豆制品: ["#f4f1e4", "#e0d7bb"],
  主食谷物: ["#fff4df", "#eed9a8"],
  菌菇: ["#f4ecdf", "#dfcbb2"],
  调味香料: ["#f8f0e4", "#ecd9b9"],
  饮品: ["#e7f4f3", "#c9e6e3"],
};
function illustration(item: CatalogItem) {
  const [light, dark] = colors[item.category] || ["#eef3ee", "#dae6dc"];
  const icon = icons[item.name] || fallback[item.category] || "🍽️";
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 240"><defs><radialGradient id="bg"><stop stop-color="${light}"/><stop offset="1" stop-color="${dark}"/></radialGradient></defs><rect width="320" height="240" rx="24" fill="url(#bg)"/><ellipse cx="160" cy="203" rx="94" ry="12" fill="#667c68" opacity=".12"/><circle cx="160" cy="112" r="86" fill="#fff" opacity=".48"/><text x="160" y="153" text-anchor="middle" font-family="Segoe UI Emoji,Apple Color Emoji,Noto Color Emoji,sans-serif" font-size="112">${icon}</text></svg>`;
  return "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svg);
}
export function catalogIllustration(name: string, category: string) {
  return illustration({
    id: "",
    name,
    category,
    default_unit: "",
    aliases: [],
  });
}
export function IngredientPhoto({
  name,
  category,
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
  const source = family || builtin || catalogIllustration(name, category);
  const kind = family ? "家庭照片" : builtin ? "参考照片" : "示意插画";
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
      className={className}
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
