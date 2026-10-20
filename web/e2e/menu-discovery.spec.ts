import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("menu discovery browses 36 photos and replaces one dish without losing the meal", async ({
  page,
  request,
}, info) => {
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: {
      email: `discovery-${randomUUID()}@example.test`,
      password: "Discovery-Fixture-2026!",
      name: "选菜验收",
    },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "选菜厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json(),
    root = `/api/households/${house.id}`;
  await page.addInitScript(
    ({ token, id }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", id);
      localStorage.setItem("ff_workspace_v2", id);
    },
    { token: user.token, id: house.id },
  );
  await page.goto("/");
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /菜单/ })
    .click();
  await expect(page.locator(".recipe-tile")).toHaveCount(3);
  await expect(page.locator(".recipe-tile img")).toHaveCount(3);
  const firstPhoto = await page
    .locator(".recipe-tile img")
    .first()
    .boundingBox();
  expect(firstPhoto!.y).toBeLessThan(page.viewportSize()!.height - 160);
  await expect
    .poll(() =>
      page
        .locator(".recipe-tile img")
        .evaluateAll((images) =>
          images.every((img) => (img as HTMLImageElement).naturalWidth === 480),
        ),
    )
    .toBe(true);
  const before = await page.locator(".recipe-tile h3").allTextContents();
  await page.getByRole("button", { name: "换一组", exact: true }).click();
  expect(await page.locator(".recipe-tile h3").allTextContents()).not.toEqual(
    before,
  );
  expect(await (await request.get(root + "/jobs", { headers })).json()).toEqual(
    [],
  );
  expect(
    await (await request.get(root + "/plans", { headers })).json(),
  ).toEqual([]);
  await page.screenshot({
    path: `../.cache/harness/menu-picks-${info.project.name}.png`,
    fullPage: true,
  });
  await page.getByRole("button", { name: /自己逛菜谱/ }).click();
  expect(await page.locator(".recipe-tile").count()).toBeGreaterThanOrEqual(36);
  await page.getByRole("button", { name: "汤", exact: true }).click();
  expect(await page.locator(".recipe-tile").count()).toBeGreaterThanOrEqual(5);
  await page.getByRole("button", { name: "全部", exact: true }).click();
  await page.getByLabel("搜索菜名或食材").fill("蒜蓉菠菜");
  await page.getByRole("button", { name: "加入蒜蓉菠菜", exact: true }).click();
  await page.getByLabel("搜索菜名或食材").fill("番茄炒蛋");
  await page.getByRole("button", { name: "加入番茄炒蛋", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "查看这一餐", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "查看这一餐", exact: true }).click();
  const tray = page.getByRole("region", { name: "这一餐已选菜" });
  await expect(tray.locator("li")).toHaveCount(2);
  await tray.getByRole("button", { name: "替换蒜蓉菠菜", exact: true }).click();
  await page.getByLabel("搜索菜名或食材").fill("清炒芦笋");
  await page.getByRole("button", { name: "换成清炒芦笋", exact: true }).click();
  await expect(tray).toContainText("番茄炒蛋");
  await expect(tray).toContainText("清炒芦笋");
  await expect(tray).not.toContainText("蒜蓉菠菜");
  await expect(tray.locator("li")).toHaveCount(2);
  await page.getByLabel("搜索菜名或食材").fill("不存在的菜");
  await expect(
    page.getByText("这次还没找到合适的菜", { exact: true }),
  ).toBeVisible();
  await expect(tray.locator("li")).toHaveCount(2);
  await page.locator("summary").filter({ hasText: "单道最多" }).click();
  await page.getByLabel("这餐避开的食材").fill("鸡蛋");
  await page.getByRole("button", { name: "晚餐", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "用这些菜安排一餐" }),
  ).toBeDisabled();
  await page.getByLabel("这餐避开的食材").fill("");
  await page.getByRole("button", { name: "用这些菜安排一餐" }).click();
  await expect(
    page.getByRole("heading", { name: "这一餐，核对一下" }),
  ).toBeVisible();
  const plans = await (await request.get(root + "/plans", { headers })).json();
  expect(plans).toHaveLength(1);
  const plan = await (
    await request.get(root + "/plans/" + plans[0].id, { headers })
  ).json();
  expect(plan.meals).toHaveLength(1);
  expect(plan.meals[0].title).toBe("清炒芦笋");
  expect(plan.meals[0].additional_recipes[0].title).toBe("番茄炒蛋");
  expect(plan.status).toBe("draft");
  expect(
    (await (await request.get(root + "/inventory", { headers })).json()).items,
  ).toEqual([]);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByLabel("搜索菜名或食材").fill("");
  await page.screenshot({
    path: `../.cache/harness/menu-browse-${info.project.name}.png`,
    fullPage: true,
  });
  // A broken generated asset must show a readable fallback, not a broken image.
  await page.route("**/recipe-photos/*.webp", (route) => route.abort());
  await page.reload();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /菜单/ })
    .click();
  await expect(page.getByText("暂无菜品参考图").first()).toBeVisible();
});
