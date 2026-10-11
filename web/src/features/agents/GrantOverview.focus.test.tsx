import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantOverview } from './GrantOverview';

const page = fixtures.GrantPageEnvelope;
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); });
it.each(['same', 'removed', 'error'])('restores focus after dialog-trigger rows are replaced: %s', async result => {
  let reads = 0;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async url => {
    if (String(url).includes('/agent-groups?')) return json({ data: { items: [], next_cursor: '' } });
    if (String(url).includes('/agent-grants?')) {
      reads++;
      if (reads > 1 && result === 'error') return json({}, 503);
      return json(reads > 1 && result === 'removed' ? { data: { ...page.data, items: [] } } : page);
    }
    throw new Error('unexpected_test_request');
  });
  render(<GrantOverview scope="focus-test-scope"/>);
  const trigger = await screen.findByRole('button', { name: '管理授权 Reader' });
  trigger.focus(); fireEvent.click(trigger);
  fireEvent.click(await screen.findByRole('button', { name: '取消' }));
  await waitFor(() => expect(reads).toBe(2));
  const restored = await screen.findByRole('button', { name: result === 'same' ? '管理授权 Reader' : '刷新授权' });
  await waitFor(() => expect(document.activeElement).toBe(restored));
  expect(trigger.isConnected).toBe(false);
  expect(fetch.mock.calls.every(([, request]) => request?.method === 'GET')).toBe(true);
});
