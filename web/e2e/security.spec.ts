import { test, expect } from "@playwright/test";

test("gateway security policy blocks injected scripts and permits the app", async ({ page, request }) => {
  const violations: string[] = [];
  await page.addInitScript(() => {
    document.addEventListener("securitypolicyviolation", (event) => {
      console.warn(`CSP blocked: ${event.violatedDirective}`);
    });
  });
  page.on("console", (message) => {
    if (message.text().startsWith("CSP blocked:")) violations.push(message.text());
  });
  const response = await page.goto("/");
  expect(response?.headers()["x-content-type-options"]).toBe("nosniff");
  expect(response?.headers()["x-frame-options"]).toBe("DENY");
  expect(response?.headers()["content-security-policy"]).toContain("script-src 'self';");
  await expect(page.getByRole("button", { name: "登录", exact: true })).toBeVisible();
  expect(violations).toEqual([]);
  await page.evaluate(() => {
    const script = document.createElement("script");
    script.textContent = "document.documentElement.dataset.injectedScript = 'executed'";
    document.head.append(script);
  });
  await expect.poll(() => violations).toContain("CSP blocked: script-src-elem");
  expect(await page.locator("html").getAttribute("data-injected-script")).toBeNull();
  for (const path of ["/api/me", "/assets/missing-security-fixture.js", "/metrics"]) {
    const error = await request.get(path);
    expect(error.status()).toBe(path === "/api/me" ? 401 : 404);
    expect(error.headers()["x-content-type-options"]).toBe("nosniff");
    expect(error.headers()["x-frame-options"]).toBe("DENY");
    expect(error.headers()["content-security-policy"]).toContain("frame-ancestors 'none'");
    if (path === "/api/me") expect(error.headers()["cache-control"]).toContain("no-store");
  }
});
