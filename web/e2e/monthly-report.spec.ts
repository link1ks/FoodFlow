import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

test("monthly costs keep unknown history and export a PNG", async ({
  page,
  request,
}, info) => {
  const email = `month-${randomUUID()}@example.test`,
    password = "Monthly-Fixture-2026!";
  const registered = await request.post("/api/register", {
    data: { email, password, name: "月报验收" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "月报厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json(),
    root = `/api/households/${house.id}`;
  const catalogue = await request.get("/api/ingredient-catalog", { headers });
  const tomato = (await catalogue.json()).find(
    (x: { name: string }) => x.name === "番茄",
  );
  const stock = await request.post(root + "/catalog-stock", {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: { catalog_id: tomato.id, quantity: "100" },
  });
  expect(stock.status()).toBe(200);
  const batch = await stock.json();
  async function deduct(quantity: string, reason: string) {
    const r = await request.post(root + "/stock", {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: {
        ingredient_id: batch.ingredient_id,
        batch_id: batch.batch_id,
        quantity,
        reason,
      },
    });
    expect(r.status()).toBe(200);
  }
  await deduct("10", "consume");
  const cost = await request.post(
    root + "/batches/" + batch.batch_id + "/cost",
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { total_cost: "5.00" },
    },
  );
  expect(cost.status()).toBe(200);
  await deduct("20", "consume");
  await deduct("30", "waste");
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /记录/ })
    .click();
  await page.getByRole("button", { name: /金额月报/ }).click();
  await expect(
    page.getByRole("heading", { name: "家庭金额月报", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("¥5.00", { exact: true })).toBeVisible();
  await expect(page.getByText("¥1.00", { exact: true })).toBeVisible();
  await expect(page.getByText("¥1.50", { exact: true })).toBeVisible();
  await expect(
    page.getByText(/出库成本覆盖 2\/3 条；未知成本 1 条/),
  ).toBeVisible();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出月报图片", exact: true }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(
    /^foodflow-month-\d{4}-\d{2}-1\.png$/,
  );
  const path = info.outputPath("monthly-report.png");
  await download.saveAs(path);
  const png = await readFile(path);
  expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
  expect(png.readUInt32BE(16)).toBe(1000);
  expect(png.readUInt32BE(20)).toBeGreaterThan(500);
  await info.attach("monthly-report", { path, contentType: "image/png" });
});
