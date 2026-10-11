import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SubmissionPreview } from './SubmissionPreview';
import type { Submission } from './submission-client';

const submission: Submission = { submission_id: 'sub-one', space_id: 'space-one', file_ref: 'file-one', version_id: 'original', agent_id: 'agent-one', grant_id: 'grant-one', source_kind: 'agent', upload_operation_ref: 'upload-one', state: 'submitted', revision: 2, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
function source(name = 'result.png', mime = 'image/png', body: BodyInit = 'original body') {
  let denied = false;
  const head = { id: 'file-one', space_id: 'space-one', version_id: 'new-head', path: 'renamed-head.html', mime_type: 'text/html', size_bytes: 999, trashed: false };
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
    const url = String(input);
    if (denied) return json({}, 403);
    if (url.endsWith('/inbox-submissions/sub-one')) return json({ ...submission, file_name: name, mime_type: 'untrusted/claim' });
    if (url.endsWith('/files/file-one')) return json({ file: head, ref: 'drivebridge://instance/file/file-one/version/new-head' });
    if (url.includes('/references?')) return json({ file: { ...head, version_id: 'original', mime_type: mime, size_bytes: 34, preview_url: 'https://external.invalid/private', compatibility_url: 'https://external.invalid/derived' } });
    if (url === '/api/v1/files/file-one/content?version=original') return new Response(body);
    throw new Error('unexpected_test_request');
  });
  return { fetch, deny: () => { denied = true; } };
}
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('uses owner-resolved original MIME and safe media URL, not head type, claimed MIME or supplied URLs', async () => {
  const { fetch } = source(); render(<SubmissionPreview submission={submission} close={() => {}}/>);
  const image = await screen.findByRole('img', { name: 'result.png' });
  expect(image.getAttribute('src')).toBe('/api/v1/files/file-one/media?version=original');
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
  expect(fetch.mock.calls.some(([url]) => String(url).includes('external.invalid'))).toBe(false);
  const reference = fetch.mock.calls.find(([url]) => String(url).includes('/references?'))![0];
  expect(new URL(String(reference), 'http://localhost').searchParams.get('ref')).toMatch(/\/version\/original$/);
  expect(screen.queryByText('交互预览')).toBeNull(); expect(screen.queryByText('生成兼容预览')).toBeNull();
  expect(await screen.findByRole('button', { name: '下载原提交版本' })).toBeTruthy();
});
it.each(['offline', 'drivebridge-context'])('clears mounted media and download on %s', async event => {
  source(); render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByRole('img'); fireEvent(window, new Event(event));
  expect(screen.queryByRole('img')).toBeNull(); expect(screen.queryByRole('button', { name: '下载原提交版本' })).toBeNull();
  expect(screen.getByRole('alert').textContent).toContain('正文已清除');
});
it('does not start a late media reader after context invalidation', async () => {
  let resolve!: (response: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  fireEvent(window, new Event('drivebridge-context'));
  await act(async () => resolve(json({ ...submission, file_name: 'late.png' })));
  expect(fetch).toHaveBeenCalledTimes(1); expect(screen.queryByRole('img')).toBeNull();
});
it.each([401, 403, 404])('clears content when original identity cannot be authorized: %s', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, status));
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByRole('alert'); expect(screen.queryByRole('button', { name: '下载原提交版本' })).toBeNull(); expect(screen.queryByRole('img')).toBeNull();
});
it('keeps old metadata responses readable without inventing an original filename', async () => {
  const { fetch } = source(); const original = fetch.getMockImplementation()!;
  fetch.mockImplementation((input, options) => String(input).endsWith('/inbox-submissions/sub-one') ? Promise.resolve(json(submission)) : original(input, options));
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  const image = await screen.findByRole('img', { name: '文件名称未提供' });
  expect(image.getAttribute('src')).toContain('version=original');
  expect(screen.queryByText('renamed-head.html')).toBeNull();
});
it('removes displayed media if download reauthorization returns privacy-preserving 404', async () => {
  const { fetch } = source(); const anchor = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  render(<SubmissionPreview submission={submission} close={() => {}}/>); await screen.findByRole('img');
  fetch.mockResolvedValueOnce(json({}, 404)); fireEvent.click(screen.getByRole('button', { name: '下载原提交版本' }));
  await screen.findByText(/正文已清除/); expect(screen.queryByRole('img')).toBeNull(); expect(anchor).not.toHaveBeenCalled();
});
it('refuses a references response with a changed original version', async () => {
  const { fetch } = source(); const original = fetch.getMockImplementation()!;
  fetch.mockImplementation((input, options) => String(input).includes('/references?') ? Promise.resolve(json({ file: { id: 'file-one', space_id: 'space-one', version_id: 'new-head' } })) : original(input, options));
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByText(/未改读最新版本/); expect(screen.queryByRole('img')).toBeNull(); expect(screen.queryByRole('button', { name: '下载原提交版本' })).toBeNull();
});
it('escapes HTML without mounting a script, iframe or interactive reader', async () => {
  source('untrusted.html', 'text/html', '<script>unsafe()</script><img src="https://external.invalid/track">');
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  expect((await screen.findByLabelText('收件原版本纯文本')).textContent).toContain('<script>unsafe()</script>');
  expect(document.querySelector('pre script, iframe, pre img')).toBeNull();
});
it('offers only explicit original download for unsupported binary without reading or converting automatically', async () => {
  const { fetch } = source('result.bin', 'application/octet-stream'); const downloads: string[] = [];
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) { downloads.push(this.href); });
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByText(/此格式无法在浏览器内直接预览/); expect(fetch).toHaveBeenCalledTimes(4);
  fireEvent.click(screen.getByRole('button', { name: '下载原提交版本' }));
  await waitFor(() => expect(downloads).toHaveLength(1)); expect(fetch).toHaveBeenCalledTimes(8);
  expect(new URL(downloads[0]).pathname + new URL(downloads[0]).search).toBe('/api/v1/files/file-one/content?version=original');
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
});
it('does not let download errors erase a failed text-read state and invent an empty file', async () => {
  const { deny } = source('binary.txt', 'text/plain', new Uint8Array([255, 0, 255]));
  render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByText(/不是可直接显示的 UTF-8/); expect(screen.queryByLabelText('收件原版本纯文本')).toBeNull();
  const anchor = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  fireEvent.click(screen.getByRole('button', { name: '下载原提交版本' }));
  await waitFor(() => expect(anchor).toHaveBeenCalledTimes(1));
  expect(screen.queryByLabelText('收件原版本纯文本')).toBeNull(); expect(screen.getByText(/不是可直接显示的 UTF-8/)).toBeTruthy();
  deny(); fireEvent.click(screen.getByRole('button', { name: '下载原提交版本' }));
  await screen.findByText(/正文已清除/); expect(anchor).toHaveBeenCalledTimes(1);
});
it.each(['unmount', 'context'])('cancels download authorization that finishes after %s', async end => {
  const { fetch } = source('result.bin', 'application/octet-stream'); const anchor = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  const view = render(<SubmissionPreview submission={submission} close={() => {}}/>);
  await screen.findByRole('button', { name: '下载原提交版本' });
  let finish!: (response: Response) => void;
  fetch.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  fireEvent.click(screen.getByRole('button', { name: '下载原提交版本' }));
  if (end === 'unmount') view.unmount(); else fireEvent(window, new Event('drivebridge-context'));
  await act(async () => finish(json(submission))); expect(anchor).not.toHaveBeenCalled();
});
