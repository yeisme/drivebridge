// UX preflight only. The owner still validates target permissions and paths.
// Match the owner's unconditional pre-allocation rejection rules; never trim
// or normalize an entered path, which could silently change its destination.
export function invalidArchivePrefix(prefix: string): boolean {
  if (prefix === '') return false;
  if (new TextEncoder().encode(prefix).byteLength > 1024 || prefix.startsWith('/') || /[\\\u0000-\u001f]/.test(prefix)) return true;
  return prefix.split('/').some(part => part === '' || part === '.' || part === '..' || part === '.inbox-staging');
}
