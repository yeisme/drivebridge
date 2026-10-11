import { expect } from "@playwright/test";
import type { Page, Locator } from "@playwright/test";
export async function record7(page: Page, row: Locator) {
  const trigger = row.getByRole("button", { name: /^查看.*详情$/ }).last();
  if (await trigger.isVisible()) await trigger.click();
}
export async function closeRecords7(page: Page) {
  const closer = page
    .locator('[data-dialog-kind="detail"] button[aria-label="关闭对话框"]')
    .last();
  for (let i = 0; i < 4 && (await closer.isVisible()); i++)
    await closer.click();
}
export async function nav7(page: Page, target: string) {
  await closeRecords7(page);
  const exit = page.getByRole("button", { name: "退出预览", exact: true });
  if (await exit.isVisible()) await exit.click();
  const cancel = page.getByRole("button", { name: "取消多选", exact: true });
  if (await cancel.isVisible()) await cancel.click();
  const names: Record<string, string> = {
    标签: "标签与属性",
    我的文件: "我的云盘",
    素材库: "我的云盘",
    "Agent 资源": "资源库",
    存储与传输: "存储管理",
    "外部 Agent": "外部调用",
    我的: "设置",
  };
  target = names[target] || target;
  const dockedNav = page.getByRole("navigation", {
    name: "工作区导航",
    exact: true,
  });
  const floatingNav = page.getByRole("navigation", {
    name: "展开的导航",
    exact: true,
  });
  if (!(await dockedNav.isVisible()) && !(await floatingNav.isVisible())) {
    await page
      .getByRole("button", { name: "显示主导航", exact: true })
      .press("Enter");
    await expect(dockedNav.or(floatingNav)).toBeVisible();
  }
  if (target === "设置") {
    await page
      .getByRole("button", { name: "个人设置", exact: true })
      .last()
      .click();
    return;
  }
  const navigation = (await page
    .getByRole("navigation", { name: "工作区导航", exact: true })
    .isVisible())
    ? page.getByRole("navigation", { name: "工作区导航", exact: true })
    : page.getByRole("navigation", { name: "展开的导航", exact: true });
  await navigation.getByRole("button", { name: target, exact: true }).click();
}
export async function folder7(page: Page, name = "山野计划") {
  const entry = page
    .locator("[data-directory-id]")
    .filter({ hasText: name })
    .first();
  await entry.click();
}
export async function file7(page: Page, id = "f1") {
  if (!(await page.locator(`[data-file-id="${id}"]`).isVisible()))
    await folder7(page);
  await page
    .locator(`[data-file-id="${id}"]`)
    .getByRole("button", { name: /^预览 / })
    .click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toBeVisible();
}
export async function choose7(page: Page, label: string, option: string) {
  if (
    label === "文档筛选" &&
    !(await page
      .getByRole("combobox", { name: label, exact: true })
      .isVisible())
  )
    await page.getByRole("button", { name: "文档视图", exact: true }).click();
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
export async function details7(page: Page, tab = "详情") {
  if (
    !(await page.getByRole("button", { name: tab, exact: true }).isVisible())
  ) {
    if (
      await page
        .getByRole("button", { name: "显示文件详情", exact: true })
        .isVisible()
    )
      await page
        .getByRole("button", { name: "显示文件详情", exact: true })
        .click();
    else await operation7(page, "显示文件详情");
  }
  await page.getByRole("button", { name: tab, exact: true }).click();
}
export async function operation7(page: Page, name: string) {
  await page.getByRole("button", { name: "文件操作", exact: true }).click();
  await page
    .getByRole("dialog")
    .last()
    .getByRole("button", { name, exact: true })
    .click();
}
