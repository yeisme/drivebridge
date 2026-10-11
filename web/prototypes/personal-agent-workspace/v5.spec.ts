import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
async function shot(page: Page, name: string) {
  if (process.env.DRIVEBRIDGE_SKIP_SCREENSHOTS === "1") return;
  const dir =
    process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/prototype-diagnostics";
  fs.mkdirSync(dir, { recursive: true });
  await page.screenshot({
    path: path.join(dir, name + ".png"),
    animations: "disabled",
  });
}
async function openFile(page: Page, id = "f1") {
  await page
    .locator(`[data-file-id="${id}"]`)
    .getByRole("button", { name: /^预览 / })
    .click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toBeVisible();
}
async function details(page: Page) {
  if (
    !(await page.getByRole("button", { name: "讨论", exact: true }).isVisible())
  )
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
}
async function inbox(page: Page) {
  const bottom = page.getByRole("navigation", { name: "底部导航" });
  if (await bottom.isVisible()) {
    await bottom.getByRole("button", { name: "我的", exact: true }).click();
    await page
      .getByRole("main")
      .getByRole("button", { name: "产物与收件", exact: true })
      .click();
  } else
    await page
      .getByRole("navigation", { name: "工作区导航" })
      .getByRole("button", { name: "产物与收件", exact: true })
      .click();
}
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
test("v5 inbox scope stays two and returns to original row", async ({
  page,
}, info) => {
  await inbox(page);
  const first = page
    .getByRole("main")
    .getByRole("button", { name: "预览", exact: true })
    .first();
  const y = (await first.boundingBox())!.y;
  await shot(page, `${info.project.name}-v5-inbox`);
  await first.click();
  const workspace = page.getByRole("region", { name: "文件预览工作台" });
  await expect(workspace).toContainText("1 / 2");
  await page.getByRole("button", { name: "下一份", exact: true }).click();
  await expect(workspace).toContainText("2 / 2");
  await expect(
    page.getByRole("button", { name: "下一份", exact: true }),
  ).toBeDisabled();
  await shot(page, `${info.project.name}-v5-inbox-preview`);
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  expect(Math.abs((await first.boundingBox())!.y - y)).toBeLessThanOrEqual(1);
  await expect(first).toBeFocused();
});
test("v5 inspector keeps renderer, scroll, version tab and copy semantics", async ({
  page,
}, info) => {
  await openFile(page);
  await page.getByRole("button", { name: "放大预览", exact: true }).click();
  await page.locator("[data-reader]").evaluate((el) => (el.scrollTop = 130));
  await page
    .locator("[data-renderer]")
    .evaluate((el) => ((el as HTMLElement).dataset.identity = "preserved"));
  const before = await page.locator("[data-preview-stage]").boundingBox();
  await details(page);
  await page.getByRole("button", { name: "版本", exact: true }).click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-identity",
    "preserved",
  );
  if (info.project.name === "desktop") {
    const after = await page.locator("[data-preview-stage]").boundingBox();
    expect(after).toEqual(before);
  }
  await page.getByRole("button", { name: "查看", exact: true }).last().click();
  await expect(page.locator('[data-details-content="history"]')).toBeVisible();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@1",
  );
  await expect(page.locator("[data-preview-workspace]")).toContainText("1 /");
  await page.getByRole("button", { name: "详情", exact: true }).click();
  await page.evaluate(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          (window as any).__copied = text;
        },
      },
    }),
  );
  await page.getByRole("button", { name: "复制定位链接", exact: true }).click();
  expect(await page.evaluate(() => (window as any).__copied)).toContain(
    "#f/personal/f1/1",
  );
  await page
    .getByRole("button", { name: "复制 Agent 引用", exact: true })
    .click();
  expect(await page.evaluate(() => (window as any).__copied)).toBe(
    "drivebridge://demo-instance/file/f1/version/demo-v1",
  );
  await page
    .getByRole("button", { name: "复制 Agent 上下文", exact: true })
    .click();
  expect(await page.evaluate(() => (window as any).__copied)).toContain(
    "固定版本：v1",
  );
  await shot(page, `${info.project.name}-v5-references`);
  await page.evaluate(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async () => {
          throw Error("denied");
        },
      },
    }),
  );
  await page.getByRole("button", { name: "复制定位链接", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "手动复制内容" })).toHaveValue(
    /#f\/personal\/f1\/1/,
  );
});
test("v5 navigation and split drag preserve active ref and preview", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop");
  const nav = page.getByRole("complementary", { name: "主导航" });
  expect((await nav.boundingBox())!.width).toBe(224);
  await page.getByRole("button", { name: "折叠主导航", exact: true }).click();
  await expect(nav).toHaveCount(0);
  await openFile(page, "f2");
  const sep = page.getByRole("separator", { name: "调整文件列表宽度" });
  await page
    .locator("[data-renderer]")
    .evaluate((el) => ((el as HTMLElement).dataset.identity = "preserved"));
  const hash = await page.evaluate(() => location.hash);
  const queue = page.getByLabel("本次预览文件列表", { exact: true });
  await queue.evaluate((el) => (el.scrollTop = 110));
  const queueScroll = await queue.evaluate((el) => el.scrollTop);
  await sep.focus();
  await page.keyboard.press("Home");
  await expect(page.locator("[data-preview-list]")).toHaveAttribute(
    "data-list-density",
    "narrow",
  );
  await shot(page, "desktop-v5-narrow-list");
  await page.keyboard.press("End");
  await expect(page.locator("[data-preview-list]")).toHaveAttribute(
    "data-list-density",
    "wide",
  );
  await shot(page, "desktop-v5-wide-list");
  const pos = (await sep.boundingBox())!;
  await page.mouse.move(pos.x + 2, pos.y + 100);
  await page.mouse.down();
  await page.mouse.move(
    (await page.locator("[data-preview-workspace]").boundingBox())!.x + 320,
    pos.y + 100,
  );
  await page.mouse.up();
  await expect(page.locator("[data-preview-list]")).toHaveAttribute(
    "data-list-density",
    "standard",
  );
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-identity",
    "preserved",
  );
  expect(await page.evaluate(() => location.hash)).toBe(hash);
  expect(await queue.evaluate((el) => el.scrollTop)).toBe(queueScroll);
  await sep.dblclick();
  await expect(sep).toHaveAttribute("aria-valuenow", "280");
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await page.getByRole("textbox", { name: "版本评论" }).fill("保留详情草稿");
  await page.getByRole("button", { name: "交给助手", exact: true }).click();
  await page
    .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
    .click();
  await expect(page.getByRole("textbox", { name: "版本评论" })).toHaveValue(
    "保留详情草稿",
  );
  await shot(page, "desktop-v5-inspector-restored");
});
test("v5 location reopens pinned samples and unavailable links stay explicit", async ({
  page,
}) => {
  await page.goto("/opendesign.html#f/personal/f1/1");
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@1",
  );
  await page.goto("/opendesign.html#f/personal/f1/99");
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toContainText("固定版本不可用");
  await page.goto("/opendesign.html#f/personal/temporary-artifact/1");
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toContainText("不存在");
  await expect(page.locator("[data-renderer]")).toHaveCount(0);
});
test("v5 responsive previews remain contained and controls reachable", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop");
  for (const width of [320, 390, 768, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/opendesign.html#f/personal/f2/2");
    await expect(page.locator("[data-renderer]")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    const stage = await page.locator("[data-preview-stage]").boundingBox();
    expect(stage!.width).toBeGreaterThan(220);
    expect(stage!.height).toBeGreaterThan(200);
    await shot(page, `v5-preview-${width}`);
    const inline = await page
      .getByRole("complementary", { name: "预览文件导航" })
      .isVisible();
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
    if (inline) {
      await expect(
        page.getByRole("complementary", { name: "预览文件导航" }),
      ).toHaveCount(0);
      await page
        .getByRole("button", { name: "显示文件列表", exact: true })
        .click();
    } else {
      await expect(page.getByRole("dialog")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(page.getByRole("dialog")).toHaveCount(0);
    }
    if (width < 768) {
      const sizes = await page
        .getByRole("region", { name: "文件预览工作台" })
        .getByRole("button")
        .evaluateAll((els) =>
          els
            .filter((el) => el.getClientRects().length)
            .map((el) => ({
              name: el.getAttribute("aria-label") || el.textContent,
              width: el.getBoundingClientRect().width,
              height: el.getBoundingClientRect().height,
            })),
        );
      expect(sizes.filter((x) => x.width < 44 || x.height < 44)).toEqual([]);
      await page
        .getByRole("button", { name: "显示文件详情", exact: true })
        .click();
      await shot(page, `v5-details-${width}`);
      await page.keyboard.press("Escape");
    }
    await page.getByRole("button", { name: "退出预览", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "我的云盘", exact: true }),
    ).toBeVisible();
  }
});

test("v5 file and version reading state and media position are independent", async ({
  page,
}, info) => {
  await openFile(page);
  await page.getByRole("button", { name: "放大预览", exact: true }).click();
  await page.locator("[data-reader]").evaluate((el) => (el.scrollTop = 160));
  const scroll = await page
    .locator("[data-reader]")
    .evaluate((el) => el.scrollTop);
  await details(page);
  await page.getByRole("button", { name: "活动", exact: true }).click();
  expect(
    await page.locator("[data-reader]").evaluate((el) => el.scrollTop),
  ).toBe(scroll);
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  const settledScroll = await page
    .locator("[data-reader]")
    .evaluate((el) => el.scrollTop);
  await page.getByRole("button", { name: "下一份", exact: true }).click();
  await page.getByRole("button", { name: "上一份", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "125%", exact: true }),
  ).toBeVisible();
  expect(
    Math.abs(
      (await page.locator("[data-reader]").evaluate((el) => el.scrollTop)) -
        settledScroll,
    ),
  ).toBeLessThanOrEqual(1);
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await openFile(page);
  await expect(
    page.getByRole("button", { name: "125%", exact: true }),
  ).toBeVisible();
  expect(
    Math.abs(
      (await page.locator("[data-reader]").evaluate((el) => el.scrollTop)) -
        settledScroll,
    ),
  ).toBeLessThanOrEqual(1);
  if (!(await page.locator("[data-preview-list]").isVisible()))
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  await page
    .locator("[data-preview-list]")
    .getByRole("button", { name: /林间片段/ })
    .click();
  await page.getByRole("slider", { name: "播放位置" }).fill("12");
  await page.getByRole("button", { name: "播放模拟片段", exact: true }).click();
  await page.getByRole("button", { name: "下一份", exact: true }).click();
  await page.getByRole("button", { name: "上一份", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "播放模拟片段", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("slider", { name: "播放位置" })).toHaveValue(
    "12",
  );
  await shot(page, `${info.project.name}-v5-media-position`);
});
test("v5 external app form has visible labels and wide clean surface", async ({
  page,
}, info) => {
  const bottom = page.getByRole("navigation", { name: "底部导航" });
  if (await bottom.isVisible()) {
    await bottom.getByRole("button", { name: "我的", exact: true }).click();
    await page
      .getByRole("main")
      .getByRole("button", { name: "外部调用", exact: true })
      .click();
  } else
    await page
      .getByRole("navigation", { name: "工作区导航" })
      .getByRole("button", { name: "外部调用", exact: true })
      .click();
  await page.getByRole("button", { name: "接入应用", exact: true }).click();
  const form = page.getByRole("dialog");
  expect(Math.round((await form.boundingBox())!.width)).toBe(
    info.project.name === "desktop" ? 640 : 366,
  );
  await expect(form.getByText("应用名称", { exact: true })).toBeVisible();
  await expect(form.getByText("应用授权目录", { exact: true })).toBeVisible();
  expect(
    await page
      .locator("[data-slot=dialog-overlay]")
      .evaluate((el) => getComputedStyle(el).backdropFilter),
  ).toBe("none");
  await expect(
    form.getByRole("button", { name: "保存演示配置" }),
  ).toBeInViewport();
  await shot(page, `${info.project.name}-v5-application-form`);
});
test("v5 tablet navigation overlays without shifting files and escape restores focus", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop");
  await page.setViewportSize({ width: 1024, height: 900 });
  const f = page.locator('[data-file-id="f1"]');
  await expect(page.getByRole("complementary", { name: "主导航" })).toHaveCount(
    0,
  );
  const before = await f.boundingBox();
  const toggle = page.getByRole("button", { name: "显示主导航", exact: true });
  await toggle.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  expect(await f.boundingBox()).toEqual(before);
  await shot(page, "tablet-v5-navigation");
  await page.keyboard.press("Escape");
  await expect(toggle).toBeFocused();
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openFile(page);
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toHaveCount(0);
});
