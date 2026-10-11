import { afterEach, expect, it, vi } from "vitest";
import { api, csrfHeaders, setCSRF } from "./transport";
import { api as legacyAPI, setCSRF as legacySetCSRF } from "../api";
import { setDataContext } from "./data-context";

afterEach(() => { vi.restoreAllMocks(); setDataContext(""); setCSRF(""); });

it("shares transport and ephemeral CSRF with legacy callers", () => {
  expect(legacyAPI).toBe(api);
  legacySetCSRF("fixture-shared");
  expect(csrfHeaders()).toEqual({ "X-CSRF-Token": "fixture-shared" });
  const returned = csrfHeaders();
  returned["X-CSRF-Token"] = "not-the-session";
  expect(csrfHeaders()["X-CSRF-Token"]).toBe("fixture-shared");
});

it("does not send a request that was already cancelled", async () => {
  const fetcher = vi.spyOn(globalThis, "fetch");
  const controller = new AbortController(); controller.abort();
  await expect(api("/example", "GET", undefined, {}, controller.signal)).rejects.toMatchObject({ name: "AbortError" });
  expect(fetcher).not.toHaveBeenCalled();
});

it("keeps plain-text old-server errors opaque", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("private upstream detail", { status: 404 }));
  await expect(api("/missing")).rejects.toMatchObject({ message: "request_failed", serverMessage: undefined });
});

it("preserves request cancellation, custom headers and context for typed responses", async () => {
  setDataContext("tenant", "space");
  setCSRF("fixture-csrf");
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: { revision: 2 } })));
  const controller = new AbortController();
  expect(await api<{ revision: number }>("/example", "PATCH", { name: "value" }, { "Idempotency-Key": "original" }, controller.signal)).toEqual({ revision: 2 });
  expect(fetcher).toHaveBeenCalledWith("/api/v1/example", expect.objectContaining({
    credentials: "same-origin", signal: controller.signal, body: JSON.stringify({ name: "value" }),
    headers: { "Content-Type": "application/json", "X-CSRF-Token": "fixture-csrf", "X-DriveBridge-Tenant": "tenant", "X-DriveBridge-Space": "space", "Idempotency-Key": "original" },
  }));
});
