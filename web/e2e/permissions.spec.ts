import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("household switching and member downgrade preserve server authorization", async ({
  page,
  request,
}) => {
  const password = "Permissions-Fixture-2026!";
  async function account(name: string) {
    const email = `permissions-${randomUUID()}@example.test`;
    const registration = await request.post("/api/register", {
      data: { email, password, name },
    });
    expect(registration.status()).toBe(200);
    return { email, ...(await registration.json()) };
  }
  const owner = await account("户主验收"),
    member = await account("成员验收");
  const ownerHeaders = { Authorization: `Bearer ${owner.token}` },
    memberHeaders = { Authorization: `Bearer ${member.token}` };
  async function household(name: string, headers: Record<string, string>) {
    const created = await request.post("/api/households", {
      headers,
      data: { name, servings: 2 },
    });
    expect(created.status()).toBe(201);
    return created.json();
  }
  const first = await household("共享厨房", ownerHeaders),
    second = await household("成员自己的厨房", memberHeaders);
  const invitation = await request.post(`/api/households/${first.id}/invites`, {
    headers: ownerHeaders,
    data: { email: member.email, role: "editor" },
  });
  expect(invitation.status()).toBe(201);
  const accepted = await request.post("/api/invites/accept", {
    headers: memberHeaders,
    data: { code: (await invitation.json()).code },
  });
  expect(accepted.status()).toBe(200);
  const catalog = await request.get("/api/ingredient-catalog", {
    headers: ownerHeaders,
  });
  expect(catalog.status()).toBe(200);
  const foods = await catalog.json();
  for (const [house, name, headers] of [
    [first, "番茄", ownerHeaders],
    [second, "黄瓜", memberHeaders],
  ] as const) {
    const stocked = await request.post(
      `/api/households/${house.id}/catalog-stock`,
      {
        headers: { ...headers, "Idempotency-Key": randomUUID() },
        data: {
          catalog_id: foods.find((x: { name: string }) => x.name === name).id,
          quantity: "100",
        },
      },
    );
    expect(stocked.status()).toBe(200);
  }
  async function login(email: string) {
    await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
    await page.getByLabel("密码", { exact: true }).fill(password);
    await page.getByRole("button", { name: "登录", exact: true }).click();
    await expect(
      page.getByRole("navigation", { name: "主导航" }),
    ).toBeVisible();
  }
  async function settings() {
    await page.getByRole("button", { name: "打开个人主页" }).click();
    await page.getByRole("button", { name: "家庭成员与偏好" }).click();
    await expect(
      page.getByRole("heading", { name: "家庭设置", exact: true }),
    ).toBeVisible();
  }
  await page.goto("/");
  await login(owner.email);
  await settings();
  const [downgrade] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "PATCH" &&
        r.url().endsWith(`/members/${member.user_id}`),
    ),
    page.getByLabel("修改 成员验收 的角色").selectOption("viewer"),
  ]);
  expect(downgrade.status()).toBe(204);
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByRole("button", { name: "退出登录", exact: true }).click();
  await login(member.email);
  await settings();
  await page.getByLabel("切换家庭").selectOption(first.id);
  const nav = page.getByRole("navigation", { name: "主导航" });
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("heading", { name: "番茄", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "黄瓜", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole("button", { name: "＋ 添加食材" })).toHaveCount(
    0,
  );
  const denied = await request.post(
    `/api/households/${first.id}/catalog-stock`,
    {
      headers: { ...memberHeaders, "Idempotency-Key": randomUUID() },
      data: { catalog_id: foods[0].id, quantity: "100" },
    },
  );
  expect(denied.status()).toBe(403);
  await settings();
  await page.getByLabel("切换家庭").selectOption(second.id);
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("heading", { name: "黄瓜", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "番茄", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole("button", { name: "＋ 添加食材" })).toBeVisible();
});
