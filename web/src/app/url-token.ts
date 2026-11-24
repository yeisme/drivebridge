export interface ExtractedUrlToken {
  token: string;
  cleanUrl: string;
}

const TOKEN_PARAM_KEYS = ["token", "key"] as const;

function stripParams(params: URLSearchParams): string | null {
  let found: string | null = null;
  for (const name of TOKEN_PARAM_KEYS) {
    const candidate = params.get(name)?.trim();
    if (candidate && !found) found = candidate;
    if (params.has(name)) params.delete(name);
  }
  return found;
}

/**
 * Extracts `?token=` (or `?key=`) from location search or hash query/fragment,
 * returning the sanitized relative URL so the browser address bar can be scrubbed
 * immediately via `history.replaceState` before any network/subresource calls.
 */
export function extractUrlLoginToken(loc: {
  pathname: string;
  search: string;
  hash: string;
}): ExtractedUrlToken | null {
  let token: string | null = null;

  const searchParams = new URLSearchParams(loc.search);
  const searchToken = stripParams(searchParams);
  if (searchToken) token = searchToken;

  let nextHash = loc.hash;
  if (loc.hash.startsWith("#token=") || loc.hash.startsWith("#key=")) {
    const hashParams = new URLSearchParams(loc.hash.slice(1));
    const directHashToken = stripParams(hashParams);
    if (directHashToken && !token) token = directHashToken;
    const rest = hashParams.toString();
    nextHash = rest ? `#${rest}` : "";
  } else if (loc.hash.startsWith("#/") && loc.hash.includes("?")) {
    const qIdx = loc.hash.indexOf("?");
    const hashRoute = loc.hash.slice(0, qIdx);
    const hashQuery = new URLSearchParams(loc.hash.slice(qIdx + 1));
    const hashRouteToken = stripParams(hashQuery);
    if (hashRouteToken && !token) token = hashRouteToken;
    const rest = hashQuery.toString();
    nextHash = rest ? `${hashRoute}?${rest}` : hashRoute;
  }

  if (!token) return null;

  const cleanSearch = searchParams.toString();
  const cleanUrl = `${loc.pathname}${cleanSearch ? `?${cleanSearch}` : ""}${nextHash}`;
  return { token, cleanUrl };
}

/**
 * Parses either a raw `dbu_...` token, a full invite URL (`https://.../?token=dbu_...`),
 * or a Baidu-Netdisk-style multi-line invite message containing a link or key.
 */
export function parsePastedInviteOrKey(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "";

  // 1. Check for URL containing ?token= or ?key= or #token=
  const urlMatch = trimmed.match(/https?:\/\/[^\s"'<>]+/i);
  if (urlMatch) {
    try {
      const parsed = new URL(urlMatch[0]);
      const extracted = extractUrlLoginToken({
        pathname: parsed.pathname,
        search: parsed.search,
        hash: parsed.hash,
      });
      if (extracted?.token) return extracted.token;
    } catch {
      /* fall through to token regex */
    }
  }

  // 2. Check for query-like token=xxx even without scheme
  const queryMatch = trimmed.match(/(?:[?&#]|\b)(?:token|key)=([!-~]{16,128})/i);
  if (queryMatch?.[1]) {
    try {
      return decodeURIComponent(queryMatch[1]);
    } catch {
      return queryMatch[1];
    }
  }

  // 3. Check for dbu_ token embedded in multi-line invitation text
  const dbuMatch = trimmed.match(/\bdbu_[0-9a-zA-Z_-]{16,124}\b/);
  if (dbuMatch?.[0]) return dbuMatch[0];

  return trimmed;
}

export function buildDirectTokenInviteUrl(origin: string, token: string, targetHash = ""): string {
  const base = origin.replace(/\/+$/, "");
  const params = new URLSearchParams({ token });
  const normalizedHash = targetHash && targetHash.startsWith("#") ? targetHash : "";
  return `${base}/?${params.toString()}${normalizedHash}`;
}

export function formatInviteShareText(params: {
  name: string;
  url: string;
  token: string;
  oneTime?: boolean;
  expiresAt?: string;
}): string {
  const modeLabel = params.oneTime ? "一次性专属免密直连（打开即登录，单次有效）" : "专属密钥直连链接";
  const expiryLine =
    params.expiresAt && !params.expiresAt.startsWith("0001")
      ? `\n有效期至：${params.expiresAt.slice(0, 10)}`
      : "";
  return `邀请你使用 DriveBridge 云盘（${params.name}）\n链接：${params.url}\n访问口令：${params.token}\n说明：${modeLabel}${expiryLine}`;
}
