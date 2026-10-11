import { test, expect, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
async function login(page: Page, user = "Alice") {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: `以 ${user} 登录` }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
}
const folder =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
  path.resolve("temp/browser-artifacts");
test("real login, drag/drop, versioning, scoped Agent and cross-user isolation", async ({
  page,
  browser,
  request,
}) => {
  await login(page);
  const name = "project-" + Date.now() + ".md";
  await page.locator("input[type=file]").setInputFiles({
    name,
    mimeType: "text/markdown",
    buffer: Buffer.from("# Original\n"),
  });
  await expect(
    page.getByRole("button", { name: new RegExp(name) }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("complementary", { name: "目录树", exact: true }).getByText("对象存储（S3 兼容）", { exact: true }),
  ).toBeVisible();
  const me = await (await page.request.get("/api/v1/me")).json();
  // Installable web app: the manifest ships with the built assets and stays
  // reachable from the deployed origin.
  await expect(page.locator('link[rel="manifest"]')).toHaveCount(1);
  expect(
    (await page.request.get("/manifest.webmanifest")).status(),
  ).toBe(200);
  const spaces = await (await page.request.get("/api/v1/spaces")).json();
  const listing = await (
    await page.request.get("/api/v1/files?space_id=" + spaces.data[0].id)
  ).json();
  const file = listing.data.find((f: any) => f.path === name);
  expect(Boolean(file)).toBe(true);
  await page.getByRole("button", { name: new RegExp("详情 " + name) }).click();
  await expect(
    page.getByText("所属存储：对象存储（S3 兼容） · 仅传输"),
  ).toBeVisible();
  // Hold the already-committed version response so a plain drop occurs before
  // React has cleared the version-picker state. The new file must stay independent.
  let releaseVersion!: () => void;
  const versionRelease = new Promise<void>(resolve => { releaseVersion = resolve; });
  let versionDelivered!: () => void;
  const delivered = new Promise<void>(resolve => { versionDelivered = resolve; });
  let holdVersion = true;
  await page.route("**/api/v1/operations/*/complete", async route => {
    if (!holdVersion) { await route.continue(); return; }
    holdVersion = false;
    const response = await route.fetch();
    if (!response.ok()) throw new Error("version_fixture_completion_failed");
    await versionRelease; await route.fulfill({ response }); versionDelivered();
  });
  await page.getByRole("button", { name: "上传新版本", exact: true }).click();
  await page.locator("input[type=file]").setInputFiles({
    name,
    mimeType: "text/markdown",
    buffer: Buffer.from("# Updated\n"),
  });
  await expect
    .poll(async () => {
      const response = await page.request.get(
        "/api/v1/files/" + file.id + "/versions",
      );
      return (await response.json()).data.length;
    })
    .toBe(2);
  const dataTransfer = await page.evaluateHandle(() => {
    const d = new DataTransfer();
    d.items.add(
      new File(["dragged file"], "drag-" + Date.now() + ".txt", {
        type: "text/plain",
      }),
    );
    return d;
  });
  await page
    .getByRole("region", { name: "文件浏览" })
    .dispatchEvent("drop", { dataTransfer });
  await expect(
    page.getByRole("button", { name: /drag-.*\.txt/ }).first(),
  ).toBeVisible();
  releaseVersion();
  await delivered;
  await page.unroute("**/api/v1/operations/*/complete");
  expect((await (await page.request.get("/api/v1/files/" + file.id + "/versions")).json()).data).toHaveLength(2);
  await dataTransfer.dispose();
  fs.mkdirSync(folder, { recursive: true });
  await page.screenshot({
    path: path.join(folder, "desktop-files.png"),
  });
  await page.getByRole("button", { name: "Agent 管理", exact: true }).click();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  await page.getByRole("button", { name: "授权机器人", exact: true }).click();
  await page.getByLabel("机器人名称").fill("Read-only assistant");
  await page.getByRole("button", { name: "创建限定授权" }).click();
  await expect(page.getByLabel("新机器人 token")).toBeVisible();
  const token = await page.getByLabel("新机器人 token").inputValue();
  expect(token.length > 64).toBe(true);
  await page.getByRole("button", { name: "已安全保存" }).click();
  const allowed = await request.get("/api/v1/files/" + file.id, {
    headers: { Authorization: "Bearer " + token },
  });
  expect(allowed.status()).toBe(200);
  const denied = await request.post("/api/v1/uploads", {
    headers: {
      Authorization: "Bearer " + token,
      "Idempotency-Key": "readonly-attempt",
    },
    data: {
      space_id: spaces.data[0].id,
      path: "blocked.txt",
      size_bytes: 1,
      sha256: "a".repeat(64),
    },
  });
  expect(denied.status()).toBe(403);
  const agentCard = page
    .locator(".agent-card")
    .filter({ has: page.getByRole("heading", { name: /Read-only assistant/ }) });
  await expect(agentCard).toBeVisible();
  await agentCard.getByRole("button", { name: "撤销授权", exact: true }).click();
  await page.getByRole("button", { name: "确认撤销" }).click();
  await expect
    .poll(async () =>
      (
        await request.get("/api/v1/spaces", {
          headers: { Authorization: "Bearer " + token },
        })
      ).status(),
    )
    .toBe(401);
  const bob = await browser.newContext();
  const bobPage = await bob.newPage();
  await login(bobPage, "Bob");
  expect((await bobPage.request.get("/api/v1/files/" + file.id)).status()).toBe(
    404,
  );
  expect(
    (await bobPage.request.get("/api/v1/operations/" + file.id)).status(),
  ).toBe(404);
  await bob.close();
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  await page.getByRole("button", { name: new RegExp("详情 " + name) }).click();
  await page.getByRole("button", { name: "移入回收站" }).click();
  await page.getByRole("button", { name: "回收站", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "恢复 " + name, exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "恢复 " + name, exact: true }).click();
  await expect(
    page.getByRole("button", { name: "恢复 " + name, exact: true }),
  ).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("navigation", { name: "手机主导航" }).getByRole("button", { name: "文件", exact: true }).click();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  await page.screenshot({
    path: path.join(folder, "mobile-files.png"),
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
test("interrupted transfer resumes the same operation after reload", async ({
  page,
}) => {
  await login(page);
  let stopped = false;
  await page.route("**/api/v1/operations/*/parts/*", async (route) => {
    if (!stopped) {
      stopped = true;
      await route.abort("failed");
    } else await route.continue();
  });
  const name = "resume-" + Date.now() + ".txt";
  const source = {
    name,
    mimeType: "text/plain",
    buffer: Buffer.from("resume this same content"),
  };
  await page.locator("input[type=file]").setInputFiles(source);
  await expect(page.getByRole("alert")).toBeVisible();
  const before = (
    await (await page.request.get("/api/v1/operations")).json()
  ).data.find((o: any) => o.path === name);
  expect(Boolean(before)).toBe(true);
  await page.unroute("**/api/v1/operations/*/parts/*");
  await page.reload();
  await page.getByRole("button", { name: "打开交付中心", exact: true }).click();
  const row = page.locator("[data-list-item]").filter({ hasText: name });
  await row.getByRole("button", { name: "恢复", exact: true }).click();
  await page.locator("input[type=file]").setInputFiles(source);
  await expect(row).toContainText("已完成");
  const after = (
    await (await page.request.get("/api/v1/operations")).json()
  ).data.filter((o: any) => o.path === name);
  expect(after.length).toBe(1);
  expect(after[0].id).toBe(before.id);
});

test("file provenance, pinned copy and result navigation", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await login(page);
  const name = "loop-" + Date.now() + ".md";
  await page.locator("input[type=file]").setInputFiles({
    name,
    mimeType: "text/markdown",
    buffer: Buffer.from("# Saved report\n"),
  });
  await expect(
    page.getByRole("button", { name: "详情 " + name, exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "详情 " + name, exact: true }).click();
  await page.getByRole("button", { name: "文件标识与引用", exact: true }).click();
  await expect(
    page.getByText("来源：drivebridge-client", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "复制文件引用", exact: true }).click();
  await expect(
    page.getByText("已复制固定版本引用；在已连接 DriveBridge 的 Agent 中使用", {
      exact: true,
    }),
  ).toBeVisible();
  fs.mkdirSync(folder, { recursive: true });
  await page.screenshot({
    path: path.join(folder, "file-provenance.png"),
  });
  const ref = await page.evaluate(() => navigator.clipboard.readText());
  expect(ref).toMatch(/^drivebridge:\/\/[^/]+\/file\/[^/]+\/version\/[^/]+$/);
  const resolved = await page.request.get(
    "/api/v1/references?ref=" + encodeURIComponent(ref),
  );
  expect(resolved.status()).toBe(200);
  expect((await resolved.json()).data.file.caller).toBe("drivebridge-client");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "打开交付中心", exact: true }).click();
  const card = page.locator("[data-list-item]").filter({ hasText: name });
  await card.getByRole("button", { name: "查看结果", exact: true }).click();
  await page.getByRole("button", { name: "文件标识与引用", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "复制文件引用", exact: true }),
  ).toBeVisible();
});
