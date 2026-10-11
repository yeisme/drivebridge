import { describe, expect, it } from "vitest";
import { backendOf, capabilityText } from "./storage";

describe("backendOf", () => {
  it("keeps OneDrive privileged capabilities", () => {
    const onedrive = backendOf("onedrive");
    expect(onedrive.directUrl).toBe(true);
    expect(onedrive.handoff).toBe(true);
    expect(capabilityText(onedrive)).toBe("传输 · 临时直链 · 参考交接");
  });

  it("treats absent backend as unknown instead of assuming OneDrive", () => {
    // 本机 view 已用 Space.Kind() 归一（""→onedrive）；客户端收到缺失/空
    // backend 时不猜测家族，按未识别诚实显示。
    expect(backendOf("").unrecognized).toBe(true);
    expect(backendOf(undefined).key).toBe("unknown");
  });

  it("marks generic rclone backends as transfer-only", () => {
    for (const kind of ["s3", "drive", "dropbox", "webdav", "sftp", "local"]) {
      const backend = backendOf(kind);
      expect(backend.directUrl).toBe(false);
      expect(backend.handoff).toBe(false);
      expect(capabilityText(backend)).toBe("仅传输");
    }
  });

  it("stays honest for unknown kinds instead of guessing", () => {
    const unknown = backendOf("some-new-nas");
    expect(unknown.unrecognized).toBe(true);
    expect(unknown.label).toContain("some-new-nas");
    expect(unknown.directUrl).toBe(false);
    expect(capabilityText(unknown)).toBe("能力未知");
  });
});
