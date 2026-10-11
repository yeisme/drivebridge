import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('formal invocation order consumes owner sorting for real sequential calls', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const spaceResponse = await fetch('/api/v1/spaces', { method: 'POST', headers, body: JSON.stringify({ name: 'Invocation ordering', connection_id: 'platform' }) });
    if (!spaceResponse.ok) throw new Error('fixture_space_failed'); const space = (await spaceResponse.json()).data;
    const response = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'invocation-order-fixture' }, body: JSON.stringify({ name: 'Order observer', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['files.read'], expiry_mode: 'never' }) });
    if (!response.ok) throw new Error('fixture_grant_failed'); const issued = (await response.json()).data;
    const ids: string[] = []; let first = '', last = '';
    for (let i = 0; i < 3; i++) {
      // Only synthetic credentials in this lexical frame, never returned/logged.
      const read = await fetch('/api/v1/files?space_id=' + encodeURIComponent(space.id) + '&prefix=references', { credentials: 'omit', headers: { Authorization: `Bearer ${issued.credential.secret}` } });
      if (!read.ok) throw new Error('fixture_read_failed'); await read.arrayBuffer();
      const list = await fetch('/api/v1/agent-invocations?agent_id=' + encodeURIComponent(issued.grant.agent_id));
      if (!list.ok) throw new Error('fixture_list_failed'); const rows = (await list.json()).data.items;
      const fresh = rows.filter((row: { invocation_id: string }) => !ids.includes(row.invocation_id));
      if (fresh.length !== 1) throw new Error('fixture_observation_mismatch');
      ids.push(fresh[0].invocation_id); if (!first) first = fresh[0].started_at; last = fresh[0].started_at;
    }
    return { ids, first, last };
  });
  const writes: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/') && request.method() !== 'GET') writes.push(request.method()); });
  await page.goto('/#/agents'); await page.getByRole('button', { name: '查看调用记录 Order observer', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Agent 调用记录', exact: true });
  await expect(dialog.getByRole('listitem')).toHaveCount(3);
  const displayed = () => dialog.getByRole('listitem').locator('p').filter({ hasText: /^调用标识：/ }).allTextContents();
  expect(await displayed()).toEqual([...original.ids].sort().map(id => '调用标识：' + id));
  await dialog.locator('summary').filter({ hasText: '日期与高级时间筛选' }).click();
  await dialog.getByLabel('开始日期', { exact: true }).fill(original.first.slice(0, 10)); await dialog.getByLabel('结束日期（含当日）', { exact: true }).fill(original.last.slice(0, 10));
  const dates = page.waitForResponse(r => r.url().includes('/agent-invocations?') && r.status() === 200);
  await dialog.getByRole('button', { name: '应用日期筛选' }).click(); await dates;
  const sorted = page.waitForResponse(r => r.url().includes('order=started_desc') && r.status() === 200);
  await dialog.getByRole('combobox', { name: '调用记录排序' }).click(); await page.getByRole('option', { name: '开始时间最新优先', exact: true }).click();
  const result = await sorted; const params = new URL(result.url()).searchParams;
  expect(params.has('started_at_or_after')).toBe(true); expect(params.has('started_before')).toBe(true); expect(params.has('cursor')).toBe(false);
  const expected = [...original.ids].reverse(); expect((await result.json()).data.items.map((row: { invocation_id: string }) => row.invocation_id)).toEqual(expected);
  await expect.poll(displayed).toEqual(expected.map(id => '调用标识：' + id));
  const filtered = page.waitForResponse(r => r.url().includes('result=denied') && r.status() === 200);
  await dialog.getByRole('combobox', { name: '调用结果筛选' }).click(); await page.getByRole('option', { name: '请求被拒绝', exact: true }).click();
  expect(new URL((await filtered).url()).searchParams.get('order')).toBe('started_desc');
  await expect(dialog.getByText(/当前筛选没有可见观察记录/)).toBeVisible(); expect(writes).toEqual([]);
});
