// Packaging-time validation only; no owner execution or credential access.
const { execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const { lstat, readFile, readdir, realpath } = require('node:fs/promises');
const { join } = require('node:path');

async function validatePackageResources(directory, platform, arch) {
  try {
    const goOS = { linux: 'linux', darwin: 'darwin', win32: 'windows' }[platform];
    const goArch = { x64: 'amd64', arm64: 'arm64' }[arch];
    // Only native packages are supported by this local unsigned assembly path.
    if (!goOS || !goArch || platform !== process.platform || arch !== process.arch) throw new Error();
    if (await realpath(directory) !== directory) throw new Error();
    const directoryStat = await lstat(directory);
    if (!directoryStat.isDirectory() || platform !== 'win32' && directoryStat.mode & 0o022) throw new Error();
    const suffix = platform === 'win32' ? '.exe' : '';
    const names = ['bundle.json', 'drivebridge' + suffix, 'pinax' + suffix].sort();
    if (JSON.stringify((await readdir(directory)).sort()) !== JSON.stringify(names)) throw new Error();
    for (const name of names) {
      const stat = await lstat(join(directory, name));
      if (!stat.isFile() || stat.size > (name === 'bundle.json' ? 4096 : 512 * 1024 * 1024) || platform !== 'win32' && stat.mode & 0o022) throw new Error();
    }
    const bundle = JSON.parse(await readFile(join(directory, 'bundle.json'), 'utf8'));
    if (!bundle || Object.keys(bundle).sort().join() !== 'drivebridge,manifest,pinax,schema_version' || bundle.schema_version !== 'drivebridge.desktop.bundle.v1') throw new Error();
    for (const name of ['drivebridge', 'pinax', 'manifest']) if (typeof bundle[name] !== 'string' || !/^sha256:[a-f0-9]{64}$/.test(bundle[name])) throw new Error();
    for (const name of ['drivebridge', 'pinax']) {
      const file = join(directory, name + suffix);
      if ('sha256:' + createHash('sha256').update(await readFile(file)).digest('hex') !== bundle[name]) throw new Error();
      // Read embedded metadata; never run a candidate executable to inspect it.
      const info = execFileSync('go', ['version', '-m', file], { encoding: 'utf8', timeout: 15000, maxBuffer: 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] });
      const settings = new Map([...info.matchAll(/^\s*build\s+([^=\s]+)=(\S+)\s*$/gm)].map(match => [match[1], match[2]]));
      if (settings.get('GOOS') !== goOS || settings.get('GOARCH') !== goArch || settings.get('CGO_ENABLED') !== '0' || settings.get('-race') === 'true') throw new Error();
    }
  } catch { throw new Error('desktop_package_resources_invalid'); }
}
module.exports = { validatePackageResources };
