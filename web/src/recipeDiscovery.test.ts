import { describe, expect, it } from "vitest";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { discoverRecipes, suggestRecipes } from "./recipeSelection";
import type { Recipe } from "./app/shared";
import photos from "./recipePhotos.json";

const recipe = (
  id: string,
  title: string,
  minutes: number,
  tags: string[],
  names: string[],
): Recipe => ({
  id,
  title,
  minutes,
  tags,
  servings: 2,
  source: "fixture",
  steps: ["fixture"],
  ingredients: names.map((name) => ({ name, quantity: "100.000", unit: "g" })),
});
const fixtures = [
  recipe("a", "蒜蓉菠菜", 10, ["蔬菜"], ["菠菜", "蒜"]),
  recipe("b", "芦笋", 12, ["蔬菜"], ["芦笋"]),
  recipe("c", "虾仁", 20, ["荤菜"], ["虾"]),
  recipe("d", "牛肉", 25, ["荤菜"], ["牛肉"]),
  recipe("e", "鸡蛋面", 20, ["主食"], ["面条", "鸡蛋"]),
  recipe("f", "南瓜粥", 45, ["早餐"], ["南瓜", "大米"]),
];
describe("recipe discovery", () => {
  it("filters search and exclusions and rotates bounded diverse suggestions without stock claims", () => {
    expect(discoverRecipes(fixtures, "鸡蛋", "全部", [])).toEqual([
      fixtures[4],
    ]);
    expect(discoverRecipes(fixtures, "", "主食", [])).toEqual(
      fixtures.slice(4),
    );
    expect(discoverRecipes(fixtures, "", "全部", ["虾", "鸡蛋"])).toEqual([
      fixtures[0],
      fixtures[1],
      fixtures[3],
      fixtures[5],
    ]);
    expect(discoverRecipes(fixtures, "不存在", "全部", [])).toEqual([]);
    expect(suggestRecipes(fixtures, 30, 0).map((r) => r.id)).toEqual([
      "a",
      "c",
      "e",
    ]);
    expect(suggestRecipes(fixtures, 30, 1).map((r) => r.id)).toEqual([
      "b",
      "d",
      "e",
    ]);
    expect(suggestRecipes(fixtures, 1, 0)).toEqual([]);
    expect(suggestRecipes([], 30, 9)).toEqual([]);
    expect(suggestRecipes([fixtures[0]], 30, 99)).toEqual([fixtures[0]]);
    expect(suggestRecipes(fixtures, NaN, 0)).toEqual([]);
  });
  it("covers all 36 reference recipes with distinct local generated photos and provenance", () => {
    const sql = [
      "002_seed.sql",
      "029_family_recipes.sql",
      "039_recipe_discovery.sql",
    ]
      .map(
        (f) =>
          readFileSync(
            new URL(`../../sql/schema/${f}`, import.meta.url),
            "utf8",
          ).split("-- +goose Down")[0],
      )
      .join("\n");
    const seeded = [
      ...sql.matchAll(
        /(?:VALUES\s*)?\(\s*'([123]0000000-[^']+)','([^']+)',2,\d+,ARRAY/g,
      ),
    ].map((m) => ({ id: m[1], name: m[2] }));
    expect(seeded).toHaveLength(36);
    expect(photos.map((p) => ({ id: p.id, name: p.name }))).toEqual(seeded);
    const manifest = JSON.parse(
      readFileSync(
        new URL("../public/recipe-photos/manifest.json", import.meta.url),
        "utf8",
      ),
    );
    const sources = JSON.parse(
      readFileSync(
        new URL("../../scripts/photos/recipe-sources.json", import.meta.url),
        "utf8",
      ),
    );
    expect(new Set(photos.map((p) => p.src)).size).toBe(36);
    for (const p of photos) {
      expect(p.src).toMatch(/^\/recipe-photos\/[a-f0-9]{12}\.webp$/);
      const entry = manifest.find((x: { id: string }) => x.id === p.id);
      const raw = readFileSync(new URL("../public" + p.src, import.meta.url));
      expect(createHash("sha256").update(raw).digest("hex")).toBe(entry.sha256);
      expect(raw.length).toBeLessThan(160000);
      expect(entry.width).toBe(480);
      expect(entry.height).toBe(360);
      expect(entry.generated).toBe(true);
      expect(
        sources.some(
          (s: { file: string; sha256: string; prompt: string }) =>
            s.file === entry.source_asset &&
            s.sha256 === entry.source_sha256 &&
            s.prompt.length > 80,
        ),
      ).toBe(true);
      expect(entry.crop).toHaveLength(4);
    }
  });
});
