import { expect, type Page } from '@playwright/test';

export async function selectFinderView(page: Page, name: '列表视图' | '网格视图' | '分栏视图' | '画廊视图') {
  if ((page.viewportSize()?.width ?? 1280) < 768) {
    await page.getByRole('button', { name: '文件视图', exact: true }).click();
    await page.getByRole('menuitem', { name, exact: true }).click();
    await expect(page.getByRole('menu', { name: '文件视图', exact: true })).toBeHidden();
  } else {
    await page.getByRole('button', { name, exact: true }).click();
  }
}
