import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
const { version } = JSON.parse(
  readFileSync(new URL("../package.json", import.meta.url), "utf8"),
) as { version: string };

test("real login, stocked inventory and primary navigation", async ({
  page,
  request,
  browser,
}, info) => {
  await info.attach("runtime", {
    body: JSON.stringify({
      browser: browser.version(),
      project: info.project.name,
      viewport: page.viewportSize(),
    }),
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
  const tomato = (await catalog.json()).find(
    (item: { name: string }) => item.name === "番茄",
  );
  expect(tomato).toBeTruthy();
  const stocked = await request.post(
    `/api/households/${household.id}/catalog-stock`,
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { catalog_id: tomato.id, quantity: "300" },
    },
  );
  expect(stocked.ok()).toBeTruthy();

  let scriptErrors = 0;
  let serverFailures = 0;
  page.on("pageerror", () => scriptErrors++);
  page.on("response", (response) => {
    if (response.url().includes("/api/") && response.status() >= 500)
      serverFailures++;
  });
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("navigation", { name: "主导航" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "隔离验收厨房 · 今天" }),
  ).toBeVisible();
  await expect(page.getByRole("region", { name: "厨房使用指南" })).toHaveCount(
    0,
  );
  await page.screenshot({ path: info.outputPath("home.png"), fullPage: true });

  const nav = page.getByRole("navigation", { name: "主导航" });
  for (const [label, heading] of [
    ["食材", "食材库存"],
    ["菜单", "安排菜单"],
    ["采购", "共享采购清单"],
    ["记录", "厨房记录"],
  ]) {
    await nav.getByRole("button", { name: new RegExp(label) }).click();
    await expect(
      page.getByRole("heading", { name: heading, exact: true }).first(),
    ).toBeVisible();
    if (label === "食材") {
      await expect(
        page.getByText("番茄", { exact: true }).first(),
      ).toBeVisible();
      await expect(
        page.getByText("300 g", { exact: true }).first(),
      ).toBeVisible();
      await page.screenshot({
        path: info.outputPath("inventory.png"),
        fullPage: true,
      });
    }
    if (label === "记录") {
      await page.getByRole("button", { name: /食材用量/ }).click();
      await expect(
        page.getByRole("heading", { name: "食材用量", exact: true }),
      ).toBeVisible();
      // Projection is asynchronous and the UI refetches every 10 seconds.
      await expect(page.getByText("番茄", { exact: true }).first()).toBeVisible(
        { timeout: 30_000 },
      );
      await page.screenshot({
        path: info.outputPath("insights.png"),
        fullPage: true,
      });
    }
  }
  await expect(nav.getByRole("button")).toHaveCount(5);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await expect(
    page.getByText("FoodFlow v" + version, { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "家庭成员与偏好" }).click();
  await expect(
    page.getByRole("heading", { name: "家庭设置", exact: true }),
  ).toBeVisible();
  expect(scriptErrors, "browser runtime errors").toBe(0);
  expect(serverFailures, "API 5xx responses").toBe(0);
});

test("gateway HTML failure is readable and login can retry", async ({
  page,
}) => {
  let fail = true;
  await page.route("**/api/login", async (route) => {
    if (fail) {
      await route.fulfill({
        status: 502,
        contentType: "text/html",
        body: "<h1>Bad Gateway</h1>",
      });
    } else {
      await route.continue();
    }
  });
  await page.goto("/");
  await page
    .getByLabel("手机号或邮箱", { exact: true })
    .fill(`missing-${randomUUID()}@example.test`);
  await page
    .getByLabel("密码", { exact: true })
    .fill("Incorrect-Fixture-2026!");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByText("Error: 服务暂时无法连接，请稍后重试", { exact: true }),
  ).toBeVisible();
  fail = false;
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByText("Error: 手机号、邮箱或密码错误", { exact: true }),
  ).toBeVisible();
});

test("onboarding, continuous stock and configured capability states", async ({
  page,
  request,
}) => {
  const registration = await request.post("/api/register", {
    data: {
      email: `ux-${randomUUID()}@example.test`,
      password: "UX-Fixture-2026!",
      name: "交互验收",
    },
  });
  expect(registration.ok()).toBeTruthy();
  const user = await registration.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "新厨房", servings: 2 },
  });
  expect(created.ok()).toBeTruthy();
  const house = await created.json();
  await page.addInitScript(
    ({ token, household }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", household);
    },
    { token: user.token, household: house.id },
  );
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "从冰箱到餐桌，今天也好好吃饭。" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "收起指南，查看今日待办" }).click();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "新厨房 · 今天" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "从冰箱到餐桌，今天也好好吃饭。" }),
  ).toHaveCount(0);
  const nav = page.getByRole("navigation", { name: "主导航" });
  await nav.getByRole("button", { name: /食材/ }).click();
  await page.getByRole("button", { name: "＋ 添加食材" }).click();
  for (const name of ["番茄", "黄瓜"]) {
    await page
      .getByRole("button", { name: `选择${name}`, exact: true })
      .click();
    await expect(page.getByLabel("本次入库数量（g）")).toHaveValue("");
    await page.getByRole("button", { name: "100 g", exact: true }).click();
    await page.getByRole("button", { name: "确认入库", exact: true }).click();
    await expect(page.getByRole("status")).toContainText(`${name} · 100 g`);
  }
  await page.getByRole("button", { name: "关闭食材选择" }).click();
  const inventory = await request.get(`/api/households/${house.id}/inventory`, {
    headers,
  });
  const stocks = await inventory.json();
  expect(
    stocks.items.filter((i: { name: string }) =>
      ["番茄", "黄瓜"].includes(i.name),
    ),
  ).toHaveLength(2);
  expect(
    stocks.batches.every(
      (b: { expires_on: string; expiry_kind: string }) =>
        !b.expires_on && b.expiry_kind === "unknown",
    ),
  ).toBeTruthy();
  await nav.getByRole("button", { name: /采购/ }).click();
  await page.getByRole("button", { name: "查菜价 · 城市参考价格" }).click();
  await page.getByRole("button", { name: "返回采购清单" }).click();
  await expect(
    page.getByRole("heading", { name: "共享采购清单" }),
  ).toBeVisible();

  let insightsCalls = 0;
  await page.route("**/api/households/*/insights*", async (route) => {
    insightsCalls++;
    await route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "统计服务暂不可用，请稍后重试" }),
    });
  });
  let enabled = false;
  await page.route("**/api/capabilities", (route) =>
    route.fulfill({
      json: {
        vision_enabled: false,
        model_enabled: false,
        insights_enabled: enabled,
      },
    }),
  );
  await page.reload();
  await nav.getByRole("button", { name: /菜单/ }).click();
  await expect(page.getByText(/演示模式：使用示例方案/)).toBeVisible();
  await page.getByRole("button", { name: "帮我推荐菜单" }).click();
  await expect(
    page.getByRole("heading", { name: "菜单规划 · 等待确认" }),
  ).toBeVisible({ timeout: 30000 });
  await nav.getByRole("button", { name: /今天/ }).click();
  await page.getByRole("button", { name: "菜单规划 · 待确认" }).click();
  await expect(
    page.getByRole("heading", { name: "菜单规划 · 等待确认" }),
  ).toBeVisible();
  const taskList = await (
    await request.get("/api/households/" + house.id + "/jobs", { headers })
  ).json();
  const taskDetail = await (
    await request.get(
      "/api/households/" +
        house.id +
        "/jobs/" +
        taskList.find((j: { kind: string }) => j.kind === "plan").id,
      { headers },
    )
  ).json();
  const draft = await (
    await request.get(
      "/api/households/" + house.id + "/plans/" + taskDetail.result.plan_id,
      { headers },
    )
  ).json();
  const confirmed = await request.post(
    "/api/households/" +
      house.id +
      "/plans/" +
      taskDetail.result.plan_id +
      "/confirm",
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { revision: draft.revision, accept_uncertain: false },
    },
  );
  expect(confirmed.ok()).toBeTruthy();
  let completionCalls = 0;
  page.on("request", (r) => {
    if (r.method() === "POST" && /\/meals\/[^/]+\/complete$/.test(r.url()))
      completionCalls++;
  });
  await nav.getByRole("button", { name: /今天/ }).click();
  await page.getByRole("button", { name: "烹饪完成", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: /确认完成/ });
  await expect(dialog).toBeVisible();
  expect(completionCalls).toBe(0);
  await dialog.getByRole("button", { name: "继续做饭" }).click();
  await expect(dialog).toHaveCount(0);
  expect(completionCalls).toBe(0);
  await page.getByRole("button", { name: "烹饪完成", exact: true }).click();
  const [completion] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        /\/meals\/[^/]+\/complete$/.test(r.url()),
    ),
    dialog.getByRole("button", { name: "确认完成并扣库" }).click(),
  ]);
  expect([200, 409]).toContain(completion.status());
  expect(completionCalls).toBe(1);
  if (await dialog.isVisible())
    await dialog.getByRole("button", { name: "继续做饭" }).click();
  await nav.getByRole("button", { name: /记录/ }).click();
  await page.getByRole("button", { name: /食材用量/ }).click();
  await expect(
    page.getByRole("heading", { name: "食材用量尚未启用" }),
  ).toBeVisible();
  expect(insightsCalls).toBe(0);
  await page.getByRole("button", { name: /库存流水/ }).click();
  await expect(
    page.getByRole("heading", { name: "番茄", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "黄瓜", exact: true }),
  ).toBeVisible();
  enabled = true;
  await page.reload();
  await nav.getByRole("button", { name: /记录/ }).click();
  await page.getByRole("button", { name: /食材用量/ }).click();
  await expect(page.getByRole("alert")).toContainText("统计服务暂不可用");
  await expect(
    page.getByRole("heading", { name: "食材用量尚未启用" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "重试", exact: true }).click();
  await expect.poll(() => insightsCalls).toBeGreaterThan(1);
});
