import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { file7, folder7, details7, choose7, operation7 } from "./v7-helpers";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
async function sheet(page: Page) {
  await page.goto("/opendesign.html#f/personal/f6/2");
  await expect(page.locator("[data-sheet-preview]")).toBeVisible();
}
async function settings(page: Page) {
  await page.getByRole("button", { name: "阅读设置", exact: true }).click();
  return page.getByRole("dialog").last();
}
async function pickSheet(page: Page, name: string) {
  await settings(page);
  await choose7(page, "工作表", name);
  await page.keyboard.press("Escape");
}
test("v91 overlay details preserve preview geometry and explicit pinning", async ({
  page,
}, info) => {
  if (info.project.name === "desktop")
    await page.setViewportSize({ width: 993, height: 953 });
  await sheet(page);
  await expect(
    page.getByRole("button", { name: "收起详情", exact: true }),
  ).toHaveCount(0);
  const stage = page.locator("[data-preview-stage]"),
    before = await stage.boundingBox();
  await stage.evaluate((el) => el.setAttribute("data-probe", "stable"));
  await page.getByRole("button", { name: "显示文件详情", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "详情", exact: true }),
  ).toBeVisible();
  expect(await stage.boundingBox()).toEqual(before);
  await expect(stage).toHaveAttribute("data-probe", "stable");
  await expect(
    page.getByRole("button", { name: "固定到右侧", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "版本", exact: true }).click();
  await expect(stage).toHaveAttribute("data-probe", "stable");
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await expect(page.locator('[data-inspector-dock="bottom"]')).toHaveCount(0);
  await expect(page.locator('[aria-label="预览播放与缩放"]')).toHaveCount(0);
  if (info.project.name === "desktop") {
    await page.setViewportSize({ width: 1440, height: 953 });
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
    await page.getByRole("button", { name: "固定到右侧", exact: true }).click();
    await expect(page.locator('[data-inspector-dock="right"]')).toBeVisible();
    expect((await stage.boundingBox())!.width).toBeGreaterThanOrEqual(440);
    await expect(stage).toHaveAttribute("data-probe", "stable");
    await page.setViewportSize({ width: 914, height: 953 });
    await expect(page.locator('[data-inspector-dock="overlay"]')).toBeVisible();
    await expect(stage).toHaveAttribute("data-probe", "stable");
    await page.setViewportSize({ width: 1440, height: 953 });
    await expect(page.locator('[data-inspector-dock="right"]')).toBeVisible();
    await page
      .getByRole("button", { name: "取消固定详情", exact: true })
      .click();
    await expect(page.locator('[data-inspector-dock="overlay"]')).toBeVisible();
  }
});
test("v91 grid uses fixed widths and dense rows, clipboard fallback retains full text", async ({
  page,
}, info) => {
  await sheet(page);
  const row = page.locator("[data-sheet-row]").first();
  expect((await row.boundingBox())!.height).toBeLessThanOrEqual(
    info.project.name === "desktop" ? 29 : 45,
  );
  expect((await page.getByRole("table").boundingBox())!.width).toBe(620);
  await page.evaluate(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: () => Promise.reject(new Error("denied")) },
    }),
  );
  await pickSheet(page, "长文本示例");
  await page
    .locator('[data-sheet-cell][data-row="0"][data-column="3"]')
    .press("Enter");
  await expect(page.getByLabel("单元格完整内容", { exact: true })).toHaveText(
    /二十分钟/,
  );
  await page.getByRole("button", { name: "复制单元格", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("手动复制");
  await page.keyboard.press("Escape");
  const column = page.getByRole("separator", {
    name: "调整 备注 列宽",
    exact: true,
  });
  const before = Number(await column.getAttribute("aria-valuenow"));
  await column.focus();
  await column.press("ArrowRight");
  await expect(column).toHaveAttribute("aria-valuenow", String(before + 16));
});
test("v91 pagination worksheet state and error recovery are bounded", async ({
  page,
}) => {
  await sheet(page);
  await pickSheet(page, "千行样本");
  await expect(page.locator("[data-sheet-row]")).toHaveCount(50);
  await settings(page);
  await page
    .getByRole("dialog")
    .last()
    .getByRole("button", { name: "表格下一页", exact: true })
    .click();
  await page.keyboard.press("Escape");
  await expect(
    page.locator('[data-sheet-cell][data-column="0"]').first(),
  ).toHaveText("明细 0051");
  await pickSheet(page, "旅行收支");
  await expect(page.locator("[data-sheet-row]")).toHaveCount(4);
  await pickSheet(page, "千行样本");
  await expect(
    page.locator('[data-sheet-cell][data-column="0"]').first(),
  ).toHaveText("明细 0051");
  await settings(page);
  await page.getByRole("textbox", { name: "查找表格内容" }).fill("0999");
  await page.keyboard.press("Escape");
  await expect(page.locator("[data-sheet-row]")).toHaveCount(1);
  await expect(page.locator('[data-sheet-cell][data-column="0"]')).toHaveText(
    "明细 0999",
  );
  await settings(page);
  await choose7(page, "表格演示状态", "读取失败");
  await page.keyboard.press("Escape");
  await expect(page.getByText("表格读取失败", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "重试当前工作表" }).click();
  await expect(page.locator("[data-sheet-row]")).toHaveCount(1);
  await pickSheet(page, "空表示例");
  await expect(
    page.getByText("这个工作表是空的", { exact: true }),
  ).toBeVisible();
});
test("v91 discussion drafts and tab selection survive file changes", async ({
  page,
}, info) => {
  await folder7(page);
  await page
    .locator('[data-file-id="f1"]')
    .getByRole("button", { name: /^预览 / })
    .click();
  await details7(page, "讨论");
  await page
    .getByRole("textbox", { name: "版本评论", exact: true })
    .fill("保留我的评论草稿");
  await page.getByRole("button", { name: "详情", exact: true }).click();
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "版本评论", exact: true }),
  ).toHaveValue("保留我的评论草稿");
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  if (
    (await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .getAttribute("aria-pressed")) === "false"
  )
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  await page.locator('[data-preview-file="f2"]').click();
  const list = page.locator('[data-preview-file="f1"]');
  if (!(await list.isVisible()))
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  await list.click();
  await page.getByRole("button", { name: "显示文件详情", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "版本评论", exact: true }),
  ).toHaveValue("保留我的评论草稿");
});
test("v91 target widths preserve bars, no bottom details and no page overflow", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Width matrix covered once");
  await sheet(page);
  for (const width of [320, 390, 768, 914, 993, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 953 });
    await expect
      .poll(() =>
        page
          .locator("[data-preview-workspace]")
          .evaluate((el) => Math.round(el.getBoundingClientRect().width)),
      )
      .toBe(width >= 1280 ? width - 224 : width);
    await expect(page.locator("[data-preview-toolbar]")).toHaveCSS(
      "height",
      "44px",
    );
    const m = await page.evaluate(() => ({
      width: innerWidth,
      scroll: document.documentElement.scrollWidth,
      right: document
        .querySelector("[data-preview-workspace]")!
        .getBoundingClientRect().right,
      toolbar: document.querySelector("[data-preview-toolbar]")!.scrollWidth,
      client: document.querySelector("[data-preview-toolbar]")!.clientWidth,
    }));
    expect(m.scroll, `${width}`).toBeLessThanOrEqual(width);
    expect(m.right).toBeLessThanOrEqual(width + 1);
    expect(m.toolbar, `${width} toolbar`).toBeLessThanOrEqual(m.client + 1);
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "收起详情", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "收起详情", exact: true }).click();
    await expect(
      page.locator(
        '[data-inspector-dock="bottom"],[aria-label="预览播放与缩放"]',
      ),
    ).toHaveCount(0);
  }
});
test("v91 sheet version positions and keyboard cells are independent", async ({
  page,
}) => {
  await sheet(page);
  await pickSheet(page, "千行样本");
  await settings(page);
  await page
    .getByRole("dialog")
    .last()
    .getByRole("button", { name: "表格下一页", exact: true })
    .click();
  await page.keyboard.press("Escape");
  const scroll = page.locator("[data-sheet-scroll]");
  await scroll.evaluate((el) => {
    el.scrollTop = 120;
  });
  await expect.poll(() => scroll.evaluate((el) => el.scrollTop)).toBe(120);
  await details7(page, "版本");
  await page.getByRole("button", { name: "查看", exact: true }).click();
  await expect(page.locator('[data-renderer="f6@1"]')).toBeVisible();
  await page.getByRole("button", { name: "查看", exact: true }).click();
  await expect(page.locator('[data-renderer="f6@2"]')).toBeVisible();
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await expect(
    page.locator('[data-sheet-cell][data-column="0"]').first(),
  ).toHaveText("明细 0051");
  await expect.poll(() => scroll.evaluate((el) => el.scrollTop)).toBe(120);
  const first = page.locator(
    '[data-sheet-cell][data-row="50"][data-column="0"]',
  );
  await first.focus();
  await first.press("ArrowRight");
  await expect(
    page.locator('[data-sheet-cell][data-row="50"][data-column="1"]'),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    page.getByLabel("单元格完整内容", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await settings(page);
  await choose7(page, "表格演示状态", "加载中");
  await page.keyboard.press("Escape");
  await expect(page.getByText("正在读取表格…", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "结束模拟加载" }).click();
  await expect(page.locator("[data-sheet-row]")).toHaveCount(50);
});
test("v91 list stays compact and media controls stay within the canvas", async ({
  page,
}, info) => {
  await file7(page, "f1");
  const list = page.locator("[data-preview-list]");
  if (!(await list.isVisible()))
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  const row = page.locator('[data-preview-file="f1"]');
  expect((await row.boundingBox())!.height).toBe(
    info.project.name === "desktop" ? 36 : 44,
  );
  await page.locator('[data-preview-file="f4"]').click();
  await expect(page.getByRole("group", { name: "媒体播放控制" })).toBeVisible();
  const frame = (await page.locator("[data-media-frame]").boundingBox())!;
  const controls = (await page
    .getByRole("group", { name: "媒体播放控制" })
    .boundingBox())!;
  expect(controls.y).toBeGreaterThanOrEqual(frame.y);
  expect(controls.y + controls.height).toBeLessThanOrEqual(
    frame.y + frame.height,
  );
  await page.getByRole("button", { name: "播放模拟片段" }).click();
  await expect(
    page.getByRole("button", { name: "暂停模拟播放" }),
  ).toBeVisible();
  if (!(await list.isVisible()))
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  await page.locator('[data-preview-file="f1"]').click();
  if (!(await list.isVisible()))
    await page
      .getByRole("button", { name: "显示文件列表", exact: true })
      .click();
  await page.locator('[data-preview-file="f4"]').click();
  await expect(
    page.getByRole("button", { name: "播放模拟片段" }),
  ).toBeVisible();
  await expect(page.locator('[aria-label="预览播放与缩放"]')).toHaveCount(0);
});
