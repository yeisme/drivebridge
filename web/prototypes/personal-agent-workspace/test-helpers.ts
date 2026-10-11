import type { Page } from "@playwright/test";
// v5.1 uses zero-width collapsed navigation; restore it before desktop/tablet route selection.
export async function revealNavigation(page: Page, target: string) {
  if (
    target === "搜索" ||
    target === "文件助手" ||
    (await page.getByRole("navigation", { name: "底部导航" }).isVisible())
  )
    return;
  if (
    !(await page.getByRole("navigation", { name: "工作区导航" }).isVisible()) &&
    !(await page.getByRole("navigation", { name: "展开的导航" }).isVisible())
  ) {
    const show = page.getByRole("button", { name: "显示主导航", exact: true });
    if (await show.isVisible()) await show.click();
  }
}
