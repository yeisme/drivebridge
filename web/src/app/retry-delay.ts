// HTTP Retry-After accepts integer seconds or an HTTP date.
export function retryAfterMs(value: string | null, now = Date.now()): number | undefined {
  if (!value?.trim()) return undefined;
  const text = value.trim();
  const seconds = /^\d+$/.test(text);
  // Numeric-looking values must not fall through to permissive date parsing.
  if (!seconds && /^[+-]?[\d.]+$/.test(text)) return undefined;
  const milliseconds = seconds ? Number(text) * 1000 : Date.parse(text) - now;
  return Number.isFinite(milliseconds) && milliseconds >= 0 ? milliseconds : undefined;
}

export function readRetryDelay(failures: number, retryAfter = 0, random = Math.random()): number {
  const base = [2000, 4000, 8000, 16000, 30000][Math.min(4, Math.max(0, failures - 1))];
  return Math.max(base + Math.floor(base * 0.1 * random), retryAfter);
}
