import { afterEach, expect, it, vi } from "vitest";
import { readOperationProjection } from "./operation-projection";

afterEach(() => vi.restoreAllMocks());
const response = (data: unknown) => new Response(JSON.stringify({ data }), { status: 200 });
it("preserves the longer Retry-After from either failed sibling", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(input => Promise.resolve(new Response("unavailable", {
    status: 503, headers: { "Retry-After": String(input).endsWith("/inbox") ? "60" : "2" },
  })));
  await expect(readOperationProjection(new AbortController().signal)).rejects.toMatchObject({ status: 503, retryAfterMs: 60000 });
});
it.each([401, 403])("prioritizes plain-text HTTP %s over a transient sibling failure", async status => {
  vi.spyOn(globalThis, "fetch").mockImplementation(input => Promise.resolve(
    new Response("unavailable", { status: String(input).endsWith("/inbox") ? status : 503 }),
  ));
  await expect(readOperationProjection(new AbortController().signal)).rejects.toMatchObject({ status, message: "request_failed" });
});
it.each(["/operations", "/inbox"])("waits for the sibling when %s fails early", async failing => {
  let finish!: (response: Response) => void;
  const sibling = new Promise<Response>(resolve => { finish = resolve; });
  vi.spyOn(globalThis, "fetch").mockImplementation(input => String(input).endsWith(failing)
    ? Promise.resolve(new Response(JSON.stringify({ error: { code: "unavailable" } }), { status: 503 })) : sibling);
  let settled = false;
  const result = readOperationProjection(new AbortController().signal).catch(error => { settled = true; return error; });
  await new Promise(resolve => setTimeout(resolve, 0));
  expect(settled).toBe(false);
  finish(response([]));
  expect((await result).message).toBe("unavailable");
});
it("returns the ordered pair only after both successful reads", async () => {
  const fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(input => Promise.resolve(response([{ id: String(input).endsWith("/inbox") ? "inbox" : "operation" }])));
  const controller = new AbortController();
  const result = await readOperationProjection(controller.signal);
  expect(result.map(rows => rows[0].id)).toEqual(["operation", "inbox"]);
  expect(fetcher.mock.calls.every(([, init]) => init?.signal === controller.signal && init.method === "GET")).toBe(true);
});
it("rejects a late pair after cancellation even if the transport ignores abort", async () => {
  let finish!: (response: Response) => void;
  vi.spyOn(globalThis, "fetch").mockImplementation(input => String(input).endsWith("/inbox")
    ? new Promise(resolve => { finish = resolve; }) : Promise.resolve(response([])));
  const controller = new AbortController();
  const result = readOperationProjection(controller.signal);
  controller.abort();
  const rejected = expect(result).rejects.toMatchObject({ name: "AbortError" });
  finish(response([]));
  await rejected;
});
