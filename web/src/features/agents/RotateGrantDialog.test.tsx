import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { RotateGrantDialog, rotationSlot } from './RotateGrantDialog';
import type { Grant } from './grant-client';
const grant = fixtures.GrantView as Grant;
const slot = rotationSlot('alice', grant.grant_id);
const key = 'grant-' + 'a'.repeat(48);
const receipt = { data: { grant_id: grant.grant_id, credential_generation: 2, credential: { state: 'not_replayable' } } };
const issued = { data: { grant: { ...grant, revision: 2, credential_generation: 2 }, credential: { state: 'issued', secret: 'fixture-rotation-secret' } } };
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => { Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_: string, options: LockOptions | (() => unknown), cb?: () => unknown) => typeof options === 'function' ? options() : cb!()) } }); });
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (value = grant, close = vi.fn()) => render(<RotateGrantDialog grant={value} scope="alice" close={close}/>);
it('requires confirmation and rotates with fixed CAS and original key, never persists secret', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(issued)); const close = vi.fn(); mount(grant,close);
  const button = await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }); expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(button); await screen.findByText('原请求已确认：凭据代次 2');
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/agent-grants/grant-fixture/rotate');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual({ expected_revision: 1 });
  expect(fetch.mock.calls[0][1]?.headers).toMatchObject({ 'Idempotency-Key': localStorage.getItem(slot) });
  expect(localStorage.length).toBe(1); expect(localStorage.getItem(slot)).toMatch(/^grant-[a-f0-9]{48}$/);
  fireEvent.click(screen.getByRole('button', { name: '已安全保存新凭据' }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(localStorage.getItem(slot)).toBeNull();
});
it('recovers the original generation after reload even if the displayed grant is newer', async () => {
  localStorage.setItem(slot,key); const fetch = vi.spyOn(globalThis,'fetch').mockResolvedValue(json(receipt));
  mount({ ...grant, credential_generation: 5, revision: 8, state: 'revoked' });
  expect(screen.queryByRole('button', { name: '确认轮转并使旧凭据失效' })).toBeNull();
  fireEvent.click(await screen.findByRole('button', { name: '查询原轮转结果' }));
  await screen.findByText('原请求已确认：凭据代次 2');
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/agent-grants/grant-fixture/rotate/lookup');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual({ idempotency_key: key });
  expect(screen.queryByLabelText('轮转后的 Agent 凭据')).toBeNull();
});
it('does not infer original generation from a rotate replay latest projection', async () => {
  const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({ data: { grant: { ...grant, credential_generation: 5 }, credential: { state: 'not_replayable' } } })).mockResolvedValueOnce(json(receipt));
  mount(); fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }));
  await screen.findByText('原请求已确认：凭据代次 2'); expect(fetch).toHaveBeenCalledTimes(2);
});
it('keeps unknown after lost response and lookup 404 without allowing another rotation', async () => {
  const fetch=vi.spyOn(globalThis,'fetch').mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({},404));
  mount(); fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }));
  fireEvent.click(await screen.findByRole('button', { name: '查询原轮转结果' }));
  await screen.findByText(/暂未查到原轮转回执/); expect(fetch).toHaveBeenCalledTimes(2);
  expect(screen.queryByRole('button', { name: '结束已拒绝尝试并刷新' })).toBeNull(); expect(localStorage.getItem(slot)).toBeTruthy();
});
it('allows ending only an explicitly rejected CAS after original lookup returns no receipt', async () => {
  const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({ error: { code: 'revision_conflict' } },409)).mockResolvedValueOnce(json({},404)); const close=vi.fn();
  mount(grant,close); fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }));
  fireEvent.click(await screen.findByRole('button', { name: '查询原轮转结果' }));
  fireEvent.click(await screen.findByRole('button', { name: '结束已拒绝尝试并刷新' }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(fetch).toHaveBeenCalledTimes(2); expect(localStorage.getItem(slot)).toBeNull();
});
it.each(['expired','revoked'] as const)('does not rotate %s grants without pending recovery', async state => {
  const fetch=vi.spyOn(globalThis,'fetch'); mount({ ...grant,state }); await screen.findByText(/不能轮转/); expect(fetch).not.toHaveBeenCalled();
});
it('clears secrets on scope change and isolates slots by subject and grant', async () => {
  vi.spyOn(globalThis,'fetch').mockResolvedValue(json(issued)); mount(); fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }));
  await screen.findByLabelText('轮转后的 Agent 凭据'); fireEvent(window,new Event('drivebridge-context'));
  expect(screen.queryByLabelText('轮转后的 Agent 凭据')).toBeNull();
  expect(rotationSlot('bob',grant.grant_id)).not.toBe(slot); expect(rotationSlot('alice','other')).not.toBe(slot);
});
it('discards a late issued credential after scope revocation', async () => {
  let finish!: (value: Response) => void;
  vi.spyOn(globalThis,'fetch').mockImplementationOnce(() => new Promise(resolve => { finish=resolve; })); mount();
  fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' }));
  await waitFor(() => expect(finish).toBeDefined()); fireEvent(window,new Event('drivebridge-context')); finish(json(issued));
  await screen.findByText(/敏感结果已清除/); expect(screen.queryByLabelText('轮转后的 Agent 凭据')).toBeNull(); expect(localStorage.getItem(slot)).toBeTruthy();
});
it('rejects a receipt for a different grant and keeps the original recovery identity', async () => {
  localStorage.setItem(slot,key); vi.spyOn(globalThis,'fetch').mockResolvedValue(json({ data: { ...receipt.data, grant_id: 'other-grant' } })); mount();
  fireEvent.click(await screen.findByRole('button', { name: '查询原轮转结果' })); await screen.findByText(/结果尚未确认/);
  expect(screen.queryByText('原请求已确认：凭据代次 2')).toBeNull(); expect(localStorage.getItem(slot)).toBe(key);
});
it('refuses to send without a recovery lock', async () => {
  Object.defineProperty(navigator,'locks',{ configurable: true,value: undefined }); const fetch=vi.spyOn(globalThis,'fetch'); mount();
  fireEvent.click(await screen.findByRole('button', { name: '确认轮转并使旧凭据失效' })); await screen.findByText(/恢复存储或跨标签页锁不可用/); expect(fetch).not.toHaveBeenCalled();
});
