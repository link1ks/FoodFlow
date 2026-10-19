import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("voice intake requires consent, editable quantities and explicit atomic confirmation", async ({
  page,
  request,
}, info) => {
  const email = `voice-${randomUUID()}@example.test`,
    password = "Voice-Fixture-2026!";
  // Provision a real synthetic session on the fixed isolated API. Account
  // tests cover gateway login; this scenario stays within its normal IP budget.
  const registered = await request.post("http://127.0.0.1:18080/api/register", {
    data: { email, password, name: "语音录入验收" },
  });
  expect(registered.status()).toBe(200);
  const user = await registered.json();
  const headers = { Authorization: `Bearer ${user.token}` };
  const created = await request.post("/api/households", {
    headers,
    data: { name: "语音厨房", servings: 2 },
  });
  expect(created.status()).toBe(201);
  const root = `/api/households/${(await created.json()).id}`;
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        getUserMedia: async () => {
          throw new DOMException("fixture denied", "NotAllowedError");
        },
      },
    });
  });
  await page.addInitScript(
    (token) => localStorage.setItem("ff_token", token),
    user.token,
  );
  const opened = await page.goto("/");
  expect(opened?.headers()["permissions-policy"]).toContain(
    "microphone=(self)",
  );
  await page.getByRole("button", { name: "打开个人主页", exact: true }).click();
  await page
    .getByRole("button", { name: "使用家庭：语音厨房", exact: true })
    .click();
  const navigation = page.getByRole("navigation", { name: "主导航" });
  await navigation.getByRole("button", { name: /食材/ }).click();
  const start = page.getByRole("button", { name: "开始录音", exact: true });
  await expect(start).toBeDisabled();
  await page.getByRole("checkbox", { name: /我同意将本次录音/ }).check();
  await start.click();
  await expect(
    page.getByText("无法使用麦克风，请检查浏览器授权，或使用食材目录。", {
      exact: true,
    }),
  ).toBeVisible();
  expect(await (await request.get(root + "/jobs", { headers })).json()).toEqual(
    [],
  );
  await page.evaluate(() => {
    const state = { stopped: 0, failStart: true };
    Object.defineProperty(window, "voiceCaptureFixture", { value: state });
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        getUserMedia: async () => ({
          getTracks: () => [{ stop: () => state.stopped++ }],
        }),
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
        if (state.failStart) {
          state.failStart = false;
          throw new Error("fixture capture startup failure");
        }
        this.state = "recording";
      }
      stop() {
        if (this.state !== "recording") return;
        this.state = "inactive";
        this.ondataavailable?.({
          data: new Blob(
            [new Uint8Array([0x1a, 0x45, 0xdf, 0xa3]), "webm fixture"],
            { type: this.mimeType },
          ),
        });
        this.onstop?.();
      }
    }
    Object.defineProperty(window, "MediaRecorder", {
      configurable: true,
      value: Capture,
    });
  });
  await start.click();
  await expect(
    page.getByText("无法使用麦克风，请检查浏览器授权，或使用食材目录。", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(start).toBeEnabled();
  await start.click();
  await expect(page.getByText(/录音中 ·/)).toBeVisible();
  await page.getByRole("button", { name: "停止录音", exact: true }).click();
  await expect(
    page.getByText("录音已完成，可以提交识别。", { exact: true }),
  ).toBeVisible();
  expect(await (await request.get(root + "/jobs", { headers })).json()).toEqual(
    [],
  );
  await start.click();
  await expect(page.getByText(/录音中 ·/)).toBeVisible();
  const stoppedBeforeLeaving = await page.evaluate(
    () =>
      (window as unknown as { voiceCaptureFixture: { stopped: number } })
        .voiceCaptureFixture.stopped,
  );
  await navigation.getByRole("button", { name: /今天/ }).click();
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { voiceCaptureFixture: { stopped: number } })
          .voiceCaptureFixture.stopped,
    ),
  ).toBeGreaterThan(stoppedBeforeLeaving);
  await navigation.getByRole("button", { name: /食材/ }).click();
  await page.getByRole("checkbox", { name: /我同意将本次录音/ }).check();
  await expect(page.getByLabel("选择食材录音")).toHaveCount(0);
  await page.getByRole("button", { name: "开始录音", exact: true }).click();
  await page.getByRole("button", { name: "停止录音", exact: true }).click();
  await page.getByRole("button", { name: "识别这段录音", exact: true }).click();
  const draft = page.getByRole("region", { name: "语音食材草稿" });
  await expect(draft).toBeVisible({ timeout: 20_000 });
  await expect(
    draft.getByText("西红柿半斤，鸡蛋六个，牛奶一瓶。", { exact: true }),
  ).toBeVisible();
  await expect(
    draft.getByLabel("语音食材 1 数量（g）", { exact: true }),
  ).toHaveValue("250.000");
  await expect(
    draft.getByLabel("语音食材 2 数量（个）", { exact: true }),
  ).toHaveValue("6.000");
  await expect(
    draft.getByLabel("语音食材 3 数量（ml）", { exact: true }),
  ).toHaveValue("");
  const confirm = draft.getByRole("button", {
    name: "确认语音草稿并入库",
    exact: true,
  });
  await expect(confirm).toBeDisabled();
  expect(
    (await (await request.get(root + "/inventory", { headers })).json())
      .batches,
  ).toHaveLength(0);
  await draft.getByLabel("语音食材 3 数量（ml）", { exact: true }).fill("500");
  await expect(confirm).toBeDisabled();
  await draft.getByRole("checkbox", { name: /我已核对食材与实际数量/ }).check();
  if (info.project.name === "mobile")
    await page.setViewportSize({ width: 320, height: 740 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: info.outputPath("voice-draft.png"),
    fullPage: true,
  });
  await confirm.click();
  await expect(draft).toHaveCount(0);
  const inventory = await (
    await request.get(root + "/inventory", { headers })
  ).json();
  expect(inventory.batches).toHaveLength(3);
  expect(
    inventory.items
      .map((item: { name: string; quantity: string }) => [
        item.name,
        item.quantity,
      ])
      .sort(),
  ).toEqual(
    [
      ["番茄", "250.000"],
      ["鸡蛋", "6.000"],
      ["牛奶", "500.000"],
    ].sort(),
  );
  await page.screenshot({
    path: info.outputPath("voice-stocked.png"),
    fullPage: true,
  });
});
