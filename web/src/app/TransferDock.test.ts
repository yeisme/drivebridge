import { describe, expect, it } from "vitest";
import { transferStatusMeta } from "./TransferDock";

describe("transferStatusMeta", () => {
  it("never reports completed while the server is still verifying durability", () => {
    const verifying = transferStatusMeta({
      status: "verifying",
      size_bytes: 1000,
      bytes_transferred: 1000,
    });
    expect(verifying.label).toBe("云端校验保存中…");
    expect(verifying.tone).toBe("verifying");
    expect(verifying.percent).toBeLessThan(100);

    const completed = transferStatusMeta({
      status: "completed",
      size_bytes: 1000,
      bytes_transferred: 1000,
    });
    expect(completed).toEqual({ label: "已完成", tone: "done", percent: 100 });
  });

  it("caps in-flight byte progress below 100% and surfaces resumable failures", () => {
    expect(
      transferStatusMeta({ status: "uploading", size_bytes: 100, bytes_transferred: 100 }).percent,
    ).toBe(95);
    expect(
      transferStatusMeta({ status: "failed", size_bytes: 100, bytes_transferred: 40 }),
    ).toEqual({ label: "中断，可续传", tone: "failed", percent: null });
  });
});
