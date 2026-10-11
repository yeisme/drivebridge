import { createRequire } from 'node:module';
import { expect, it } from 'vitest';
const require = createRequire(import.meta.url);
const config = require('../forge.config.cjs');
const metadata = require('../package.json');
const { checkPackage } = require('../scripts/check-package.cjs');

it('pins all native makers to the CLI and Fuse plugin version and exposes make', () => {
  for (const name of ['maker-dmg', 'maker-squirrel', 'maker-deb', 'plugin-fuses']) {
    expect(metadata.devDependencies[`@electron-forge/${name}`]).toBe(metadata.devDependencies['@electron-forge/cli']);
  }
  expect(metadata.devDependencies['@electron-forge/cli']).toBe('8.0.1');
  expect(metadata.scripts.make).toBe('electron-forge make');
  expect(metadata.scripts['check:package']).toBe('node scripts/check-package.cjs');
  expect(metadata.author).toBeTruthy();
  expect(metadata.description).toBeTruthy();
  expect(metadata.homepage).toBeTruthy();
});

it('configures only DMG, Squirrel Setup EXE and DEB on their native platforms', () => {
  expect(config.makers.map(({ name, platforms }: { name: string; platforms: string[] }) => [name, platforms])).toEqual([
    ['@electron-forge/maker-dmg', ['darwin']],
    ['@electron-forge/maker-squirrel', ['win32']],
    ['@electron-forge/maker-deb', ['linux']],
  ]);
  const squirrel = config.makers[1].config;
  expect(squirrel.name).toBe('drivebridge_desktop');
  expect(squirrel.exe).toBe('drivebridge-desktop.exe');
  expect(squirrel.setupExe).toBe('DriveBridge-Setup.exe');
  expect(squirrel.noMsi).toBe(true);
  expect(config.makers[2].config.options.icon.scalable).toMatch(/[/\\]web[/\\]icon\.svg$/);
  expect(config.packagerConfig.asar).toBe(true);
  expect(config.packagerConfig.ignore('/src/private-state')).toBe(true);
  expect(config.packagerConfig.ignore('/host-bin/bundle.json')).toBe(true);
  expect(config.packagerConfig.ignore('/web/index.html')).toBe(false);
  expect(config.packagerConfig.ignore('/dist/main.js')).toBe(false);
});

it.each([['darwin', 'x64'], ['win32', 'arm64'], ['linux', 'arm64'], ['linux', 'ia32']])('rejects non-baseline %s/%s targets before reading resources', async (platform, arch) => {
  await expect(config.hooks.prePackage(config, platform, arch)).rejects.toThrow('desktop_package_target_unsupported');
});

it('rejects missing packaged bundles through the automatic post-package check', async () => {
  await expect(config.hooks.postPackage(config, { outputPaths: ['missing-native-package'], platform: process.platform, arch: process.arch })).rejects.toThrow('desktop_package_resources_invalid');
});

it('refuses cross-platform package verification rather than reporting native acceptance', async () => {
  await expect(checkPackage('missing-native-package', process.platform === 'win32' ? 'linux' : 'win32', process.arch)).rejects.toThrow('desktop_package_check_unsupported');
});
