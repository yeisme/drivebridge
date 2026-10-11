import { test, expect } from '@playwright/test';
import path from 'node:path';

test('Chinese activity labels and readable folder options across themes and widths', async ({ page }) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== '1', 'Requires disposable team demo');
  await page.route('**/api/v1/audit', route => route.fulfill({ json: { data: [
    { id: 'read', action: 'agents.list', outcome: 'success', at: '2026-09-23T09:24:22Z' },
    { id: 'intent', action: 'agents.create', outcome: 'intent', at: '2026-09-23T09:24:21Z' },
    { id: 'unknown', action: 'future.action', outcome: 'future', at: '2026-09-23T09:24:20Z' },
  ] } }));
  await page.goto('/');
  await page.getByRole('button', { name: '无需账号，进入本地体验' }).click();
  await expect(page.getByRole('button', { name: '打开网盘 体验空间', exact: true })).toBeVisible();
  const tenant = (await (await page.request.get('/api/v1/governance')).json()).data.tenant_id;
  for (const theme of ['light', 'dark'] as const) for (const width of [1213, 959, 390]) {
    await page.setViewportSize({ width, height: 963 });
    await page.emulateMedia({ colorScheme: theme });
    await page.evaluate(hash => { location.hash = hash; }, `#/agents?tenant=${tenant}&view=legacy`);
    await page.getByRole('tab', { name: '旧版机器人授权', exact: true }).click();
    const activities = page.getByLabel('最近活动', { exact: true });
    await expect(activities.locator('[title="agents.list"]')).toHaveText('查看机器人授权');
    await expect(activities).toContainText('操作请求');
    await expect(activities.locator('[title="future.action"]')).toHaveText('其他操作');
    await page.screenshot({ animations: 'disabled', path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `activity-${theme}-${width}.png`) });
    await page.evaluate(hash => { location.hash = hash; }, `#/drives?tenant=${tenant}`);
    await page.getByRole('button', { name: '打开网盘 体验空间', exact: true }).click();
    await page.getByRole('button', { name: '新建', exact: true }).click();
    await page.getByRole('menuitem', { name: '新建文件夹', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '新建文件夹', exact: true });
    await dialog.getByRole('textbox', { name: '名称', exact: true }).fill('项目/参考资料');
    await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
    await dialog.getByRole('checkbox', { name: '创建多级目录（例如：2026/参考资料）' }).check();
    await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeEnabled();
    const checkbox = dialog.getByRole('checkbox', { name: '创建后打开文件夹' });
    const box = await checkbox.boundingBox();
    const label = await dialog.getByText('创建后打开文件夹', { exact: true }).boundingBox();
    expect(Math.abs(box!.y + box!.height / 2 - label!.y - label!.height / 2)).toBeLessThan(2);
    await expect.poll(async () => { const b = await dialog.boundingBox(); return b!.x >= 0 && b!.x + b!.width <= width && b!.width <= 480; }).toBe(true);
    await page.screenshot({ animations: 'disabled', path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `folder-${theme}-${width}.png`) });
    await dialog.getByRole('button', { name: '取消', exact: true }).click();
    await expect(dialog).toHaveCount(0);
  }
});
