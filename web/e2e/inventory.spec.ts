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
  expect(entries).toHaveLength(200);
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
  await page.getByRole("button", { name: "打开个人主页", exact: true }).click();
  await page
    .getByRole("button", { name: "使用家庭：新鲜日常", exact: true })
    .click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  const cards = page.locator(".ff-stock-card");
  await expect(cards).toHaveCount(3);
  await expect(page.locator(".ff-stock-details[open]")).toHaveCount(0);
  await expect(
    cards.getByRole("button", { name: "再买一批" }).first(),
  ).not.toBeVisible();
  await cards.first().locator(".ff-stock-details > summary").click();
  await expect(
    cards.first().getByRole("button", { name: "再买一批" }),
  ).toBeVisible();
  await cards.first().locator(".ff-stock-details > summary").click();
  const tomatoCard = cards.filter({
    has: page.getByRole("heading", { name: "番茄", exact: true }),
  });
  const nutrition = tomatoCard.getByRole("region", {
    name: "番茄营养价值",
    exact: true,
  });
  await expect(nutrition).toHaveCount(0);
  if (info.project.name === "desktop") {
    await tomatoCard.hover();
    await page.waitForTimeout(1200);
    await expect(nutrition).toHaveCount(0);
    await expect(nutrition).toBeVisible({ timeout: 2500 });
  } else {
    await tomatoCard
      .getByRole("button", { name: "查看番茄营养价值", exact: true })
      .click();
    await expect(nutrition).toBeVisible();
  }
  await expect(nutrition).toContainText("每 100 克");
  await expect(nutrition).toContainText("蛋白质");
  await page.screenshot({
    path: info.outputPath("nutrition-overlay.png"),
    fullPage: true,
  });
  await nutrition.getByRole("button", { name: "关闭营养信息" }).click();
  await page.mouse.move(0, 0);
  await expect(nutrition).toHaveCount(0);
  const stats = page.getByRole("region", { name: "库存统计" });
  await expect(stats).toContainText("家中有库存 2 种");
  await expect(stats).toContainText("优先处理 2 批");
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
  expect(
    before.batches.every(
      (b: { expires_on: string; expires_at: string }) =>
        !b.expires_on && !b.expires_at,
    ),
  ).toBeTruthy();
  const reminders = await (
    await request.get(root + "/expiring", { headers })
  ).json();
  expect(reminders).toHaveLength(2);
  expect(
    reminders.every((r: { expires_on: string }) => r.expires_on === ""),
  ).toBeTruthy();
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
  await picker.getByLabel("搜索食材名称或别名").fill("芦笋");
  await expect(
    picker.getByRole("button", { name: "选择芦笋", exact: true }),
  ).toBeVisible();
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
