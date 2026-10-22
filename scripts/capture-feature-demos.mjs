// Record real UI operations against the fixed isolated acceptance deployment.
// Account/session and labelled starting data are fixtures; recorded writes use UI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const require = createRequire(new URL("../web/package.json", import.meta.url));
const { chromium, request, expect } = require("@playwright/test");
const baseURL = "http://127.0.0.1:15173";
const run = new Date().toISOString().replaceAll(/[:.]/g, "-");
const scratch = resolve(root, ".cache/harness/real-feature-demos", run);
// Assets stay private until a person has reviewed the actual videos/keyframes.
const output = resolve(scratch, "reviewed-candidates");
const viewport = { width: 1280, height: 900 };
const docker = (...args) =>
  execFileSync("docker", args, { cwd: root, encoding: "utf8" }).trim();
const serviceImages = {};
for (const service of ["db", "api", "worker", "web", "review-fixture"]) {
  const id = docker(
    "compose",
    "--project-directory",
    ".",
    "-p",
    "foodflow-acceptance",
    "--env-file",
    ".cache/acceptance.env",
    "-f",
    "deploy/compose.acceptance.yaml",
    "ps",
    "-q",
    service,
  );
  assert.ok(id, `Isolated acceptance ${service} must be running`);
  const labels = JSON.parse(
    docker("inspect", "--format", "{{json .Config.Labels}}", id),
  );
  assert.equal(labels["com.docker.compose.project"], "foodflow-acceptance");
  assert.equal(labels["com.docker.compose.service"], service);
  if (["api", "worker", "web"].includes(service)) {
    const imageID = docker("inspect", "--format", "{{.Image}}", id);
    const imageLabels = JSON.parse(
      docker(
        "image",
        "inspect",
        "--format",
        "{{json .Config.Labels}}",
        imageID,
      ),
    );
    const digest = imageLabels?.["io.foodflow.source-digest"];
    assert.match(
      digest || "",
      /^[a-f0-9]{64}$/,
      `${service} needs a source-bound image`,
    );
    serviceImages[service] = { image: imageID, source_digest: digest };
  }
  if (service === "worker") {
    assert.equal(
      docker("exec", id, "printenv", "COMMUNITY_REVIEW_MODEL_ENDPOINT"),
      "http://review-fixture:8080/v1",
      "Only the local deterministic review fixture is allowed",
    );
  }
}
assert.equal(serviceImages.api.source_digest, serviceImages.web.source_digest);
assert.equal(
  serviceImages.api.source_digest,
  serviceImages.worker.source_digest,
);
// Validate the encoder before creating any fixture data.
const ffmpeg = process.env.FFMPEG_PATH || "ffmpeg";
execFileSync(ffmpeg, ["-version"], { stdio: "pipe" });
await mkdir(output, { recursive: true });
const api = await request.newContext({ baseURL });
const scenes = [];
let browser, token, kitchen;
const call = async (method, path, data, auth = true) => {
  const response = await api.fetch(path, {
    method,
    data,
    headers: auth
      ? { Authorization: `Bearer ${token}`, "Idempotency-Key": randomUUID() }
      : {},
  });
  assert.ok(
    response.ok(),
    `${method} ${new URL(path, baseURL).pathname}: HTTP ${response.status()}`,
  );
  return response.json();
};
const inventory = () => call("GET", kitchen + "/inventory");
const ledger = () => call("GET", kitchen + "/ledger");
const quantity = (stock, name) =>
  stock.items.find((item) => item.name === name)?.quantity;
const navigate = (page, name) =>
  page
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: new RegExp(name) })
    .click();
const pause = (page, ms = 1000) => page.waitForTimeout(ms); // Readable real-time footage.
async function settle(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      [...document.images]
        .filter((image) => {
          const bounds = image.getBoundingClientRect();
          return bounds.bottom > 0 && bounds.top < innerHeight;
        })
        .map((image) => image.decode().catch(() => {})),
    );
    await new Promise(requestAnimationFrame);
  });
  assert.ok(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "Recorded page must fit viewport",
  );
}
async function record(name, description, action) {
  const folder = resolve(scratch, name);
  await mkdir(folder, { recursive: true });
  const context = await browser.newContext({
    viewport,
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    recordVideo: { dir: folder, size: viewport },
    // Session provisioning is not part of the published clip. No password is filmed.
    storageState: {
      cookies: [],
      origins: [
        { origin: baseURL, localStorage: [{ name: "ff_token", value: token }] },
      ],
    },
  });
  const page = await context.newPage();
  const errors = [];
  const checkpoints = [];
  page.setDefaultTimeout(20000);
  page.on("pageerror", () => errors.push("browser-script-error"));
  page.on("response", (response) => {
    if (
      response.url().startsWith(baseURL + "/api/") &&
      response.status() >= 500
    )
      errors.push(`api-${response.status()}`);
  });
  // Only this deployment's pages/assets/API are accessible from the recording browser.
  await page.route("**/*", (route) => {
    const url = new URL(route.request().url());
    return url.origin === baseURL || ["data:", "blob:"].includes(url.protocol)
      ? route.continue()
      : route.abort();
  });
  const checkpoint = async (label) => {
    await settle(page);
    await pause(page, 1300);
    const file = `${name}-${String(checkpoints.length + 1).padStart(2, "0")}.png`;
    const bytes = await page.screenshot({ path: resolve(folder, file) });
    checkpoints.push({
      label,
      file: `${name}/${file}`,
      sha256: createHash("sha256").update(bytes).digest("hex"),
    });
  };
  try {
    await page.goto(baseURL);
    await expect(
      page.getByText("个人厨房：我的厨房", { exact: true }),
    ).toBeVisible();
    await action(page, checkpoint);
    assert.deepEqual(errors, [], `${name}: application errors`);
    await pause(page, 1800);
  } catch (error) {
    // Private failed frames remain available; they are never promoted automatically.
    await page
      .screenshot({ path: resolve(folder, "failed.png") })
      .catch(() => {});
    throw error;
  } finally {
    await context.close();
  }
  const video = await page.video().path();
  const target = resolve(output, `${name}.gif`);
  execFileSync(
    ffmpeg,
    [
      "-y",
      "-loglevel",
      "error",
      "-i",
      video,
      "-filter_complex",
      "fps=8,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=3:diff_mode=rectangle",
      "-loop",
      "0",
      target,
    ],
    { cwd: root, stdio: "pipe", timeout: 120000 },
  );
  const bytes = await readFile(target);
  assert.equal(bytes.subarray(0, 6).toString(), "GIF89a");
  assert.equal(bytes.readUInt16LE(6), 960);
  assert.ok(
    bytes.length < 12 * 1024 * 1024,
    `${name}: review clip length/encoding before publication`,
  );
  scenes.push({
    name,
    description,
    file: `${name}.gif`,
    bytes: bytes.length,
    sha256: createHash("sha256").update(bytes).digest("hex"),
    checkpoints,
    status: "captured-awaiting-visual-review",
  });
  await writeManifest();
  console.log(
    `Captured ${name}: ${checkpoints.length} checked UI states, ${bytes.length} bytes`,
  );
}
async function writeManifest() {
  await writeFile(
    resolve(scratch, "manifest.json"),
    JSON.stringify(
      {
        version: 1,
        captured_at: run,
        base_url: baseURL,
        revision: execFileSync("git", ["rev-parse", "HEAD"], {
          cwd: root,
          encoding: "utf8",
        }).trim(),
        source_images: serviceImages,
        browser: browser?.version(),
        channel: process.env.PLAYWRIGHT_CHANNEL || "chromium",
        viewport,
        gif_width: 960,
        fps: 8,
        playback_speed: 1,
        account:
          "new fictional example.test account; credentials not retained in manifest",
        fixture_setup:
          "API creates account, personal kitchen and labelled starting inventory/cost history; all demonstrated business steps use real UI and API",
        model_scope:
          "no menu model dispatch; community uses local deterministic review fixture",
        limitations:
          "Desktop recording, synthetic data; no real provider, microphone, SMS, physical device, administrator admission or production acceptance claim",
        scenes,
      },
      null,
      2,
    ) + "\n",
  );
}
try {
  const registered = await call(
    "POST",
    "http://127.0.0.1:18080/api/register",
    {
      email: `showcase-${randomUUID()}@example.test`,
      password: `Showcase-${randomUUID()}!`,
      name: "食光体验厨房",
    },
    false,
  );
  token = registered.token;
  assert.equal(
    (await call("GET", "/api/capabilities")).model_enabled,
    false,
    "Capture requires unpaid deterministic menu mode",
  );
  const house = await call("PUT", "/api/me/personal-kitchen");
  assert.equal(house.kind, "personal");
  assert.equal(house.servings, 1);
  kitchen = `/api/households/${house.id}`;
  const catalog = await call("GET", "/api/ingredient-catalog");
  const tomato = catalog.find((item) => item.name === "番茄");
  assert.ok(tomato);
  // These pre-existing facts are explicitly documented as setup, never presented as filmed writes.
  const batch = await call("POST", kitchen + "/catalog-stock", {
    catalog_id: tomato.id,
    quantity: "100",
  });
  const deduct = (quantity, reason) =>
    call("POST", kitchen + "/stock", {
      ingredient_id: batch.ingredient_id,
      batch_id: batch.batch_id,
      quantity,
      reason,
    });
  await deduct("10", "consume"); // Historical unknown cost stays unknown after cost is added.
  await call("POST", `${kitchen}/batches/${batch.batch_id}/cost`, {
    total_cost: "5.00",
  });
  await deduct("20", "consume");
  await deduct("30", "waste");
  browser = await chromium.launch({
    channel: process.env.PLAYWRIGHT_CHANNEL === "msedge" ? "msedge" : undefined,
    slowMo: 120,
  });

  await record(
    "intake",
    "食材：真实目录选生菜、输入50g、确认入库",
    async (page, shot) => {
      await navigate(page, "食材");
      await shot("食材库存与现有番茄");
      await page
        .getByRole("button", { name: "添加新食材", exact: true })
        .click();
      await page.getByLabel("搜索食材名称或别名").fill("生菜");
      await page.getByRole("button", { name: "选择生菜", exact: true }).click();
      await page.getByLabel("本次入库数量（g）").fill("50");
      await shot("核对生菜50g，尚未入库");
      assert.equal(quantity(await inventory(), "生菜"), undefined);
      await page.getByRole("button", { name: "确认入库", exact: true }).click();
      await expect(page.getByRole("status")).toContainText("生菜 · 50 g");
      await page
        .getByRole("button", { name: "关闭食材选择", exact: true })
        .click();
      await expect(
        page.getByText("50 g", { exact: true }).first(),
      ).toBeVisible();
      assert.equal(quantity(await inventory(), "生菜"), "50.000");
      // DOM visibility alone does not mean the result is inside the recorded frame.
      await page
        .getByText("50 g", { exact: true })
        .first()
        .scrollIntoViewIfNeeded();
      await expect(
        page.getByText("50 g", { exact: true }).first(),
      ).toBeInViewport();
      await shot("确认后库存出现50g生菜");
    },
  );
  await record(
    "plan",
    "菜单：搜索菜谱、安排晚餐、核对并确认采购缺口",
    async (page, shot) => {
      const before = await inventory(),
        facts = await ledger();
      await navigate(page, "菜单");
      await shot("菜单灵感与菜谱入口");
      await page.getByRole("button", { name: /自己逛菜谱/ }).click();
      await page.getByLabel("搜索菜名或食材").fill("蒜香生菜");
      await page
        .getByRole("button", { name: "加入蒜香生菜", exact: true })
        .click();
      await page.getByRole("button", { name: "晚餐", exact: true }).click();
      await page.getByRole("button", { name: "用这些菜安排一餐" }).click();
      await expect(
        page.getByText("晚餐 · 蒜香生菜", { exact: true }),
      ).toBeVisible();
      await shot("1人晚餐等待核对确认");
      assert.deepEqual(await inventory(), before);
      assert.deepEqual(await ledger(), facts);
      await page
        .getByRole("button", { name: "确认菜单并生成采购", exact: true })
        .click();
      await expect
        .poll(async () => (await call("GET", kitchen + "/shopping")).length)
        .toBe(2);
      assert.deepEqual(await inventory(), before);
      assert.deepEqual(await ledger(), facts);
      const shopping = await call("GET", kitchen + "/shopping");
      assert.equal(
        shopping.find((item) => item.name === "生菜").needed,
        "100.000",
      );
      assert.equal(
        shopping.find((item) => item.name === "蒜").needed,
        "10.000",
      );
      await page.getByText("已安排的菜单", { exact: true }).click();
      const confirmed = page.getByText(/到「今天」查看今日菜单/);
      await confirmed.scrollIntoViewIfNeeded();
      await expect(confirmed).toBeInViewport();
      await shot("菜单确认，采购缺口已生成，库存未扣减");
    },
  );
  await record(
    "shopping",
    "采购：核对实际买入数量、确认入库",
    async (page, shot) => {
      await navigate(page, "采购");
      await expect(page.getByText("生菜", { exact: true })).toBeVisible();
      await shot("真实计算的生菜100g与蒜10g缺口");
      for (const [name, amount] of [
        ["生菜", "100.000"],
        ["蒜", "10.000"],
      ]) {
        const card = page
          .locator(".rounded-2xl")
          .filter({ has: page.getByText(name, { exact: true }) })
          .filter({ has: page.getByRole("button", { name: "勾选已买并入库" }) })
          .last();
        await card
          .getByRole("button", { name: "勾选已买并入库", exact: true })
          .click();
        const dialog = page.getByRole("dialog", { name: "采购入库确认" });
        await expect(dialog.getByLabel("实际采购量（g）")).toHaveValue(amount);
        await dialog.getByLabel("有效期（可清空）").fill("");
        await shot(`${name}采购入库确认`);
        await dialog
          .getByRole("button", { name: "确认并入库", exact: true })
          .click();
        await expect(dialog).toHaveCount(0);
      }
      const stock = await inventory();
      assert.equal(quantity(stock, "生菜"), "150.000");
      assert.equal(quantity(stock, "蒜"), "10.000");
      await shot("两项采购均已入库");
    },
  );
  await record(
    "today",
    "今天：确认烹饪完成、准确扣库、刷新保留结果",
    async (page, shot) => {
      const dismiss = page.getByRole("button", {
        name: "收起指南，查看今日待办",
      });
      if (await dismiss.isVisible()) await dismiss.click();
      await page
        .getByRole("button", { name: "烹饪完成", exact: true })
        .scrollIntoViewIfNeeded();
      await shot("今天已安排的蒜香生菜");
      await page.getByRole("button", { name: "烹饪完成", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: /确认完成/ });
      await shot("完成前再次明确确认扣库");
      assert.equal(quantity(await inventory(), "生菜"), "150.000");
      await dialog
        .getByRole("button", { name: "确认完成并扣库", exact: true })
        .click();
      await expect(dialog).toHaveCount(0);
      await expect(page.getByText("已完成", { exact: true })).toBeVisible();
      await page.reload();
      await expect(page.getByText("已完成", { exact: true })).toBeVisible();
      assert.equal(quantity(await inventory(), "生菜"), "0.000");
      assert.equal(quantity(await inventory(), "蒜"), "0.000");
      await page.getByText("已完成", { exact: true }).scrollIntoViewIfNeeded();
      await expect(page.getByText("已完成", { exact: true })).toBeInViewport();
      await shot("刷新后仍是已完成，库存精确扣减");
    },
  );
  await record(
    "report",
    "记录：库存流水、成本覆盖与实际PNG下载",
    async (page, shot) => {
      await navigate(page, "记录");
      await page.getByRole("button", { name: /库存流水/ }).click();
      await expect(page.getByText("-100 g", { exact: true })).toBeVisible();
      await shot("实际采购与烹饪流水");
      await page.getByRole("button", { name: /金额月报/ }).click();
      await expect(
        page.getByRole("heading", { name: "我的金额月报", exact: true }),
      ).toBeVisible();
      await expect(page.getByText("¥5.00", { exact: true })).toBeVisible();
      await expect(
        page.getByText(/出库成本覆盖 2\/6 条；未知成本 4 条/),
      ).toBeVisible();
      await shot("已知金额与未知成本同时保留");
      const download = page.waitForEvent("download");
      await page
        .getByRole("button", { name: "导出月报图片", exact: true })
        .click();
      const saved = await download;
      assert.match(
        saved.suggestedFilename(),
        /^foodflow-month-\d{4}-\d{2}-1\.png$/,
      );
      const path = resolve(scratch, "exported-monthly-report.png");
      await saved.saveAs(path);
      const bytes = await readFile(path);
      assert.equal(bytes.subarray(0, 8).toString("hex"), "89504e470d0a1a0a");
      await shot("点击导出，实际下载的PNG保留在录制目录");
    },
  );
  await record(
    "community",
    "社区：真实填写配方、同意本地初审、收藏与查看",
    async (page, shot) => {
      const title = `一碗番茄汤 · ${randomUUID().slice(0, 4)}`;
      await navigate(page, "社区");
      await page
        .getByRole("button", { name: "分享我的配方", exact: true })
        .click();
      const dialog = page.getByRole("dialog", { name: "分享一道拿手菜" });
      await dialog.getByLabel("给这道菜起个名字").fill(title);
      await dialog
        .getByLabel("厨房里的小故事")
        .fill("虚构演示配方：下班后，用普通食材做一碗热汤。");
      await dialog
        .getByLabel("食材 1", { exact: true })
        .selectOption(tomato.id);
      await dialog.getByLabel("用量 1", { exact: true }).fill("200");
      await dialog
        .getByLabel("步骤 1", { exact: true })
        .fill("番茄洗净切块，下锅煮熟后盛出。");
      await expect(
        dialog.getByRole("button", { name: "发布这份心意" }),
      ).toBeDisabled();
      await shot("实际编辑配方，尚未确认发布");
      await dialog.getByLabel(/确认公开分享这份配方/).check();
      await dialog
        .getByRole("button", { name: "发布这份心意", exact: true })
        .click();
      await expect(dialog).toHaveCount(0);
      await page.getByRole("button", { name: "我的配方", exact: true }).click();
      const card = page.locator(".community-post-card").filter({
        has: page.getByRole("button", { name: title, exact: true }),
      });
      await expect(
        card.getByRole("status", { name: "发布初审状态" }),
      ).toContainText("初审通过", { timeout: 30000 });
      await card.scrollIntoViewIfNeeded();
      await shot("本地确定性审核通过后的真实动态");
      await page
        .getByRole("button", { name: `收藏：${title}`, exact: true })
        .click();
      await page.getByRole("button", { name: "我的收藏", exact: true }).click();
      await page
        .getByRole("button", { name: `查看配方：${title}`, exact: true })
        .click();
      await expect(
        page.getByRole("dialog", { name: title, exact: true }),
      ).toContainText("200");
      await shot("实际收藏并查看配方详情");
    },
  );
  await record(
    "kitchen",
    "个人中心：创建共享家庭、验证隔离、切回个人库存",
    async (page, shot) => {
      await page
        .getByRole("button", { name: "打开个人主页", exact: true })
        .click();
      await shot("个人厨房与可选家庭");
      await page.getByText("创建或加入家庭", { exact: true }).click();
      await page.getByLabel("家庭名称", { exact: true }).fill("周末共享厨房");
      await page
        .getByRole("button", { name: "创建家庭", exact: true })
        .scrollIntoViewIfNeeded();
      await expect(
        page.getByLabel("家庭名称", { exact: true }),
      ).toBeInViewport();
      await expect(
        page.getByRole("button", { name: "创建家庭", exact: true }),
      ).toBeInViewport();
      await shot("填写新家庭，个人数据仍独立");
      await page.getByRole("button", { name: "创建家庭", exact: true }).click();
      await expect(
        page.getByText("当前家庭：周末共享厨房", { exact: true }),
      ).toBeVisible();
      await navigate(page, "食材");
      await expect(
        page.getByRole("heading", { name: "番茄", exact: true }),
      ).toHaveCount(0);
      await shot("新家庭为空，没有混入个人库存");
      await page
        .getByRole("button", { name: "打开个人主页", exact: true })
        .click();
      await page.getByRole("button", { name: /使用个人厨房/ }).click();
      await navigate(page, "食材");
      await expect(
        page.getByText("40 g", { exact: true }).first(),
      ).toBeVisible();
      assert.equal(quantity(await inventory(), "番茄"), "40.000");
      await page
        .getByText("40 g", { exact: true })
        .first()
        .scrollIntoViewIfNeeded();
      await expect(
        page.getByText("40 g", { exact: true }).first(),
      ).toBeInViewport();
      await shot("切回个人厨房，原有番茄40g仍保留");
    },
  );
  console.log(
    `All ${scenes.length} UI recordings captured. Review ${scratch}; copy only reviewed GIFs to docs/assets/demos.`,
  );
} finally {
  await browser?.close();
  await api.dispose();
}
