import { expect, test } from '@playwright/test';
import path from 'node:path';

const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || 'temp/browser-artifacts';

test('personal primary pages do not stack legacy surfaces, and compatibility remains keyboard reachable', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    for (const [route, primary, legacy, region, legacyTarget] of [
      ['home', '我的工作台', '容量与活动观察', '个人工作台摘要', '容量'],
      ['agents', 'Agent 管理', '旧版机器人授权', '新协议 Agent 授权', '还没有获准的机器人'],
      ['tasks', '任务与计划', '传输记录与恢复', '持久操作任务', '还没有传输记录'],
      ['inbox', '待验收与归档', '历史传输收件', 'Agent 提交收件', '最近没有产物'],
    ]) {
      await page.goto('/#/' + route);
      await expect(page.getByRole('region', { name: region, exact: true })).toBeVisible();
      await expect(page.getByRole('tab', { name: primary, exact: true })).toHaveAttribute('aria-selected', 'true');
      await expect(page.getByRole('heading', { name: legacyTarget, exact: true })).toHaveCount(0);
      await page.waitForLoadState('networkidle');
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${route} ${width}`).toBe(true);
      await page.screenshot({ path: path.join(artifacts, `unified-${route}-${width}.png`), animations: 'disabled' });
      await page.getByRole('tab', { name: primary, exact: true }).focus();
      await page.keyboard.press('ArrowRight');
      await page.keyboard.press('Enter');
      await expect(page.getByRole('tab', { name: new RegExp('^' + legacy) })).toHaveAttribute('aria-selected', 'true');
      await expect(page.getByRole('region', { name: region, exact: true })).toHaveCount(0);
      // The old page remains operable, not merely a disabled explanation.
      if (route === 'agents') await expect(page.getByRole('button', { name: '授权机器人', exact: true })).toBeVisible();
      else if (route === 'tasks' || route === 'inbox') await expect(page.getByRole('searchbox', { name: '搜索文件、来源或任务' })).toBeVisible();
      else await expect(page.getByRole('heading', { name: '容量', exact: true })).toBeVisible();
      await page.getByRole('tab', { name: primary, exact: true }).click();
      await expect(page.getByRole('region', { name: region, exact: true })).toBeVisible();
    }
  }
  await page.goto('/#/overview');
  await expect(page.getByRole('tab', { name: '容量与活动观察' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('heading', { name: '容量', exact: true })).toBeVisible();
  await page.goto('/#/transfers');
  await expect(page.getByRole('tab', { name: /^传输记录与恢复/ })).toHaveAttribute('aria-selected', 'true');
  await page.getByRole('button', { name: '待处理产物', exact: true }).click();
  await expect(page).toHaveURL(/inbox\?view=legacy/);
  await expect(page.getByRole('tab', { name: '历史传输收件' })).toHaveAttribute('aria-selected', 'true');
  await page.reload();
  await expect(page.getByRole('tab', { name: '历史传输收件' })).toHaveAttribute('aria-selected', 'true');
  await page.getByRole('button', { name: '查看传输记录', exact: true }).click();
  await expect(page.getByRole('tab', { name: /^传输记录与恢复/ })).toHaveAttribute('aria-selected', 'true');
  expect(errors).toEqual([]);
});
