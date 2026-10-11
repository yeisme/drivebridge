import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { ContextAssistant } from './ContextAssistant';
import { FileDetailSheet } from './FileDetailSheet';

const file = { id: 'file', space_id: 'space', path: '/private/note.md', version_id: 'new', size_bytes: 4, mime_type: 'text/markdown', updated_at: '2026-01-01T00:00:00Z', trashed: false };
const ref = 'drivebridge://fixture/file/file/version/old';
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
function requests() {
  return vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
    const url = String(input);
    if (url.includes('/references?')) return json({ ref, file: { ...file, version_id: 'old' }, sha256: 'a'.repeat(64) });
    if (url.endsWith('/versions')) return json([]);
    return json({ file, ref: 'drivebridge://fixture/file/file/version/new' });
  });
}
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('opens only on intent, explains each runtime gap, and copies a reauthorized old reference without model calls', async () => {
  const user = userEvent.setup();
  const clipboard = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();
  const fetch = requests();
  render(<ContextAssistant fileId="file" versionId="old" />);
  expect(fetch).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: '上下文助手' }));
  await screen.findByText(ref);
  for (const reason of ['会话', '分析同意', '预算', '计划']) expect(screen.getByText(new RegExp(`未提供.*${reason}`))).toBeTruthy();
  await user.click(screen.getByRole('button', { name: '复制外部 Agent 固定引用' }));
  await screen.findByText('已复制固定版本引用；未发送给任何 Agent。');
  expect(clipboard).toHaveBeenCalledWith(ref);
  const references = fetch.mock.calls.filter(([url]) => String(url).includes('/references?'));
  expect(references).toHaveLength(2);
  expect(references.every(([url]) => String(url).includes(encodeURIComponent(ref)))).toBe(true);
  expect(fetch.mock.calls.every(([, request]) => request?.method === 'GET')).toBe(true);
});

it.each(['Escape', 'button'])('closes the assistant via %s and restores entry focus', async close => {
  requests(); const user = userEvent.setup();
  render(<ContextAssistant fileId="file" versionId="old" />);
  const entry = screen.getByRole('button', { name: '上下文助手' });
  await user.click(entry); await screen.findByText(ref);
  const button = screen.getByRole('button', { name: '收起上下文助手' });
  button.focus();
  if (close === 'Escape') await user.keyboard('{Escape}'); else await user.click(button);
  expect(screen.queryByText(ref)).toBeNull();
  expect(document.activeElement).toBe(entry);
});

it.each(['drivebridge-context', 'offline'])('clears context and rejects a late resolve after %s', async event => {
  let finish!: (response: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
    if (String(input).includes('/references?')) return new Promise(resolve => { finish = resolve; });
    return json({ file, ref: 'drivebridge://fixture/file/file/version/new' });
  });
  render(<ContextAssistant fileId="file" versionId="old" />);
  fireEvent.click(screen.getByRole('button', { name: '上下文助手' }));
  await waitFor(() => expect(finish).toBeTypeOf('function'));
  fireEvent(window, new Event(event));
  finish(json({ ref, file: { ...file, version_id: 'old' }, sha256: 'a'.repeat(64) }));
  await waitFor(() => expect(screen.queryByText(ref)).toBeNull());
  expect(screen.queryByRole('button', { name: '复制外部 Agent 固定引用' })).toBeNull();
  fireEvent(window, new Event('online'));
  expect(fetch).toHaveBeenCalledTimes(3);
});

it.each(['denied', 'wrong-version', 'wrong-file'])('never substitutes or copies an unreadable/mismatched context: %s', async reason => {
  const user = userEvent.setup(); const clipboard = vi.spyOn(navigator.clipboard, 'writeText');
  vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
    if (!String(input).includes('/references?')) return json({ file, ref: 'drivebridge://fixture/file/file/version/new' });
    if (reason === 'denied') return json({}, 403);
    return json({ ref, file: { ...file, id: reason === 'wrong-file' ? 'other' : 'file', version_id: reason === 'wrong-version' ? 'new' : 'old' }, sha256: 'a'.repeat(64) });
  });
  render(<ContextAssistant fileId="file" versionId="old" />);
  await user.click(screen.getByRole('button', { name: '上下文助手' }));
  await screen.findByRole('alert');
  expect(screen.queryByText(ref)).toBeNull();
  expect(screen.queryByRole('button', { name: '复制外部 Agent 固定引用' })).toBeNull();
  expect(clipboard).not.toHaveBeenCalled();
});

it('clears previously readable context when copying discovers revoked permission', async () => {
  const user = userEvent.setup(); const clipboard = vi.spyOn(navigator.clipboard, 'writeText');
  const fetch = requests(); const onDenied = vi.fn();
  render(<ContextAssistant fileId="file" versionId="old" onDenied={onDenied} />);
  await user.click(screen.getByRole('button', { name: '上下文助手' })); await screen.findByText(ref);
  fetch.mockResolvedValue(json({}, 403));
  await user.click(screen.getByRole('button', { name: '复制外部 Agent 固定引用' }));
  await screen.findByRole('alert');
  expect(screen.queryByText(ref)).toBeNull(); expect(clipboard).not.toHaveBeenCalled(); expect(onDenied).toHaveBeenCalledOnce();
});

it('uses the file detail selected version and clears the whole drawer on context revocation', async () => {
  requests();
  render(<FileDetailSheet fileId="file" versionId="old" open onClose={vi.fn()} />);
  await screen.findByText(file.path);
  fireEvent.click(screen.getByRole('button', { name: '上下文助手' })); await screen.findByText(ref);
  fireEvent(window, new Event('drivebridge-context'));
  const dialog = screen.getByRole('dialog');
  expect(within(dialog).queryByText(ref)).toBeNull(); expect(within(dialog).queryByText(file.path)).toBeNull();
  expect(within(dialog).getByText(/上下文已清除/)).toBeTruthy();
});

it('drops the old fixed context when the selected version changes without resolving the new version automatically', async () => {
  const fetch = requests();
  const view = render(<ContextAssistant fileId="file" versionId="old" />);
  fireEvent.click(screen.getByRole('button', { name: '上下文助手' })); await screen.findByText(ref);
  const reads = fetch.mock.calls.length;
  view.rerender(<ContextAssistant fileId="file" versionId="new" />);
  expect(screen.queryByText(ref)).toBeNull();
  expect(screen.queryByRole('region', { name: '固定上下文助手' })).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(reads);
});

it('keeps file details open when Escape closes the assistant from its entry', async () => {
  requests(); const user = userEvent.setup(); const close = vi.fn();
  render(<FileDetailSheet fileId="file" versionId="old" open onClose={close} />);
  const entry = await screen.findByRole('button', { name: '上下文助手' });
  await user.click(entry); await screen.findByText(ref);
  await user.keyboard('{Escape}');
  await waitFor(() => expect(screen.queryByRole('region', { name: '固定上下文助手' })).toBeNull());
  expect(screen.getByRole('dialog')).toBeTruthy(); expect(close).not.toHaveBeenCalled();
  expect(document.activeElement).toBe(entry);
});
