import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PinaxSyncPanel } from "./PinaxSyncPanel";
import type { DesktopBridge } from "../../generated/desktop-ipc";

const projection = { spec_version: "1.0", mode: "json", command: "sync.status", status: "success", summary: "up-to-date", facts: { sync_status: "configured", last_synced_revision: "sha256:local", remote_revision: "sha256:remote", remote_write: "false" } } as const;
const bridge: Pick<DesktopBridge, "syncStatus" | "syncDiff" | "listSyncConflicts"> = {
  syncStatus: vi.fn().mockResolvedValue({ data: projection }),
  syncDiff: vi.fn().mockResolvedValue({ data: { ...projection, command: "sync.diff", summary: "no changes" } }),
  listSyncConflicts: vi.fn().mockResolvedValue({ data: { ...projection, command: "sync.conflicts.list" } }),
};

afterEach(() => { cleanup(); vi.clearAllMocks(); });

it("renders owner sync facts and keeps mutations disabled until host admission", async () => {
  render(<PinaxSyncPanel bridge={bridge} sourceId="source" />);
  expect(await screen.findByText("configured")).toBeTruthy();
  expect(screen.getByText("no changes")).toBeTruthy();
  expect(screen.getByText("sha256:local")).toBeTruthy();
  expect((screen.getByRole("button", { name: "确认 Push" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "确认 Pull" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/operation\.show \/ operation\.reconcile/)).toBeTruthy();
});

it("uses conflict-list facts and does not invent a zero for an unknown count", async () => {
  vi.mocked(bridge.listSyncConflicts).mockResolvedValueOnce({ data: { ...projection, command: "sync.conflicts.list", facts: { conflicts: "3" } } });
  const { rerender } = render(<PinaxSyncPanel bridge={bridge} sourceId="source" />);
  expect(await screen.findByText("3")).toBeTruthy();
  rerender(<PinaxSyncPanel bridge={bridge} sourceId="other" />);
  await waitFor(() => expect(screen.queryByText("3")).toBeNull());
  expect(screen.queryByText("0")).toBeNull();
});

it("clears old source facts immediately and ignores late responses", async () => {
  const { rerender } = render(<PinaxSyncPanel bridge={bridge} sourceId="source" />);
  await screen.findByText("sha256:local");
  let finish!: (value: never) => void;
  vi.mocked(bridge.syncStatus).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  rerender(<PinaxSyncPanel bridge={bridge} sourceId="slow" />);
  expect(screen.queryByText("sha256:local")).toBeNull();
  vi.mocked(bridge.syncStatus).mockResolvedValueOnce({ data: { ...projection, facts: { sync_status: "configured", last_synced_revision: "current-source" } } });
  rerender(<PinaxSyncPanel bridge={bridge} sourceId="current" />);
  await screen.findByText("current-source");
  await act(async () => { finish({ data: { ...projection, facts: { sync_status: "configured", last_synced_revision: "stale-source" } } } as never); });
  expect(screen.queryByText("stale-source")).toBeNull();
  expect(screen.getByText("current-source")).toBeTruthy();
});

it("does not refetch when a parent wraps the same bridge methods", async () => {
  const { rerender } = render(<PinaxSyncPanel bridge={{ ...bridge }} sourceId="source" />);
  await screen.findByText("sha256:local");
  rerender(<PinaxSyncPanel bridge={{ ...bridge }} sourceId="source" />);
  await act(async () => {});
  expect(bridge.syncStatus).toHaveBeenCalledTimes(1);
});

it.each(["failed", "partial"])("does not display a %s projection as a healthy sync", async status => {
  vi.mocked(bridge.syncStatus).mockResolvedValueOnce({ data: { ...projection, status, error: { code: "owner_unavailable", message: "Unavailable" } } });
  render(<PinaxSyncPanel bridge={bridge} sourceId="source" />);
  expect((await screen.findByRole("alert")).textContent).toContain("owner_unavailable");
  expect(screen.queryByText("sha256:local")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));
  await screen.findByText("sha256:local");
});
