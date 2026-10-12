import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { expectReadable } from "./readability";

test("personal kitchen is default, with optional family creation, join and isolated stock", async ({
  page,
  request,
}, info) => {
  const email = `personal-${randomUUID()}@example.test`,
    password = "Personal-Browser-2026!";
  // Fixture accounts use the fixed isolated API peer. UI logins still go through
  // the real gateway, keeping the complete browser suite within its normal IP budget.
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: { email, password, name: "个人食光验收" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json(),
    headers = { Authorization: `Bearer ${user.token}` };
  const login = async () => {
    await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
    await page.getByLabel("密码", { exact: true }).fill(password);
    await page.getByRole("button", { name: "登录", exact: true }).click();
    await expect(
      page.getByText("个人厨房：我的厨房", { exact: true }),
    ).toBeVisible();
  };
  await page.goto("/");
  await login();
  await expect(
    page.getByRole("heading", { name: "创建家庭", exact: true }),
  ).toHaveCount(0);
  const personal = await (
    await request.put("/api/me/personal-kitchen", { headers })
  ).json();
  expect(personal.servings).toBe(1);
  expect(personal.kind).toBe("personal");
  const catalog = await (
    await request.get("/api/ingredient-catalog", { headers })
  ).json();
  const tomato = catalog.find((x: { name: string }) => x.name === "番茄");
  const stock = await request.post(
    `/api/households/${personal.id}/catalog-stock`,
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { catalog_id: tomato.id, quantity: "200.125" },
    },
  );
  expect(stock.status()).toBe(200);
  await page.reload();
  const nav = page.getByRole("navigation", { name: "主导航" });
  await nav.getByRole("button", { name: /菜单/ }).click();
  await expect(page.getByLabel("人数", { exact: true })).toHaveValue("1");
  await page.getByRole("button", { name: /连续几餐/ }).click();
  await page.getByRole("button", { name: /展开规划/ }).click();
  await expect(page.getByLabel("每餐人数", { exact: true })).toHaveValue("1");
  await nav.getByRole("button", { name: /采购/ }).click();
  await expect(
    page.getByRole("heading", { name: "我的采购清单", exact: true }),
  ).toBeVisible();
  await nav.getByRole("button", { name: /记录/ }).click();
  await expect(
    page.getByRole("heading", { name: "我的营养雷达", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: /金额月报/ }).click();
  await expect(
    page.getByRole("heading", { name: "我的金额月报", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await expectReadable(page.locator(".ff-personal-space p").last());
  await expect(
    page.getByText("暂未加入家庭，个人厨房的所有功能已经可以使用。", {
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "个人厨房偏好", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "个人厨房设置", exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("默认用餐人数", { exact: true })).toHaveValue(
    "1",
  );
  await expect(page.getByLabel("邀请邮箱", { exact: true })).toHaveCount(0);
  const denied = await request.post(`/api/households/${personal.id}/invites`, {
    headers,
    data: { email: "fictional@example.test", role: "editor" },
  });
  expect(denied.status()).toBe(403);
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByText("创建或加入家庭", { exact: true }).click();
  await page.getByLabel("家庭名称", { exact: true }).fill("周末共享厨房");
  await page.getByRole("button", { name: "创建家庭", exact: true }).click();
  await expect(
    page.getByText("当前家庭：周末共享厨房", { exact: true }),
  ).toBeVisible();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("heading", { name: "番茄", exact: true }),
  ).toHaveCount(0);
  await page.reload();
  await expect(
    page.getByText("当前家庭：周末共享厨房", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByRole("button", { name: /使用个人厨房/ }).click();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByText("200.125 g", { exact: true }).first(),
  ).toBeVisible();
  const friend = await request.post("http://127.0.0.1:18080/api/register", {
    data: {
      email: `friend-${randomUUID()}@example.test`,
      password,
      name: "家人验收",
    },
  });
  expect(friend.status()).toBe(200);
  const friendHeaders = {
    Authorization: `Bearer ${(await friend.json()).token}`,
  };
  const shared = await request.post("/api/households", {
    headers: friendHeaders,
    data: { name: "家人的厨房", servings: 2 },
  });
  expect(shared.status()).toBe(201);
  const family = await shared.json();
  const invitation = await request.post(
    `/api/households/${family.id}/invites`,
    { headers: friendHeaders, data: { email, role: "viewer" } },
  );
  expect(invitation.status()).toBe(201);
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByText("创建或加入家庭", { exact: true }).click();
  await page
    .getByLabel("家庭邀请码", { exact: true })
    .fill((await invitation.json()).code);
  await page.getByRole("button", { name: "加入家庭", exact: true }).click();
  await expect(
    page.getByText("当前家庭：家人的厨房", { exact: true }),
  ).toBeVisible();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("button", { name: "＋ 添加食材", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByRole("button", { name: "退出登录", exact: true }).click();
  await login();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByText("200.125 g", { exact: true }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "打开个人主页" }).click();
  if (info.project.name === "mobile")
    await page.setViewportSize({ width: 320, height: 740 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page
    .locator('[aria-label="个人厨房与家庭共享"]')
    .screenshot({ path: info.outputPath("personal-and-family.png") });
});
