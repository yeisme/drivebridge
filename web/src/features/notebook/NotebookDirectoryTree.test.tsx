import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { NotebookDirectoryTree } from './NotebookDirectoryTree';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('loads the real root directory and selects a nested folder', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(JSON.stringify({ data: { entries: [{ kind: 'directory', path: 'notes' }, { kind: 'directory', path: 'archive' }], next_cursor: '' } }), { status: 200 }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ data: { entries: [{ kind: 'directory', path: 'notes/drafts' }], next_cursor: '' } }), { status: 200 }));
  const select = vi.fn();
  render(<NotebookDirectoryTree space="space" selected="" onSelect={select}/>);
  await screen.findByRole('button', { name: 'notes' });
  fireEvent.click(screen.getByRole('button', { name: 'notes' }));
  expect(select).toHaveBeenCalledWith('notes');
  fireEvent.click(screen.getByRole('button', { name: /展开 notes/ }));
  await screen.findByRole('button', { name: 'drafts' });
  expect(fetch).toHaveBeenCalledWith('/api/v1/browser?space_id=space&path=notes&cursor=', expect.objectContaining({ method: 'GET' }));
});

it('clears the tree when the context is revoked', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ data: { entries: [{ kind: 'directory', path: 'notes' }], next_cursor: '' } }), { status: 200 }));
  render(<NotebookDirectoryTree space="space" selected="" onSelect={vi.fn()}/>);
  await screen.findByRole('button', { name: 'notes' });
  fireEvent(window, new Event('drivebridge-context'));
  await screen.findByText('目录权限已失效。');
  expect(screen.queryByRole('button', { name: 'notes' })).toBeNull();
});
