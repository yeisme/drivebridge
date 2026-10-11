import { revealNavigation } from "./test-helpers";
async function previewDetails(page: Page) {
  if (
    !(await page.getByRole("button", { name: "讨论", exact: true }).isVisible())
  )
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
}
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

async function nav(page: Page, target: string) {
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

async function choose(page: Page, label: string, value: string) {
  await page.getByRole("combobox", { name: label, exact: true }).last().click();
  await page.getByRole("option", { name: value, exact: true }).click();
}
async function shot(page: Page, name: string) {
  if (process.env.DRIVEBRIDGE_SKIP_SCREENSHOTS === "1") return;
  const dir =
    process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/prototype-diagnostics";
  fs.mkdirSync(dir, { recursive: true });
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
    path: path.join(dir, `${name}.png`),
    animations: "disabled",
  });
}
async function select(page: Page, id: string) {
  await page.locator(`[data-file-id="${id}"]`).getByRole("checkbox").check();
}
test.beforeEach(async ({ page }) => {
  await page.goto("/opendesign.html");
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
  await nav(page, "我的工作台");
  await expect(
    page.getByRole("heading", { name: "素材，接着创作。", exact: true }),
  ).toBeVisible();
});

test("v2 folder navigation, naming preview and file selection", async ({
  page,
}, info) => {
  await page
    .getByRole("main")
    .getByRole("button", { name: /^山野计划.*份文件/ })
    .click();
  await expect(page.locator('[data-file-id="f5"]')).toHaveCount(0);
  await page.getByRole("button", { name: "新建目录", exact: true }).click();
  await page
    .getByRole("textbox", { name: "文件夹名称", exact: true })
    .fill("交付候选");
  await page.getByRole("button", { name: "创建文件夹", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "交付候选", exact: true }),
  ).toBeVisible();
  await select(page, "f1");
  await select(page, "f2");
  await batchAction(page, "移动");
  await choose(page, "整理动作", "批量重命名");
  await page
    .getByRole("textbox", { name: "名称前缀", exact: true })
    .fill("候选");
  await expect(page.getByRole("dialog").last()).toContainText("候选_01.pdf");
  await page.getByRole("button", { name: "生成修改计划", exact: true }).click();
  const task = page.locator('[data-task-id="DB-025"]');
  await task.getByRole("button", { name: "检查并确认", exact: true }).click();
  await page
    .getByRole("button", { name: "确认执行这份计划", exact: true })
    .click();
  await expect(task.getByText("已完成", { exact: true }).first()).toBeVisible();
  await nav(page, "素材库");
  await expect(page.locator('[data-file-id="f1"]')).toContainText(
    "候选_01.pdf",
  );
  await shot(page, `${info.project.name}-v2-files`);
});

test("v2 reference collections preserve source location and groups", async ({
  page,
}, info) => {
  await nav(page, "素材库");
  await select(page, "f1");
  await select(page, "f2");
  await batchAction(page, "加入集合");
  await page
    .getByRole("textbox", { name: "集合名称", exact: true })
    .fill("山行备选");
  await page.getByRole("button", { name: "确认保存集合", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "山行备选", exact: true }),
  ).toBeVisible();
  await choose(page, "分组 秋日山行 · 视觉参考.jpg", "交付");
  await page
    .getByRole("main")
    .getByRole("button", { name: "交付", exact: true })
    .first()
    .click();
  await expect(page.locator("[data-file-id]")).toHaveCount(1);
  await shot(page, `${info.project.name}-v2-collection`);
  await page
    .locator('[data-file-id="f2"]')
    .getByRole("button", { name: "预览 秋日山行 · 视觉参考.jpg", exact: true })
    .click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toContainText("1 / 1");
  await previewDetails(page);
  await expect(page.locator("[data-details-content=info]")).toContainText(
    "山野计划",
  );
});

test("v2 saved filters become a smart collection", async ({ page }) => {
  await nav(page, "素材库");
  await page.getByRole("button", { name: "图片", exact: true }).click();
  await page.getByRole("button", { name: "筛选条件", exact: true }).click();
  await choose(page, "素材来源", "云端");
  await page
    .getByRole("button", { name: "保存为智能集合", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "集合名称", exact: true })
    .fill("云端图片");
  await page.getByRole("button", { name: "确认保存集合", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "云端图片", exact: true }),
  ).toBeVisible();
  await expect(page.locator("[data-file-id]")).toHaveCount(2);
});

test("v2 conversations keep drafts and fixed citations across new contexts", async ({
  page,
}, info) => {
  await nav(page, "文件助手");
  await page
    .getByRole("textbox", { name: "向助手提问" })
    .fill("请梳理这次选定资料");
  await page.getByRole("button", { name: "发送消息", exact: true }).click();
  await page.getByRole("button", { name: "允许本次分析", exact: true }).click();
  await expect(
    page.getByText("DriveBridge 助手 · 示例回答", { exact: true }),
  ).toBeVisible();
  await page.getByRole("textbox", { name: "向助手提问" }).fill("保留这份草稿");
  await page.getByRole("button", { name: "新建会话", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "向助手提问" })).toHaveValue(
    "",
  );
  await page.getByRole("button", { name: "会话历史", exact: true }).click();
  await page
    .getByRole("dialog")
    .last()
    .getByRole("button", { name: "山野计划 · 素材梳理", exact: true })
    .click();
  await expect(page.getByRole("textbox", { name: "向助手提问" })).toHaveValue(
    "保留这份草稿",
  );
  await page.getByRole("button", { name: "添加上下文", exact: true }).click();
  await page
    .getByRole("textbox", { name: "搜索上下文", exact: true })
    .fill("视觉参考.jpg");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", { name: /\[1\].*创作简报.*v3/ }),
  ).toBeVisible();
  await page.getByRole("button", { name: "发送消息", exact: true }).click();
  await page.getByRole("button", { name: "暂不分析", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "向助手提问" })).toHaveValue(
    "保留这份草稿",
  );
  await shot(page, `${info.project.name}-v2-conversation`);
});

test("v2 edited plans invalidate approvals and skip excluded members", async ({
  page,
}) => {
  await nav(page, "任务中心");
  const task = page.locator('[data-task-id="DB-024"]');
  await task.getByRole("button", { name: "调整计划", exact: true }).click();
  await choose(page, "调整目标目录", "我的资料");
  await page
    .getByRole("checkbox", {
      name: "包含 山间来信 · 分镜脚本.docx",
      exact: true,
    })
    .uncheck();
  await page.getByRole("button", { name: "保存新修订", exact: true }).click();
  await expect(task).toContainText("r2");
  await task.getByRole("button", { name: "检查并确认", exact: true }).click();
  await expect(page.getByRole("dialog").last()).toContainText("本次跳过");
  await page
    .getByRole("button", { name: "确认执行这份计划", exact: true })
    .click();
  await expect(task.getByText("已完成", { exact: true }).first()).toBeVisible();
  await task.getByRole("button", { name: "任务记录", exact: true }).click();
  await expect(page.getByRole("dialog").last()).toContainText("历史修订");
  await expect(
    page.getByRole("dialog").last().getByText("跳过", { exact: true }),
  ).toBeVisible();
});

test("v2 compares and restores content snapshots", async ({ page }, info) => {
  await nav(page, "素材库");
  await page
    .locator('[data-file-id="f1"]')
    .getByRole("button", { name: "预览 山野计划 · 创作简报.pdf", exact: true })
    .click();
  await page.getByRole("button", { name: "比较版本", exact: true }).click();
  await choose(page, "左侧版本", "v1 · 历史");
  await expect(page.getByRole("dialog").last()).toContainText("最初的想法");
  await shot(page, `${info.project.name}-v2-compare`);
  await page.getByRole("button", { name: "关闭对话框", exact: true }).click();
  await previewDetails(page);
  await page.getByRole("button", { name: "版本", exact: true }).click();
  await page.getByRole("button", { name: "恢复", exact: true }).last().click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.locator('[data-details-content="history"]')).toContainText(
    "版本 4",
  );
  await page.getByRole("button", { name: "查看", exact: true }).first().click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@4",
  );
  await expect(page.locator("[data-renderer]")).toContainText("最初的想法");
});

test("v2 multi-file delivery freezes members and options", async ({
  page,
}, info) => {
  await nav(page, "素材库");
  await select(page, "f1");
  await select(page, "f2");
  await batchAction(page, "分享");
  await choose(page, "分享有效期", "30 天");
  await page
    .getByRole("checkbox", { name: "允许接收人下载", exact: true })
    .uncheck();
  await page
    .getByRole("button", { name: "确认交付固定版本", exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "素材交付包 · 2 个固定版本",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toContainText("仅预览");
  await expect(page.getByRole("article")).toContainText("30 天");
  await shot(page, `${info.project.name}-v2-delivery`);
});

test("v2 responsive layout, reachable composer, and stable preview", async ({
  page,
}, info) => {
  if (info.project.name === "desktop") {
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: width < 768 ? 844 : 1000 });
      await expect(page.getByRole("main")).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      if (width >= 768 && width < 1200)
        await expect(
          page.getByRole("button", { name: "空间用量", exact: true }),
        ).toHaveCount(0);
      await shot(page, `v2-home-${width}`);
    }
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("[data-compact=true]")).toBeVisible();
  await nav(page, "文件助手");
  await page
    .getByRole("button", { name: "这些资料的创作方向是什么？", exact: true })
    .click();
  await page.getByRole("button", { name: "允许本次分析", exact: true }).click();
  await expect(
    page.getByText("DriveBridge 助手 · 示例回答", { exact: true }),
  ).toBeVisible();
  const input = page.getByRole("textbox", { name: "向助手提问" });
  let rect = (await input.boundingBox())!;
  expect(rect.y + rect.height).toBeLessThanOrEqual(844);
  await expect(
    page.getByRole("button", { name: "发送消息", exact: true }),
  ).toBeInViewport();
  await shot(page, `${info.project.name}-v2-composer`);
  await input.focus();
  await page.setViewportSize({ width: 390, height: 480 });
  await expect
    .poll(async () => {
      const b = (await input.boundingBox())!;
      return b.y + b.height;
    })
    .toBeLessThanOrEqual(480);
  await expect(
    page.getByRole("button", { name: "发送消息", exact: true }),
  ).toBeInViewport();
  await shot(page, `${info.project.name}-v2-keyboard`);
});
