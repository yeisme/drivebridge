import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { AgentInvocations } from './AgentInvocations';

const page = fixtures.InvocationPageEnvelope;
const item = page.data.items[0];
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it.each(['keyboard', 'close button'])('shows original observation facts and restores focus after %s dismissal', async method => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(page)));
  const user = userEvent.setup();
  render(<AgentInvocations agent={item.agent_id} scope="detail-fixture" onDenied={vi.fn()} />);
  const trigger = await screen.findByRole('button', { name: `查看调用详情 ${item.invocation_id}` });
  trigger.focus(); await user.keyboard('{Enter}');
  const dialog = await screen.findByRole('dialog', { name: '调用记录详情' });
  for (const value of [item.invocation_id, item.agent_id, item.grant_id, item.request_id, item.action, page.data.observed_at]) {
    expect(within(dialog).getAllByText(value!).length).toBeGreaterThan(0);
  }
  expect(within(dialog).getByText(/未提供原任务 kind/)).toBeTruthy();
  expect(within(dialog).getByText(/不代表当前授权仍有效/)).toBeTruthy();
  expect(within(dialog).getByText(/未提供经证明的原 operation 引用/)).toBeTruthy();
  expect(within(dialog).getByText(/未提供经证明的资源引用/)).toBeTruthy();
  expect(within(dialog).queryByRole('link')).toBeNull();
  expect(within(dialog).queryByRole('button', { name: /执行|重试/ })).toBeNull();
  if (method === 'keyboard') await user.keyboard('{Escape}');
  else await user.click(within(dialog).getByRole('button', { name: '关闭' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(trigger));
  expect(fetch).toHaveBeenCalledTimes(1);
});

it.each(['drivebridge-context', 'offline'])('clears an open invocation detail on %s without executing or rereading', async event => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(page)));
  render(<AgentInvocations agent={item.agent_id} scope="detail-fixture" onDenied={vi.fn()} />);
  fireEvent.click(await screen.findByRole('button', { name: `查看调用详情 ${item.invocation_id}` }));
  await screen.findByRole('dialog');
  fireEvent(window, new Event(event));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.queryByText(item.request_id)).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(1);
});
