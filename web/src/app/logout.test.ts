import { afterEach, expect, it, vi } from "vitest";
import { endSession } from "./logout";
import { csrfHeaders, setCSRF } from "./transport";
import { getDataContext, setDataContext } from "./data-context";

afterEach(() => { vi.restoreAllMocks(); setCSRF(""); setDataContext(""); });

it("clears local identity scope and CSRF only after confirmed logout", async () => {
  setCSRF("fixture-csrf"); setDataContext("tenant", "space");
  const before = getDataContext().revision;
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 204 }));
  await endSession();
  expect(fetcher).toHaveBeenCalledWith("/auth/logout", { method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": "fixture-csrf" } });
  expect(getDataContext()).toEqual({ tenant: "", space: "", revision: before + 1 });
  expect(csrfHeaders()["X-CSRF-Token"]).toBe("");
});

it.each([403, 500])("retains identity when logout fails with %s", async status => {
  setCSRF("fixture-csrf"); setDataContext("tenant", "space");
  const before = getDataContext();
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("private upstream detail", { status }));
  await expect(endSession()).rejects.toThrow("request_failed");
  expect(getDataContext()).toBe(before);
  expect(csrfHeaders()["X-CSRF-Token"]).toBe("fixture-csrf");
});

it("does not clear the session on an unknown network result", async () => {
  setCSRF("fixture-csrf"); setDataContext("tenant", "space");
  const before = getDataContext();
  vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("network failure"));
  await expect(endSession()).rejects.toThrow();
  expect(getDataContext()).toBe(before);
  expect(csrfHeaders()["X-CSRF-Token"]).toBe("fixture-csrf");
});
