import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import type { Grant } from './grant-client';
import { GrantConnectionDialog } from './GrantConnectionDialog';

const grant = fixtures.GrantPageEnvelope.data.items[0] as Grant;
function clipboard(copy: (value: string) => Promise<void>) {
  vi.stubGlobal('navigator', Object.assign(Object.create(navigator), { clipboard: { writeText: copy } }));
}
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
it.each(['mcp', 'http'] as const)('renders a scoped %s template without requests or persistence', async protocol => {
  const fetch = vi.spyOn(globalThis, 'fetch');
  const storage = vi.spyOn(Storage.prototype, 'setItem');
  const copy = vi.fn().mockResolvedValue(undefined);
  clipboard(copy);
  render(<GrantConnectionDialog grant={{ ...grant, protocol, prefix: "資料/a'b &c", actions: ['files.read'] }} close={() => {}}/>);
  fireEvent.change(screen.getByLabelText('Agent 服务地址'), { target: { value: 'https://drive.example.invalid/' } });
  const content = screen.getByLabelText('占位接入配置').textContent!;
  expect(content).toContain('Bearer <AGENT_TOKEN>');
  if (protocol === 'mcp') {
    expect(JSON.parse(content).mcpServers.drivebridge).toEqual({ type: 'http', url: 'https://drive.example.invalid/mcp', headers: { Authorization: 'Bearer <AGENT_TOKEN>' } });
    expect(screen.getByText(/tools\/list/)).toBeTruthy();
  } else {
    expect(content).toContain('/api/v1/files?');
    expect(content).toContain(new URLSearchParams({ space_id: grant.space_id, prefix: "資料/a'b &c" }).toString());
  }
  expect(screen.getByText(/原 operation/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '复制占位配置' }));
  await screen.findByText(/已复制占位配置/);
  expect(copy).toHaveBeenCalledWith(content);
  expect(fetch).not.toHaveBeenCalled(); expect(storage).not.toHaveBeenCalled();
});
it.each(['https://user:secret@example.invalid', 'javascript:alert(1)', 'https://example.invalid/?token=synthetic', 'https://example.invalid/#synthetic', 'https://example.invalid/nested', 'https://example.invalid?'])('rejects ambiguous or credential-bearing service address %s', value => {
  render(<GrantConnectionDialog grant={grant} close={() => {}}/>);
  fireEvent.change(screen.getByLabelText('Agent 服务地址'), { target: { value } });
  expect(screen.queryByLabelText('占位接入配置')).toBeNull();
  expect((screen.getByRole('button', { name: '复制占位配置' }) as HTMLButtonElement).disabled).toBe(true);
});
it('handles clipboard failure without claiming setup, exposes inactive state and clears on context change', async () => {
  const close = vi.fn();
  const copy = vi.fn().mockRejectedValue(new Error('denied'));
  clipboard(copy);
  render(<GrantConnectionDialog grant={{ ...grant, state: 'revoked' }} close={close}/>);
  expect(screen.getByText(/此授权已撤销/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '复制占位配置' }));
  await screen.findByText(/无法访问剪贴板/);
  fireEvent(window, new Event('drivebridge-context'));
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1));
  expect(screen.queryByLabelText('占位接入配置')).toBeNull();
});
it('does not report a late clipboard completion for a changed address or unmounted dialog', async () => {
  let finish!: () => void;
  clipboard(() => new Promise<void>(resolve => { finish = resolve; }));
  const view = render(<GrantConnectionDialog grant={grant} close={() => {}}/>);
  fireEvent.click(screen.getByRole('button', { name: '复制占位配置' }));
  fireEvent.change(screen.getByLabelText('Agent 服务地址'), { target: { value: 'https://other.example.invalid' } });
  finish();
  await waitFor(() => expect((screen.getByRole('button', { name: '复制占位配置' }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.queryByText(/已复制占位配置/)).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '复制占位配置' }));
  view.unmount(); finish();
  expect(screen.queryByRole('dialog')).toBeNull();
});
it('does not advertise write tools or a files call when files.read is absent', () => {
  render(<GrantConnectionDialog grant={{ ...grant, protocol: 'http', actions: ['inbox.submit'] }} close={() => {}}/>);
  expect(screen.getByLabelText('占位接入配置').textContent).toContain('/api/v1/capabilities');
  expect(screen.getByLabelText('占位接入配置').textContent).not.toContain('/api/v1/files?');
  expect(screen.getByText(/HTTP 数据传输/)).toBeTruthy();
});
