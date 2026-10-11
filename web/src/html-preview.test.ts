import { expect, it } from "vitest";
import { HTML_PREVIEW_LIMIT, interactivePreviewURL, readPreviewHTML } from "./html-preview";

it("only enables the version-pinned same-origin media contract", () => {
  const origin = "https://files.example.com";
  expect(interactivePreviewURL("/api/v1/files/id/media?version=v1", origin)).toBe(origin + "/api/v1/files/id/media?version=v1&preview=interactive");
  expect(interactivePreviewURL("https://other.example.com/api/v1/files/id/media?version=v1", origin)).toBeUndefined();
  expect(interactivePreviewURL("/api/v1/files/id/media", origin)).toBeUndefined();
  expect(interactivePreviewURL("/api/v1/local/file/preview?version=v1", origin)).toBeUndefined();
});
it("enforces the actual byte limit even without a trustworthy Content-Length", async () => {
  await expect(readPreviewHTML(new Response("a".repeat(HTML_PREVIEW_LIMIT + 1)))).rejects.toThrow("preview_too_large");
  await expect(readPreviewHTML(new Response("按钮与动画"))).resolves.toBe("按钮与动画");
  await expect(readPreviewHTML(new Response("no", { status: 403 }))).rejects.toThrow("preview_load_failed");
});
