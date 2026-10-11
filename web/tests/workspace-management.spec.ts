import { expect, test } from '@playwright/test';
import path from 'node:path';

test('management routes read actual capabilities and rules without triggering execution', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  // Seed a disabled rule using the real disposable fixture. Never run it.
  const seeded = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const spaces = (await (await fetch('/api/v1/spaces')).json()).data;
    const space = spaces.find((item: { team_id?: string }) => !item.team_id);
    if (!space) throw new Error('fixture_space_missing');
    const response = await fetch('/api/v1/spaces/' + encodeURIComponent(space.id) + '/automation-rules', {
      method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token },
      body: JSON.stringify({ action: 'add_label', action_detail: 'topic:workspace-review', prefix: 'notes', enabled: false, budget_items: 10, expected_revision: 0 }),
    });
    if (!response.ok) throw new Error('fixture_rule_seed_failed');
    const rule = (await response.json()).data[0];
    return { space: space.id as string, id: rule.rule_id as string, revision: rule.revision as number };
  });
  const writes: string[] = [];
  const checks: string[] = [];
  page.on('request', request => {
    const url = new URL(request.url());
    if (!['GET', 'HEAD'].includes(request.method()) && url.pathname.startsWith('/api/')) {
      if (url.pathname === '/api/v1/capabilities/check') checks.push(request.postData() || '');
      else writes.push(request.method() + ' ' + url.pathname);
    }
  });
  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: width < 768 ? 844 : 1000 });
    await page.goto('/#/devices');
    const devices = page.getByRole('region', { name: '设备与离线', exact: true });
    await expect(devices).toBeVisible();
    await expect(devices.getByText(/不证明服务、存储或其他设备在线/)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `management-devices-${width}.png`), animations: 'disabled' });
    await devices.getByRole('button', { name: '查看服务能力' }).click();
    await expect(page).toHaveURL(/#\/capabilities/);
    const capabilities = page.getByRole('region', { name: '能力与连接说明', exact: true });
    await expect(capabilities.getByRole('button', { name: '检查能力 file_list' })).toBeVisible();
    const beforeChecks = checks.length;
    await expect(capabilities.getByRole('region', { name: '动作可用性观察' })).toHaveCount(0);
    const response = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/capabilities/check');
    await capabilities.getByRole('button', { name: '检查能力 file_list' }).click();
    expect((await response).status()).toBe(200);
    await expect(capabilities.getByRole('region', { name: '动作可用性观察' })).toContainText('当前条件允许');
    expect(checks.length).toBe(beforeChecks + 1);
    expect(JSON.parse(checks.at(-1)!)).toEqual({ action: 'file_list', space_id: seeded.space });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `management-capabilities-${width}.png`), animations: 'disabled' });
    await page.goto('/#/automation');
    const automation = page.getByRole('region', { name: '备份与整理', exact: true });
    await expect(automation.getByText('添加标签 · topic:workspace-review')).toBeVisible();
    await expect(automation.getByText('未启用', { exact: true })).toBeVisible();
    await expect(automation.getByText(/不代表有后台执行器/)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `management-automation-${width}.png`), animations: 'disabled' });
    await automation.getByRole('button', { name: '重读整理规则' }).click();
    await expect(automation.getByText('添加标签 · topic:workspace-review')).toBeVisible();
    await page.reload();
    await expect(automation.getByText('添加标签 · topic:workspace-review')).toBeVisible();
  }
  const after = await page.evaluate(async ({ space, id }) => {
    const response = await fetch('/api/v1/spaces/' + encodeURIComponent(space) + '/automation-rules');
    if (!response.ok) throw new Error('fixture_rule_readback_failed');
    const rule = (await response.json()).data.find((item: { rule_id: string }) => item.rule_id === id);
    return { enabled: rule.enabled, revision: rule.revision, checkpoint: rule.checkpoint };
  }, seeded);
  expect(after).toEqual({ enabled: false, revision: seeded.revision, checkpoint: 0 });
  if (process.env.DRIVEBRIDGE_TEST_AUTOMATION_MANAGEMENT !== '1') {
    const automation = page.getByRole('region', { name: '备份与整理', exact: true });
    await automation.getByRole('button', { name: '配置与运行', exact: true }).click();
    await expect(automation.getByText(/服务端尚未开放人工管理/)).toBeVisible();
    await expect(automation.getByRole('button', { name: '新建规则/恢复', exact: true })).toHaveCount(0);
  }
  expect(writes).toEqual([]);
  expect(errors).toEqual([]);
});
