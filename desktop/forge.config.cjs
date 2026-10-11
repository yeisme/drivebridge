const { join } = require('node:path');
const { validatePackageResources } = require('./scripts/package-resources.cjs');
const { checkPackage } = require('./scripts/check-package.cjs');
const metadata = require('./package.json');
const { FusesPlugin } = require('@electron-forge/plugin-fuses');
const { FuseV1Options, FuseVersion } = require('@electron/fuses');

module.exports = {
  packagerConfig: {
    asar: true,
    extraResource: [join(__dirname, 'host-bin')],
    // Explicit local Electron archive directory avoids downloads in offline checks.
    ...(process.env.DRIVEBRIDGE_ELECTRON_ZIP_DIR ? { electronZipDir: process.env.DRIVEBRIDGE_ELECTRON_ZIP_DIR } : {}),
    executableName: 'drivebridge-desktop',
    ignore: (path) => path !== '' && !/^\/(dist|web)(\/|$)/.test(path) && path !== '/package.json',
  },
  hooks: {
    prePackage: async (_config, platform, arch) => {
      // D07 baseline only; other architectures remain candidates, not releases.
      if ({ darwin: 'arm64', win32: 'x64', linux: 'x64' }[platform] !== arch) throw new Error('desktop_package_target_unsupported');
      await validatePackageResources(join(__dirname, 'host-bin'), platform, arch);
    },
    postPackage: async (_config, { outputPaths, platform, arch }) => {
      for (const path of outputPaths) await checkPackage(path, platform, arch);
    },
  },
  // Native unsigned installers only. No signing, publishing or auto-update feed.
  makers: [
    { name: '@electron-forge/maker-dmg', platforms: ['darwin'], config: { format: 'ULFO' } },
    { name: '@electron-forge/maker-squirrel', platforms: ['win32'], config: {
      name: metadata.name.replaceAll('-', '_'), authors: metadata.author, description: metadata.description,
      exe: 'drivebridge-desktop.exe', setupExe: 'DriveBridge-Setup.exe', noMsi: true,
    } },
    { name: '@electron-forge/maker-deb', platforms: ['linux'], config: { options: {
      name: metadata.name, productName: 'DriveBridge', genericName: 'Knowledge reader',
      bin: 'drivebridge-desktop', maintainer: metadata.author, homepage: metadata.homepage,
      description: metadata.description, categories: ['Office'],
      icon: { scalable: join(__dirname, 'web', 'icon.svg') },
    } } },
  ],
  plugins: [new FusesPlugin({
    version: FuseVersion.V1,
    [FuseV1Options.RunAsNode]: false,
    [FuseV1Options.EnableCookieEncryption]: true,
    [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
    [FuseV1Options.EnableNodeCliInspectArguments]: false,
    [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]: true,
    [FuseV1Options.OnlyLoadAppFromAsar]: true,
  })],
};
