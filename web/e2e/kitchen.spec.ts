import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("real login, stocked inventory and primary navigation", async ({ page, request, browser }, info) => {
  await info.attach("runtime", {
    body: JSON.stringify({ browser: browser.version(), project: info.project.name, viewport: page.viewportSize() }),
    contentType: "application/json",
  });
  const email = `browser-${randomUUID()}@example.test`;
  const password = "Browser-Fixture-2026!";
  const registration = await request.post("/api/register", {
    data: { email, password, name: "浏览器验收" },
  });
  expect(registration.status()).toBe(200);
  const user = await registration.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "隔离验收厨房", servings: 2 },
  });
  expect(created.ok()).toBeTruthy();
  const household = await created.json();
  const catalog = await request.get("/api/ingredient-catalog", { headers });
  expect(catalog.ok()).toBeTruthy();
  const tomato = (await catalog.json()).find((item: { name: string }) => item.name === "番茄");
  expect(tomato).toBeTruthy();
  const stocked = await request.post(`/api/households/${household.id}/catalog-stock`, {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: { catalog_id: tomato.id, quantity: "300" },
  });
  expect(stocked.ok()).toBeTruthy();

  let scriptErrors = 0;
  let serverFailures = 0;
  page.on("pageerror", () => scriptErrors++);
  page.on("response", (response) => {
    if (response.url().includes("/api/") && response.status() >= 500) serverFailures++;
  });
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("navigation", { name: "主导航" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "从冰箱到餐桌，今天也好好吃饭。" })).toBeVisible();
  await page.screenshot({ path: info.outputPath("home.png"), fullPage: true });

  const nav = page.getByRole("navigation", { name: "主导航" });
  for (const [label, heading] of [
    ["食材库存", "食材库存"],
    ["安排菜单", "安排菜单"],
    ["采购清单", "共享采购清单"],
    ["营养与建议", "营养与建议"],
    ["厨房收支", "厨房收支"],
  ]) {
    await nav.getByRole("button", { name: new RegExp(label) }).click();
    await expect(page.getByRole("heading", { name: heading, exact: true }).first()).toBeVisible();
    if (label === "食材库存") {
      await expect(page.getByText("番茄", { exact: true }).first()).toBeVisible();
      await expect(page.getByText("300 g", { exact: true }).first()).toBeVisible();
      await page.screenshot({ path: info.outputPath("inventory.png"), fullPage: true });
    }
    if (label === "厨房收支") {
      // Projection is asynchronous and the UI refetches every 10 seconds.
      await expect(page.getByText("番茄", { exact: true }).first()).toBeVisible({ timeout: 30_000 });
      await page.screenshot({ path: info.outputPath("insights.png"), fullPage: true });
    }
  }
  expect(scriptErrors, "browser runtime errors").toBe(0);
  expect(serverFailures, "API 5xx responses").toBe(0);
});

test("gateway HTML failure is readable and login can retry", async ({ page }) => {
  let fail = true;
  await page.route("**/api/login", async (route) => {
    if (fail) {
      await route.fulfill({ status: 502, contentType: "text/html", body: "<h1>Bad Gateway</h1>" });
    } else {
      await route.continue();
    }
  });
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(`missing-${randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("Incorrect-Fixture-2026!");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByText("Error: 服务暂时无法连接，请稍后重试", { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByText("Error: 手机号、邮箱或密码错误", { exact: true })).toBeVisible();
});
