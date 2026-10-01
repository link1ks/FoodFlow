import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("family recipe preview and scaled procurement", async ({
  page,
  request,
}) => {
  const email = `recipe-${randomUUID()}@example.test`;
  const password = "Recipe-Fixture-2026!";
  const registered = await request.post("/api/register", {
    data: { email, password, name: "菜谱验收" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "家庭菜谱", servings: 3 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json();
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "收起指南，查看今日待办" }).click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /菜单/ })
    .click();
  await page
    .getByLabel("选择菜谱手工规划")
    .selectOption("20000000-0000-4000-8000-000000000001");
  const preview = page.getByRole("region", { name: "所选菜谱详情" });
  await expect(preview).toContainText("蒜蓉菠菜 · 2 人份");
  await expect(preview).toContainText("菠菜 300.000 g");
  await expect(preview).toContainText("菠菜洗净沥干，蒜切末");
  await expect(preview).toContainText("份量和耗时为估计");
  await page.getByLabel("人数", { exact: true }).fill("3");
  await page.getByRole("button", { name: "用这道菜安排" }).click();
  await expect(
    page.getByText("晚餐 · 蒜蓉菠菜", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "确认菜单并生成采购", exact: true })
    .click();
  await expect(page.getByText(/到「今天」查看今日菜单/)).toBeVisible();
  const response = await request.get(`/api/households/${house.id}/shopping`, {
    headers,
  });
  expect(response.status()).toBe(200);
  const shopping = await response.json();
  expect(
    shopping
      .map((x: { name: string; needed: string }) => [x.name, x.needed])
      .sort(),
  ).toEqual(
    [
      ["菠菜", "450.000"],
      ["蒜", "15.000"],
    ].sort(),
  );
  const inventory = await request.get(`/api/households/${house.id}/inventory`, {
    headers,
  });
  expect(inventory.status()).toBe(200);
  expect((await inventory.json()).items).toEqual([]);
});
