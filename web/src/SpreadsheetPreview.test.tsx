import "@testing-library/jest-dom/vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SpreadsheetPreview from "./SpreadsheetPreview";
import { parseSpreadsheet } from "./spreadsheet-preview";
import {
  SPREADSHEET_LIMIT,
  type SpreadsheetOptions,
} from "./spreadsheet-source";

class PreviewWorker {
  static instances: PreviewWorker[] = [];
  onmessage?: (event: { data: unknown }) => void;
  onerror?: () => void;
  terminate = vi.fn();
  constructor() {
    PreviewWorker.instances.push(this);
  }
  postMessage({
    buffer,
    options,
  }: {
    buffer: ArrayBuffer;
    options: SpreadsheetOptions;
  }) {
    Promise.resolve().then(() => {
      try {
        this.onmessage?.({ data: { data: parseSpreadsheet(buffer, options) } });
      } catch (error) {
        this.onmessage?.({ data: { error: (error as Error).message } });
      }
    });
  }
}
const props = {
  url: "/api/v1/files/f/media?version=v1",
  name: "表.csv",
  size: 20,
  format: "csv" as const,
};

beforeEach(() => {
  PreviewWorker.instances = [];
  vi.stubGlobal("Worker", PreviewWorker);
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response('编号,内容\n001,"<img src=x onerror=alert(1)>"'),
      ),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("SpreadsheetPreview", () => {
  it("fetches with same-origin credentials and renders inert text with accessible row/column labels", async () => {
    render(<SpreadsheetPreview {...props} />);
    expect(screen.getByRole("status")).toHaveTextContent("正在读取表格");
    expect(await screen.findByRole("table")).toBeVisible();
    expect(screen.getByRole("cell", { name: "001" })).toBeVisible();
    expect(
      screen.getByRole("cell", { name: "<img src=x onerror=alert(1)>" }),
    ).toBeVisible();
    expect(document.querySelector(".spreadsheet-grid img")).toBeNull();
    expect(screen.getByRole("columnheader", { name: "A" })).toBeVisible();
    expect(screen.getByRole("rowheader", { name: "2" })).toBeVisible();
    expect(screen.getByRole("region")).toHaveAttribute("tabindex", "0");
    expect(fetch).toHaveBeenCalledWith(
      props.url,
      expect.objectContaining({
        credentials: "same-origin",
        signal: expect.any(AbortSignal),
      }),
    );
  });
  it("pages through bounded rows without refetching", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        Array.from({ length: 105 }, (_, i) => `row-${i + 1},data`).join("\n"),
      ),
    );
    render(<SpreadsheetPreview {...props} />);
    expect(await screen.findByRole("cell", { name: "row-100" })).toBeVisible();
    expect(screen.queryByRole("cell", { name: "row-101" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "下一页表格" }));
    expect(screen.getByRole("cell", { name: "row-101" })).toBeVisible();
    expect(screen.getByRole("rowheader", { name: "101" })).toBeVisible();
    expect(screen.getByRole("button", { name: "下一页表格" })).toBeDisabled();
    expect(fetch).toHaveBeenCalledTimes(1);
  });
  it("recovers from a permission failure using explicit reload", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response("", { status: 403 }))
      .mockResolvedValueOnce(new Response("a,b\n1,2"));
    render(<SpreadsheetPreview {...props} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("没有预览权限");
    fireEvent.click(screen.getByRole("button", { name: "重新加载" }));
    expect(await screen.findByRole("table")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(2);
  });
  it("shows empty and oversized files without misleading table output", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(""));
    const view = render(<SpreadsheetPreview {...props} />);
    expect(await screen.findByText("此工作表为空。")).toBeVisible();
    view.unmount();
    vi.mocked(fetch).mockClear();
    render(<SpreadsheetPreview {...props} size={SPREADSHEET_LIMIT + 1} />);
    expect(screen.getByText("表格超过 16 MiB，请下载查看。")).toBeVisible();
    expect(fetch).not.toHaveBeenCalled();
  });
  it("terminates parsing on unmount and aborts a pending fetch", async () => {
    const view = render(<SpreadsheetPreview {...props} />);
    await screen.findByRole("table");
    const signal = vi.mocked(fetch).mock.calls[0][1]!.signal!;
    view.unmount();
    expect(signal.aborted).toBe(true);
    expect(PreviewWorker.instances[0].terminate).toHaveBeenCalled();
  });
  it("ignores stale fetch results after the source changes", async () => {
    let resolve!: (response: Response) => void;
    vi.mocked(fetch)
      .mockImplementationOnce(
        () =>
          new Promise((value) => {
            resolve = value;
          }),
      )
      .mockResolvedValueOnce(new Response("new,value"));
    const view = render(<SpreadsheetPreview {...props} />);
    view.rerender(<SpreadsheetPreview {...props} url="/new" />);
    expect(await screen.findByRole("cell", { name: "new" })).toBeVisible();
    await act(async () => {
      resolve(new Response("stale,value"));
    });
    expect(screen.queryByRole("cell", { name: "stale" })).toBeNull();
  });
  it("times out a stalled parser and terminates its worker", async () => {
    vi.spyOn(PreviewWorker.prototype, "postMessage").mockImplementation(
      () => {},
    );
    render(<SpreadsheetPreview {...props} />);
    await waitFor(() => expect(PreviewWorker.instances).toHaveLength(1));
    vi.useFakeTimers();
    // Re-render a new source so its 30-second deadline uses the fake clock.
    cleanup();
    render(<SpreadsheetPreview {...props} url="/timeout" />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(screen.getByRole("alert")).toHaveTextContent("读取超时");
    expect(PreviewWorker.instances.at(-1)?.terminate).toHaveBeenCalled();
  });
});
