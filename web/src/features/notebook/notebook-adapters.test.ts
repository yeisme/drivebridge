import { afterEach, expect, it, vi } from "vitest";
import { setDataContext } from "../../app/data-context";
import { NOTEBOOK_READ_LIMIT, readMarkdownFile } from "./notebook-adapters";
const ref = { sourceId: "space", objectId: "file/a", revision: "version&1" };
afterEach(() => { vi.unstubAllGlobals(); setDataContext(""); });

it("pins the content request and sends the current scope without following redirects", async () => {
  setDataContext("tenant", "space");
  const fetcher = vi.fn().mockResolvedValue(new Response("# Real content"));
  vi.stubGlobal("fetch", fetcher);
  expect(await readMarkdownFile(ref, new AbortController().signal)).toBe("# Real content");
  expect(fetcher).toHaveBeenCalledWith("/api/v1/files/file%2Fa/content?version=version%261", expect.objectContaining({ redirect: "error", credentials: "same-origin", headers: { "X-DriveBridge-Tenant": "tenant", "X-DriveBridge-Space": "space" } }));
});

it("verifies personal-space membership through owner metadata", async () => {
  setDataContext("tenant");
  const fetcher = vi.fn()
    .mockResolvedValueOnce(Response.json({ data: { file: { space_id: "space", trashed: false } } }))
    .mockResolvedValueOnce(new Response("personal"));
  vi.stubGlobal("fetch", fetcher);
  expect(await readMarkdownFile(ref, new AbortController().signal)).toBe("personal");
  expect(fetcher).toHaveBeenCalledTimes(2);
  expect(fetcher.mock.calls[1][1].headers).toEqual({ "X-DriveBridge-Tenant": "tenant" });
});

it("refuses personal metadata from a different source without fetching bytes", async () => {
  setDataContext("tenant");
  const fetcher = vi.fn().mockResolvedValue(Response.json({ data: { file: { space_id: "other", trashed: false } } }));
  vi.stubGlobal("fetch", fetcher);
  await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ state: "forbidden" });
  expect(fetcher).toHaveBeenCalledOnce();
});

it("maps personal metadata denial without retrying or requesting bytes", async () => {
  setDataContext("tenant");
  for (const [status, state] of [[401, "forbidden"], [403, "forbidden"], [404, "stale"], [409, "stale"], [500, "error"]] as const) {
    const fetcher = vi.fn().mockResolvedValue(Response.json({ error: { code: "request_failed" } }, { status }));
    vi.stubGlobal("fetch", fetcher);
    await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ state });
    expect(fetcher).toHaveBeenCalledOnce();
  }
});

it("rejects source mismatches before requesting content", async () => {
  setDataContext("tenant", "other");
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ state: "unsupported" });
  expect(fetcher).not.toHaveBeenCalled();
});

it("rejects late responses after a context change even if fetch ignores abort", async () => {
  setDataContext("tenant", "space");
  vi.stubGlobal("fetch", vi.fn(async () => { setDataContext("other", "space"); return new Response("private"); }));
  await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ name: "AbortError" });
});

it("bounds streams independently of a missing or dishonest content length", async () => {
  setDataContext("tenant", "space");
  vi.stubGlobal("fetch", vi.fn(async () => new Response(new Uint8Array(NOTEBOOK_READ_LIMIT + 1), { headers: { "Content-Length": "1" } })));
  await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ state: "unsupported" });
});

it("distinguishes permission loss from unavailable fixed versions", async () => {
  setDataContext("tenant", "space");
  for (const [status, state] of [[403, "forbidden"], [409, "stale"]] as const) {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("", { status })));
    await expect(readMarkdownFile(ref, new AbortController().signal)).rejects.toMatchObject({ state });
  }
});
