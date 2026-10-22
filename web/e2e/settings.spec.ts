import {
  test,
  expect,
  type Page,
  type APIRequestContext,
} from "@playwright/test";
import { randomUUID } from "node:crypto";

async function openSettings(page: Page, request: APIRequestContext) {
  const email = `settings-${randomUUID()}@example.test`;
  const password = "Settings-Fixture-2026!";
  // Settings scenarios reuse a real API-issued session. Dedicated account and
  // kitchen scenarios cover login without exhausting the shared proxy budget.
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: { email, password, name: "设置体验验收" },
  });
  expect(registered.status()).toBe(200);
  const { token } = await registered.json();
  const headers = { Authorization: `Bearer ${token}` };
  const personal = await request.put("/api/me/personal-kitchen", { headers });
  expect(personal.ok()).toBeTruthy();
  const kitchen = await personal.json();
  const root = `/api/households/${kitchen.id}`;
  await page.addInitScript(
    (session) => localStorage.setItem("ff_token", session),
    token,
  );
  await page.goto("/");
  await page.getByRole("button", { name: "打开个人主页", exact: true }).click();
  await page.getByRole("button", { name: "个人厨房偏好", exact: true }).click();
  await expect(page.getByLabel("名称", { exact: true })).toHaveValue(
    kitchen.name,
  );
  return { headers, kitchen, root };
}

test("kitchen settings preserve unsaved edits during refresh and reset on kitchen switch", async ({
  page,
  request,
}) => {
  const { headers, root } = await openSettings(page, request);
  await page.getByLabel("名称", { exact: true }).fill("还没保存的厨房名称");
  await page.getByLabel("默认用餐人数", { exact: true }).fill("3");
  await page
    .getByLabel("饮食偏好与忌口", { exact: true })
    .fill("本地正在编辑的偏好");
  const updated = await request.patch(root, {
    headers,
    data: {
      name: "其他会话的新名称",
      servings: 4,
      preferences: "其他会话更新",
      excluded_ingredients: ["花生"],
    },
  });
  expect(updated.ok()).toBeTruthy();
  const refreshed = page.waitForResponse(
    (r) => r.url().endsWith(root) && r.request().method() === "GET",
  );
  await page.getByRole("button", { name: "刷新", exact: true }).click();
  await refreshed;
  // This untouched field proves the changed server response reached the UI.
  await expect(
    page.getByLabel("例如：鸡蛋、花生", { exact: true }),
  ).toHaveValue("花生");
  await expect(page.getByLabel("名称", { exact: true })).toHaveValue(
    "还没保存的厨房名称",
  );
  await expect(page.getByLabel("默认用餐人数", { exact: true })).toHaveValue(
    "3",
  );
  await expect(page.getByLabel("饮食偏好与忌口", { exact: true })).toHaveValue(
    "本地正在编辑的偏好",
  );

  const created = await request.post("/api/households", {
    headers,
    data: { name: "另一间验收厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const family = await created.json();
  await page.getByRole("button", { name: "刷新", exact: true }).click();
  await expect(
    page.getByRole("option", { name: "家庭 · 另一间验收厨房", exact: true }),
  ).toBeAttached();
  await page
    .getByRole("combobox", { name: "切换厨房", exact: true })
    .selectOption(family.id);
  await expect(page.getByLabel("名称", { exact: true })).toHaveValue(
    "另一间验收厨房",
  );
  await expect(page.getByLabel("默认用餐人数", { exact: true })).toHaveValue(
    "2",
  );
  await expect(page.getByLabel("饮食偏好与忌口", { exact: true })).toHaveValue(
    "",
  );
});

test("kitchen settings report save progress, recover from errors and confirm persisted values", async ({
  page,
  request,
}) => {
  const { headers, root } = await openSettings(page, request);
  await page.getByLabel("名称", { exact: true }).fill("保存反馈厨房");
  let releaseSave: (() => void) | undefined;
  let writes = 0;
  const gate = new Promise<void>((resolve) => {
    releaseSave = resolve;
  });
  await page.route(`**${root}`, async (route) => {
    if (route.request().method() !== "PATCH") return route.continue();
    writes++;
    if (writes === 1)
      return route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: "验收模拟：暂时无法保存" }),
      });
    await gate;
    await route.continue();
  });
  await page.getByRole("button", { name: "保存设置", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("验收模拟：暂时无法保存");
  await page.getByRole("button", { name: "保存设置", exact: true }).click();
  try {
    await expect(
      page.getByRole("button", { name: "保存中…", exact: true }),
    ).toBeDisabled();
    await expect(page.getByLabel("名称", { exact: true })).toBeDisabled();
    await expect(page.getByRole("alert")).toHaveCount(0);
    expect(writes).toBe(2);
  } finally {
    releaseSave?.();
  }
  await expect(page.getByRole("status")).toContainText("厨房设置已保存");
  await expect(
    page.getByRole("button", { name: "保存设置", exact: true }),
  ).toBeEnabled();
  const stored = await request.get(root, { headers });
  expect((await stored.json()).name).toBe("保存反馈厨房");
  await page.getByLabel("名称", { exact: true }).fill("继续修改");
  await expect(page.getByText("厨房设置已保存", { exact: true })).toHaveCount(
    0,
  );
});
