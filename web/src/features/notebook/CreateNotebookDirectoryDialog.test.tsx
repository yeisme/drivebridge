import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { CreateNotebookDirectoryDialog } from './CreateNotebookDirectoryDialog';
import { createNotebookDirectory, lookupNotebookDirectory } from './notebook-client';

vi.mock('./notebook-client', () => ({ createNotebookDirectory: vi.fn(), lookupNotebookDirectory: vi.fn() }));
vi.mock('../../components/AppDialog', () => ({ AppDialog: ({ title, children }: { title: string; children: React.ReactNode }) => <section role="dialog" aria-label={title}>{children}</section> }));

const originalLocks = Object.getOwnPropertyDescriptor(navigator, 'locks');
afterEach(() => { cleanup(); localStorage.clear(); vi.clearAllMocks(); vi.restoreAllMocks(); if (originalLocks) Object.defineProperty(navigator, 'locks', originalLocks); else Reflect.deleteProperty(navigator, 'locks'); });
function installLocks() { Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!() } }); }

it('creates a real child directory under the selected physical path', async () => {
  installLocks(); vi.mocked(createNotebookDirectory).mockResolvedValue({ found: true, status: 'completed', directory: { id: 'dir', space_id: 'space', path: 'notes/drafts' } });
  const created = vi.fn();
  render(<CreateNotebookDirectoryDialog actor="alice" space="space" parentPath="notes" close={vi.fn()} onCreated={created} onDenied={vi.fn()}/>);
  await screen.findByRole('textbox', { name: '目录名称' });
  fireEvent.change(screen.getByRole('textbox', { name: '目录名称' }), { target: { value: 'drafts' } });
  fireEvent.click(screen.getByRole('button', { name: '确认创建' }));
  await waitFor(() => expect(createNotebookDirectory).toHaveBeenCalledWith('space', 'notes/drafts', expect.stringMatching(/^directory-[a-f0-9]{32}$/), expect.any(AbortSignal)));
  expect(created).toHaveBeenCalledWith('notes/drafts');
});

it('looks up the original key after an unknown directory response', async () => {
  installLocks(); const gate = Promise.withResolvers<Awaited<ReturnType<typeof createNotebookDirectory>>>(); vi.mocked(createNotebookDirectory).mockReturnValueOnce(gate.promise); vi.mocked(lookupNotebookDirectory).mockResolvedValue({ found: true, status: 'completed', directory: { id: 'dir', space_id: 'space', path: 'drafts' } });
  const created = vi.fn();
  render(<CreateNotebookDirectoryDialog actor="alice" space="space" parentPath="" close={vi.fn()} onCreated={created} onDenied={vi.fn()}/>);
  await screen.findByRole('textbox', { name: '目录名称' });
  fireEvent.change(screen.getByRole('textbox', { name: '目录名称' }), { target: { value: 'drafts' } });
  fireEvent.click(screen.getByRole('button', { name: '确认创建' }));
  await waitFor(() => expect(createNotebookDirectory).toHaveBeenCalledTimes(1));
  gate.reject(new Error('network_lost'));
  await screen.findByText(/结果待回查/);
  fireEvent.click(screen.getByRole('button', { name: '查询原创建' }));
  await waitFor(() => expect(lookupNotebookDirectory).toHaveBeenCalledTimes(1));
  expect(created).toHaveBeenCalledWith('drafts');
});

it('explains missing Web Locks without sending a directory request', async () => {
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
  render(<CreateNotebookDirectoryDialog actor="alice" space="space" parentPath="" close={vi.fn()} onCreated={vi.fn()} onDenied={vi.fn()}/>);
  await screen.findByText(/Web Locks/);
  expect(createNotebookDirectory).not.toHaveBeenCalled();
});
