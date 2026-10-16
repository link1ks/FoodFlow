// Capture the real application with fictional data in the fixed acceptance stack.
// No production browser/session, provider configuration or default database is used.
import assert from "node:assert/strict";
import { randomUUID, createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";

const root = fileURLToPath(new URL("../", import.meta.url));
const require = createRequire(new URL("../web/package.json", import.meta.url));
const { chromium, request, expect, devices } = require("@playwright/test");
const output = resolve(root, ".cache/harness/showcase");
const baseURL = "http://127.0.0.1:15173";
const docker = (...args) =>
  execFileSync("docker", args, { cwd: root, encoding: "utf8" }).trim();
for (const service of ["db", "api", "worker", "web"]) {
  const id = docker(
    "compose",
    "--env-file",
    ".cache/acceptance.env",
    "-f",
    "compose.acceptance.yaml",
    "ps",
    "-q",
    service,
  );
  assert.ok(id, `Acceptance ${service} must be running`);
  const labels = JSON.parse(
    docker("inspect", "--format", "{{json .Config.Labels}}", id),
  );
  assert.equal(labels["com.docker.compose.project"], "foodflow-acceptance");
  assert.equal(labels["com.docker.compose.service"], service);
  if (service === "worker") {
    assert.equal(
      docker("exec", id, "printenv", "COMMUNITY_REVIEW_MODEL_ENDPOINT"),
      "http://review-fixture:8080/v1",
    );
  }
}
await mkdir(output, { recursive: true });
const api = await request.newContext({ baseURL });
let browser;
const captures = [];
try {
  const call = async (method, path, data, authorized = true) => {
    const response = await api.fetch("/api" + path, {
      method,
      data,
      headers: authorized
        ? { Authorization: `Bearer ${token}`, "Idempotency-Key": randomUUID() }
        : {},
    });
    assert.ok(
      response.ok(),
      `${method} ${path.split("?")[0]} returned ${response.status()}`,
    );
    return response.json();
  };
  const email = `showcase-${randomUUID()}@example.test`;
  const password = `Showcase-${randomUUID()}!`;
  const registered = await call(
    "POST",
    "/register",
    { email, password, name: "食光演示" },
    false,
  );
  const token = registered.token;
  const house = await call("PUT", "/me/personal-kitchen");
  const kitchen = `/households/${house.id}`;
  const catalog = await call("GET", "/ingredient-catalog");
  assert.equal(
    (await call("GET", "/capabilities")).model_enabled,
    false,
    "Menu capture requires deterministic demo mode",
  );
  const today = new Date().toLocaleDateString("en-CA", {
    timeZone: "Asia/Shanghai",
  });
  const tomorrow = new Date(Date.now() + 86400000).toLocaleDateString("en-CA", {
    timeZone: "Asia/Shanghai",
  });
  const batches = {};
  for (const [name, quantity, cost, expires] of [
    ["番茄", "600", "6.00", tomorrow],
    ["鸡蛋", "6", "7.20", ""],
    ["土豆", "800", "4.80", ""],
    ["胡萝卜", "300", "2.40", ""],
    ["生菜", "50", "1.00", tomorrow],
    ["西兰花", "350", "5.60", ""],
    ["牛奶", "500", "4.50", ""],
    ["苹果", "500", "6.50", ""],
  ]) {
    const item = catalog.find((x) => x.name === name);
    assert.ok(item, `Missing catalogue ingredient ${name}`);
    const batch = await call("POST", kitchen + "/catalog-stock", {
      catalog_id: item.id,
      quantity,
      bought_on: today,
      location: ["土豆", "苹果"].includes(name) ? "常温" : "冷藏",
      ...(expires ? { expires_on: expires } : {}),
    });
    batches[name] = batch;
    await call("POST", `${kitchen}/batches/${batch.batch_id}/cost`, {
      total_cost: cost,
    });
  }
  // Known costs and a deliberately unknown historical cost keep report coverage honest.
  const stock = async (name, quantity, reason) =>
    call("POST", kitchen + "/stock", {
      ...batches[name],
      quantity,
      reason,
    });
  await stock("番茄", "100", "consume");
  await stock("胡萝卜", "50", "waste");
  const cucumber = catalog.find((x) => x.name === "黄瓜");
  batches["黄瓜"] = await call("POST", kitchen + "/catalog-stock", {
    catalog_id: cucumber.id,
    quantity: "200",
  });
  await stock("黄瓜", "50", "consume");

  const recipes = await call("GET", "/recipes");
  const recipe = recipes.find(
    (x) => x.id === "10000000-0000-4000-8000-000000000004",
  );
  assert.ok(recipe);
  const plan = await call("POST", kitchen + "/plans", {
    meals: [{ day: today, meal: "dinner", servings: 1, recipe_id: recipe.id }],
  });
  const detail = await call("GET", kitchen + "/plans/" + plan.id);
  await call("POST", kitchen + "/plans/" + plan.id + "/confirm", {
    revision: detail.revision,
    accept_uncertain: false,
  });
  const inventoryBefore = await call("GET", kitchen + "/inventory");
  const ledgerBefore = await call("GET", kitchen + "/ledger");

  const post = await call("POST", "/community/posts", {
    title: "一碗热乎的番茄汤",
    story: "演示配方 · 把普通食材，做成下班后的一碗热汤。",
    servings: 1,
    minutes: 15,
    tags: ["家常", "快手"],
    ingredients: [
      {
        catalog_id: catalog.find((x) => x.name === "番茄").id,
        quantity: "200",
        unit: "g",
      },
    ],
    steps: ["番茄洗净切块，保留汁水。", "下锅煮熟，确认熟透后盛出。"],
    confirm_public: true,
    confirm_review: true,
  });
  await expect
    .poll(
      async () =>
        (await call("GET", `/community/posts/${post.id}`)).review_status,
      { timeout: 30000 },
    )
    .toBe("approved");

  browser = await chromium.launch({
    channel: process.env.PLAYWRIGHT_CHANNEL === "msedge" ? "msedge" : undefined,
  });
  const errors = [];
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1020 },
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  const monitor = (target) => {
    target.on("pageerror", () => errors.push("browser-script-error"));
    target.on("response", (r) => {
      if (r.url().startsWith(baseURL + "/api/") && r.status() >= 500)
        errors.push(`api-${r.status()}`);
    });
  };
  monitor(page);
  const login = async (target) => {
    await target.goto(baseURL);
    await target.getByLabel("手机号或邮箱", { exact: true }).fill(email);
    await target.getByLabel("密码", { exact: true }).fill(password);
    await target.getByRole("button", { name: "登录", exact: true }).click();
    await expect(
      target.getByText("个人厨房：我的厨房", { exact: true }),
    ).toBeVisible();
  };
  const navigate = async (name) => {
    await page
      .getByRole("navigation", { name: "主导航" })
      .getByRole("button", { name: new RegExp(name) })
      .click();
  };
  const capture = async (target, name, anchor) => {
    if (anchor) {
      await target
        .locator(anchor)
        .evaluate((element) => element.scrollIntoView({ block: "start" }));
      await target.evaluate(() => window.scrollBy(0, -110));
    } else {
      await target.evaluate(() => window.scrollTo(0, 0));
    }
    await target.evaluate(async () => {
      await document.fonts.ready;
      await new Promise(requestAnimationFrame);
      // Offscreen lazy images deliberately have not started loading.
      await Promise.all(
        [...document.images]
          .filter((img) => {
            const bounds = img.getBoundingClientRect();
            return bounds.top < innerHeight && bounds.bottom > 0;
          })
          .map((img) => img.decode().catch(() => {})),
      );
    });
    assert.ok(
      await target.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      `${name}: horizontal overflow`,
    );
    const image = await target.screenshot({
      path: resolve(output, name + ".png"),
      animations: "disabled",
      scale: "css",
    });
    captures.push({
      file: name + ".png",
      width: image.readUInt32BE(16),
      height: image.readUInt32BE(20),
      sha256: createHash("sha256").update(image).digest("hex"),
    });
    console.log(`Captured ${name}`);
  };
  await login(page);
  await expect(page.getByRole("region", { name: "厨房状态" })).toContainText(
    "家中食材",
  );
  await expect(page.getByText(/蒜香生菜/).first()).toBeVisible();
  await capture(page, "today-desktop");
  await navigate("食材");
  await expect(page.getByText("番茄", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("500 g", { exact: true }).first()).toBeVisible();
  await capture(page, "inventory-desktop", "#inventory-list");
  await page.getByRole("button", { name: "＋ 添加食材" }).click();
  await expect(
    page.getByRole("button", { name: "选择番茄", exact: true }),
  ).toBeVisible();
  await capture(page, "catalog-desktop");
  await page.getByRole("button", { name: "关闭食材选择" }).click();
  await navigate("菜单");
  await page.getByLabel("选择菜谱手工规划").selectOption(recipe.id);
  await expect(
    page.getByRole("region", { name: "所选菜谱详情" }),
  ).toContainText("蒜香生菜");
  await capture(page, "menu-desktop");
  await page.getByRole("button", { name: "帮我推荐菜单", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "菜单规划 · 等待确认", exact: true }),
  ).toBeVisible({ timeout: 30000 });
  await expect(
    page.getByText("演示模式：未调用真实模型，使用确定性菜谱选择。"),
  ).toBeVisible();
  await capture(page, "ai-draft-desktop");
  await navigate("采购");
  await expect(page.getByText("生菜", { exact: true })).toBeVisible();
  await capture(page, "shopping-desktop");
  await page
    .getByRole("button", { name: "勾选已买并入库", exact: true })
    .first()
    .click();
  await expect(
    page.getByRole("dialog", { name: "采购入库确认" }),
  ).toBeVisible();
  await capture(page, "purchase-confirm-desktop");
  await page.getByRole("button", { name: "暂不入库", exact: true }).click();
  await navigate("记录");
  await page.getByRole("button", { name: /金额月报/ }).click();
  await expect(
    page.getByRole("heading", { name: "我的金额月报", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText(/出库成本覆盖 2\/3 条；未知成本 1 条/),
  ).toBeVisible();
  await capture(page, "monthly-desktop");
  await navigate("社区");
  await page.getByRole("button", { name: "我的配方", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "一碗热乎的番茄汤", exact: true }),
  ).toBeVisible();
  await capture(page, "community-desktop", ".community-post-card");

  const mobile = await browser.newContext({
    ...devices["Pixel 7"],
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    reducedMotion: "reduce",
  });
  const phone = await mobile.newPage();
  phone.setDefaultTimeout(15000);
  monitor(phone);
  await login(phone);
  await expect(phone.getByRole("region", { name: "厨房状态" })).toContainText(
    "家中食材",
  );
  await capture(phone, "today-mobile");
  await phone
    .getByRole("navigation", { name: "主导航" })
    .getByRole("button", { name: /食材/ })
    .click();
  await expect(phone.getByText("番茄", { exact: true }).first()).toBeVisible();
  await capture(phone, "inventory-mobile", "#inventory-list");
  assert.deepEqual(errors, [], "Application errors during capture");
  assert.deepEqual(
    await call("GET", kitchen + "/inventory"),
    inventoryBefore,
    "Draft generation and unconfirmed procurement must not change inventory",
  );
  assert.deepEqual(
    await call("GET", kitchen + "/ledger"),
    ledgerBefore,
    "Unconfirmed actions must not append stock facts",
  );
  await writeFile(
    resolve(output, "manifest.json"),
    JSON.stringify(
      {
        captured_at: new Date().toISOString(),
        revision: execFileSync("git", ["rev-parse", "HEAD"], {
          cwd: root,
          encoding: "utf8",
        }).trim(),
        browser: browser.version(),
        channel: process.env.PLAYWRIGHT_CHANNEL || "chromium",
        source:
          "isolated acceptance stack; fictional personal kitchen; deterministic community review",
        captures,
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    `Saved ${captures.length} screenshots under .cache/harness/showcase; review before publishing.`,
  );
} finally {
  await browser?.close();
  await api.dispose();
}
