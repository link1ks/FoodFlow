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
  const rice = initial.items.find((i: { name: string }) => i.name === "大米");
  const riceBatch = initial.batches.find(
    (b: { ingredient_id: string }) => b.ingredient_id === rice.id,
  );
  expect(
    (
      await request.post(root + "/stock", {
        headers: { ...headers, "Idempotency-Key": randomUUID() },
        data: {
          ingredient_id: rice.id,
          batch_id: riceBatch.id,
          quantity: "125",
          reason: "consume",
        },
      })
    ).status(),
  ).toBe(200);
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
    await page.waitForTimeout(2300);
    await expect(nutrition).toHaveCount(0);
  }
  await expect(tomatoCard.locator(".ff-nutrition-trigger")).toHaveCount(0);
  await expect(tomatoCard).toHaveClass(/ff-stock-card-priority/);
  await expect(
    tomatoCard.locator(".ff-stock-title-row .ff-priority-badge"),
  ).toHaveText("优先处理");
  const sizes = await cards.evaluateAll((nodes) =>
    nodes.map((node) => ({
      w: node.getBoundingClientRect().width,
      h: node.getBoundingClientRect().height,
    })),
  );
  expect(
    Math.max(...sizes.map((s) => s.h)) - Math.min(...sizes.map((s) => s.h)),
  ).toBeLessThan(2);
  expect(
    Math.max(...sizes.map((s) => s.w)) - Math.min(...sizes.map((s) => s.w)),
  ).toBeLessThan(2);
  await expect(tomatoCard.locator(".ff-expiry-unknown")).toBeVisible();
  const emptyCard = cards.filter({
    has: page.getByRole("heading", { name: "黄瓜", exact: true }),
  });
  const lowBox = await emptyCard
    .getByText("库存偏低", { exact: true })
    .boundingBox();
  const emptyBox = await emptyCard
    .getByText("已用完", { exact: true })
    .boundingBox();
  expect(
    lowBox && emptyBox && emptyBox.y >= lowBox.y + lowBox.height,
  ).toBeTruthy();
  await expect(tomatoCard.getByLabel("番茄期限与用量")).toContainText(
    "未设期限",
  );
  await expect(
    tomatoCard.getByRole("meter", { name: "番茄第1批剩余比例" }),
  ).toHaveAttribute("aria-valuenow", "100");
  const riceCard = cards.filter({
    has: page.getByRole("heading", { name: "大米", exact: true }),
  });
  await expect(riceCard.getByLabel("大米期限与用量")).toContainText("375 g");
  await expect(
    riceCard.getByRole("meter", { name: "大米第1批剩余比例" }),
  ).toHaveAttribute("aria-valuenow", "75");
  expect(
    before.batches.find((b: { id: string }) => b.id === riceBatch.id)
      .stocked_quantity,
  ).toBe("500.000");
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
  // A replenishment has its own expiry and baseline; never blend batch percentages.
  expect(
    (
      await request.post(root + "/catalog-stock", {
        headers: { ...headers, "Idempotency-Key": randomUUID() },
        data: {
          catalog_id: entries.find((x: { name: string }) => x.name === "大米")
            .id,
          quantity: "100",
          expires_on: "2099-01-01",
          bought_on: "2026-01-01",
        },
      })
    ).status(),
  ).toBe(200);
  expect(
    (
      await request.post(root + "/catalog-stock", {
        headers: { ...headers, "Idempotency-Key": randomUUID() },
        data: {
          catalog_id: entries.find((x: { name: string }) => x.name === "食盐")
            .id,
          quantity: "200",
        },
      })
    ).status(),
  ).toBe(200);
  await page.reload();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  const saltCard = page
    .locator(".ff-stock-card")
    .filter({ has: page.getByRole("heading", { name: "食盐", exact: true }) });
  await expect(saltCard.locator(".ff-condiment")).toHaveCount(0);
  await expect(saltCard.getByLabel("为食盐上传图片")).toBeAttached();
  const shelf = page.getByRole("region", { name: "常用调味品", exact: true });
  await expect(
    shelf.getByRole("heading", { name: "食用油", exact: true }),
  ).toBeVisible();
  await expect(
    shelf.getByRole("heading", { name: "酱油", exact: true }),
  ).toBeVisible();
  await expect(
    shelf.getByRole("img", { name: "食用油容器示意，未添加", exact: true }),
  ).toBeVisible();
  await expect(
    shelf.getByRole("img", { name: "食盐容器示意，待设置容量", exact: true }),
  ).toBeVisible();
  await shelf
    .getByRole("button", { name: "设置食盐容量", exact: true })
    .click();
  await shelf.getByLabel("满瓶容量 g", { exact: true }).fill("500");
  await shelf.getByRole("checkbox", { name: /我确认启用配方估算/ }).check();
  await shelf.getByRole("button", { name: "确认启用", exact: true }).click();
  const bottle = shelf
    .locator(".ff-pantry-bottle")
    .filter({ has: page.getByRole("heading", { name: "食盐", exact: true }) });
  await expect(
    bottle.getByRole("img", { name: /食盐容器示意/ }),
  ).toHaveAccessibleName(/满瓶容量 500.000 g，40%/);
  const fill = bottle.locator(".ff-container-fill");
  const fillOffset = () =>
    fill.evaluate(
      (el) => new DOMMatrixReadOnly(getComputedStyle(el).transform).m42,
    );
  await expect.poll(fillOffset).toBeCloseTo(67.2, 1);
  // A transition event on this same node catches remounts that would jump/restart the fill.
  await fill.evaluate((el) =>
    el.addEventListener(
      "transitionrun",
      () => el.setAttribute("data-test-transition", "started"),
      { once: true },
    ),
  );
  await bottle
    .getByRole("button", { name: "校准 / 管理", exact: true })
    .click();
  await bottle.getByLabel("食盐余量校准").press("Home");
  for (let step = 0; step < 50; step++)
    await bottle.getByLabel("食盐余量校准").press("ArrowRight");
  await bottle
    .getByRole("button", { name: "确认校准为 50%", exact: true })
    .click();
  await expect(
    bottle.getByRole("img", { name: /食盐容器示意/ }),
  ).toHaveAccessibleName(/剩余 250.000 g，满瓶容量 500.000 g，50%/);
  await expect(saltCard.locator(".ff-stock-quantity")).toHaveText("250 g");
  await expect(fill).toHaveAttribute("data-test-transition", "started");
  await expect.poll(fillOffset).toBeCloseTo(56, 1);
  await fill.evaluate((el) => {
    el.removeAttribute("data-test-transition");
    el.addEventListener(
      "transitionrun",
      () => el.setAttribute("data-test-transition", "started"),
      { once: true },
    );
  });
  await bottle
    .getByRole("button", { name: "校准 / 管理", exact: true })
    .click();
  await bottle.getByLabel("食盐余量校准").press("Home");
  for (let step = 0; step < 25; step++)
    await bottle.getByLabel("食盐余量校准").press("ArrowRight");
  await bottle
    .getByRole("button", { name: "确认校准为 25%", exact: true })
    .click();
  await expect(fill).toHaveAttribute("data-test-transition", "started");
  await expect.poll(fillOffset).toBeCloseTo(84, 1);
  await expect(saltCard.locator(".ff-stock-quantity")).toHaveText("125 g");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect
    .poll(() => fill.evaluate((el) => getComputedStyle(el).transitionDuration))
    .toBe("0s");
  await page.emulateMedia({ reducedMotion: "no-preference" });

  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: info.outputPath("common-seasoning-shelf.png"),
    fullPage: true,
  });
  const replenished = page
    .locator(".ff-stock-card")
    .filter({ has: page.getByRole("heading", { name: "大米", exact: true }) });
  await expect(replenished.getByLabel("大米期限与用量")).toContainText(
    "2099-01-01",
  );
  await expect(
    replenished.getByRole("meter", { name: /批剩余比例/ }),
  ).toHaveCount(2);
  await expect(
    replenished.getByRole("meter", { name: /批剩余比例/ }).first(),
  ).toHaveAttribute("aria-valuenow", "100");
  await expect(
    replenished.getByRole("meter", { name: /批剩余比例/ }).nth(1),
  ).toHaveAttribute("aria-valuenow", "75");
  await expect(
    replenished.getByRole("meter", { name: /有效期剩余比例/ }),
  ).toBeVisible();
  await expect(replenished.locator(".ff-stock-details[open]")).toHaveCount(0);
});
