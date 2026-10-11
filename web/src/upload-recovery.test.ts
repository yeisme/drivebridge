import { afterEach, describe, expect, it, vi } from "vitest";
import { upload, setDataContext, type Operation } from "./api";

const empty = () => new File([], "empty.txt", { type: "text/plain" });
const original: Operation = {
  id: "original-operation", kind: "upload", space_id: "space", path: "empty.txt",
  size_bytes: 0, sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  status: "awaiting_upload", bytes_transferred: 0,
};
const response = (data: unknown) => new Response(JSON.stringify({ data }));
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); setDataContext(""); });

describe("upload recovery boundary before shell migration", () => {
  it("creates a safe upload identity on HTTP without randomUUID", async () => {
    const random = vi.fn((bytes: Uint8Array) => { bytes.fill(42); return bytes; });
    vi.stubGlobal('crypto', { getRandomValues: random });
    const fetcher = vi.spyOn(globalThis, 'fetch').mockResolvedValue(response({ ...original, status: 'completed' }));
    await upload(empty(), 'space', 'empty.txt', vi.fn(), new AbortController().signal);
    expect(random).toHaveBeenCalledOnce();
    const headers = new Headers(fetcher.mock.calls[0][1]?.headers);
    expect(headers.get('Idempotency-Key')).toMatch(/^upload-[a-f0-9]{48}$/);
    expect(fetcher).toHaveBeenCalledOnce();
  });
  it("refuses to upload when secure randomness is unavailable", async () => {
    vi.stubGlobal('crypto', {});
    const fetcher = vi.spyOn(globalThis, 'fetch');
    await expect(upload(empty(), 'space', 'empty.txt', vi.fn(), new AbortController().signal)).rejects.toThrow();
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("reuses the original operation and only reports completed after server completion", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(response({ received_parts: [] }))
      .mockResolvedValueOnce(response({ ...original, status: "completed", version_id: "fixed-version" }));
    const progress = vi.fn();
    const result = await upload(empty(), "space", "empty.txt", progress, new AbortController().signal, original);
    expect(fetcher.mock.calls.map(([url]) => url)).toEqual([
      "/api/v1/operations/original-operation", "/api/v1/operations/original-operation/complete",
    ]);
    expect(progress.mock.calls.map(([op]) => op.status)).toEqual(["awaiting_upload", "verifying", "completed"]);
    expect(result.version_id).toBe("fixed-version");
  });
  it("rejects mismatched source content before any network action", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch");
    await expect(upload(empty(), "space", "empty.txt", vi.fn(), new AbortController().signal,
      { ...original, sha256: "wrong-digest" })).rejects.toThrow("content_mismatch");
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("does not complete the old operation after a context switch during recovery", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      setDataContext("another-tenant", "another-space");
      return response({ received_parts: [] });
    });
    await expect(upload(empty(), "space", "empty.txt", vi.fn(), new AbortController().signal, original))
      .rejects.toMatchObject({ name: "AbortError" });
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("keeps failed publication out of completed progress", async () => {
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(response({ received_parts: [] }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "forbidden" } }), { status: 403 }));
    const progress = vi.fn();
    await expect(upload(empty(), "space", "empty.txt", progress, new AbortController().signal, original))
      .rejects.toThrow("forbidden");
    expect(progress.mock.calls.map(([op]) => op.status)).toEqual(["awaiting_upload", "verifying"]);
  });
  it("does not publish completed progress for a response arriving after cancellation", async () => {
    const controller = new AbortController();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(response({ received_parts: [] }))
      .mockImplementationOnce(async () => {
        controller.abort();
        return response({ ...original, status: "completed" });
      });
    const progress = vi.fn();
    await expect(upload(empty(), "space", "empty.txt", progress, controller.signal, original))
      .rejects.toMatchObject({ name: "AbortError" });
    expect(progress.mock.calls.map(([op]) => op.status)).toEqual(["awaiting_upload", "verifying"]);
  });
  it("does not restart a completed operation or an aborted upload", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch");
    const completed = { ...original, status: "completed" };
    expect(await upload(empty(), "space", "empty.txt", vi.fn(), new AbortController().signal, completed)).toBe(completed);
    const controller = new AbortController(); controller.abort();
    await expect(upload(empty(), "space", "empty.txt", vi.fn(), controller.signal, original))
      .rejects.toMatchObject({ name: "AbortError" });
    expect(fetcher).not.toHaveBeenCalled();
  });
});
