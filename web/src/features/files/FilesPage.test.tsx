import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { ComponentProps } from "react";
import { FilesPage } from "./FilesPage";

const finderSpy = vi.hoisted(() => vi.fn());
vi.mock("../../Finder", () => ({ Finder: (props: unknown) => { finderSpy(props); return <div>fixture finder</div>; } }));
vi.mock("./DriveHome", () => ({ DriveHome: () => <div>fixture home</div> }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });
const space = { id: "space-a", name: "Fixture", connection_id: "platform", used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
const props: ComponentProps<typeof FilesPage> = {
  open: false, focused: false, routeHash: "#/drive/space-a", spaces: [space], connections: [], governance: undefined,
  openDrive: vi.fn(), home: vi.fn(), finder: { space: space.id, prefix: "", setPrefix: vi.fn(), revision: [], reload: async () => {}, backendLabel: "fixture", upload: vi.fn(), onPreview: vi.fn(), onShare: vi.fn() },
};
it("never mounts Finder for the root list or inaccessible drive", () => {
  const view = render(<FilesPage {...props} />);
  expect(screen.getByText("fixture home")).toBeTruthy();
  view.rerender(<FilesPage {...props} open />);
  expect(screen.getByRole("status").textContent).toContain("可访问范围");
  fireEvent.click(screen.getByRole("button", { name: "返回我的网盘" }));
  expect(props.home).toHaveBeenCalledTimes(1);
  expect(finderSpy).not.toHaveBeenCalled();
});
it("preserves callbacks and limits focus mode to the selected accessible drive", () => {
  render(<FilesPage {...props} open focused currentSpace={space} connections={[{ id: "disabled", name: "Disabled", kind: "onedrive", disabled: true, status: "" }]} />);
  const forwarded = finderSpy.mock.calls.at(-1)![0];
  expect(forwarded.spaces).toEqual([space]);
  expect(forwarded.onedriveConnections).toEqual([]);
  expect(forwarded.upload).toBe(props.finder.upload);
  expect(forwarded.onPreview).toBe(props.finder.onPreview);
  forwarded.setSpace("foreign");
  expect(props.openDrive).not.toHaveBeenCalled();
  forwarded.setSpace(space.id);
  expect(props.openDrive).toHaveBeenCalledWith(space);
});
