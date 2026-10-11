import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
async function file(page: Page, id = "f1") {
  await page
    .locator(`[data-file-id="${id}"]`)
    .getByRole("button", { name: /^预览 / })
    .click();
  await expect(page.locator("[data-preview-workspace]")).toBeVisible();
}
async function agent(page: Page) {
  await page
    .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
    .click();
  await expect(page.locator("[data-global-agent]")).toBeVisible();
}
async function showDetails(page: Page) {
  if (
    !(await page.getByRole("button", { name: "讨论", exact: true }).isVisible())
  )
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
}

test("v5.1 global Agent and page details coexist and survive docking changes", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await page.setViewportSize({ width: 1920, height: 1000 });
  await file(page);
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await page
    .getByRole("textbox", { name: "版本评论", exact: true })
    .fill("保留页面内草稿");
  await page
    .locator("[data-renderer]")
    .evaluate((el) => ((el as HTMLElement).dataset.identity = "same-content"));
  await agent(page);
  await expect(page.locator("[data-inspector-dock=right]")).toBeVisible();
  expect(
    await page.locator("[data-preview-workspace] [data-global-agent]").count(),
  ).toBe(0);
  const global = await page.locator("[data-global-agent]").boundingBox();
  const bar = await page.getByRole("banner").boundingBox();
  expect(Math.abs(global!.y - (bar!.y + bar!.height))).toBeLessThanOrEqual(1);
  expect(Math.abs(global!.y + global!.height - 1000)).toBeLessThanOrEqual(1);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(page.locator("[data-inspector-dock=bottom]")).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "版本评论", exact: true }),
  ).toHaveValue("保留页面内草稿");
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-identity",
    "same-content",
  );
  await page.getByRole("textbox", { name: "版本评论", exact: true }).focus();
  await page.keyboard.press("Escape");
  await expect(page.locator('[aria-label="页面文件详情"]')).toBeHidden();
  await expect(page.locator("[data-global-agent]")).toBeVisible();
  await showDetails(page);
  await page
    .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
    .click();
  await expect(page.locator("[data-global-agent]")).toBeHidden();
  await expect(page.locator("[data-inspector-dock=right]")).toBeVisible();
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await expect(page.locator('[aria-label="页面文件详情"]')).toBeHidden();
  expect((await page.locator("[data-agent-host]").boundingBox())!.width).toBe(
    0,
  );
});
test("v5.1 Agent host preserves conversation when navigating pages and opening references", async ({
  page,
}, info) => {
  await agent(page);
  const input = page.getByRole("textbox", { name: "向助手提问", exact: true });
  await input.fill("应用级会话草稿");
  await page
    .getByRole("region", { name: "助手会话" })
    .evaluate((el) => ((el as HTMLElement).dataset.identity = "persistent"));
  const context = await page.locator("[data-context-rail]").innerText();
  await page
    .locator("[data-context-rail] [data-file-preview-trigger]")
    .first()
    .click();
  await expect(page.locator("[data-preview-workspace]")).toBeVisible();
  if (info.project.name === "desktop") {
    await expect(page.locator("[data-global-agent]")).toBeVisible();
    await expect(input).toHaveValue("应用级会话草稿");
    await page
      .getByRole("navigation", { name: "工作区导航" })
      .getByRole("button", { name: "任务中心", exact: true })
      .click();
    await expect(page.locator("[data-global-agent]")).toBeVisible();
  } else {
    await expect(page.locator("[data-global-agent]")).toBeHidden();
    await agent(page);
  }
  await expect(page.getByRole("region", { name: "助手会话" })).toHaveAttribute(
    "data-identity",
    "persistent",
  );
  await expect(input).toHaveValue("应用级会话草稿");
  expect(await page.locator("[data-context-rail]").innerText()).toBe(context);
});
test("v5.1 navigation and file list collapse to zero and restore from toolbar", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await page.getByRole("button", { name: "收起主导航", exact: true }).click();
  await expect(
    page.getByRole("complementary", { name: "主导航", exact: true }),
  ).toHaveCount(0);
  expect((await page.getByRole("banner").boundingBox())!.x).toBe(0);
  await page.getByRole("button", { name: "显示主导航", exact: true }).click();
  expect(
    (await page
      .getByRole("complementary", { name: "主导航", exact: true })
      .boundingBox())!.width,
  ).toBe(224);
  await file(page);
  const before = await page.locator("[data-preview-stage]").boundingBox();
  await page.getByRole("button", { name: "显示文件列表", exact: true }).click();
  await expect(
    page.getByRole("separator", { name: "调整文件列表宽度" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("complementary", { name: "预览文件导航" }),
  ).toHaveCount(0);
  expect(
    (await page.locator("[data-preview-stage]").boundingBox())!.width,
  ).toBeGreaterThan(before!.width);
  await page.getByRole("button", { name: "显示文件列表", exact: true }).click();
  await expect(
    page.getByRole("complementary", { name: "预览文件导航" }),
  ).toBeVisible();
  await page.setViewportSize({ width: 1024, height: 900 });
  await expect(
    page.getByRole("complementary", { name: "主导航", exact: true }),
  ).toHaveCount(0);
  const rect = await page.locator("[data-preview-workspace]").boundingBox();
  await page.getByRole("button", { name: "显示主导航", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "工作区导航" })).toBeVisible();
  expect(await page.locator("[data-preview-workspace]").boundingBox()).toEqual(
    rect,
  );
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "显示主导航", exact: true }),
  ).toBeFocused();
});
test("v5.1 hover opens at 400ms without navigation and click bypasses the timer", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  const first = page
    .locator("[data-file-id=f1]")
    .getByRole("button", { name: /^预览 / });
  const rect = await first.boundingBox();
  await first.hover();
  await page.clock.runFor(399);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await page.clock.runFor(1);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(1);
  await page.clock.runFor(32);
  await expect(page.locator("[data-hover-preview]")).toContainText("山野计划");
  expect(await page.evaluate(() => location.hash)).toBe("");
  expect(await first.boundingBox()).toEqual(rect);
  await expect(page.locator("[data-preview-workspace]")).toHaveCount(0);
  await page.locator("[data-hover-preview]").hover();
  await page.clock.runFor(180);
  await expect(page.locator("[data-hover-preview]")).toBeVisible();
  await first.hover();
  await page.clock.runFor(1);
  await expect(page.locator("[data-hover-preview]")).toBeVisible();
  await page.mouse.move(10, 10);
  await page.clock.runFor(149);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(1);
  await page.clock.runFor(1);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await first.hover();
  await page.clock.runFor(200);
  await first.click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@3",
  );
  await page.clock.runFor(500);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
});
test("v5.1 stale hovers, checkbox and tags never take over the page", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  const first = page
    .locator("[data-file-id=f1]")
    .getByRole("button", { name: /^预览 / });
  const second = page
    .locator("[data-file-id=f2]")
    .getByRole("button", { name: /^预览 / });
  await first.hover();
  await page.clock.runFor(399);
  await second.hover();
  await page.clock.runFor(1);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await page.clock.runFor(399);
  await expect(page.locator("[data-hover-preview]")).toContainText("秋日山行");
  await page.keyboard.press("Escape");
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await expect(page.locator("[data-preview-workspace]")).toHaveCount(0);
  await page.locator("[data-file-id=f1]").getByRole("checkbox").hover();
  await page.clock.runFor(450);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await page.locator('[data-file-id=f1] [aria-label="文件标签"]').hover();
  await page.clock.runFor(450);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await first.hover();
  await page.clock.runFor(200);
  await page
    .getByRole("main")
    .evaluate((el) => el.dispatchEvent(new Event("scroll")));
  await page.clock.runFor(500);
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
});
test("v5.1 hover reference works inside Agent and Escape dismisses only the card", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await agent(page);
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  const ref = page
    .locator("[data-context-rail] [data-file-preview-trigger]")
    .first();
  await ref.hover();
  await page.clock.runFor(450);
  await expect(page.locator("[data-hover-preview]")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  await expect(page.locator("[data-global-agent]")).toBeVisible();
  await page.mouse.move(10, 10);
  await ref.hover();
  await page.clock.runFor(450);
  await page.getByRole("button", { name: "打开此版本", exact: true }).click();
  await expect(page.locator("[data-preview-workspace]")).toBeVisible();
  await expect(page.locator("[data-global-agent]")).toBeVisible();
});
test("v5.1 responsive panel bounds, touch and reduced motion", async ({
  page,
}, info) => {
  if (info.project.name === "mobile") {
    await page
      .locator("[data-file-id=f1]")
      .getByRole("button", { name: /^预览 / })
      .tap();
    await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
    await showDetails(page);
    await expect(page.locator("[data-inspector-dock=sheet]")).toBeVisible();
    await agent(page);
    await expect(
      page.getByRole("dialog", { name: "Agent 侧栏" }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .click();
    await expect(page.locator("[data-inspector-dock=sheet]")).toBeVisible();
    return;
  }
  await page.emulateMedia({ reducedMotion: "reduce" });
  for (const width of [320, 390, 768, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/opendesign.html#f/personal/f2/2");
    await expect(page.locator("[data-renderer]")).toBeVisible();
    await agent(page);
    if (width >= 1280) {
      await showDetails(page);
      const a = (await page.locator("[data-global-agent]").boundingBox())!;
      const d = (await page
        .locator('[aria-label="页面文件详情"]')
        .boundingBox())!;
      const p = (await page.locator("[data-preview-workspace]").boundingBox())!;
      expect(d.x + d.width).toBeLessThanOrEqual(p.x + p.width + 1);
      expect(d.x + d.width).toBeLessThanOrEqual(a.x + 1);
    }
    await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .click();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
  }
});

test("v5.1 gallery clicks jump directly and hover references cover task and inbox entrypoints", async ({
  page,
}, info) => {
  await page.getByRole("button", { name: "画廊视图", exact: true }).click();
  await file(page, "f2");
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f2@2",
  );
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  const bottom = page.getByRole("navigation", { name: "底部导航" });
  if (await bottom.isVisible())
    await bottom.getByRole("button", { name: "任务", exact: true }).click();
  else
    await page
      .getByRole("navigation", { name: "工作区导航" })
      .getByRole("button", { name: "任务中心", exact: true })
      .click();
  const link = page
    .locator("[data-task-id=DB-024] [data-file-preview-trigger]")
    .first();
  await expect(link).toHaveAttribute("data-file-preview-trigger", "f1@3");
  if (info.project.name === "desktop") {
    await link.hover();
    await expect(page.locator("[data-hover-preview]")).toBeVisible();
    await expect(page.locator("[data-hover-preview]")).toContainText("v3");
  }
  await link.click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@3",
  );
  await expect(page.locator("[data-preview-workspace]")).toContainText(
    "任务 DB-024",
  );
});

test("v5.1 confirmation Escape closes only the dialog above Agent", async ({
  page,
}) => {
  await agent(page);
  await page
    .getByRole("button", { name: "这些资料的创作方向是什么？", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "确认本次内容分析" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("dialog", { name: "确认本次内容分析" }),
  ).toHaveCount(0);
  await expect(page.locator("[data-global-agent]")).toBeVisible();
});
