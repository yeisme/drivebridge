import { test, expect } from "@playwright/test";
import { nav7, choose7 } from "./v7-helpers";
const errors = new WeakMap();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
test("v92 single cell selection compact disclosure and copy", async ({
  page,
}, info) => {
  await page.goto("/opendesign.html#f/personal/f6/2");
  const cell = page.locator('[data-sheet-cell][data-row="1"][data-column="0"]');
  await cell.click();
  await expect(page.locator("[data-cell-popover]")).toHaveCount(0);
  if (info.project.name === "mobile")
    await page.getByRole("button", { name: "查看所选单元格内容" }).click();
  else await cell.dblclick();
  const popup = page.locator("[data-cell-popover]");
  await expect(popup).toBeVisible();
  expect((await popup.boundingBox())!.height).toBeLessThan(160);
  expect((await popup.boundingBox())!.width).toBeLessThanOrEqual(320);
  await expect(popup.getByRole("textbox")).toHaveCount(0);
  await expect(popup.getByLabel("单元格完整内容")).toHaveText("住宿");
  await page.getByRole("button", { name: "关闭单元格内容" }).click();
  await expect(cell).toBeFocused();
  await page.evaluate(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: () => Promise.reject(new Error("denied")) },
    }),
  );
  await cell.press("Control+c");
  await expect(popup).toBeVisible();
  await expect(popup).toContainText("手动复制");
  await page.keyboard.press("Escape");
  await expect(popup).toHaveCount(0);
  await cell.press("Enter");
  await expect(popup).toBeVisible();
  await page.locator("[data-sheet-scroll]").dispatchEvent("scroll");
  await expect(popup).toHaveCount(0);
});
test("v92 knowledge compact favorite filter and stable editing", async ({
  page,
}, info) => {
  await nav7(page, "知识库");
  const row = page.locator('[data-knowledge-file="k-guide"]');
  await expect(row).toBeVisible();
  expect((await row.boundingBox())!.height).toBeLessThanOrEqual(
    info.project.name === "desktop" ? 37 : 60,
  );
  const before = await row.boundingBox();
  await row.getByRole("button", { name: /^收藏 / }).click();
  expect(await row.boundingBox()).toEqual(before);
  await choose7(page, "文档筛选", "已收藏");
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "继续编辑", exact: true }).focus();
  await row.getByRole("button", { name: "继续编辑", exact: true }).click();
  await expect(page.locator(".cm-editor")).toBeVisible();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  if (info.project.name === "mobile")
    await page.getByRole("button", { name: "文档视图", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "文档筛选" })).toContainText(
    "已收藏",
  );
});
test("v92 resource filter draft cancel and compact rows", async ({
  page,
}, info) => {
  await nav7(page, "资源库");
  const rows = page.locator("[data-resource-list] [data-find-entry]");
  const count = await rows.count();
  const y = (await rows.first().boundingBox())!.y;
  await rows.first().getByRole("checkbox").check();
  await expect(page.getByRole("toolbar", { name: "批量操作" })).toBeVisible();
  expect((await rows.first().boundingBox())!.y).toBe(y);
  await page.getByRole("button", { name: "取消多选", exact: true }).click();
  expect(count).toBeGreaterThan(0);
  expect((await rows.first().boundingBox())!.height).toBeLessThanOrEqual(
    info.project.name === "desktop" ? 37 : 60,
  );
  await page.getByRole("button", { name: "筛选资源", exact: true }).click();
  await choose7(page, "资源类型", "提示词模板");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(rows).toHaveCount(count);
  await page.getByRole("button", { name: "筛选资源", exact: true }).click();
  await choose7(page, "资源类型", "提示词模板");
  await page.getByRole("button", { name: "应用", exact: true }).click();
  expect(await rows.count()).toBeLessThan(count);
  await rows.first().locator("[data-entry-name]").click();
  await expect(page.getByRole("dialog")).toBeVisible();
});
test("v92 document and resource fixtures mount one page and restore navigation", async ({
  page,
}) => {
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page
    .getByRole("button", { name: "文档与资源分页样本", exact: true })
    .click();
  await nav7(page, "知识库");
  await expect(page.locator("[data-knowledge-file]")).toHaveCount(50);
  expect(
    await page
      .locator("[data-file-toolbar]")
      .evaluate((el) => el.scrollWidth - el.clientWidth),
  ).toBeLessThanOrEqual(1);
  const first = await page
    .locator("[data-knowledge-file]")
    .first()
    .getAttribute("data-knowledge-file");
  await page.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(page.locator("[data-knowledge-file]")).toHaveCount(50);
  expect(
    await page
      .locator("[data-knowledge-file]")
      .first()
      .getAttribute("data-knowledge-file"),
  ).not.toBe(first);
  await page.locator("[data-knowledge-file] [data-entry-name]").first().click();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await expect(page.getByLabel("列表分页")).toContainText("2/3");
  await nav7(page, "资源库");
  await expect(
    page.locator("[data-resource-list] [data-find-entry]"),
  ).toHaveCount(50);
  expect(
    await page
      .locator("[data-file-toolbar]")
      .evaluate((el) => el.scrollWidth - el.clientWidth),
  ).toBeLessThanOrEqual(1);
  await page.getByRole("button", { name: "下一页", exact: true }).click();
  expect(
    await page.locator("[data-resource-list] [data-find-entry]").count(),
  ).toBeLessThanOrEqual(50);
});
test("v92 all page density and responsive surfaces", async ({ page }, info) => {
  test.setTimeout(180000);
  page.setDefaultTimeout(7000);
  if (info.project.name === "mobile") test.skip();
  const routes = [
    "我的工作台",
    "我的云盘",
    "知识库",
    "资源库",
    "项目集合",
    "标签与属性",
    "搜索",
    "任务中心",
    "产物与收件",
    "共享与协作",
    "分享与交付",
    "外部调用",
    "存储管理",
    "设备与离线",
    "备份与整理",
    "回收站",
    "能力与连接说明",
    "设置",
  ];
  for (const theme of ["切换深色", "切换浅色"]) {
    await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
    await page.getByRole("button", { name: theme, exact: true }).click();
    await page.keyboard.press("Escape");
    for (const route of routes) {
      await nav7(page, route);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - innerWidth,
        ),
        `${theme} ${route}`,
      ).toBeLessThanOrEqual(1);
      const heading = page.locator("[data-page-heading]").first();
      if (await heading.isVisible())
        expect((await heading.boundingBox())!.height).toBe(44);
    }
  }
  for (const width of [320, 390, 768, 855, 914, 993, 1024, 1305, 1440, 1920]) {
    await page.setViewportSize({ width, height: 953 });
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    for (const route of [
      "知识库",
      "资源库",
      "项目集合",
      "搜索",
      "任务中心",
      "产物与收件",
      "存储管理",
      "设备与离线",
      "设置",
    ]) {
      await nav7(page, route);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - innerWidth,
        ),
        `${width} ${route}`,
      ).toBeLessThanOrEqual(1);
      if (["知识库", "资源库"].includes(route))
        expect(
          (await page.locator("[data-view-toolbar]").boundingBox())!.height,
        ).toBe(44);
    }
  }
});
test("v92 favorites use dense identity rows and keep batch geometry", async ({
  page,
}, info) => {
  await nav7(page, "我的云盘");
  await page
    .locator("[data-directory-id]")
    .filter({ hasText: "山野计划" })
    .first()
    .click();
  await expect(
    page
      .locator('[data-file-id="f1"]')
      .getByRole("button", { name: /^取消收藏 / }),
  ).toHaveAttribute("aria-pressed", "true");
  if (
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .isVisible()
  )
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .click();
  await page
    .getByRole("button", { name: /本盘收藏/ })
    .first()
    .click();
  const row = page
    .locator("[data-bookmark]")
    .filter({ hasText: "山野计划 · 创作简报.pdf" });
  await expect(row).toBeVisible();
  const before = await row.boundingBox();
  await row.getByRole("checkbox").check();
  expect((await row.boundingBox())!.y).toBe(before!.y);
  await page.getByRole("button", { name: "取消多选", exact: true }).click();
  await page.getByRole("textbox", { name: "搜索本盘收藏" }).fill("创作简报");
  if (info.project.name === "desktop")
    for (const width of [320, 390, 914, 1920]) {
      await page.setViewportSize({ width, height: 953 });
      await page.evaluate(
        () =>
          new Promise<void>((r) =>
            requestAnimationFrame(() => requestAnimationFrame(() => r())),
          ),
      );
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - innerWidth,
        ),
      ).toBeLessThanOrEqual(1);
    }
  await page.getByRole("button", { name: "清除收藏搜索" }).click();
  await expect(page.getByRole("textbox", { name: "搜索本盘收藏" })).toHaveValue(
    "",
  );
});
