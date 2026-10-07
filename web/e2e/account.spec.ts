import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("saved recovery code resets an email account and revokes all previous sessions", async ({
  page,
  request,
}) => {
  const email = `recovery-${randomUUID()}@example.test`,
    password = "Recovery-Fixture-2026!",
    next = "Recovery-New-2026!";
  const registered = await request.post("/api/register", {
    data: { email, password, name: "恢复码验收" },
  });
  expect(registered.ok()).toBeTruthy();
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "恢复码厨房", servings: 2 },
  });
  expect(created.ok()).toBeTruthy();
  const house = await created.json();
  await page.addInitScript(
    ({ token, id }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", id);
    },
    { token: user.token, id: house.id },
  );
  await page.goto("/");
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await expect(
    page.getByRole("heading", { name: "账号恢复码", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "AI 调用额度", exact: true }),
  ).toBeVisible();
  await page.getByLabel("生成恢复码的当前密码", { exact: true }).fill(password);
  await page
    .getByRole("checkbox", {
      name: "确认重新生成，旧恢复码全部失效。",
      exact: true,
    })
    .check();
  await page
    .getByRole("button", { name: "生成新的恢复码", exact: true })
    .click();
  const code = await page
    .getByTestId("issued-recovery-codes")
    .locator("code")
    .first()
    .innerText();
  await page
    .getByRole("button", { name: "我已妥善保存，隐藏恢复码", exact: true })
    .click();
  await page.getByRole("button", { name: "退出登录", exact: true }).click();
  await page.getByRole("button", { name: "忘记密码", exact: true }).click();
  await page.getByLabel("找回账号的邮箱或手机号", { exact: true }).fill(email);
  await page.getByLabel("已保存的恢复码", { exact: true }).fill(code);
  await page.getByLabel("新密码", { exact: true }).fill(next);
  await expect(
    page.getByRole("button", { name: "确认用恢复码重置", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("checkbox", {
      name: "确认重置密码，所有设备和剩余恢复码将失效。",
      exact: true,
    })
    .check();
  await page
    .getByRole("button", { name: "确认用恢复码重置", exact: true })
    .click();
  await expect(page.getByText("密码已重置，请使用新密码登录。")).toBeVisible();
  expect((await request.get("/api/me", { headers })).status()).toBe(401);
  expect(
    (
      await request.post("/api/auth/recovery/reset", {
        data: { account: email, code, password: next, confirm: true },
      })
    ).status(),
  ).toBe(400);
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(next);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "打开个人主页" }),
  ).toBeVisible();
});

test("account recovery is unavailable without SMS and never resets on a checkbox alone", async ({
  page,
}) => {
  let resets = 0;
  page.on("request", (r) => {
    if (r.url().endsWith("/auth/password/reset")) resets++;
  });
  await page.goto("/");
  await page.getByRole("button", { name: "忘记密码", exact: true }).click();
  await expect(page.getByRole("heading", { name: "找回密码" })).toBeVisible();
  await page
    .getByRole("button", { name: "使用手机号短信", exact: true })
    .click();
  await page.getByLabel("已验证手机号", { exact: true }).fill("13800138888");
  await page
    .getByLabel("新密码", { exact: true })
    .fill("Recovery-Fixture-2026!");
  await expect(
    page.getByText("短信服务尚未配置，此操作暂不可用。请保留现有登录方式。"),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "获取验证码" })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await expect(
    page.getByRole("button", { name: "确认重置密码", exact: true }),
  ).toBeDisabled();
  expect(resets).toBe(0);
  await page.getByRole("button", { name: "返回登录" }).click();
  await expect(
    page.getByRole("button", { name: "登录", exact: true }),
  ).toBeVisible();
});

test("email account merge explains scope and requires configured SMS", async ({
  page,
  request,
}) => {
  const registration = await request.post("/api/register", {
    data: {
      email: `account-${randomUUID()}@example.test`,
      password: "Account-Fixture-2026!",
      name: "账号验收",
    },
  });
  expect(registration.ok()).toBeTruthy();
  const user = await registration.json(),
    headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "账号厨房", servings: 2 },
  });
  expect(created.ok()).toBeTruthy();
  const house = await created.json();
  await page.addInitScript(
    ({ token, id }) => {
      localStorage.setItem("ff_token", token);
      localStorage.setItem("ff_household", id);
    },
    { token: user.token, id: house.id },
  );
  await page.goto("/");
  await page.getByRole("button", { name: "打开个人主页" }).click();
  await page.getByRole("button", { name: "开始合并账号", exact: true }).click();
  await page.getByLabel("来源已验证手机号账号").fill("13800138888");
  await expect(page.getByLabel("合并手机号验证码")).toBeVisible();
  await page.getByRole("checkbox", { name: /确认合并/ }).check();
  await expect(
    page.getByRole("button", { name: "确认合并账号", exact: true }),
  ).toBeDisabled();
  await expect(
    page
      .getByText("短信服务尚未配置，此操作暂不可用。请保留现有登录方式。")
      .last(),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
});
