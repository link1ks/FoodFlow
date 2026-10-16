// Render authored virtual scenarios, without an API, account, model or database.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const require = createRequire(new URL("../web/package.json", import.meta.url));
const { chromium } = require("@playwright/test");
const scratch = resolve(root, ".cache/harness/animated-demos");
const output = resolve(root, "docs/assets/demos");
const fps = 10;
const source = await readFile(
  resolve(root, "scripts/feature-demos.html"),
  "utf8",
);
await mkdir(output, { recursive: true });
await mkdir(scratch, { recursive: true });
const browser = await chromium.launch({
  channel: process.env.PLAYWRIGHT_CHANNEL === "msedge" ? "msedge" : undefined,
});
const results = [];
try {
  const page = await browser.newPage({
    viewport: { width: 1080, height: 640 },
    deviceScaleFactor: 1,
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/*", (route) => route.abort()); // Everything is authored locally.
  await page.setContent(source);
  await page.evaluate(() => document.fonts.ready);
  const scenes = await page.evaluate(() => window.demoScenes);
  for (const [name, duration] of Object.entries(scenes)) {
    const frames = resolve(scratch, name);
    await mkdir(frames, { recursive: true });
    const count = duration * fps;
    for (let frame = 0; frame < count; frame++) {
      await page.evaluate(({ name, time }) => window.renderDemo(name, time), {
        name,
        time: frame / fps,
      });
      if (frame % fps === 0) {
        const overflow = await page.evaluate(() =>
          [...document.querySelectorAll("svg text")].some((el) => {
            const bounds = el.getBBox();
            return (
              bounds.x < 0 ||
              bounds.y < 0 ||
              bounds.x + bounds.width > 1080 ||
              bounds.y + bounds.height > 640
            );
          }),
        );
        assert.equal(
          overflow,
          false,
          `${name} has clipped text at frame ${frame}`,
        );
      }
      await page.screenshot({
        path: resolve(frames, `frame-${String(frame).padStart(4, "0")}.png`),
        animations: "disabled",
      });
    }
    assert.deepEqual(errors, [], "Virtual scene runtime errors");
    const target = resolve(output, `${name}.gif`);
    execFileSync(
      process.env.FFMPEG_PATH || "ffmpeg",
      [
        "-y",
        "-loglevel",
        "error",
        "-framerate",
        String(fps),
        "-i",
        resolve(frames, "frame-%04d.png"),
        "-filter_complex",
        "[0:v]split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=3:diff_mode=rectangle",
        "-loop",
        "0",
        "-frames:v",
        String(count),
        target,
      ],
      { cwd: root, stdio: "pipe", timeout: 120000 },
    );
    const bytes = await readFile(target);
    assert.equal(bytes.subarray(0, 6).toString(), "GIF89a");
    assert.equal(bytes.readUInt16LE(6), 1080);
    assert.equal(bytes.readUInt16LE(8), 640);
    assert.ok(
      bytes.length < 4 * 1024 * 1024,
      "Keep each README animation below 4 MiB",
    );
    results.push({
      file: `${name}.gif`,
      duration_seconds: duration,
      fps,
      frames: count,
      bytes: bytes.length,
      sha256: createHash("sha256").update(bytes).digest("hex"),
    });
    console.log(
      `Rendered ${name}.gif: ${count} frames, ${duration}s, ${bytes.length} bytes`,
    );
  }
  await writeFile(
    resolve(scratch, "manifest.json"),
    JSON.stringify(
      {
        kind: "authored virtual scenarios; no application execution or model dispatch",
        source_sha256: createHash("sha256").update(source).digest("hex"),
        browser: browser.version(),
        channel: process.env.PLAYWRIGHT_CHANNEL || "chromium",
        width: 1080,
        height: 640,
        animations: results,
      },
      null,
      2,
    ) + "\n",
  );
} finally {
  await browser.close();
}
