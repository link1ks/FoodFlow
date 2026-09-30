import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  outputDir: "../.cache/harness/browser-artifacts",
  timeout: 45_000,
  expect: { timeout: 10_000 },
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: [
    ["list"],
    ["json", { outputFile: "../.cache/harness/browser.json" }],
  ],
  use: {
    // Hardcoded isolated environment: never use a user's logged-in browser or live DB.
    baseURL: "http://127.0.0.1:15173",
    channel: process.env.PLAYWRIGHT_CHANNEL === "msedge" ? "msedge" : undefined,
    trace: "off",
    video: "off",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    {
      name: "mobile",
      use: { ...devices["Pixel 7"], defaultBrowserType: "chromium" },
    },
  ],
});
