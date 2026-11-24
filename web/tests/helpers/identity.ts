import { expect, type Page } from '@playwright/test';

// Use the disposable OIDC provider's public approval flow. A fresh subject
// keeps empty/page-sized assertions independent of other tests' durable work.
export async function loginFreshUser(page: Page, prefix: string) {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  const approval = new URL((await page.getByRole('link', { name: '以 Alice 登录' }).getAttribute('href'))!, page.url());
  approval.searchParams.set('user', prefix + '-' + crypto.randomUUID());
  await page.goto(approval.href);
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
}
