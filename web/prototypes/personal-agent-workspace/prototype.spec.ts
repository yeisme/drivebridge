import { revealNavigation } from "./test-helpers";
async function previewDetails(page: Page) {
  if (
    !(await page.getByRole("button", { name: "讨论", exact: true }).isVisible())
  )
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
}
import { expect, test as base } from "@playwright/test";
import type { Page } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";

const test = base.extend<{ errors: string[] }>({
  errors: [
    async ({ page }, use) => {
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("requestfailed", (request) =>
        errors.push(`request_failed:${new URL(request.url()).pathname}`),
      );
      await use(errors);
      expect(errors, "No JavaScript errors or failed asset requests").toEqual(
        [],
      );
    },
    { auto: true },
  ],
});
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/prototype-diagnostics";
async function screenshot(page: Page, name: string) {
  if (process.env.DRIVEBRIDGE_SKIP_SCREENSHOTS === "1") return;
  fs.mkdirSync(artifacts, { recursive: true });
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    );
  });
  await page.screenshot({
    path: path.join(artifacts, `${name}.png`),
    animations: "disabled",
  });
}
async function choose(page: Page, label: string, option: string) {
  if (
    label === "切换空间" &&
    (await page
      .getByRole("button", { name: "切换空间", exact: true })
      .isVisible())
  ) {
    await page.getByRole("button", { name: "切换空间", exact: true }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: /山野 · 团队空间/ })
      .click();
    return;
  }
  if (
    label.startsWith("演示") &&
    !(await page
      .getByRole("combobox", { name: label, exact: true })
      .isVisible())
  )
    await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page.getByRole("combobox", { name: label, exact: true }).last().click();
  await page.getByRole("option", { name: option, exact: true }).click();
}
async function scenario(page: Page, name: string) {
  await choose(page, "演示场景", name);
  await expect(
    page.getByRole("combobox", { name: "演示场景", exact: true }),
  ).toHaveCount(0);
}
async function navigate(page: Page, target: string) {
  if (
    await page
      .getByRole("button", { name: "退出预览", exact: true })
      .isVisible()
  )
    await page.getByRole("button", { name: "退出预览", exact: true }).click();
  const close = page.getByRole("button", {
    name: "关闭 Agent 侧栏",
    exact: true,
  });
  if (await close.isVisible()) await close.click();
  const cancel = page.getByRole("button", { name: "取消多选", exact: true });
  if (await cancel.isVisible()) await cancel.click();
  if (target === "文件助手") {
    await page
      .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
      .click();
    return;
  }
  await revealNavigation(page, target);
  const label =
    target === "我的文件" || target === "素材库"
      ? "我的云盘"
      : target === "外部 Agent"
        ? "外部调用"
        : target;
  const bottom = page.getByRole("navigation", { name: "底部导航" });
  if (await bottom.isVisible()) {
    const map: Record<string, string> = {
      我的云盘: "文件",
      我的工作台: "首页",
      任务中心: "任务",
      我的: "我的",
    };
    await bottom
      .getByRole("button", { name: map[label] || "我的", exact: true })
      .click();
    if (!map[label])
      await page
        .getByRole("main")
        .getByRole("button", { name: label, exact: true })
        .click();
  } else if (label === "我的")
    await page.getByRole("button", { name: "个人设置", exact: true }).click();
  else
    await page
      .getByRole("navigation")
      .getByRole("button", { name: label, exact: true })
      .first()
      .click();
}
async function batchAction(page: Page, label: string) {
  const bar = page.getByRole("toolbar", { name: "批量操作" });
  const btn = bar.getByRole("button", { name: label, exact: true });
  if (await btn.isVisible()) await btn.click();
  else {
    await bar
      .getByRole("button", { name: "更多批量操作", exact: true })
      .click();
    await page.getByRole("menuitem", { name: label, exact: true }).click();
  }
}

async function confirm(page: Page) {
  await page
    .getByRole("button", { name: "检查并确认", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: "确认执行这份计划", exact: true })
    .click();
}
test.beforeEach(async ({ page }) => {
  await page.goto("/opendesign.html");
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
  await navigate(page, "我的工作台");
  await expect(
    page.getByRole("heading", { name: "素材，接着创作。" }),
  ).toBeVisible();
});

test("01 phone import, verify, classify, and archive", async ({
  page,
}, info) => {
  await scenario(page, "01 手机采集与归档");
  await page.getByRole("button", { name: "手机导入", exact: true }).click();
  await page.getByRole("button", { name: "选择手机照片", exact: true }).click();
  const upload = page.locator('[data-task-id="DB-025"]');
  await expect(
    upload.getByText("已完成", { exact: true }).first(),
  ).toBeVisible();
  await navigate(page, "产物与收件");
  const item = page
    .getByRole("article")
    .filter({ hasText: "山行随拍 · 手机导入.jpg" });
  await item.getByRole("button", { name: "请助手归类", exact: true }).click();
  await confirm(page);
  const plan = page.locator('[data-task-id="DB-026"]');
  await expect(plan.getByText("部分完成", { exact: true })).toBeVisible();
  await plan.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(plan.getByText("已完成", { exact: true }).first()).toBeVisible();
  await navigate(page, "我的文件");
  await expect(
    page.getByRole("button", { name: /山行随拍 · 手机导入/ }).first(),
  ).toBeVisible();
  await screenshot(page, `${info.project.name}-01-import`);
});

test("02 shared plan confirmation, partial recovery, and compensation", async ({
  page,
}, info) => {
  await scenario(page, "02 跨端确认与恢复");
  if (info.project.name === "desktop") {
    await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
    await page.getByRole("button", { name: "模拟手机", exact: true }).click();
  }
  await confirm(page);
  const card = page.locator('[data-task-id="DB-024"]');
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await screenshot(page, `${info.project.name}-02-partial`);
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await expect(page.locator("[data-task-id]")).toHaveCount(1);
  await card.getByRole("button", { name: "生成恢复计划", exact: true }).click();
  await expect(
    page
      .locator('[data-task-id="DB-025"]')
      .getByText("待确认", { exact: true }),
  ).toBeVisible();
});

test("03 scoped question, fixed-version citation, and save result", async ({
  page,
}, info) => {
  await scenario(page, "03 文档提问与引用");
  await page
    .getByRole("button", { name: "这些资料的创作方向是什么？", exact: true })
    .click();
  await expect(page.getByRole("dialog").last()).toContainText(
    "不会开启全盘分析",
  );
  await page.getByRole("button", { name: "允许本次分析", exact: true }).click();
  await expect(
    page.getByText("DriveBridge 助手 · 示例回答", { exact: true }),
  ).toBeVisible();
  await screenshot(page, `${info.project.name}-03-answer`);
  await page.getByRole("button", { name: /\[1\].*创作简报/ }).click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@3",
  );
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  if (
    !(await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .isVisible())
  )
    await page
      .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
      .click();
  await page
    .getByRole("button", { name: "保存回答到收件箱", exact: true })
    .click();
  await navigate(page, "产物与收件");
  await expect(
    page.getByRole("heading", { name: "资料问答 · 已保存.md", exact: true }),
  ).toBeVisible();
});

test("04 external Agent setup and artifact acceptance", async ({
  page,
}, info) => {
  await scenario(page, "04 外部 Agent 产物");
  await page.getByRole("button", { name: "查看资料助手", exact: true }).click();
  await page.getByRole("button", { name: "设置接入", exact: true }).click();
  await page.getByRole("button", { name: "下一步：接入说明" }).click();
  await page.getByRole("button", { name: "下一步：验证连接" }).click();
  await page.getByRole("button", { name: "完成演示接入", exact: true }).click();
  await page.getByRole("button", { name: "模拟处理资料", exact: true }).click();
  await expect(
    page
      .locator('[data-task-id="DB-025"]')
      .getByText("已完成", { exact: true })
      .first(),
  ).toBeVisible();
  await navigate(page, "产物与收件");
  const artifact = page
    .getByRole("article")
    .filter({ hasText: "山野资料 · Agent 摘要.md" });
  await artifact.getByRole("button", { name: "预览", exact: true }).click();
  await screenshot(page, `${info.project.name}-04-artifact-preview`);
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await artifact
    .getByRole("button", { name: "接收并归档", exact: true })
    .click();
  await expect(artifact).toHaveCount(0);
});

test("05 transfer conflict preserves originals and resumes same operation", async ({
  page,
}, info) => {
  await scenario(page, "05 跨存储复制");
  await page.getByRole("button", { name: "预览复制计划", exact: true }).click();
  await confirm(page);
  const card = page.locator('[data-task-id="DB-025"]');
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await expect(card).toContainText("尚未覆盖");
  await screenshot(page, `${info.project.name}-05-transfer-conflict`);
  await card
    .getByRole("button", { name: "保留两份并继续", exact: true })
    .click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await navigate(page, "我的文件");
  await page
    .getByRole("navigation", { name: "存储切换" })
    .getByRole("button", { name: /S3 · 归档/ })
    .click();
  await page
    .getByRole("textbox", { name: "搜索文件", exact: true })
    .fill("副本");
  await expect(page.locator("[data-file-id]")).toHaveCount(1);
});

test("06 offline reading and explicit version update", async ({
  page,
}, info) => {
  await scenario(page, "06 离线与版本冲突");
  await page.getByRole("button", { name: "模拟断网", exact: true }).click();
  await page
    .getByRole("button", { name: "离线阅读", exact: true })
    .first()
    .click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@2",
  );
  await screenshot(page, `${info.project.name}-06-offline`);
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await page
    .getByRole("button", { name: "重新连接", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: "更新副本", exact: true })
    .first()
    .click();
  await expect(
    page.getByText("云端 v3 · 本设备 v3", { exact: true }),
  ).toBeVisible();
});

test("07 version comments, fixed delivery and revocation", async ({
  page,
}, info) => {
  await scenario(page, "07 版本讨论与交付");
  await page
    .getByRole("button", { name: /山野计划 · 创作简报.pdf/ })
    .first()
    .click();
  await choose(page, "查看版本", "v1 · 历史");
  await previewDetails(page);
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await page
    .getByRole("textbox", { name: "版本评论", exact: true })
    .fill("此处保留最初的创作方向。示例评论。");
  await page.getByRole("button", { name: "发送评论", exact: true }).click();
  await expect(page.getByText("你 · v1", { exact: true })).toBeVisible();
  if (await page.getByRole("dialog").isVisible())
    await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "交付此版本", exact: true }).click();
  await page
    .getByRole("button", { name: "确认交付固定版本", exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "山野计划 · 创作简报.pdf · v1",
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "模拟对方接收", exact: true }).click();
  await screenshot(page, `${info.project.name}-07-delivery`);
  await page.getByRole("button", { name: "撤销分享", exact: true }).click();
  await expect(page.getByRole("dialog").last()).toContainText("无法召回");
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByText("已撤销", { exact: true })).toBeVisible();
});

test("08 scoped rule pauses and resumes original task", async ({
  page,
}, info) => {
  await scenario(page, "08 备份与自动整理");
  await page.getByRole("button", { name: "配置规则", exact: true }).click();
  await choose(page, "规则类型", "新收件 · 自动归档");
  await expect(page.getByRole("dialog").last()).toContainText("删除传播：关闭");
  await page
    .getByRole("button", { name: "确认范围并启用", exact: true })
    .click();
  await page
    .getByRole("button", { name: "模拟条件受限的运行", exact: true })
    .click();
  const card = page.locator('[data-task-id="DB-025"]');
  await expect(card.getByText("已暂停", { exact: true })).toBeVisible();
  await screenshot(page, `${info.project.name}-08-rule-paused`);
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
});

test("stale confirmation and space switch do not submit hidden work", async ({
  page,
}) => {
  await scenario(page, "02 跨端确认与恢复");
  await page.getByRole("button", { name: "检查并确认", exact: true }).click();
  await page
    .getByRole("button", { name: "演示异常：确认前计划发生变化", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "确认执行这份计划", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "关闭对话框", exact: true }).click();
  await navigate(page, "我的");
  await page
    .getByRole("main")
    .getByRole("button", { name: "账户与空间", exact: true })
    .click();
  await choose(page, "切换空间", "团队空间");
  await navigate(page, "任务中心");
  await expect(page.locator("[data-task-id]")).toHaveCount(0);
});

test("unknown result is reconciled before the original operation resumes", async ({
  page,
}) => {
  await choose(page, "演示状态", "结果未知");
  const card = page.locator('[data-task-id="DB-024"]');
  await expect(card.getByText("结果待确认", { exact: true })).toBeVisible();
  await expect(
    card.getByRole("button", { name: "恢复未完成项", exact: true }),
  ).toHaveCount(0);
  await card.getByRole("button", { name: "查询原操作", exact: true }).click();
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await expect(page.locator("[data-task-id]")).toHaveCount(1);
});

test("responsive states, keyboard, focus and reduced motion", async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== "desktop",
    "One matrix pass covers all dimensions.",
  );
  for (const width of [1440, 768, 390, 320]) {
    await page.setViewportSize({ width, height: width < 740 ? 844 : 1000 });
    await expect(page.getByRole("main")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await screenshot(page, `layout-${width}`);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  for (const name of [
    "加载中",
    "空数据",
    "无权限",
    "连接失效",
    "不支持",
    "加载失败",
  ]) {
    await choose(page, "演示状态", name);
    await screenshot(page, `mobile-state-${name}`);
    if (name === "无权限")
      await expect(page.getByRole("main")).not.toContainText("创作简报");
  }
  await choose(page, "演示状态", "正常状态");
  await page.getByRole("button", { name: "搜索", exact: true }).focus();
  await page.keyboard.press("Control+k");
  await expect(page.getByRole("dialog").last()).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog").last()).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "搜索", exact: true }),
  ).toBeFocused();
  const buttons = await page
    .getByRole("navigation", { name: "底部导航" })
    .getByRole("button")
    .all();
  for (const button of buttons)
    expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await choose(page, "演示状态", "加载中");
  expect(
    await page
      .locator(".animate-spin")
      .evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("none");
  await choose(page, "演示状态", "正常状态");
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page.getByRole("button", { name: "切换深色", exact: true }).click();
  await page.keyboard.press("Escape");
  await screenshot(page, "mobile-dark");
});
