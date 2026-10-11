import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SearchFileReference } from './SearchFileReference';
const props = { file: 'file', version: 'old', close: vi.fn(), onDenied: vi.fn() };
const current = { id: 'file', space_id: 'space', version_id: 'new', path: 'note.txt', trashed: false };
const ref = 'drivebridge://instance/file/file/version/old';
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
function metadata() {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ file: current }))
    .mockResolvedValueOnce(json({ ref: 'drivebridge://instance/file/file/version/new' }))
    .mockResolvedValueOnce(json({ file: { ...current, version_id: 'old' }, ref }));
}
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });
it('resolves the original version and reads body only on a second explicit click', async () => {
  const fetch = metadata().mockResolvedValueOnce(json({ file: current })).mockResolvedValueOnce(new Response('original text'));
  render(<SearchFileReference {...props}/>);
  await screen.findByText(ref);
  expect(fetch).toHaveBeenCalledTimes(3);
  expect(fetch.mock.calls[2][0]).toBe('/api/v1/references?ref=' + encodeURIComponent(ref));
  expect(screen.getByRole('heading', { name: '搜索结果原版本' })).toBe(document.activeElement);
  fireEvent.click(screen.getByRole('button', { name: '读取搜索原版本正文' }));
  await screen.findByText('original text');
  expect(String(fetch.mock.calls[4][0])).toContain('version=old');
  fireEvent.click(screen.getByRole('button', { name: '关闭搜索原版本正文' }));
  expect(screen.queryByText('original text')).toBeNull();
});
it('rejects a resolved latest version instead of silently substituting the head', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ file: current })).mockResolvedValueOnce(json({ ref: 'drivebridge://instance/file/file/version/new' })).mockResolvedValueOnce(json({ file: current, ref }));
  render(<SearchFileReference {...props}/>); await screen.findByText(/未改读最新版本/);
  expect(screen.queryByRole('button', { name: '读取搜索原版本正文' })).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('clears metadata and rejects late %s results', async event => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<SearchFileReference {...props}/>); fireEvent(window, new Event(event)); finish(json({ file: current }));
  await waitFor(() => expect(screen.queryByRole('button', { name: '读取搜索原版本正文' })).toBeNull());
  fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([401, 403])('propagates metadata permission loss %s', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, status));
  render(<SearchFileReference {...props}/>); await waitFor(() => expect(props.onDenied).toHaveBeenCalledTimes(1));
  expect(screen.queryByText(ref)).toBeNull();
});
it('unmount aborts an in-flight metadata request', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(() => {}));
  const view = render(<SearchFileReference {...props}/>); view.unmount();
  expect(fetch.mock.calls[0][1]?.signal?.aborted).toBe(true);
});
