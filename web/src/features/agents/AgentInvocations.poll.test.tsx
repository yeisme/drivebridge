import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { AgentInvocations } from './AgentInvocations';

type Read = (signal: AbortSignal) => Promise<unknown>;
type Accept = (value: any) => void;
type BeforeResume = (signal: AbortSignal) => Promise<void>;

vi.mock('../../app/poll', () => ({
  poll: (read: Read, accept: Accept, _delay: number | (() => number), _onAuth?: (error: Error) => void, beforeResume?: BeforeResume) => {
    const controller = new AbortController();
    void (async () => {
      if (beforeResume) await beforeResume(controller.signal);
      const value = await read(controller.signal);
      if (!controller.signal.aborted) accept(value);
    })();
    return () => controller.abort();
  },
}));

const page = fixtures.InvocationPageEnvelope;
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const scope = JSON.stringify(['instance', 'alice', 'tenant', 'space']);

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('rechecks the trusted scope before polling and updates the read-only observation', async () => {
  const row = page.data.items[0];
  const finished = { ...row, result: 'succeeded' as const, finished_at: '2026-01-01T00:00:02Z' };
  const fetch = vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(json({ data: { ...page.data, items: [row] } }))
    .mockResolvedValueOnce(json({ data: { user: { id: 'alice' }, instance_id: 'instance' } }))
    .mockResolvedValueOnce(json({ data: { ...page.data, items: [finished] } }));
  render(<AgentInvocations agent={row.agent_id} scope={scope} onDenied={vi.fn()} />);
  await waitFor(() => expect(screen.getByText('成功')).toBeTruthy());
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/me');
  expect(String(fetch.mock.calls[2][0])).toContain('/api/v1/agent-invocations?agent_id=' + row.agent_id);
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
});
