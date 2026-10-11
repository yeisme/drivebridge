import { expect, it } from 'vitest';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { HostClient, verifyBundle } from './host-client';

it('rejects absent or malformed bundle metadata without revealing paths', async () => {
  const root = await mkdtemp(join(tmpdir(), 'host-bundle-test-'));
  try {
    await expect(verifyBundle(root)).rejects.toThrow('desktop_host_unavailable');
    await writeFile(join(root, 'bundle.json'), '{"executable":"/private/secret"}');
    await expect(verifyBundle(root)).rejects.toThrow('desktop_host_unavailable');
  } finally { await rm(root, { recursive: true, force: true }); }
});

it.skipIf(process.env.DRIVEBRIDGE_HOST_INTEGRATION !== '1')('runs the real Go host with a plain directory refusal and reaps it', async () => {
  const root = await mkdtemp(join(tmpdir(), 'host-native-test-'));
  const host = await HostClient.start(join(process.cwd(), 'host-bin'));
  try {
    expect(host.alive).toBe(true);
    const result = await host.preview(root);
    expect(result.decision).toBe('blocked');
    expect(result.reason).toBe('owner_unavailable');
    expect(JSON.stringify(result)).not.toContain(root);
  } finally { await host.close(); await rm(root, { recursive: true, force: true }); }
  expect(host.alive).toBe(false);
}, 30000);
