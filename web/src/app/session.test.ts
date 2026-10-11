import { afterEach, expect, it, vi } from "vitest";
import { loadSession, verifySessionIdentity } from "./session";
import { csrfHeaders } from "./transport";
import { api, setCSRF, setDataContext } from "../api";

const session = { user: { id: "human", name: "Fixture" }, csrf_token: "fixture-next", admin: false };
afterEach(() => { vi.restoreAllMocks(); setDataContext(""); setCSRF(""); });

it.each(["same", "different-user", "different-instance"])("checks %s without replacing CSRF", async kind => {
  setCSRF("fixture-original");
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: {
    ...session, user: { ...session.user, id: kind === "different-user" ? "other" : "human" },
    instance_id: kind === "different-instance" ? "other-instance" : "instance-a",
  } })));
  const result = verifySessionIdentity("human", "instance-a", new AbortController().signal);
  if (kind === "same") await expect(result).resolves.toBeUndefined();
  else await expect(result).rejects.toMatchObject({ status: 401, message: "unauthorized" });
  expect(csrfHeaders()["X-CSRF-Token"]).toBe("fixture-original");
});

it("loads the session without space scope and installs CSRF for later mutations", async () => {
  setDataContext("tenant", "space");
  const fetcher = vi.spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(new Response(JSON.stringify({ data: session })))
    .mockResolvedValueOnce(new Response(JSON.stringify({ data: {} })));
  const controller = new AbortController();
  expect(await loadSession(controller.signal)).toEqual(session);
  const init = fetcher.mock.calls[0][1]!;
  expect(init.signal).toBe(controller.signal);
  expect(init.headers).toEqual({ "X-DriveBridge-Tenant": "tenant" });
  await api("/agents", "POST", {});
  expect(fetcher.mock.calls[1][1]?.headers).toMatchObject({ "X-CSRF-Token": "fixture-next" });
});

it("does not install CSRF when a transport resolves after cancellation", async () => {
  setCSRF("fixture-original");
  const controller = new AbortController();
  const fetcher = vi.spyOn(globalThis, "fetch").mockImplementationOnce(async () => {
    controller.abort();
    return new Response(JSON.stringify({ data: session }));
  }).mockResolvedValue(new Response(JSON.stringify({ data: {} })));
  await expect(loadSession(controller.signal)).rejects.toMatchObject({ name: "AbortError" });
  await api("/agents", "POST", {});
  expect(fetcher.mock.calls[1][1]?.headers).toMatchObject({ "X-CSRF-Token": "fixture-original" });
});

it.each(["unauthorized", "context-change"])("does not install CSRF after %s", async (failure) => {
  setCSRF("fixture-original");
  const fetcher = vi.spyOn(globalThis, "fetch").mockImplementationOnce(async () => {
    if (failure === "context-change") {
      setDataContext("another");
      return new Response(JSON.stringify({ data: session }));
    }
    return new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 });
  }).mockResolvedValue(new Response(JSON.stringify({ data: {} })));
  await expect(loadSession()).rejects.toMatchObject(failure === "context-change" ? { name: "AbortError" } : { message: "unauthorized" });
  await api("/agents", "POST", {});
  expect(fetcher.mock.calls[1][1]?.headers).toMatchObject({ "X-CSRF-Token": "fixture-original" });
});
