import { expect, test } from '@playwright/test';

test.use({ trace: 'off', video: 'off', screenshot: 'off', timezoneId: 'America/New_York' });
test('real Agent read appears as an observation without UI execution or fabricated success', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const spaceResponse = await fetch('/api/v1/spaces', { method: 'POST', headers, body: JSON.stringify({ name: 'Invocation fixture', connection_id: 'platform' }) });
    if (!spaceResponse.ok) throw new Error('fixture_space_failed');
    const space = (await spaceResponse.json()).data;
    const response = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'invocation-fixture' }, body: JSON.stringify({ name: 'Call observer fixture', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['files.read'], expiry_mode: 'never' }) });
    if (!response.ok) throw new Error('fixture_grant_failed');
    // Credential stays in this lexical browser frame; never returned to runner.
    const issued = (await response.json()).data;
    const read = await fetch(`/api/v1/files?space_id=${encodeURIComponent(space.id)}&prefix=references`, { credentials: 'omit', headers: { Authorization: `Bearer ${issued.credential.secret}` } });
    if (!read.ok) throw new Error('fixture_agent_read_failed'); await read.arrayBuffer();
    const list = await fetch(`/api/v1/agent-invocations?agent_id=${encodeURIComponent(issued.grant.agent_id)}`);
    if (!list.ok) throw new Error('fixture_observation_failed');
    const observed = (await list.json()).data;
    if (observed.items.length !== 1 || observed.items[0].result !== 'unknown') throw new Error('fixture_observation_mismatch');
    return { agent: issued.grant.agent_id as string, id: observed.items[0].invocation_id as string, request: observed.items[0].request_id as string, started: observed.items[0].started_at as string };
  });
  const writes: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/') && !['GET', 'HEAD'].includes(request.method())) writes.push(request.method()); });
  await page.goto('/#/agents');
  const trigger = page.getByRole('button', { name: '查看调用记录 Call observer fixture', exact: true });
  await trigger.click();
  const dialog = page.getByRole('dialog', { name: 'Agent 调用记录', exact: true });
  await expect(dialog.getByText(`调用标识：${original.id}`, { exact: true })).toBeVisible();
  await expect(dialog.getByText(`请求标识：${original.request}`, { exact: true })).toBeVisible();
  await expect(dialog.getByRole('heading', { name: 'HTTP · files.list', exact: true })).toBeVisible();
  await expect(dialog.getByText('结果未知 · 未独立证明业务效果', { exact: true })).toBeVisible();
  await expect(dialog.getByText(/非全量调用观察/)).toBeVisible();
  await expect(dialog.getByRole('button', { name: /执行|重试业务/ })).toHaveCount(0);
  const refreshed = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
  await dialog.getByRole('button', { name: '重读调用记录', exact: true }).click(); await refreshed;
  await expect(dialog.getByRole('listitem')).toHaveCount(1);
  await dialog.locator('summary').filter({ hasText: '日期与高级时间筛选' }).click();
  const instant = Date.parse(original.started);
  const start = new Date(instant - 60000).toISOString(), end = new Date(instant + 60000).toISOString();
  const offsetStart = new Date(instant - 60000 + 8 * 3600000).toISOString().replace('Z', '+08:00');
  await dialog.getByRole('textbox', { name: '开始时间晚于' }).fill(offsetStart);
  await dialog.getByRole('textbox', { name: '开始时间早于' }).fill(end);
  async function applyTime() {
    const response = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
    await dialog.getByRole('button', { name: '应用时间筛选', exact: true }).click();
    return await response;
  }
  const bounded = await applyTime();
  expect(new URL(bounded.url()).searchParams.get('started_after')).toBe(start);
  expect(new URL(bounded.url()).searchParams.get('started_before')).toBe(end);
  await expect(dialog.getByRole('listitem')).toHaveCount(1);
  // A strictly earlier upper bound excludes the real observation on the owner.
  await dialog.getByRole('textbox', { name: '开始时间晚于' }).fill('');
  await dialog.getByRole('textbox', { name: '开始时间早于' }).fill(start);
  const excluded = await applyTime(); expect((await excluded.json()).data.items).toEqual([]);
  await expect(dialog.getByText(/当前筛选没有可见观察记录/)).toBeVisible();
  const clearing = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
  await dialog.getByRole('button', { name: '清除时间筛选', exact: true }).click();
  expect(new URL((await clearing).url()).searchParams.has('started_before')).toBe(false);
  await expect(dialog.getByRole('listitem')).toHaveCount(1);
  await dialog.getByRole('textbox', { name: '开始时间晚于' }).fill(offsetStart);
  await dialog.getByRole('textbox', { name: '开始时间早于' }).fill(end); await applyTime();
  const filtering = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
  await dialog.getByRole('combobox', { name: '调用结果筛选' }).click();
  await page.getByRole('option', { name: '请求被拒绝', exact: true }).click();
  const filtered = await filtering; const query = new URL(filtered.url()).searchParams;
  expect(query.get('result')).toBe('denied'); expect(query.get('started_after')).toBe(start); expect(query.get('started_before')).toBe(end);
  expect((await filtered.json()).data.items).toEqual([]);
  await expect(dialog.getByText(/当前筛选没有可见观察记录/)).toBeVisible();
  // Calendar mode replaces (rather than combines with) legacy exclusive bounds.
  await dialog.getByRole('combobox', { name: '调用结果筛选' }).click();
  await page.getByRole('option', { name: '全部结果', exact: true }).click();
  await expect(dialog.getByRole('listitem')).toHaveCount(1);
  const day = original.started.slice(0, 10);
  await dialog.getByLabel('开始日期', { exact: true }).fill(day);
  await dialog.getByLabel('结束日期（含当日）', { exact: true }).fill(day);
  const calendarRead = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
  await dialog.getByRole('button', { name: '应用日期筛选' }).click();
  const calendarQuery = new URL((await calendarRead).url()).searchParams;
  expect(calendarQuery.get('started_at_or_after')).toBe(day + 'T00:00:00.000Z'); expect(calendarQuery.has('started_after')).toBe(false);
  await expect(dialog.getByRole('listitem')).toHaveCount(1);
  await dialog.getByRole('combobox', { name: '日期时区' }).click();
  await page.getByRole('option', { name: '本机时区（America/New_York）', exact: true }).click();
  for (const [date, hours, startUTC, endUTC] of [
    ['2026-03-08', 23, '2026-03-08T05:00:00.000Z', '2026-03-09T04:00:00.000Z'],
    ['2026-11-01', 25, '2026-11-01T04:00:00.000Z', '2026-11-02T05:00:00.000Z'],
  ] as const) {
    await dialog.getByLabel('开始日期', { exact: true }).fill(date);
    await dialog.getByLabel('结束日期（含当日）', { exact: true }).fill(date);
    const response = page.waitForResponse(response => response.url().includes('/agent-invocations?') && response.status() === 200);
    await dialog.getByRole('button', { name: '应用日期筛选' }).click();
    const params = new URL((await response).url()).searchParams;
    expect(params.get('started_at_or_after')).toBe(startUTC); expect(params.get('started_before')).toBe(endUTC);
    expect((Date.parse(endUTC) - Date.parse(startUTC)) / 3600000).toBe(hours);
    await expect(dialog.getByText(/当前筛选没有可见观察记录/)).toBeVisible();
  }
  // Brazil's skipped midnight must not be silently normalized to 01:00.
  const skippedContext = await browser.newContext({ timezoneId: 'America/Sao_Paulo' });
  try {
    const skipped = await skippedContext.newPage(); await skipped.goto('/');
    await skipped.getByRole('link', { name: '使用账号登录' }).click(); await skipped.getByRole('link', { name: '以 Alice 登录' }).click();
    await expect(skipped.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
    await skipped.goto('/#/agents'); await skipped.getByRole('button', { name: '查看调用记录 Call observer fixture', exact: true }).click();
    const view = skipped.getByRole('dialog', { name: 'Agent 调用记录', exact: true }); await expect(view.getByRole('listitem')).toHaveCount(1);
    let requests = 0; skipped.on('request', request => { if (request.url().includes('/agent-invocations?')) requests++; });
    await view.locator('summary').filter({ hasText: '日期与高级时间筛选' }).click();
    await view.getByRole('combobox', { name: '日期时区' }).click(); await skipped.getByRole('option', { name: '本机时区（America/Sao_Paulo）', exact: true }).click();
    await view.getByLabel('开始日期', { exact: true }).fill('2018-11-04'); await view.getByLabel('结束日期（含当日）', { exact: true }).fill('2018-11-04');
    await view.getByRole('button', { name: '应用日期筛选' }).click(); await expect(view.getByText(/日期无效/)).toBeVisible();
    expect(requests).toBe(0); await expect(view.getByRole('listitem')).toHaveCount(1);
  } finally { await skippedContext.close(); }
  const ids = await page.evaluate(async agent => {
    const response = await fetch(`/api/v1/agent-invocations?agent_id=${encodeURIComponent(agent)}`);
    if (!response.ok) throw new Error('readback_failed');
    return (await response.json()).data.items.map((row: { invocation_id: string }) => row.invocation_id);
  }, original.agent);
  expect(ids).toEqual([original.id]); expect(writes).toEqual([]);
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
});
