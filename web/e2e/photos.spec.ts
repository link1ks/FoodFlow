import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
const photos = JSON.parse(
  readFileSync(new URL("../src/catalogPhotos.json", import.meta.url), "utf8"),
) as { name: string; src: string }[];

test("studio catalogue images, family upload priority and failed-image fallback", async ({
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
  const photoChecks = await page.evaluate(
    async (items) =>
      Promise.all(
        items.map(async ({ src }) => {
          const image = new Image();
          image.src = src;
          await image.decode();
          const canvas = document.createElement("canvas");
          canvas.width = 480;
          canvas.height = 360;
          const context = canvas.getContext("2d")!;
          context.drawImage(image, 0, 0);
          const corners = [
            [0, 0],
            [479, 0],
            [0, 359],
            [479, 359],
          ].map(([x, y]) => Array.from(context.getImageData(x, y, 1, 1).data));
          return {
            width: image.naturalWidth,
            height: image.naturalHeight,
            corners,
          };
        }),
      ),
    photos,
  );
  for (const photoCheck of photoChecks) {
    expect([photoCheck.width, photoCheck.height]).toEqual([480, 360]);
    for (const corner of photoCheck.corners) {
      for (const [index, value] of [250, 248, 243].entries())
        expect(Math.abs(corner[index] - value)).toBeLessThanOrEqual(4);
      expect(corner[3]).toBe(255);
    }
  }
  await page.getByLabel("手机号或邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "打开个人主页", exact: true }).click();
  await page
    .getByRole("button", { name: "使用家庭：照片验收厨房", exact: true })
    .click();
  const nav = page.getByRole("navigation", { name: "主导航" });
  await expect(nav).toBeVisible();
  await expect(page.locator(".ff-header")).toHaveCSS(
    "background-color",
    "rgb(255, 249, 237)",
  );
  await nav.getByRole("button", { name: /食材/ }).click();
  const photo = photos.find((item) => item.name === "番茄")!;
  const reference = page.getByRole("img", {
    name: "番茄参考图",
    exact: true,
  });
  await expect(reference).toHaveAttribute("src", photo.src);
  await expect(reference).not.toHaveAttribute("title");
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
  await expect(picker.getByRole("link", { name: "图片说明" })).toHaveAttribute(
    "href",
    "/ingredient-photos/credits.html",
  );
  await expect
    .poll(() =>
      picker
        .getByRole("img", { name: "番茄参考图", exact: true })
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
    page.getByRole("button", { name: "移除番茄照片", exact: true }).click(),
  ]);
  expect(removed.status()).toBe(204);
  await expect(reference).toHaveAttribute("src", photo.src);
  await expect(reference).not.toHaveAttribute("title");
  await page.route("**" + photo.src, (route) => route.abort());
  await page.reload();
  await nav.getByRole("button", { name: /食材/ }).click();
  await expect(
    page.getByRole("img", { name: "番茄暂无参考图", exact: true }),
  ).toHaveText("暂无参考图");
});
