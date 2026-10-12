import { test, expect } from "@playwright/test";
import { expectReadable } from "./readability";

test("login text, field outlines and secondary actions stay distinct", async ({
  page,
}, info) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "欢迎回来" })).toBeVisible();
  for (const selector of [
    ".ff-auth-story > p:first-of-type",
    ".ff-auth-story > p:last-of-type",
    ".ff-auth-card .ff-field:first-of-type",
    ".ff-auth-card form > p",
    ".ff-auth-link:first-of-type",
  ])
    await expectReadable(page.locator(selector));
  const field = page.getByLabel("手机号或邮箱", { exact: true });
  await expectReadable(field);
  // Borders must be visible even before focus and without placeholder text.
  const border = await field.evaluate((element) => {
    const style = getComputedStyle(element);
    const light = (value: string) => {
      const c = (value.match(/[\d.]+/g) || [])
        .slice(0, 3)
        .map(Number)
        .map((n) => {
          const s = n / 255;
          return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
        });
      return c[0] * 0.2126 + c[1] * 0.7152 + c[2] * 0.0722;
    };
    const a = light(style.borderTopColor),
      b = light(style.backgroundColor);
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
  });
  expect(border, "unfocused field boundary contrast").toBeGreaterThanOrEqual(3);
  await page.screenshot({
    path: info.outputPath("login-clarity.png"),
    fullPage: true,
  });
  if (info.project.name === "mobile") {
    await page.setViewportSize({ width: 320, height: 740 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page
      .getByRole("button", { name: "登录", exact: true })
      .scrollIntoViewIfNeeded();
    await expect(
      page.getByRole("button", { name: "登录", exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: info.outputPath("login-clarity-320.png"),
      fullPage: true,
    });
  }
});
