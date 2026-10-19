import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";
import { catalogPhoto, catalogPhotos } from "./catalogPhotos";

describe("studio catalogue references", () => {
  it("covers the seeded catalogue with local reviewed generated assets and provenance", () => {
    const metadata = JSON.parse(
      readFileSync(
        new URL("../public/ingredient-photos/manifest.json", import.meta.url),
        "utf8",
      ),
    ) as {
      name: string;
      src: string;
      generated: boolean;
      style: string;
      sha256: string;
      width: number;
      height: number;
      background: string;
      source_asset: string;
      source_sha256: string;
      crop: number[];
    }[];
    const sources = JSON.parse(
      readFileSync(
        new URL("../../scripts/photos/studio-sources.json", import.meta.url),
        "utf8",
      ),
    ) as { file: string; sha256: string; prompt: string }[];
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
      ...readFileSync(new URL("037_catalog_200.sql", schema), "utf8")
        .split("-- +goose Down")[0]
        .matchAll(/\('([^']+)',\s*'[^']+',\s*'[^']+'/g),
    ].map((m) => m[1]);
    expect(names).toHaveLength(200);
    expect(new Set(catalogPhotos.map((p) => p.name))).toEqual(new Set(names));
    expect(metadata).toHaveLength(names.length);
    for (const name of names) {
      const photo = metadata.find((p) => p.name === name)!;
      expect(catalogPhoto(name)).toEqual({
        name,
        src: photo.src,
        generated: true,
      });
      expect(photo.generated).toBe(true);
      expect(photo.style).toBe("photorealistic-studio");
      expect(photo.src).toMatch(/^\/ingredient-photos\/[a-f0-9]{12}\.webp$/);
      const raw = readFileSync(
        new URL("../public" + photo.src, import.meta.url),
      );
      expect(createHash("sha256").update(raw).digest("hex")).toBe(photo.sha256);
      expect(raw.length).toBeLessThanOrEqual(160_000);
      expect([photo.width, photo.height]).toEqual([480, 360]);
      expect(photo.background).toBe("#faf8f3");
      const source = sources.find((s) => s.file === photo.source_asset)!;
      expect(source.sha256).toBe(photo.source_sha256);
      expect(source.sha256).toMatch(/^[a-f0-9]{64}$/);
      expect(source.prompt.length).toBeGreaterThan(100);
      expect(photo.crop).toHaveLength(4);
      expect(photo.crop[2]).toBeGreaterThan(photo.crop[0]);
      expect(photo.crop[3]).toBeGreaterThan(photo.crop[1]);
    }
    const files = readdirSync(
      new URL("../public/ingredient-photos/", import.meta.url),
    ).filter((f) => f.endsWith(".webp"));
    expect(files.sort()).toEqual(
      catalogPhotos.map((p) => p.src.split("/").pop()).sort(),
    );
    expect(catalogPhoto("自定义食材")).toBeUndefined();
  });
});
