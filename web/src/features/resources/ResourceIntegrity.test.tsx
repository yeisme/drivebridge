import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { ResourceIntegrity } from './ResourceIntegrity';
const receipt = fixtures.ResourceIntegrityReceiptEnvelope.data;
const props = { descriptor: receipt.descriptor_id, revision: 1, close: vi.fn(), onDenied: vi.fn() };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });
const execute = () => { fireEvent.click(screen.getByRole('button', { name: '准备检查固定字节' })); fireEvent.click(screen.getByRole('button', { name: '确认检查固定字节' })); };
it('requires explicit confirmation and presents narrow historical evidence', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(receipt)); render(<ResourceIntegrity {...props}/>);
  expect(fetch).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '准备检查固定字节' })); expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '取消检查' })); expect(fetch).not.toHaveBeenCalled(); execute();
  await screen.findByText('历史编码与字节完整性检查通过'); expect(screen.getByText(/不代表 Skill 格式合规/)).toBeTruthy();
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/resource-descriptors/descriptor-fixture/checks');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual({ expected_revision: 1, checker_version: 'utf8-integrity-v1' });
  expect(screen.queryByRole('button', { name: '准备检查固定字节' })).toBeNull();
});
it('unknown execution only offers original lookup, which restores receipt without another execution', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(receipt)); render(<ResourceIntegrity {...props}/>); execute();
  await screen.findByText(/结果未知/); expect(screen.queryByRole('button', { name: '准备检查固定字节' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '查询原编码检查回执' })); await screen.findByText(/原检查标识/);
  expect(fetch).toHaveBeenCalledTimes(2); expect(fetch.mock.calls[1][0]).toMatch(/checks\/lookup$/);
});
it('missing lookup is not a failed check and requires a fresh human confirmation', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, 404)); render(<ResourceIntegrity {...props}/>);
  fireEvent.click(screen.getByRole('button', { name: '查询原编码检查回执' })); await screen.findByText(/不代表从未执行或检查失败/);
  expect(fetch).toHaveBeenCalledTimes(1); fireEvent.click(screen.getByRole('button', { name: '准备检查固定字节' })); expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([{ descriptor_id: 'foreign' }, { descriptor_revision: 2 }, { checker_version: 'execute' }, { check_scope: 'safe' }, { state: 'installed' }, { check_id: '' }, { checked_at: 'invalid' }])('rejects mismatched receipt %j', async patch => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ ...receipt, ...patch })); render(<ResourceIntegrity {...props}/>); execute(); await screen.findByText(/结果未知/); expect(screen.queryByText(/原检查标识/)).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('clears pending output on %s and ignores late response and online', async event => {
  let finish!: (r: Response) => void; const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<ResourceIntegrity {...props}/>); execute(); fireEvent(window, new Event(event)); await act(async () => { finish(json(receipt)); });
  expect(screen.queryByText(/原检查标识/)).toBeNull(); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: '查询原编码检查回执' })).toHaveProperty('disabled', true);
});
it('403 propagates revocation to parent and 409 never permits execution', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, 409)).mockResolvedValueOnce(json({}, 403)); render(<ResourceIntegrity {...props}/>); execute();
  await screen.findByText(/固定目标已变化/); expect(screen.queryByRole('button', { name: '准备检查固定字节' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '查询原编码检查回执' })); await waitFor(() => expect(props.onDenied).toHaveBeenCalledOnce()); expect(fetch).toHaveBeenCalledTimes(2);
});
it('displays encoding failure without claiming unsafe or installing', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ ...receipt, state: 'failed' })); render(<ResourceIntegrity {...props}/>); execute(); await screen.findByText(/历史编码检查未通过/);
});
