// Read-only browser recheck of the existing default site. Never authenticate,
// submit business writes, invoke models, or reuse the user's browser profile.
import { chromium } from "@playwright/test";
import { readFile, mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { resolve } from "node:path";

const base = "http://localhost:5173";
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const browser = await chromium.launch({
  channel: process.env.PLAYWRIGHT_CHANNEL === "msedge" ? "msedge" : undefined,
});
try {
  const context = await browser.newContext();
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== base || !["GET", "HEAD"].includes(request.method())) {
      await route.abort();
    } else {
      await route.continue();
    }
  });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", () => errors.push("pageerror"));
  const response = await page.goto(base, { waitUntil: "networkidle" });
  if (response?.status() !== 200) throw new Error("Default homepage is unavailable");
  await page.getByLabel("手机号或邮箱", { exact: true }).waitFor();
  await page.getByRole("button", { name: "登录", exact: true }).waitFor();
  const ready = await context.request.get(`${base}/health/ready`);
  if (ready.status() !== 200) throw new Error("Default readiness failed");
  const html = await response.body();
  const expected = await readFile(resolve("web/dist/index.html"));
  if (digest(html) !== digest(expected)) throw new Error("Served HTML differs from verified web/dist");
  const assets = [...new Set([...html.toString().matchAll(/(?:src|href)="(\/assets\/[^"]+\.(?:js|css))"/g)].map((m) => m[1]))];
  if (!assets.length) throw new Error("No built frontend assets");
  for (const path of assets) {
    if (!/^\/assets\/[A-Za-z0-9._-]+\.(js|css)$/.test(path)) throw new Error("Invalid asset path");
    const served = await context.request.get(base + path);
    const local = await readFile(resolve("web/dist", "." + path));
    if (served.status() !== 200 || digest(await served.body()) !== digest(local)) {
      throw new Error("Default frontend build differs from verified build");
    }
  }
  if (errors.length) throw new Error("Default page has script errors");
  await mkdir(".cache/harness/main-site", { recursive: true });
  await page.screenshot({ path: ".cache/harness/main-site/home.png", fullPage: true });
  console.log(JSON.stringify({ homepage: 200, readiness: 200, verified_assets: assets.length, browser: browser.version(), scope: "Unauthenticated read-only homepage and build identity. Business interactions run in isolated acceptance." }));
  await context.close();
} finally {
  await browser.close();
}
