import { expect, it, vi } from 'vitest';
import { HostLifetime } from './host-lifecycle';

it('invalidates synchronously and shares cleanup between suspend, close and quit', async () => {
  let finish!: () => void;
  const host = { alive: true, close: vi.fn(() => new Promise<void>(resolve => { finish = resolve; })) };
  const start = vi.fn(async () => host);
  const lifetime = new HostLifetime(start);
  await lifetime.ready(); expect(lifetime.current).toBe(host);
  const closed = lifetime.stop(); expect(lifetime.current).toBeUndefined();
  expect(lifetime.stop()).toBe(closed);
  await Promise.resolve(); expect(host.close).toHaveBeenCalledTimes(1);
  finish(); await closed;
  await lifetime.ready(); expect(lifetime.current).toBeUndefined(); expect(start).toHaveBeenCalledTimes(1);
});
it('closes an owner arriving after suspend instead of publishing it', async () => {
  let finish!: (value: { alive: boolean; close: () => Promise<void> }) => void;
  const lifetime = new HostLifetime(() => new Promise<{ alive: boolean; close: () => Promise<void> }>(resolve => { finish = resolve; }));
  await Promise.resolve(); const closed = lifetime.stop();
  const host = { alive: true, close: vi.fn(async () => {}) }; finish(host);
  await lifetime.ready(); expect(lifetime.current).toBeUndefined(); await closed;
  expect(host.close).toHaveBeenCalledTimes(1);
});
it('failed startup remains unavailable and cleanup failure stays observable', async () => {
  const failed = new HostLifetime(async () => { throw new Error('private launch detail'); });
  await failed.ready(); expect(failed.current).toBeUndefined(); await failed.stop();
  const lifetime = new HostLifetime(async () => ({ alive: true, close: async () => { throw new Error('desktop_host_cleanup_timeout'); } }));
  await lifetime.ready(); await expect(lifetime.stop()).rejects.toThrow('desktop_host_cleanup_timeout');
  expect(lifetime.current).toBeUndefined(); await expect(lifetime.stop()).rejects.toThrow('desktop_host_cleanup_timeout');
});

it.each(['suspend', 'revoke', 'window', 'exit'] as const)('fences a late body after %s', async reason => {
  const host = { alive: true, close: async () => {} };
  const lifetime = new HostLifetime(async () => host);
  await lifetime.ready();
  let windowValid = true;
  let finish!: (body: string) => void;
  const pending = lifetime.request(host, () => { if (!windowValid) throw new Error('window_invalid'); }, () => new Promise<string>(resolve => { finish = resolve; }));
  const rejected = expect(pending).rejects.toThrow(reason === 'window' ? 'window_invalid' : 'desktop_session_invalid');
  if (reason === 'suspend') await lifetime.stop();
  if (reason === 'revoke') lifetime.invalidateRequests();
  if (reason === 'window') windowValid = false;
  if (reason === 'exit') host.alive = false;
  finish('private body'); await rejected;
});
it('revocation replaces late errors and fresh calls retain their own generation', async () => {
  const host = { alive: true, close: async () => {} };
  const lifetime = new HostLifetime(async () => host); await lifetime.ready();
  let fail!: (error: Error) => void;
  const pending = lifetime.request(host, () => {}, () => new Promise<never>((_, reject) => { fail = reject; }));
  const rejected = expect(pending).rejects.toThrow('desktop_session_invalid');
  lifetime.invalidateRequests(); fail(new Error('private upstream error')); await rejected;
  expect(await lifetime.request(host, () => {}, async () => 'current')).toBe('current');
});
