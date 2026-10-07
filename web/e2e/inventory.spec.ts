import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("inventory filters preserve meal selection and stock at narrow widths", async ({
  page,
  request,
}, info) => {
  const email = `inventory-ui-${randomUUID()}@example.test`;
  const password = "Inventory-Fixture-2026!";
  const registered = await request.post("/api/register", {
    data: { email, password, name: "库存界面验收" },
  });
  expect(registered.status()).toBe(200);
  const headers = {
    Authorization: `Bearer ${(await registered.json()).token}`,
  };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "新鲜日常", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const root = `/api/households/${(await created.json()).id}`;
  const catalog = await request.get("/api/ingredient-catalog", { headers });
  expect(catalog.status()).toBe(200);
  const entries = await catalog.json();
  for (const [name, quantity] of [
    ["番茄", "300"],
    ["大米", "500"],
    ["黄瓜", "1"],
  ]) {
    const response = await request.post(root + "/catalog-stock", {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: {
        catalog_id: entries.find((x: { name: string }) => x.name === name).id,
        quantity,
      },
    });
    expect(response.status()).toBe(200);
  }
  const stocked = await request.get(root + "/inventory", { headers });
  const initial = await stocked.json();
  const cucumber = initial.items.find(
    (i: { name: string }) => i.name === "黄瓜",
  );
  const batch = initial.batches.find(
    (b: { ingredient_id: string }) => b.ingredient_id === cucumber.id,
  );
  const consumed = await request.post(root + "/stock", {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: {
      ingredient_id: cucumber.id,
      batch_id: batch.id,
      quantity: "1",
      reason: "consume",
    },
  });
  expect(consumed.status()).toBe(200);
  const beforeResponse = await request.get(root + "/inventory", { headers });
  const before = await beforeResponse.json();
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  const cards = page.locator(".ff-stock-card");
  await expect(cards).toHaveCount(3);
  const stats = page.getByRole("region", { name: "库存统计" });
  await expect(stats).toContainText("家中有库存 2 种");
  await expect(stats).toContainText("优先处理 0 批");
  await page.screenshot({
    path: info.outputPath("inventory-overview.png"),
    fullPage: true,
  });
  await page
    .getByRole("checkbox", { name: "选作本次食材：番茄", exact: true })
    .check();
  await page.getByLabel("搜索库存食材").fill("不存在的食材");
  await expect(
    page.getByRole("heading", { name: "没有找到符合条件的食材" }),
  ).toBeVisible();
  await expect(stats).toContainText("本次已选 1 种");
  await page.getByRole("button", { name: "清除筛选", exact: true }).click();
  await expect(
    page.getByRole("checkbox", { name: "选作本次食材：番茄", exact: true }),
  ).toBeChecked();
  await page.getByLabel("库存分类").selectOption("主食谷物");
  await expect(cards).toHaveCount(1);
  await expect(cards).toContainText("大米");
  await page.getByLabel("搜索库存食材").fill("番");
  await expect(cards).toHaveCount(0);
  await page.getByRole("button", { name: "清除筛选", exact: true }).click();
  await page.getByRole("button", { name: "只看有库存", exact: true }).click();
  await expect(cards).toHaveCount(2);
  await expect(
    page.getByRole("checkbox", { name: "选作本次食材：黄瓜", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "清除筛选", exact: true }).click();
  await expect(
    page.getByRole("checkbox", { name: "选作本次食材：黄瓜", exact: true }),
  ).toBeDisabled();
  const afterResponse = await request.get(root + "/inventory", { headers });
  expect(await afterResponse.json()).toEqual(before);
  if (info.project.name === "mobile") {
    await page.setViewportSize({ width: 320, height: 740 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: info.outputPath("inventory-narrow.png"),
      fullPage: true,
    });
  }
  await page.getByRole("button", { name: "＋ 添加食材", exact: true }).click();
  const picker = page.getByRole("dialog", { name: "选择食材" });
  await picker.getByRole("button", { name: "蔬菜", exact: true }).click();
  await expect(
    picker.getByRole("button", { name: "蔬菜", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await picker.getByLabel("搜索食材名称或别名").fill("番茄");
  await picker.getByRole("button", { name: "选择番茄", exact: true }).click();
  await expect(picker.getByLabel("本次入库数量（g）")).toBeVisible();
  await page.screenshot({
    path: info.outputPath("inventory-picker-detail.png"),
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await picker.getByRole("button", { name: "关闭食材选择" }).click();
  await expect(
    page.getByRole("checkbox", { name: "选作本次食材：番茄", exact: true }),
  ).toBeChecked();
});
