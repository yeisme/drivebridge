// Deterministic local module configuration; no network or credential access.
import { writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
const root = new URL('../', import.meta.url);
const forgeVersion = '8.0.1';
const files = {
  'package.json': {
    name: 'drivebridge-desktop', version: '0.1.0', private: true,
    description: 'DriveBridge native host for the shared Web application',
    author: 'Yeisme', homepage: 'https://github.com/yeisme/drivebridge',
    engines: { node: '>=22.13.0' },
    license: 'ISC', main: 'dist/main.js',
    scripts: {
      build: 'pnpm --dir ../web build && node scripts/build.mjs',
      'build:host': 'node scripts/build.mjs',
      start: 'electron-forge start', package: 'electron-forge package', make: 'electron-forge make',
      'check:package': 'node scripts/check-package.cjs',
      test: 'vitest run', typecheck: 'tsc --noEmit',
      contracts: 'node scripts/host-contracts.mjs', 'contracts:check': 'node scripts/host-contracts.mjs --check',
    },
    config: { forge: './forge.config.cjs' },
    dependencies: { 'electron-squirrel-startup': '1.0.1' },
    devDependencies: {
      ajv: '8.17.1', 'openapi-typescript': '7.10.1',
      '@types/electron-squirrel-startup': '1.0.2',
      electron: '44.5.1', '@electron-forge/cli': forgeVersion,
      '@electron-forge/maker-dmg': forgeVersion, '@electron-forge/maker-squirrel': forgeVersion,
      '@electron-forge/maker-deb': forgeVersion,
      '@electron-forge/plugin-fuses': forgeVersion, '@electron/fuses': '2.1.3',
      '@types/node': '22.19.0', typescript: '5.9.3', vitest: '4.1.11', esbuild: '0.28.2',
    },
  },
  'tsconfig.json': {
    compilerOptions: { target: 'ES2022', module: 'ESNext', moduleResolution: 'Bundler', strict: true, noEmit: true, skipLibCheck: true, resolveJsonModule: true, esModuleInterop: true, types: ['node'], lib: ['ES2022', 'DOM'] },
    include: ['src/**/*.ts'],
  },
};
execFileSync('pnpm', ['config', 'set', 'node-linker', 'hoisted', '--location', 'project'], { cwd: root, stdio: 'pipe' });
for (const [path, data] of Object.entries(files)) await writeFile(new URL(path, root), JSON.stringify(data, null, 2) + '\n');
