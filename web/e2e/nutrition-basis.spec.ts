import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("batch edible mass confirmation preserves stock and ledger", async ({
  page,
  request,
}, info) => {
  const registered = await request.post("/api/register", {
    data: {
      email: `nutrition-${randomUUID()}@example.test`,
      password: "Nutrition-Fixture-2026!",
      name: "营养验收",
    },
  });
  expect(registered.ok()).toBeTruthy();
  const user = await registered.json(),
    headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "称量厨房", servings: 2 },
  });
  expect(created.ok()).toBeTruthy();
  const house = await created.json(),
    root = `/api/households/${house.id}`;
  const catalog = await (
      await request.get("/api/ingredient-catalog", { headers })
    ).json(),
    egg = catalog.find((x: { name: string }) => x.name === "鸡蛋");
  const stock = await request.post(root + "/catalog-stock", {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: { catalog_id: egg.id, quantity: "4" },
  });
  expect(stock.ok()).toBeTruthy();
  const batch = await stock.json();
  await page.addInitScript(
    ({ token, id }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", id);
    },
    { token: user.token, id: house.id },
  );
  await page.goto("/");
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  await page.getByRole("button", { name: "鸡蛋营养换算", exact: true }).click();
  await page.getByLabel("称量样本数量（个）").fill("2");
  await page.getByLabel("样本可食净重（g）").fill("100");
  await expect(
    page.getByRole("button", { name: "确认营养依据", exact: true }),
  ).toBeDisabled();
  await page.getByRole("checkbox", { name: /确认以上是本批称重/ }).check();
  const [saved] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" && r.url().endsWith("/nutrition-basis"),
    ),
    page.getByRole("button", { name: "确认营养依据", exact: true }).click(),
  ]);
  expect(saved.status()).toBe(200);
  await expect(
    page.getByText("当前依据：2 个 对应 100 g 可食部 · 版本 1"),
  ).toBeVisible();
  const inventory = await (
    await request.get(root + "/inventory", { headers })
  ).json();
  expect(inventory.items[0].quantity).toBe("4.000");
  const ledger = await (
    await request.get(root + "/ledger", { headers })
  ).json();
  expect(ledger).toHaveLength(1);
  const basis = await (
    await request.get(
      root + "/batches/" + batch.batch_id + "/nutrition-basis",
      { headers },
    )
  ).json();
  expect(basis.revision).toBe(1);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: info.outputPath("nutrition-basis.png"),
    fullPage: true,
  });
});
