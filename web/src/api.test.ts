import { describe, it, expect, vi } from "vitest";
import { api, setCSRF, bytes, errorText } from "./api";
describe("API boundary", () => {
  it("uses only the session CSRF token for mutations", async () => {
    setCSRF("fixture-csrf");
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } })));
    expect(await api("/agents", "POST", { name: "assistant" })).toEqual({
      ok: true,
    });
    const init = fetcher.mock.calls[0][1]!;
    expect(init.credentials).toBe("same-origin");
    expect((init.headers as Record<string, string>)["X-CSRF-Token"]).toBe(
      "fixture-csrf",
    );
    expect(
      (init.headers as Record<string, string>)["Authorization"],
    ).toBeUndefined();
  });
  it("does not display arbitrary provider error text", () => {
    expect(
      errorText(new Error("https://private.invalid/?secret=fixture")),
    ).not.toContain("https://");
    expect(errorText(new Error("quota_exceeded"))).toContain("容量");
    // UI errors use locale entries rather than arbitrary upstream messages.
    const serverError = new Error("user_key_duplicate") as Error & { serverMessage?: string };
    expect(errorText(serverError)).not.toContain("操作未完成");
    expect(errorText(new Error("unlabeled_new_code"))).toBe("操作未完成，请稍后重试。");
    expect(bytes(1024)).toBe("1.0 KB");
  });
  it("rejects failed mutations instead of treating them as success", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "forbidden" } }), {
        status: 403,
      }),
    );
    await expect(api("/agents", "POST", {})).rejects.toThrow("forbidden");
  });
});

it("copies the selected version rather than a concurrently updated version", async () => {
  const { copyReference } = await import("./api");
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  const fetcher = vi
    .spyOn(globalThis, "fetch")
    .mockReset()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          data: { ref: "drivebridge://instance/file/file/version/new" },
        }),
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          data: {
            ref: "drivebridge://instance/file/file/version/old",
            file: {},
          },
        }),
      ),
    );
  await copyReference({
    id: "file",
    version_id: "old",
    space_id: "space",
    path: "report.md",
    size_bytes: 10,
    mime_type: "text/plain",
    trashed: false,
    updated_at: "",
  });
  expect(String(fetcher.mock.calls[1][0])).toContain(
    encodeURIComponent("/version/old"),
  );
  expect(writeText).toHaveBeenCalledWith(
    "drivebridge://instance/file/file/version/old",
  );
});

it("explains a key-login origin mismatch without suggesting a futile refresh", async () => {
  const { keyLogin } = await import("./api");
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({error:{code:"csrf_rejected"}}), {status:403}));
  await expect(keyLogin("fixture-only-key")).rejects.toThrow("login_origin_mismatch");
  expect(errorText(new Error("login_origin_mismatch"))).toContain("访问地址");
});

it("localizes missing resources and suppresses untranslated upstream fallback",()=>{const e=Object.assign(new Error("not_found"),{serverMessage:"The referenced resource does not exist or is not visible to this identity."});expect(errorText(e)).toBe("资源不存在，或当前身份无权访问。");expect(errorText(Object.assign(new Error("unknown_code"),{serverMessage:"English provider error"}))).toBe("操作未完成，请稍后重试。");});

it("localizes Aigora unified-login governance codes with actionable guidance", () => {
  expect(errorText(new Error("unified_login_required"))).toContain("统一登录");
  expect(errorText(new Error("identity_mapping_required"))).toContain("身份映射");
  expect(errorText(new Error("identity_unavailable"))).toContain("身份服务");
  expect(errorText(new Error("service_admission_required"))).toContain("准入");
  expect(errorText(new Error("insufficient_funds"))).toContain("余额");
  expect(errorText(new Error("cross_tenant_admin_context_required"))).toContain("跨租户");
  expect(errorText(new Error("account_recovery_required"))).toContain("恢复");
});
