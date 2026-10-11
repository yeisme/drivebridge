import { expect, it } from 'vitest';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
// @ts-expect-error Existing build-only artifact writer.
import { applyArtifacts } from '../../web/scripts/contract-artifacts.mjs';

it('detects private generated contract drift without overwriting it', async () => {
  const root = await mkdtemp(join(tmpdir(), 'host-contract-check-'));
  try {
    const artifacts: Record<string, string> = {};
    for (const name of ['host.ts', 'host-schemas.json']) artifacts[name] = await readFile(new URL(`./generated/${name}`, import.meta.url), 'utf8');
    await applyArtifacts(root, artifacts);
    for (const [name, content] of Object.entries(artifacts)) {
      await writeFile(join(root, name), content + '\n');
      await expect(applyArtifacts(root, artifacts, true)).rejects.toThrow(name);
      expect(await readFile(join(root, name), 'utf8')).toBe(content + '\n');
      await applyArtifacts(root, artifacts);
    }
    await expect(applyArtifacts(root, artifacts, true)).resolves.toBeUndefined();
  } finally { await rm(root, { recursive: true, force: true }); }
});
