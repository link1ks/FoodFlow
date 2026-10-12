import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { expectReadable } from "./readability";

test("community recipe publishing, private saves and weekly rewards", async ({
  page,
  request,
}, info) => {
  const email = `community-${randomUUID()}@example.test`;
  const password = "Community-Browser-2026!";
  const registered = await request.post("/api/register", {
    data: { email, password, name: "社区验收厨房" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const catalogue = await request.get("/api/ingredient-catalog", { headers });
  const tomato = (await catalogue.json()).find(
    (item: { name: string }) => item.name === "番茄",
  );
  const title = `一碗番茄的心意 ${randomUUID().slice(0, 6)}`;
  let scriptErrors = 0,
    serverFailures = 0;
  page.on("pageerror", () => scriptErrors++);
  page.on("response", (r) => {
    if (r.url().includes("/api/community/") && r.status() >= 500)
      serverFailures++;
  });
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /社区/ })
    .click();
  await expect(
    page.getByRole("heading", { name: /让你的拿手菜/ }),
  ).toBeVisible();
  for (const selector of [
    ".community-hero .community-eyebrow",
    ".community-hero .ff-button",
    ".community-tabs button.active",
    ".community-tabs button:not(.active)",
    ".community-week-preview > p",
    ".community-contribution-card > p",
  ]) {
    await expectReadable(page.locator(selector).first());
  }
  await page.getByRole("button", { name: "分享我的配方" }).click();
  const dialog = page.getByRole("dialog", { name: "分享一道拿手菜" });
  await dialog.getByLabel("给这道菜起个名字").fill(title);
  await dialog
    .getByLabel("厨房里的小故事")
    .fill("虚构验收配方：下班后，把平常的食材做成一碗热乎的晚餐。");
  await dialog.getByLabel("食材 1", { exact: true }).selectOption(tomato.id);
  await dialog.getByLabel("用量 1", { exact: true }).fill("200.125");
  await dialog
    .getByLabel("步骤 1", { exact: true })
    .fill("番茄洗净切块，保留汁水。");
  await dialog.getByRole("button", { name: "添加下一步" }).click();
  await dialog
    .getByLabel("步骤 2", { exact: true })
    .fill("下锅煮熟，确认熟透后盛出。");
  await expect(
    dialog.getByRole("button", { name: "发布这份心意" }),
  ).toBeDisabled();
  await dialog.getByLabel(/确认公开分享这份配方/).check();
  await dialog.getByRole("button", { name: "发布这份心意" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: title, exact: true }),
  ).toBeVisible();
  const feed = await request.get("/api/community/posts?filter=mine", {
    headers,
  });
  await expectReadable(
    page
      .locator(".community-post-card")
      .first()
      .locator(".community-post-story"),
  );
  const post = (await feed.json()).posts[0];
  expect(post.ingredients[0].quantity).toBe("200.125");
  await expect(
    page.getByRole("status", { name: "发布初审状态" }).first(),
  ).toContainText("初审通过");
  await expect(
    page.getByRole("button", { name: `点赞：${title}`, exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("button", { name: `收藏：${title}`, exact: true })
    .click();
  await page.getByRole("button", { name: "我的收藏", exact: true }).click();
  await expect(
    page.getByRole("button", { name: title, exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: `查看配方：${title}` }).click();
  const detail = page.getByRole("dialog", { name: title, exact: true });
  await expect(detail).toContainText("200.125");
  await expect(detail).toContainText("下锅煮熟");
  await detail.getByRole("button", { name: "关闭窗口" }).click();
  const stranger = await request.post("/api/register", {
    data: {
      email: `reader-${randomUUID()}@example.test`,
      password,
      name: "虚构读者",
    },
  });
  expect(stranger.status()).toBe(200);
  const strangerHeaders = {
    Authorization: `Bearer ${(await stranger.json()).token}`,
  };
  const privateFeed = await request.get("/api/community/posts?filter=saved", {
    headers: strangerHeaders,
  });
  expect((await privateFeed.json()).posts).toEqual([]);
  const sharedTitle = `读者的拿手配方 ${randomUUID().slice(0, 6)}`;
  const shared = await request.post("/api/community/posts", {
    headers: { ...strangerHeaders, "Idempotency-Key": randomUUID() },
    data: {
      title: sharedTitle,
      servings: 2,
      minutes: 10,
      tags: ["家常"],
      ingredients: [{ catalog_id: tomato.id, quantity: "150", unit: "g" }],
      steps: ["洗净番茄，切块后下锅煮熟。"],
      confirm_public: true,
      confirm_review: true,
    },
  });
  expect(shared.status()).toBe(200);
  const sharedID = (await shared.json()).id;
  await expect
    .poll(
      async () =>
        (
          await (
            await request.get(`/api/community/posts/${sharedID}`, {
              headers: strangerHeaders,
            })
          ).json()
        ).review_status,
    )
    .toBe("approved");
  await page.getByRole("button", { name: "最新分享", exact: true }).click();
  await page
    .getByRole("button", { name: `点赞：${sharedTitle}`, exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: `取消点赞：${sharedTitle}`, exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  const review = await request.post(
    `/api/community/candidates/${randomUUID()}/review`,
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { accept: true, confirm: true },
    },
  );
  expect(review.status()).toBe(403);
  await page.getByRole("tab", { name: "每周精选", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "一周的心意，三道好配方" }),
  ).toBeVisible();
  const weeklyResponse = await request.get("/api/community/weekly", {
    headers,
  });
  const weeklyBoard = await weeklyResponse.json();
  expect(weeklyBoard.current.length).toBeGreaterThan(0);
  await expect(
    page.getByRole("heading", {
      name: weeklyBoard.current[0].title,
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "生成上周候选" })).toHaveCount(
    0,
  );
  await page.getByRole("tab", { name: "积分小铺", exact: true }).click();
  await expect(page.getByRole("heading", { name: "新芽创作者" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "兑换", exact: true }).first(),
  ).toBeDisabled();
  await expect(
    page.locator(".community-ledger-row").filter({ hasText: "分享完整配方" }),
  ).toBeVisible();
  await page.screenshot({
    path: info.outputPath("community-shop.png"),
    fullPage: true,
  });
  await page.getByRole("tab", { name: "厨房动态", exact: true }).click();
  await page.screenshot({
    path: info.outputPath("community-feed.png"),
    fullPage: true,
  });
  // Both household and no-household navigation expose the same global community.
  const household = await request.post("/api/households", {
    headers,
    data: { name: "社区验收家庭", servings: 2 },
  });
  expect(household.ok()).toBeTruthy();
  await page.reload();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /社区/ })
    .click();
  await expect(
    page.getByRole("heading", { name: /让你的拿手菜/ }),
  ).toBeVisible();
  if (info.project.name === "mobile") {
    await page.setViewportSize({ width: 320, height: 740 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
    await page.getByRole("button", { name: "分享我的配方" }).click();
    await expect(
      page.getByRole("dialog", { name: "分享一道拿手菜" }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: info.outputPath("community-publish-narrow.png"),
      fullPage: true,
    });
    await page.getByRole("button", { name: "关闭窗口" }).click();
  }
  await page.screenshot({
    path: info.outputPath("community-navigation.png"),
    fullPage: true,
  });
  expect(scriptErrors).toBe(0);
  expect(serverFailures).toBe(0);
});
