import { createRequire } from 'node:module';
import { mkdtemp, rm, writeFile, cp, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, it } from 'vitest';
const require = createRequire(import.meta.url);
const { validatePackageResources } = require('../scripts/package-resources.cjs') as { validatePackageResources: (directory: string, platform: string, arch: string) => Promise<void> };

it('rejects missing resources and unsupported or non-native targets with safe errors', async () => {
  const root = await mkdtemp(join(tmpdir(), 'desktop-package-'));
  try {
    for (const [platform, arch] of [[process.platform, process.arch], ['all', 'all'], [process.platform === 'win32' ? 'linux' : 'win32', 'x64']]) {
      await expect(validatePackageResources(root, platform, arch)).rejects.toThrow('desktop_package_resources_invalid');
    }
    await writeFile(join(root, 'bundle.json'), '{}');
    await expect(validatePackageResources(root, process.platform, process.arch)).rejects.toThrow('desktop_package_resources_invalid');
  } finally { await rm(root, { recursive: true, force: true }); }
});
it.skipIf(process.env.DRIVEBRIDGE_HOST_INTEGRATION !== '1')('validates the real native pure-Go bundle and rejects extra files, digest drift and aliases', async () => {
  const root = await mkdtemp(join(tmpdir(), 'desktop-package-'));
  const bundle = join(root, 'host-bin');
  try {
    await cp(join(process.cwd(), 'host-bin'), bundle, { recursive: true });
    await expect(validatePackageResources(bundle, process.platform, process.arch)).resolves.toBeUndefined();
    await writeFile(join(bundle, 'private-state'), 'synthetic');
    await expect(validatePackageResources(bundle, process.platform, process.arch)).rejects.toThrow('desktop_package_resources_invalid');
    await rm(join(bundle, 'private-state'));
    if (process.platform !== 'win32') {
      const alias = join(root, 'alias'); await symlink(bundle, alias);
      await expect(validatePackageResources(alias, process.platform, process.arch)).rejects.toThrow('desktop_package_resources_invalid');
    }
    await writeFile(join(bundle, 'bundle.json'), JSON.stringify({ schema_version: 'drivebridge.desktop.bundle.v1', drivebridge: 'sha256:' + '0'.repeat(64), pinax: 'sha256:' + '0'.repeat(64), manifest: 'sha256:' + 'a'.repeat(64) }));
    await expect(validatePackageResources(bundle, process.platform, process.arch)).rejects.toThrow('desktop_package_resources_invalid');
  } finally { await rm(root, { recursive: true, force: true }); }
}, 30000);
