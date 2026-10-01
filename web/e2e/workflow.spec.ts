import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("UI kitchen workflow purchases shortages and consumes exactly once", async ({
  page,
  request,
}, info) => {
  test.setTimeout(90_000);
  const email = `workflow-${randomUUID()}@example.test`;
  const password = "Workflow-Fixture-2026!";
  // Account creation is fixture setup; all kitchen writes below use the UI.
  const registered = await request.post("/api/register", {
    data: { email, password, name: "流程验收" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "完整流程厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json();
  const root = `/api/households/${house.id}`;
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "收起指南，查看今日待办" }).click();
  const nav = page.getByRole("navigation", { name: "主导航" });
  await nav.getByRole("button", { name: /食材/ }).click();
  await page.getByRole("button", { name: "＋ 添加食材" }).click();
  await page.getByRole("button", { name: "选择生菜", exact: true }).click();
  await page.getByLabel("本次入库数量（g）").fill("100");
  await page.getByRole("button", { name: "确认入库", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("生菜 · 100 g");
  await page.getByRole("button", { name: "关闭食材选择" }).click();
  await nav.getByRole("button", { name: /菜单/ }).click();
  await page
    .locator("select")
    .filter({
      has: page.locator('option[value="10000000-0000-4000-8000-000000000004"]'),
    })
    .first()
    .selectOption("10000000-0000-4000-8000-000000000004");
  await page.getByRole("button", { name: "用这道菜安排" }).click();
  await expect(
    page.getByText("晚餐 · 蒜香生菜", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "确认菜单并生成采购", exact: true })
    .click();
  await expect(page.getByText(/到「今天」查看今日菜单/)).toBeVisible();
  await nav.getByRole("button", { name: /采购/ }).click();
  for (const [name, quantity] of [
    ["生菜", "200.000"],
    ["蒜", "20.000"],
  ]) {
    const card = page
      .locator(".rounded-2xl")
      .filter({ has: page.getByText(name, { exact: true }) })
      .filter({ has: page.getByRole("button", { name: "勾选已买并入库" }) })
      .last();
    await card.getByRole("button", { name: "勾选已买并入库" }).click();
    const dialog = page.getByRole("dialog", { name: "采购入库确认" });
    await expect(dialog.getByLabel("实际采购量（g）")).toHaveValue(quantity);
    await dialog.getByLabel("有效期（可清空）").fill("");
    await dialog
      .getByRole("button", { name: "确认并入库", exact: true })
      .click();
    await expect(dialog).toHaveCount(0);
  }
  const readInventory = async () => {
    const response = await request.get(root + "/inventory", { headers });
    expect(response.status()).toBe(200);
    return response.json();
  };
  const before = await readInventory();
  expect(
    before.items.find((x: { name: string }) => x.name === "生菜").quantity,
  ).toBe("300.000");
  expect(
    before.items.find((x: { name: string }) => x.name === "蒜").quantity,
  ).toBe("20.000");
  await nav.getByRole("button", { name: /今天/ }).click();
  await page.getByRole("button", { name: "烹饪完成", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: /确认完成/ });
  const [completion] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        /\/meals\/[^/]+\/complete$/.test(r.url()),
    ),
    dialog.getByRole("button", { name: "确认完成并扣库" }).click(),
  ]);
  expect(completion.status()).toBe(200);
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "烹饪完成", exact: true }),
  ).toHaveCount(0);
  const after = await readInventory();
  expect(
    after.items.every((x: { quantity: string }) => x.quantity === "0.000"),
  ).toBeTruthy();
  const ledgerResponse = await request.get(root + "/ledger", { headers });
  expect(ledgerResponse.status()).toBe(200);
  const ledger = await ledgerResponse.json();
  expect(ledger).toHaveLength(6);
  expect(
    ledger
      .filter((x: { reason: string }) => x.reason === "consume")
      .map((x: { ingredient: string; quantity: string }) => [
        x.ingredient,
        x.quantity,
      ])
      .sort(),
  ).toEqual([
    ["生菜", "-100.000"],
    ["生菜", "-200.000"],
    ["蒜", "-20.000"],
  ]);
  // Replay the captured write twice; same-key replay succeeds without another deduction.
  const sent = completion.request();
  const replayHeaders = await sent.allHeaders();
  for (let i = 0; i < 2; i++) {
    const replay = await request.post(sent.url(), {
      headers: replayHeaders,
      data: sent.postDataJSON(),
    });
    expect(replay.status()).toBe(200);
  }
  expect(await readInventory()).toEqual(after);
  const replayLedger = await request.get(root + "/ledger", { headers });
  expect(await replayLedger.json()).toEqual(ledger);
  await nav.getByRole("button", { name: /记录/ }).click();
  await page.getByRole("button", { name: /库存流水/ }).click();
  await expect(page.getByText("-100 g", { exact: true })).toBeVisible();
  await expect(page.getByText("-200 g", { exact: true })).toBeVisible();
  await expect(page.getByText("-20 g", { exact: true })).toBeVisible();
  await page.screenshot({
    path: info.outputPath("successful-workflow.png"),
    fullPage: true,
  });
});
