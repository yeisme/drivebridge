import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { DriveHome, connectionObservation } from "./DriveHome";

afterEach(() => { cleanup(); history.replaceState(null, "", "/"); });
it("reports observed configuration without inventing online status", () => {
  expect(connectionObservation()).toBe("连接状态未知");
  const connection = { id: "fixture", kind: "s3", name: "Fixture", disabled: false, status: "connected" };
  expect(connectionObservation(connection)).toBe("已配置，尚未验证");
  expect(connectionObservation({ ...connection, status: "verified" })).toBe("已验证");
  expect(connectionObservation({ ...connection, status: "verified", disabled: true })).toBe("已断开");
});
it("keeps the original drive navigable when connection observation is unavailable", () => {
  const space = { id: "space", name: "Fixture drive", connection_id: "missing", used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
  const open = vi.fn();
  render(<DriveHome spaces={[space]} connections={[]} open={open} />);
  fireEvent.click(screen.getByRole("button", { name: "卡片视图" }));
  expect(screen.getByText("连接状态未知")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "打开网盘 Fixture drive" }));
  expect(open).toHaveBeenCalledWith(space);
});
