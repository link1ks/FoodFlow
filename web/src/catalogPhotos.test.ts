import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";
import { catalogPhoto, catalogPhotos } from "./catalogPhotos";

describe("catalogue photographs", () => {
  it("gives every seeded ingredient a licensed local photograph", () => {
    const metadata = JSON.parse(
      readFileSync(
        new URL("../public/ingredient-photos/manifest.json", import.meta.url),
        "utf8",
      ),
    ) as ((typeof catalogPhotos)[number] & {
      source: string;
      sha256: string;
      width: number;
      height: number;
      reviewed: string;
    })[];
    const schema = new URL("../../sql/schema/", import.meta.url);
    const seed = readFileSync(
      new URL("007_ingredient_catalog.sql", schema),
      "utf8",
    );
    const extra = readFileSync(
      new URL("016_benchmark_seed.sql", schema),
      "utf8",
    ).split("AS x(name)")[0];
    const names = [
      ...seed.matchAll(/\('([^']+)',\s*'[^']+',\s*'[^']+'/g),
      ...extra.matchAll(/\('([^']+)'\)/g),
    ].map((match) => match[1]);
    expect(names.length).toBe(91);
    expect(new Set(catalogPhotos.map((photo) => photo.name))).toEqual(
      new Set(names),
    );
    for (const name of names) {
      const photo = metadata.find((photo) => photo.name === name)!;
      expect(catalogPhoto(name)).toEqual({
        name: photo.name,
        src: photo.src,
        author: photo.author,
        license: photo.license,
      });
      expect(photo.src).toMatch(/^\/ingredient-photos\/[a-f0-9]{12}\.webp$/);
      expect(photo.license).toMatch(/^(CC BY(?:-SA)? |CC0|Public domain)/);
      expect(photo.author.trim()).not.toBe("");
      expect(photo.source).toMatch(
        /^https:\/\/commons\.wikimedia\.org\/wiki\/File:/,
      );
      const raw = readFileSync(
        new URL("../public" + photo.src, import.meta.url),
      );
      expect(createHash("sha256").update(raw).digest("hex")).toBe(photo.sha256);
      expect(raw.length).toBeLessThanOrEqual(160_000);
      expect(photo.width).toBeLessThanOrEqual(480);
      expect(photo.height).toBeLessThanOrEqual(480);
      expect(photo.reviewed).toBe("2026-10-01");
    }
    const files = readdirSync(
      new URL("../public/ingredient-photos/", import.meta.url),
    ).filter((file) => file.endsWith(".webp"));
    expect(files.sort()).toEqual(
      catalogPhotos.map((photo) => photo.src.split("/").pop()).sort(),
    );
    expect(catalogPhoto("自定义食材")).toBeUndefined();
  });
});
