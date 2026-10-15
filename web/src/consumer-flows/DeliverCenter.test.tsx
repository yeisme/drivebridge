/// <reference lib="es2024.promise" />
import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { DeliverCenter } from './DeliverCenter';
import { getPrivateNote, setPrivateNote } from './client';
import { listFileThreads } from '../info-workspace/client';
vi.mock('./client', async original => ({ ...await original<typeof import('./client')>(), getPrivateNote: vi.fn(), setPrivateNote: vi.fn() }));
vi.mock('../info-workspace/client', () => ({ listFileThreads: vi.fn() }));
vi.mock('./PrepareDelivery', () => ({ PrepareDelivery: () => <div>Real source selection</div> }));
afterEach(() => { cleanup(); localStorage.clear(); vi.resetAllMocks(); });
it('reports private-note read errors explicitly and keeps drafts in memory only', async () => {
  vi.mocked(listFileThreads).mockResolvedValue([]); vi.mocked(getPrivateNote).mockResolvedValue({ body: 'server note', revision: 4 }).mockRejectedValueOnce(new Error('unavailable'));
  render(<DeliverCenter actor="alice" spaces={[]} capability="available"/>); fireEvent.change(screen.getByLabelText('文件 ID'), { target: { value: 'file' } });
  await screen.findByText(/私人备注读取失败/); expect(screen.getByRole('button', { name: '保存私人备注' }).hasAttribute('disabled')).toBe(true);
  fireEvent.change(screen.getByLabelText('私人备注'), { target: { value: 'private in-memory draft' } });
  expect(Object.values(localStorage).join()).not.toContain('private in-memory draft');
  fireEvent.click(screen.getByRole('button', { name: '重读讨论与私人备注' })); await waitFor(() => expect(screen.getByRole('button', { name: '保存私人备注' }).hasAttribute('disabled')).toBe(false));
  expect(screen.getByLabelText('私人备注')).toHaveProperty('value', 'private in-memory draft');
  fireEvent.click(screen.getByRole('button', { name: '保存私人备注' })); await waitFor(() => expect(setPrivateNote).toHaveBeenCalledWith('file', { body: 'private in-memory draft', expected_revision: 4 }, expect.any(AbortSignal)));
});
it('aborts old file observations and clears private text on permission loss', async () => {
  const gate = Promise.withResolvers<{ body: string; revision: number }>(); vi.mocked(listFileThreads).mockResolvedValue([]); vi.mocked(getPrivateNote).mockReturnValueOnce(gate.promise).mockRejectedValueOnce(Object.assign(new Error('forbidden'), { status: 403 }));
  render(<DeliverCenter actor="alice" spaces={[]} capability="available"/>); fireEvent.change(screen.getByLabelText('文件 ID'), { target: { value: 'old' } }); fireEvent.change(screen.getByLabelText('文件 ID'), { target: { value: 'new' } });
  await screen.findByText(/权限或范围已变化，讨论与私人备注已清除/); gate.resolve({ body: 'old private note', revision: 1 }); await waitFor(() => expect(screen.getByLabelText('私人备注')).toHaveProperty('value', ''));
});
it('preserves a newer note draft when an older save acknowledges and rereads', async () => {
  vi.mocked(listFileThreads).mockResolvedValue([]);
  vi.mocked(getPrivateNote).mockResolvedValueOnce({ body: 'initial', revision: 1 }).mockResolvedValue({ body: 'draft A', revision: 2 });
  const gate = Promise.withResolvers<void>(); vi.mocked(setPrivateNote).mockReturnValueOnce(gate.promise);
  render(<DeliverCenter actor="alice" spaces={[]} capability="available"/>); fireEvent.change(screen.getByLabelText('文件 ID'), { target: { value: 'file' } });
  await waitFor(() => expect(screen.getByLabelText('私人备注')).toHaveProperty('value', 'initial'));
  fireEvent.change(screen.getByLabelText('私人备注'), { target: { value: 'draft A' } }); fireEvent.click(screen.getByRole('button', { name: '保存私人备注' }));
  fireEvent.change(screen.getByLabelText('私人备注'), { target: { value: 'draft B' } }); gate.resolve();
  await waitFor(() => expect(getPrivateNote).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.getByRole('button', { name: '保存私人备注' })).toHaveProperty('disabled', false));
  expect(screen.getByLabelText('私人备注')).toHaveProperty('value', 'draft B');
  fireEvent.click(screen.getByRole('button', { name: '保存私人备注' }));
  await waitFor(() => expect(setPrivateNote).toHaveBeenLastCalledWith('file', { body: 'draft B', expected_revision: 2 }, expect.any(AbortSignal)));
});
it('clears a note draft when absent-note file authorization is revoked', async () => {
  vi.mocked(listFileThreads).mockResolvedValue([]); vi.mocked(getPrivateNote).mockRejectedValue(Object.assign(new Error('not_found'), { status: 404 }));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ error: { code: 'forbidden' } }), { status: 403 }));
  render(<DeliverCenter actor="alice" spaces={[]} capability="available"/>); fireEvent.change(screen.getByLabelText('文件 ID'), { target: { value: 'file' } }); fireEvent.change(screen.getByLabelText('私人备注'), { target: { value: 'private draft' } });
  await screen.findByText(/权限或范围已变化，讨论与私人备注已清除/);
  expect(screen.getByLabelText('私人备注')).toHaveProperty('value', ''); fetch.mockRestore();
});
