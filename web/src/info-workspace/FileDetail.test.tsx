import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FileDetailSheet } from "./FileDetailSheet";
import { detailPreferenceKey } from "../app/detail-preference";

const envelope = (data: unknown) =>
  new Response(JSON.stringify({ data }), { status: 200, headers: { "Content-Type": "application/json" } });
const failure = (code: string, status = 400) =>
  new Response(JSON.stringify({ error: { code } }), { status, headers: { "Content-Type": "application/json" } });

const fileFixture = {
  id: "f-1",
  space_id: "sp-a",
  path: "/交付/镜头-1.png",
  version_id: "v-new",
  size_bytes: 8192,
  mime_type: "image/png",
  trashed: false,
  updated_at: "2026-09-25T00:00:00Z",
  ref: "drivebridge://instance/file/f-1/version/v-new",
};
const versionFixtures = [
  { id: "v-old", sha256: "o".repeat(64), size_bytes: 4096, mime_type: "image/png", created_at: "2026-09-20T00:00:00Z" },
  { id: "v-new", sha256: "n".repeat(64), size_bytes: 8192, mime_type: "image/png", created_at: "2026-09-25T00:00:00Z" },
];
const operationFixture = {
  id: "op-1",
  kind: "upload",
  space_id: "sp-a",
  path: "/交付/镜头-1.png",
  size_bytes: 4096,
  sha256: "o".repeat(64),
  status: "completed",
  bytes_transferred: 4096,
  file_id: "f-1",
  version_id: "v-old",
  caller: "drivebridge-client",
  task_ref: "task-77",
  source_id: "external-1",
  source_version: "ext-v3",
  source_space_id: "sp-source",
  source_path: "/incoming/asset.png",
  updated_at: "2026-09-20T00:05:00Z",
};

type Handler = (url: string) => Response;

function mockFetch(handler: Handler) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(((input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    return Promise.resolve(handler(url));
  }) as typeof fetch);
}

function defaultHandler(overrides?: { references?: Response; threads?: Response; bindings?: Response; operations?: Response }): Handler {
  return (url) => {
    if (overrides?.operations && url.includes("/api/v1/operations")) return overrides.operations;
    if (url.includes("/api/v1/operations/op-1"))
      // operations.get nests the payload under `data.operation`.
      return envelope({ operation: operationFixture });
    if (url.includes("/api/v1/operations")) return envelope([operationFixture]);
    if (overrides?.references && url.includes("/api/v1/references")) return overrides.references;
    if (url.includes("/api/v1/references"))
      return envelope({
        file: { ...fileFixture, size_bytes: 4096, ref: "drivebridge://instance/file/f-1/version/v-old" },
        ref: "drivebridge://instance/file/f-1/version/v-old",
        sha256: "o".repeat(64),
      });
    if (url.includes("/files/f-1/versions")) return envelope(versionFixtures);
    if (overrides?.threads && url.includes("/files/f-1/threads")) return overrides.threads;
    if (url.includes("/files/f-1/threads"))
      return envelope([
        {
          thread_id: "t-1",
          file_id: "f-1",
          version_id: "v-old",
          revision: 1,
          state: "open",
          author_kind: "user",
        },
      ]);
    if (url.includes("/api/v1/files/f-1"))
      // files.stat nests the file under `data.file` next to the top-level ref.
      return envelope({ file: fileFixture, ref: fileFixture.ref });
    if (overrides?.bindings && url.includes("/api/v1/content-bindings")) return overrides.bindings;
    if (url.includes("/api/v1/content-bindings")) return envelope([]);
    if (url.includes("/api/v1/audit/events"))
      return envelope({
        events: [
          { id: 1, at: "2026-09-20T00:05:00Z", actor_kind: "user", action: "uploads.create", outcome: "success", correlation_id: "c-1" },
        ],
        next_cursor: 0,
      });
    return failure("not_found", 404);
  };
}

function renderSheet(versionId?: string) {
  return render(
    <FileDetailSheet fileId="f-1" versionId={versionId} open onClose={() => {}} />,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  sessionStorage.clear();
});

describe("TestFPInformationWorkspace", () => {
  it("remounts width state across user and instance changes without copying preferences", async () => {
    mockFetch(defaultHandler());
    const alice = { instance: "fixture-width-instance", user: "alice" };
    const bob = { instance: alice.instance, user: "bob" };
    const elsewhere = { instance: "fixture-other-instance", user: "alice" };
    const keys = [alice, bob, elsewhere].map(identity => detailPreferenceKey(identity)!);
    for (const key of keys) localStorage.removeItem(key);
    try {
      localStorage.setItem(keys[0], "440");
      localStorage.setItem(keys[1], "520");
      const onClose = vi.fn();
      const view = render(<FileDetailSheet fileId="f-1" open onClose={onClose} preferenceIdentity={alice} />);
      const width = () => screen.getByRole("separator", { name: "调整详情宽度" });
      await waitFor(() => expect(width().getAttribute("aria-valuenow")).toBe("440"));
      fireEvent.keyDown(width(), { key: "ArrowLeft" });
      expect(localStorage.getItem(keys[0])).toBe("456");
      view.rerender(<FileDetailSheet fileId="f-1" open onClose={onClose} preferenceIdentity={bob} />);
      expect(width().getAttribute("aria-valuenow")).toBe("520");
      view.rerender(<FileDetailSheet fileId="f-1" open onClose={onClose} preferenceIdentity={elsewhere} />);
      expect(width().getAttribute("aria-valuenow")).toBe("360");
      view.rerender(<FileDetailSheet fileId="f-1" open onClose={onClose} />);
      fireEvent.keyDown(width(), { key: "End" });
      expect(localStorage.getItem(keys[2])).toBeNull();
      view.rerender(<FileDetailSheet fileId="f-1" open onClose={onClose} preferenceIdentity={alice} />);
      expect(width().getAttribute("aria-valuenow")).toBe("456");
      expect(localStorage.getItem(keys[1])).toBe("520");
    } finally {
      cleanup();
      for (const key of keys) localStorage.removeItem(key);
    }
  });
  it("offers the six detail tabs with the file overview first", async () => {
    mockFetch(defaultHandler());
    renderSheet();
    for (const label of ["概览", "版本", "来源与引用", "内容理解", "讨论", "活动"]) {
      expect(screen.getByRole("tab", { name: label })).toBeTruthy();
    }
    expect(await screen.findByText("/交付/镜头-1.png")).toBeTruthy();
    // Technical identifiers stay collapsed but copyable.
    expect(screen.getByText("技术标识（可复制）")).toBeTruthy();
    expect(screen.getByRole("button", { name: "复制文件引用" })).toBeTruthy();
  });

  it("pins the selected old version even though a newer version exists", async () => {
    const fetcher = mockFetch(defaultHandler());
    renderSheet("v-old");
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    const facts = (await screen.findAllByText(/所选版本/))[0].closest("dl")!;
    // The pinned resolution targets the selected version, not the newest.
    const referenceCall = fetcher.mock.calls
      .map(([input]) => (typeof input === "string" ? input : ""))
      .find((url) => url.includes("/api/v1/references"))!;
    expect(referenceCall).toContain(encodeURIComponent("drivebridge://instance/file/f-1/version/v-old"));
    expect(facts.textContent).toContain("v-old");
    expect(facts.textContent).toContain("o".repeat(64));
    expect(facts.textContent).not.toContain("n".repeat(64));
  });

  it("shows an unreadable version with recovery instead of substituting the newest", async () => {
    const fetcher = mockFetch(defaultHandler({ references: failure("not_found", 404) }));
    renderSheet("v-old");
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    const recovery = (await screen.findAllByRole("alert")).find((node) =>
      node.textContent?.includes("所选版本当前不可读"),
    )!;
    expect(recovery.textContent).toContain("所选版本当前不可读");
    expect(recovery.textContent).toContain("不会自动替换为最新版本");
    // No new-version facts were substituted while the old ref was pinned.
    expect(screen.queryByText(/n{64}/)).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "改选当前版本" }));
    await waitFor(() => {
      const referenceCalls = fetcher.mock.calls
        .map(([input]) => (typeof input === "string" ? input : ""))
        .filter((url) => url.includes("/api/v1/references"));
      expect(referenceCalls.at(-1)).toContain(encodeURIComponent("drivebridge://instance/file/f-1/version/v-new"));
    });
  });

  it("reads back the original operation by its original ID", async () => {
    const fetcher = mockFetch(defaultHandler());
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "来源与引用" }));
    const source = await screen.findByRole("region", { name: "来源与引用" });
    expect(source.textContent).toContain("drivebridge-client");
    expect(source.textContent).toContain("external-1");
    expect(source.textContent).toContain("/incoming/asset.png");
    await userEvent.click(screen.getByRole("button", { name: "回读原操作" }));
    const readback = await screen.findByRole("region", { name: /原操作回读/ });
    expect(readback.textContent).toContain("op-1");
    expect(readback.textContent).toContain("已完成");
    const readbackCall = fetcher.mock.calls
      .map(([input]) => (typeof input === "string" ? input : ""))
      .find((url) => url.endsWith("/api/v1/operations/op-1"));
    expect(readbackCall).toBeTruthy();
  });

  it("reports unsupported content bindings without affecting the file itself", async () => {
    mockFetch(defaultHandler({ bindings: failure("unsupported", 400) }));
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "内容理解" }));
    const section = await screen.findByRole("region", { name: "内容理解" });
    await waitFor(() => expect(section.textContent).toContain("此部署未启用对应能力"));
    expect(section.textContent).toContain("处理不可用不影响文件原件读取");
  });

  it("keeps the discussion draft local when threads are unsupported", async () => {
    mockFetch(defaultHandler({ threads: failure("unsupported", 400) }));
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "讨论" }));
    const section = await screen.findByRole("region", { name: "讨论" });
    await waitFor(() => expect(section.textContent).toContain("此部署未启用对应能力"));
    const box = screen.getByLabelText(/新讨论草稿/);
    await userEvent.type(box, "请确认这个版本的标题。");
    const stored = JSON.parse(sessionStorage.getItem("drivebridge:info-workspace:draft") || "{}");
    expect(stored.discussion).toBe("请确认这个版本的标题。");
    expect(Object.keys(stored).join(",")).not.toContain("token");
  });

  it("moves between detail tabs with arrow keys", async () => {
    mockFetch(defaultHandler());
    renderSheet();
    const tabs = screen.getAllByRole("tab");
    tabs[0].focus();
    expect(document.activeElement).toBe(tabs[0]);
    fireEvent.keyDown(tabs[0], { key: "ArrowRight" });
    expect(tabs[1].getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(tabs[1]);
    fireEvent.keyDown(tabs[1], { key: "ArrowLeft" });
    expect(tabs[0].getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(tabs[0]);
  });

  it("lists file activity from the audit surface", async () => {
    mockFetch(defaultHandler());
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "活动" }));
    const section = await screen.findByRole("region", { name: "活动" });
    await waitFor(() => expect(section.textContent).toContain("创建上传"));
  });

  it("renders the fetched threads on the discussion tab instead of an empty ready list", async () => {
    // Regression: the threads fetch resolved but only the section state was
    // stored, so a ready discussion tab always showed zero threads.
    mockFetch(defaultHandler());
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "讨论" }));
    const section = await screen.findByRole("region", { name: "讨论" });
    await waitFor(() => {
      expect(section.textContent).toContain("进行中");
      expect(section.textContent).toContain("锚定版本 v-old");
      expect(section.textContent).toContain("第 1 次修订");
    });
  });


  it("renders real content bindings from the implemented binding_id/file_ref/state shape", async () => {
    // Drift D6: bindings arrive as ContentBindingView; the old
    // file_id/version_id/status normalizer dropped every one of them.
    const fetcher = mockFetch(
      defaultHandler({
        bindings: envelope([
          {
            binding_id: "bind-1",
            file_ref: "drivebridge://instance/file/f-1/version/v-old",
            project_ref: "project-a",
            purpose: "indexing",
            state: "searchable",
            policy_revision: 4,
          },
        ]),
      }),
    );
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "内容理解" }));
    const section = await screen.findByRole("region", { name: "内容理解" });
    await waitFor(() => expect(section.textContent).toContain("可检索"));
    expect(section.textContent).toContain("project-a");
    expect(section.textContent).toContain("drivebridge://instance/file/f-1/version/v-old");
    // Bindings are listed per space and narrowed here by the file's ref:
    // the request must carry space_id, not the pre-integration file_id param.
    const bindingCall = fetcher.mock.calls
      .map(([input]) => (typeof input === "string" ? input : ""))
      .find((url) => url.includes("/api/v1/content-bindings"))!;
    expect(bindingCall).toContain("space_id=sp-a");
  });

  it("lists file-level threads as 文件级 instead of dropping them", async () => {
    // Drift D11: file-level anchors answer version_id:"" and used to be
    // silently dropped, leaving the 文件级 branch dead.
    mockFetch(
      defaultHandler({
        threads: envelope({
          file_id: "f-1",
          threads: [
            { thread_id: "t-file", file_id: "f-1", version_id: "", revision: 1, state: "open", author_kind: "user" },
            { thread_id: "t-ver", file_id: "f-1", version_id: "v-old", revision: 2, state: "resolved", author_kind: "user" },
          ],
          next_cursor: "",
        }),
      }),
    );
    renderSheet();
    await userEvent.click(screen.getByRole("tab", { name: "讨论" }));
    const section = await screen.findByRole("region", { name: "讨论" });
    await waitFor(() => {
      expect(section.textContent).toContain("锚定 文件级");
      expect(section.textContent).toContain("锚定版本 v-old");
    });
  });
});
