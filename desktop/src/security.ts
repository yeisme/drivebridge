import { realpath, stat, readFile } from 'node:fs/promises';
import { resolve, relative, isAbsolute, extname } from 'node:path';

export const APP_URL = 'drivebridge-app://app/';
export const CSP = [
  "default-src 'none'", "script-src 'self'", "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:", "font-src 'self'", "connect-src 'self'",
  "worker-src 'self' blob:", "media-src 'self' blob:", "object-src 'none'",
  "base-uri 'none'", "form-action 'none'", "frame-src 'none'", "frame-ancestors 'none'",
].join('; ');

export function isAppDocument(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'drivebridge-app:' && url.hostname === 'app' && !url.port &&
      !url.username && !url.password && (url.pathname === '/' || url.pathname === '/index.html') && !url.search;
  } catch { return false; }
}

export function requireMainFrame(sender: { destroyed: boolean; sameContents: boolean; sameFrame: boolean; url: string }): void {
  if (sender.destroyed || !sender.sameContents || !sender.sameFrame || !isAppDocument(sender.url)) throw new Error('desktop_sender_invalid');
}
export function requireEmptyRequest(value: unknown): void {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).join() !== 'input') throw new Error('desktop_request_invalid');
  const input = (value as { input: unknown }).input;
  if (!input || typeof input !== 'object' || Array.isArray(input) || Object.keys(input).length) throw new Error('desktop_request_invalid');
}
const types: Record<string, string> = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8',
  '.woff2': 'font/woff2', '.woff': 'font/woff', '.ttf': 'font/ttf', '.pfb': 'application/octet-stream', '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg',
  '.svg': 'image/svg+xml', '.ico': 'image/x-icon', '.webp': 'image/webp', '.wasm': 'application/wasm', '.bcmap': 'application/octet-stream',
};
function inside(root: string, path: string): boolean {
  const rel = relative(root, path);
  return rel !== '' && rel !== '..' && !rel.startsWith('../') && !rel.startsWith('..\\') && !isAbsolute(rel);
}

// Only packaged Web resources, never a user path, SPA fallback, API proxy or
// asset handle. Resolve symlinks before checking containment as defense in depth.
export async function appResource(root: string, requestURL: string, method: string): Promise<Response> {
  const headers = { 'Content-Security-Policy': CSP, 'X-Content-Type-Options': 'nosniff', 'Cache-Control': 'no-store' };
  const denied = () => new Response('Not found', { status: 404, headers });
  if (method !== 'GET' && method !== 'HEAD') return denied();
  try {
    const url = new URL(requestURL);
    if (url.protocol !== 'drivebridge-app:' || url.hostname !== 'app' || url.port || url.username || url.password || url.search) return denied();
    const path = decodeURIComponent(url.pathname);
    if (path.includes('\\') || path.includes('\0') || path.split('/').some(part => part === '..' || part.startsWith('.'))) return denied();
    const file = path === '/' ? '/index.html' : path;
    if (file !== '/index.html' && !file.startsWith('/assets/') && !file.startsWith('/pdfjs/')) return denied();
    const contentType = types[extname(file)];
    if (!contentType || extname(file) === '.html' && file !== '/index.html') return denied();
    const canonicalRoot = await realpath(root);
    const target = await realpath(resolve(canonicalRoot, '.' + file));
    if (!inside(canonicalRoot, target)) return denied();
    const info = await stat(target);
    if (!info.isFile() || info.size > 32 * 1024 * 1024) return denied();
    const body = method === 'HEAD' ? null : new Uint8Array(await readFile(target));
    return new Response(body, { headers: { ...headers, 'Content-Type': contentType } });
  } catch { return denied(); }
}
