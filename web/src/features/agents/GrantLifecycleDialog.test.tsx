import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantLifecycleDialog } from './GrantLifecycleDialog';
import { changeSlot, claimChange, clearChange, readChange } from './grant-change-recovery';
import type { Grant, GrantChangeAction } from './grant-client';
const grant = fixtures.GrantView as Grant;
const slot = changeSlot('alice',grant.grant_id), key = 'grant-' + 'a'.repeat(48);
const receipt = (action: GrantChangeAction, revision = 2) => ({ data: { grant_id: grant.grant_id, action, result_revision: revision, committed_at: '2026-01-01T00:00:00Z' } });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const locks = Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(() => Object.defineProperty(navigator,'locks',{ configurable: true,value: { request: vi.fn(async (_: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator,'locks',locks); else Reflect.deleteProperty(navigator,'locks'); });
const mount = (value = grant, close = vi.fn()) => render(<GrantLifecycleDialog grant={value} scope="alice" close={close}/>);
async function select(label: string, choice: string) {
  fireEvent.click(await screen.findByRole('combobox',{ name: label }));
  const option = await screen.findByRole('option',{ name: choice }); fireEvent.pointerDown(option,{ pointerType: 'mouse' }); fireEvent.click(option);
}
it.each([['disable','禁用授权','active'],['enable','恢复授权','disabled'],['revoke','永久撤销','active']] as const)('submits %s with CAS then proves the original result via lookup', async (action,label,state) => {
  const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({ data: { ...grant,revision: 9 } })).mockResolvedValueOnce(json(receipt(action)));
  const close=vi.fn(); mount({ ...grant,state },close); if(action==='revoke') await select('变更动作',label);
  fireEvent.click(await screen.findByRole('button',{ name: '确认'+label })); await screen.findByText(`原变更已提交：${label} · 结果版本 2`);
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual(action==='revoke'?{ expected_revision: 1 }:{ expected_revision:1,enabled: action==='enable' });
  expect(fetch.mock.calls[0][1]?.method).toBe(action==='revoke'?'POST':'PATCH');
  const saved=readChange(slot)!; expect(saved.action).toBe(action);
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ action,idempotency_key:saved.key });
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-grants/grant-fixture/changes/lookup');
  fireEvent.click(screen.getByRole('button',{ name:'确认回执并刷新' }));await waitFor(()=>expect(close).toHaveBeenCalledOnce());expect(readChange(slot)).toBeNull();
});
it.each([[3600,'1 小时'],[86400,'1 天'],[604800,'7 天'],[2592000,'30 天'],[7776000,'90 天'],[31536000,'365 天'],[0,'长期（可随时撤销）']])('changes expiry %s explicitly without credential rotation',async(seconds,label)=>{
  const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({ data:grant })).mockResolvedValueOnce(json(receipt('expiry')));mount();
  await select('变更动作','修改期限');await select('新的授权期限',String(label));
  fireEvent.click(screen.getByRole('button',{ name:'确认修改期限' }));await screen.findByText('原变更已提交：修改期限 · 结果版本 2');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual(seconds?{ expected_revision:1,expiry_mode:'duration',expires_in_seconds:seconds }:{ expected_revision:1,expiry_mode:'never' });
});
it('restores the original action after remount even if the current grant state differs',async()=>{
  const fetch=vi.spyOn(globalThis,'fetch').mockRejectedValueOnce(new TypeError('lost')).mockResolvedValueOnce(json(receipt('disable')));
  const view=mount();fireEvent.click(await screen.findByRole('button',{ name:'确认禁用授权' }));await screen.findByRole('button',{ name:'查询原变更结果' });
  const original=readChange(slot)!;view.unmount();mount({ ...grant,state:'revoked',revision:8 });
  fireEvent.click(await screen.findByRole('button',{ name:'查询原变更结果' }));await screen.findByText('原变更已提交：禁用授权 · 结果版本 2');
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ action:'disable',idempotency_key:original.key });expect(fetch).toHaveBeenCalledTimes(2);
});
it('keeps unknown after lookup404 and never clears the slot or auto-writes',async()=>{
  localStorage.setItem(slot,'expiry:'+key);const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValue(json({},404));mount();
  fireEvent.click(await screen.findByRole('button',{ name:'查询原变更结果' }));await screen.findByText(/暂未查到原回执/);
  expect(fetch).toHaveBeenCalledTimes(1);expect(readChange(slot)?.key).toBe(key);expect(screen.queryByRole('button',{ name:'保留意图并重读授权' })).toBeNull();
});
it('preserves expiry intent across a proven CAS rejection and readback without automatic resubmit',async()=>{
  const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({ error:{ code:'revision_conflict' } },409)).mockResolvedValueOnce(json({},404)).mockResolvedValueOnce(json({ data:{ ...grant,revision:4 } }));mount();
  await select('变更动作','修改期限');await select('新的授权期限','7 天');fireEvent.click(screen.getByRole('button',{ name:'确认修改期限' }));
  fireEvent.click(await screen.findByRole('button',{ name:'查询原变更结果' }));fireEvent.click(await screen.findByRole('button',{ name:'保留意图并重读授权' }));
  await screen.findByText('本次确认：修改期限 · 7 天，基于版本 4。');expect(fetch).toHaveBeenCalledTimes(3);expect(readChange(slot)).toBeNull();
});
it('rejects a mismatched receipt and clears displayed results on scope change',async()=>{
  localStorage.setItem(slot,'disable:'+key);vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(receipt('enable'))).mockResolvedValueOnce(json(receipt('disable')));mount();
  fireEvent.click(await screen.findByRole('button',{ name:'查询原变更结果' }));await screen.findByText(/尚未取得可确认/);expect(screen.queryByText(/原变更已提交/)).toBeNull();
  fireEvent.click(screen.getByRole('button',{ name:'查询原变更结果' }));await screen.findByText(/原变更已提交/);fireEvent(window,new Event('drivebridge-context'));expect(screen.queryByText(/原变更已提交/)).toBeNull();
});
it('serializes different actions and fails closed on corruption or missing locks',async()=>{
  const first=await claimChange(slot,'disable',new AbortController().signal);const second=await claimChange(slot,'expiry',new AbortController().signal);
  expect(first.claimed).toBe(true);expect(second).toEqual({ pending:first.pending,claimed:false });
  localStorage.setItem(slot,'expiry:'+key);await expect(clearChange(slot,first.pending)).rejects.toThrow('recovery_identity_changed');
  expect(changeSlot('bob',grant.grant_id)).not.toBe(slot);expect(changeSlot('alice','other')).not.toBe(slot);
  localStorage.setItem(slot,'broken');const view=mount();await screen.findByText(/恢复存储或跨标签页锁不可用/);view.unmount();localStorage.clear();
  Object.defineProperty(navigator,'locks',{ configurable:true,value:undefined });const fetch=vi.spyOn(globalThis,'fetch');mount();fireEvent.click(await screen.findByRole('button',{ name:'确认禁用授权' }));await screen.findByText(/恢复存储或跨标签页锁不可用/);expect(fetch).not.toHaveBeenCalled();
});
it('drops a late receipt after revocation and keeps the recovery identity',async()=>{
  localStorage.setItem(slot,'disable:'+key);let finish!: (response:Response)=>void;
  vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));mount();
  fireEvent.click(await screen.findByRole('button',{name:'查询原变更结果'}));await waitFor(()=>expect(finish).toBeDefined());
  fireEvent(window,new Event('drivebridge-context'));finish(json(receipt('disable')));await screen.findByText(/权限或范围已变化/);
  expect(screen.queryByText(/原变更已提交/)).toBeNull();expect(readChange(slot)?.key).toBe(key);
});
it.each(['expired','revoked'] as const)('keeps %s terminal without pending recovery',async state=>{
  mount({ ...grant,state });await screen.findByText(/不能恢复或续期/);expect(screen.queryByRole('combobox')).toBeNull();
});
