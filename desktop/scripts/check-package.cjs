// Inspect a locally assembled native package; this is not signing or OS acceptance.
const { join, resolve } = require('node:path');
const { lstat } = require('node:fs/promises');
const { validatePackageResources } = require('./package-resources.cjs');
async function checkPackage(path, platform = process.platform, arch = process.arch) {
  if (!path || !['linux', 'darwin', 'win32'].includes(platform) || platform !== process.platform || arch !== process.arch) throw new Error('desktop_package_check_unsupported');
  const root = resolve(path);
  const app = platform === 'darwin' ? join(root, 'drivebridge-desktop.app') : root;
  const resources = platform === 'darwin' ? join(app, 'Contents', 'Resources') : join(root, 'resources');
  await validatePackageResources(join(resources, 'host-bin'), platform, arch);
  const asar = await import('@electron/asar');
  const archive = join(resources, 'app.asar');
  if (!(await lstat(archive)).isFile()) throw new Error('desktop_package_archive_invalid');
  const entries = asar.listPackage(archive).map(value => value.replaceAll('\\', '/'));
  for (const required of ['/dist/main.js', '/dist/preload.js', '/web/index.html', '/package.json']) if (!entries.includes(required)) throw new Error('desktop_package_entry_missing');
  if (entries.some(value => value !== '/package.json' && !/^\/(dist|web)(\/|$)/.test(value))) throw new Error('desktop_package_unexpected_content');
  const { getCurrentFuseWire, FuseV1Options, FuseState } = await import('@electron/fuses');
  const executable = platform === 'darwin' ? app : join(root, 'drivebridge-desktop' + (platform === 'win32' ? '.exe' : ''));
  const wire = await getCurrentFuseWire(executable);
  for (const option of [FuseV1Options.RunAsNode, FuseV1Options.EnableNodeOptionsEnvironmentVariable, FuseV1Options.EnableNodeCliInspectArguments]) if (wire[option] !== FuseState.DISABLE) throw new Error('desktop_package_fuses_invalid');
  for (const option of [FuseV1Options.EnableCookieEncryption, FuseV1Options.EnableEmbeddedAsarIntegrityValidation, FuseV1Options.OnlyLoadAppFromAsar]) if (wire[option] !== FuseState.ENABLE) throw new Error('desktop_package_fuses_invalid');
  return { platform, arch };
}
module.exports = { checkPackage };
if (require.main === module) checkPackage(process.argv[2]).then(({ platform, arch }) => {
  console.log(`Native ${platform}/${arch} package resources, ASAR contents and fuses verified; unsigned, not platform acceptance.`);
}).catch(() => { console.error('desktop_package_check_failed'); process.exitCode = 1; });
