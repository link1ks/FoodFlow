import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { expectReadable } from "./readability";

test("community cover rejection, explicit resubmission and private service failure", async ({
  page,
  request,
}, info) => {
  const registration = await request.post(
    "http://127.0.0.1:18080/api/register",
    {
      data: {
        email: `review-${randomUUID()}@example.test`,
        password: "Review-Browser-2026!",
        name: "虚构封面验收",
      },
    },
  );
  expect(registration.status()).toBe(200);
  const user = await registration.json(),
    headers = { Authorization: `Bearer ${user.token}` };
  // A legitimate fictional session avoids adding login attempts to the shared
  // gateway budget. Existing account tests exercise the actual login screen.
  await page.addInitScript(
    (token) => localStorage.setItem("ff_token", token),
    user.token,
  );
  let errors = 0;
  page.on("pageerror", () => errors++);
  const catalog = await (
    await request.get("/api/ingredient-catalog", { headers })
  ).json();
  const tomato = catalog.find((x: { name: string }) => x.name === "番茄");
  await page.goto("/");
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /社区/ })
    .click();
  await page.getByRole("button", { name: "分享我的配方" }).click();
  let dialog = page.getByRole("dialog", { name: "分享一道拿手菜" });
  const title = `初审拒绝封面 ${randomUUID().slice(0, 6)}`;
  await dialog.getByLabel("给这道菜起个名字").fill(title);
  await dialog.getByLabel("食材 1", { exact: true }).selectOption(tomato.id);
  await dialog.getByLabel("用量 1", { exact: true }).fill("100.125");
  await dialog
    .getByLabel("步骤 1", { exact: true })
    .fill("番茄洗净切块，下锅煮熟后盛出。");
  // Produce an ordinary local PNG without using remote or personal photographs.
  const png = await page.screenshot();
  await dialog
    .getByLabel("上传菜谱封面")
    .setInputFiles({
      name: "fictional-cover.png",
      mimeType: "image/png",
      buffer: png,
    });
  await expect(dialog.getByAltText("待发布的封面")).toBeVisible();
  await dialog.getByLabel(/确认公开分享这份配方/).check();
  await dialog.getByRole("button", { name: "发布这份心意" }).click();
  await expect(dialog).toHaveCount(0);
  let card = page
    .locator(".community-post-card")
    .filter({ has: page.getByRole("button", { name: title, exact: true }) });
  await expect(
    card.getByRole("status", { name: "发布初审状态" }),
  ).toContainText("需要修改");
  await expect(card).toContainText("封面包含广告信息");
  await expectReadable(card.locator(".community-publication-review p"));
  await expect(
    card.getByRole("button", { name: `收藏：${title}`, exact: true }),
  ).toBeDisabled();
  const mine = await (
      await request.get("/api/community/posts?filter=mine", { headers })
    ).json(),
    id = mine.posts[0].id;
  expect(mine.posts[0].cover_reviewed).toBe(true);
  const stranger = await (
    await request.post("http://127.0.0.1:18080/api/register", {
      data: {
        email: `viewer-${randomUUID()}@example.test`,
        password: "Review-Browser-2026!",
        name: "虚构读者",
      },
    })
  ).json();
  const otherHeaders = { Authorization: `Bearer ${stranger.token}` };
  expect(
    (
      await request.get(`/api/community/posts/${id}`, { headers: otherHeaders })
    ).status(),
  ).toBe(404);
  expect(
    (
      await request.get(`/api/community/posts/${id}/cover`, {
        headers: otherHeaders,
      })
    ).status(),
  ).toBe(404);
  await page.screenshot({
    path: info.outputPath("community-cover-rejected.png"),
    fullPage: true,
  });
  await card.getByRole("button", { name: "修改并重新提交" }).click();
  dialog = page.getByRole("dialog", { name: "修改这份配方" });
  await expect(dialog.getByLabel("用量 1", { exact: true })).toHaveValue(
    "100.125",
  );
  await expect(dialog.getByLabel(/确认公开分享这份配方/)).not.toBeChecked();
  await expect(
    dialog.getByRole("button", { name: "重新提交初审" }),
  ).toBeDisabled();
  const corrected = `封面重审通过 ${randomUUID().slice(0, 6)}`;
  await dialog.getByLabel("给这道菜起个名字").fill(corrected);
  // The deterministic model uses fixture titles to select a verdict. Keeping the
  // normalized cover proves the image is resent; this is not real image accuracy.
  await dialog.getByLabel(/确认公开分享这份配方/).check();
  await dialog.getByRole("button", { name: "重新提交初审" }).click();
  await expect(dialog).toHaveCount(0);
  card = page
    .locator(".community-post-card")
    .filter({
      has: page.getByRole("button", { name: corrected, exact: true }),
    });
  await expect(
    card.getByRole("status", { name: "发布初审状态" }),
  ).toContainText("初审通过");
  await expect(
    card.getByRole("button", { name: `收藏：${corrected}`, exact: true }),
  ).toBeEnabled();
  const approved = await (
    await request.get(`/api/community/posts/${id}`, { headers: otherHeaders })
  ).json();
  expect(approved.review_version).toBe(2);
  expect(approved.cover_reviewed).toBe(true);
  expect(
    (await (await request.get("/api/community/account", { headers })).json())
      .balance,
  ).toBe(5);
  await expectReadable(card.locator(".community-publication-review p"));
  await page.screenshot({
    path: info.outputPath("community-cover-approved.png"),
    fullPage: true,
  });
  const failed = await request.post("/api/community/posts", {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: {
      title: `初审服务异常 ${randomUUID().slice(0, 6)}`,
      servings: 1,
      minutes: 10,
      tags: ["家常"],
      ingredients: [{ catalog_id: tomato.id, quantity: "200", unit: "g" }],
      steps: ["洗净番茄，切块后煮熟。"],
      confirm_public: true,
      confirm_review: true,
    },
  });
  expect(failed.status()).toBe(200);
  const failedID = (await failed.json()).id;
  await expect
    .poll(
      async () =>
        (
          await (
            await request.get(`/api/community/posts/${failedID}`, { headers })
          ).json()
        ).review_status,
    )
    .toBe("unavailable");
  expect(
    (
      await request.get(`/api/community/posts/${failedID}`, {
        headers: otherHeaders,
      })
    ).status(),
  ).toBe(404);
  expect(
    (await (await request.get("/api/community/account", { headers })).json())
      .balance,
  ).toBe(5);
  await page.reload();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /社区/ })
    .click();
  await page.getByRole("button", { name: "我的配方", exact: true }).click();
  await expect(
    page
      .getByRole("status", { name: "发布初审状态" })
      .filter({ hasText: "审核暂不可用" }),
  ).toBeVisible();
  if (info.project.name === "mobile")
    await page.setViewportSize({ width: 320, height: 740 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("community-review-statuses.png"),
    fullPage: true,
  });
  expect(errors).toBe(0);
});
