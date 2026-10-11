export const HTML_PREVIEW_LIMIT = 2 * 1024 * 1024;
export function interactivePreviewURL(source: string, origin: string): string | undefined {
  const url = new URL(source, origin);
  if (url.origin !== origin || !/^\/api\/v1\/files\/[^/]+\/media$/.test(url.pathname) || !url.searchParams.get("version")) return undefined;
  url.searchParams.set("preview", "interactive");
  return url.href;
}

export async function readPreviewHTML(response: Response): Promise<string> {
  if (!response.ok || !response.body) throw new Error("preview_load_failed");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let size = 0;
  let text = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) return text + decoder.decode();
      size += value.byteLength;
      if (size > HTML_PREVIEW_LIMIT) throw new Error("preview_too_large");
      text += decoder.decode(value, { stream: true });
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
}
