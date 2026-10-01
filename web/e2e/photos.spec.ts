import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
const photos = JSON.parse(
  readFileSync(new URL("../src/catalogPhotos.json", import.meta.url), "utf8"),
) as { name: string; src: string }[];

test("real catalogue photos, family upload priority and failed-image fallback", async ({
  page,
  request,
}, info) => {
  const email = `photos-${randomUUID()}@example.test`,
    password = "Photos-Fixture-2026!";
  const registration = await request.post("/api/register", {
    data: { email, password, name: "照片验收" },
  });
  expect(registration.status()).toBe(200);
  const headers = {
    Authorization: `Bearer ${(await registration.json()).token}`,
  };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "照片验收厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const house = await created.json();
  const catalog = await request.get("/api/ingredient-catalog", { headers });
  expect(catalog.status()).toBe(200);
  const tomato = (await catalog.json()).find(
    (item: { name: string }) => item.name === "番茄",
  );
  const stocked = await request.post(
    `/api/households/${house.id}/catalog-stock`,
    {
      headers: { ...headers, "Idempotency-Key": randomUUID() },
      data: { catalog_id: tomato.id, quantity: "100" },
    },
  );
  expect(stocked.status()).toBe(200);
  await page.goto("/");
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const nav = page.getByRole("navigation", { name: "主导航" });
  await expect(nav).toBeVisible();
  await nav.getByRole("button", { name: /食材/ }).click();
  const photo = photos.find((item) => item.name === "番茄")!;
  const reference = page.getByRole("img", {
    name: "番茄参考照片",
    exact: true,
  });
  await expect(reference).toHaveAttribute("src", photo.src);
  await expect
    .poll(() =>
      reference.evaluate(
        (img: HTMLImageElement) => img.complete && img.naturalWidth > 0,
      ),
    )
    .toBeTruthy();
  await page.getByRole("button", { name: "＋ 添加食材" }).click();
  const picker = page.getByRole("dialog", { name: "选择食材" });
  await expect(picker.getByRole("button", { name: /^选择/ })).toHaveCount(91);
  await expect(
    picker.getByRole("link", { name: "照片来源与授权" }),
  ).toHaveAttribute("href", "/ingredient-photos/credits.html");
  await expect
    .poll(() =>
      picker
        .getByRole("img", { name: "番茄参考照片", exact: true })
        .evaluate(
          (img: HTMLImageElement) => img.complete && img.naturalWidth > 0,
        ),
    )
    .toBeTruthy();
  await page.screenshot({
    path: info.outputPath("ingredient-picker-photos.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "关闭食材选择" }).click();
  const [uploaded] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        /\/ingredients\/[^/]+\/image$/.test(r.url()),
    ),
    page.getByLabel("为番茄上传图片").setInputFiles({
      name: "tomato.webp",
      mimeType: "image/webp",
      buffer: readFileSync(new URL("../public" + photo.src, import.meta.url)),
    }),
  ]);
  expect(uploaded.status()).toBe(200);
  await expect(
    page.getByRole("img", { name: "番茄家庭照片", exact: true }),
  ).toHaveAttribute("src", /^blob:/);
  await expect
    .poll(() =>
      page
        .getByRole("img", { name: "番茄家庭照片", exact: true })
        .evaluate(
          (img: HTMLImageElement) => img.complete && img.naturalWidth > 0,
        ),
    )
    .toBeTruthy();
  const [removed] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "DELETE" &&
        /\/ingredients\/[^/]+\/image$/.test(r.url()),
    ),
    page.getByRole("button", { name: "移除", exact: true }).click(),
  ]);
  expect(removed.status()).toBe(204);
  await expect(reference).toHaveAttribute("src", photo.src);
  await page.route("**" + photo.src, (route) => route.abort());
  await page.reload();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("img", { name: "番茄示意插画", exact: true }),
  ).toHaveAttribute("src", /^data:image\/svg\+xml/);
});
