import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("menu requires a deliberate meal selection and cancelled recommendations stay dismissed", async ({
  page,
  request,
}) => {
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: {
      email: `menu-cancel-${randomUUID()}@example.test`,
      password: "Menu-Fixture-2026!",
      name: "菜单取消验收",
    },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "菜单取消厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json();
  const root = `/api/households/${house.id}`;
  await page.addInitScript(
    ({ token, id }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", id);
      localStorage.setItem("ff_workspace_v2", id);
    },
    { token: user.token, id: house.id },
  );
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "主导航" });
  await nav.getByRole("button", { name: /菜单/ }).click();
  const recommend = page.getByRole("button", {
    name: "帮我推荐菜单",
    exact: true,
  });
  await expect(recommend).toBeDisabled();
  expect(await (await request.get(root + "/jobs", { headers })).json()).toEqual(
    [],
  );
  await page
    .getByRole("combobox", { name: "餐次", exact: true })
    .selectOption("dinner");
  await expect(recommend).toBeEnabled();
  await recommend.click();
  await page
    .getByRole("button", { name: "取消推荐方案", exact: true })
    .click({ timeout: 30_000 });
  await expect(
    page.getByRole("heading", { name: "菜单规划 · 等待确认" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "确认菜单并生成采购", exact: true }),
  ).toHaveCount(0);
  await page.reload();
  await nav.getByRole("button", { name: /菜单/ }).click();
  await expect(
    page.getByRole("button", { name: "取消推荐方案", exact: true }),
  ).toHaveCount(0);
  const jobs = await (await request.get(root + "/jobs", { headers })).json();
  expect(jobs).toHaveLength(1);
  expect(jobs[0].status).toBe("cancelled");
  const plans = await (await request.get(root + "/plans", { headers })).json();
  expect(plans).toHaveLength(1);
  expect(plans[0].status).toBe("rejected");
  expect(
    (await (await request.get(root + "/inventory", { headers })).json())
      .batches,
  ).toHaveLength(0);
});
