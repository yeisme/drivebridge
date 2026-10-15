/// <reference lib="es2024.promise" />
import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { PrepareDelivery } from './PrepareDelivery';
import type { Space } from '../api';
const spaces: Space[] = [{ id: 'space', name: 'Space', connection_id: 'platform', used_bytes: 0, reserved_bytes: 0, max_bytes: 100 }];
const json = (data: unknown) => new Response(JSON.stringify({ data }), { status: 200 });
afterEach(() => { cleanup(); vi.restoreAllMocks(); localStorage.clear(); });
function locks() { Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!() } }); }
it('requires a real share revision and historical snapshot before prepare', async () => {
  locks();
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async input => { const url = String(input); if (url.endsWith('/shares')) return json([{ id: 'share', name: 'Share', revision: 3, revoked: false }]); if (url.includes('/file-collections?')) return json([{ collection_id: 'collection', name: 'Collection', space_id: 'space' }]); if (url.includes('/file-collections/collection/snapshots')) return json({ collection_id: 'collection', observed_at: '2026-01-01T00:00:00Z', items: [{ snapshot_id: 'a'.repeat(48), collection_revision: 2, created_at: '2026-01-01T00:00:00Z' }] }); if (url.includes('/delivery-manifests')) return json({ share_id: 'share', manifest_revision: 1, phase: 'verifying', required_items: 1, verified_items: 0, complete: false, retention_ready: false }); return json({ found: false }); });
  render(<PrepareDelivery actor="alice" spaces={spaces} onManifest={vi.fn()}/>);
  fireEvent.click(screen.getByRole('button', { name: '读取分享与集合来源' }));
  fireEvent.click(await screen.findByRole('combobox', { name: '选择分享' }));
  const share = await screen.findByRole('option', { name: 'Share · 修订 3' }); fireEvent.pointerDown(share, { pointerType: 'mouse' }); fireEvent.click(share);
  fireEvent.click(screen.getByRole('combobox', { name: '交付来源集合' }));
  const collection = await screen.findByRole('option', { name: 'Collection' }); fireEvent.pointerDown(collection, { pointerType: 'mouse' }); fireEvent.click(collection);
  fireEvent.click(await screen.findByRole('combobox', { name: '交付固定快照' }));
  const snapshot = await screen.findByRole('option', { name: /修订 2/ }); fireEvent.pointerDown(snapshot, { pointerType: 'mouse' }); fireEvent.click(snapshot);
  fireEvent.click(screen.getByRole('button', { name: '准备清单' }));
  await waitFor(() => expect(fetch.mock.calls.some(([, init]) => JSON.parse(String(init?.body || '{}')).expected_share_revision === 3)).toBe(true));
});
it('clears delivery observations and enters denied on a source authorization failure', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ error: { code: 'forbidden' } }), { status: 403 }));
  render(<PrepareDelivery actor="alice" spaces={spaces} onManifest={vi.fn()}/>);
  fireEvent.click(screen.getByRole('button', { name: '读取分享与集合来源' }));
  await screen.findByText('权限或范围已变化，交付来源已清除。');
  expect(screen.queryByRole('combobox')).toBeNull();
  expect(screen.getByRole('button', { name: '读取分享与集合来源' })).toHaveProperty('disabled', true);
});
