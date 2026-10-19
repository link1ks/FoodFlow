import { test, expect, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";

async function recordFixture(page: Page, marker: number) {
  await page.evaluate((marker) => {
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        getUserMedia: async () => ({ getTracks: () => [{ stop() {} }] }),
      },
    });
    class Capture {
      static isTypeSupported() {
        return true;
      }
      state = "inactive";
      mimeType = "audio/webm";
      ondataavailable: ((event: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      start() {
        this.state = "recording";
      }
      stop() {
        if (this.state !== "recording") return;
        this.state = "inactive";
        this.ondataavailable?.({
          data: new Blob(
            [
              new Uint8Array([0x1a, 0x45, 0xdf, 0xa3]),
              "webm fixture",
              new Uint8Array([marker]),
            ],
            {
              type: this.mimeType,
            },
          ),
        });
        this.onstop?.();
      }
    }
    Object.defineProperty(window, "MediaRecorder", {
      configurable: true,
      value: Capture,
    });
  }, marker);
  await page.getByRole("button", { name: "开始录音", exact: true }).click();
  await page.getByRole("button", { name: "停止录音", exact: true }).click();
}

test("intake rejects unrelated voice and image and confirms explicit corrections", async ({
  page,
  request,
}) => {
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: {
      email: `scope-${randomUUID()}@example.test`,
      password: "Scope-Fixture-2026!",
      name: "录入范围验收",
    },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "范围厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const root = `/api/households/${(await created.json()).id}`;
  await page.addInitScript(
    (token) => localStorage.setItem("ff_token", token),
    user.token,
  );
  await page.goto("/");
  await page.getByRole("button", { name: "打开个人主页", exact: true }).click();
  await page
    .getByRole("button", { name: "使用家庭：范围厨房", exact: true })
    .click();
  await page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  await page.getByRole("checkbox", { name: /我同意将本次录音/ }).check();
  await recordFixture(page, 42);
  await page.getByRole("button", { name: "识别这段录音", exact: true }).click();
  const outside = page
    .getByRole("region", { name: "语音食材草稿" })
    .filter({ hasText: "明天提醒我买鸡蛋三个" });
  await expect(outside).toBeVisible({ timeout: 20_000 });
  await expect(
    outside.getByRole("button", { name: "确认语音草稿并入库" }),
  ).toHaveCount(0);
  await expect(
    outside.getByRole("button", { name: "补充一项食材" }),
  ).toHaveCount(0);
  const jobs = await (await request.get(root + "/jobs", { headers })).json();
  const voice = jobs.find((j: { kind: string }) => j.kind === "voice");
  const catalog = await (
    await request.get("/api/ingredient-catalog", { headers })
  ).json();
  const forged = await request.post(root + `/jobs/${voice.id}/voice/confirm`, {
    headers: { ...headers, "Idempotency-Key": randomUUID() },
    data: {
      confirm: true,
      items: [{ catalog_id: catalog[0].id, quantity: "1" }],
    },
  });
  expect(forged.status()).toBe(409);

  // Synthetic PNG marker is interpreted only by the isolated model fixture.
  const png = Buffer.concat([
    Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1ioAAAAASUVORK5CYII=",
      "base64",
    ),
    Buffer.from([42]),
  ]);
  await page
    .getByLabel("选择食材图片")
    .setInputFiles({ name: "outside.png", mimeType: "image/png", buffer: png });
  await page.getByRole("button", { name: "提交识别任务", exact: true }).click();
  await expect(page.getByText(/未识别到明确的单种食材/)).toBeVisible({
    timeout: 20_000,
  });
  await expect(page.getByLabel("识别食材目录")).toHaveCount(0);
  png[png.length - 1] = 43;
  await page
    .getByLabel("选择食材图片")
    .setInputFiles({ name: "food.png", mimeType: "image/png", buffer: png });
  await page.getByRole("button", { name: "提交识别任务", exact: true }).click();
  await expect(page.getByLabel("识别食材目录")).toBeVisible({
    timeout: 20_000,
  });
  await expect(
    page
      .getByLabel("识别食材目录")
      .getByRole("option", { name: "番茄", exact: true }),
  ).toHaveCount(1);
  await page.getByLabel("实际数量", { exact: true }).fill("200");
  await page
    .getByRole("button", { name: "确认识别内容并入库", exact: true })
    .click();
  await expect(page.getByLabel("识别食材目录")).toHaveCount(0);

  await recordFixture(page, 43);
  await page.getByRole("button", { name: "识别这段录音", exact: true }).click();
  const corrected = page
    .getByRole("region", { name: "语音食材草稿" })
    .filter({ hasText: "鸡蛋三个，改成四个" });
  await expect(corrected).toBeVisible({ timeout: 20_000 });
  await expect(
    corrected.getByLabel("语音食材 1 数量（个）", { exact: true }),
  ).toHaveValue("4.000");
  await corrected.getByRole("checkbox").check();
  await corrected
    .getByRole("button", { name: "确认语音草稿并入库", exact: true })
    .click();
  await expect(corrected).toHaveCount(0);
  const inventory = await (
    await request.get(root + "/inventory", { headers })
  ).json();
  expect(
    inventory.items
      .map((v: { name: string; quantity: string }) => [v.name, v.quantity])
      .sort(),
  ).toEqual(
    [
      ["番茄", "200.000"],
      ["鸡蛋", "4.000"],
    ].sort(),
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth + 1,
    ),
  ).toBe(true);
});
