import React from "react";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { afterEach, it, expect, vi } from "vitest";
import { DriveHome, filterSpaces } from "./Workspace";
import type { Space } from "./api";
afterEach(cleanup);
it("opens the exact team and storage scope selected at the first level", async () => {
  const personal = {
    id: "personal",
    name: "个人素材",
    connection_id: "s3",
    used_bytes: 0,
    reserved_bytes: 0,
    max_bytes: 100,
  } as Space;
  const team = {
    ...personal,
    id: "team-space",
    name: "团队拍摄",
    team_id: "team",
    connection_id: "onedrive",
  };
  const open = vi.fn();
  render(
    <DriveHome
      spaces={[personal, team]}
      connections={[
        {
          id: "s3",
          name: "S3",
          kind: "s3",
          status: "connected",
          disabled: false,
        },
        {
          id: "onedrive",
          name: "OneDrive",
          kind: "onedrive",
          status: "connected",
          disabled: false,
        },
      ]}
      open={open}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "团队网盘" }));
  expect(
    screen.queryByRole("button", { name: "打开网盘 个人素材" }),
  ).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "打开网盘 团队拍摄" }));
  expect(open).toHaveBeenCalledWith(team);
  // The Base UI select choreography cannot settle under jsdom; the popup
  // interaction itself is verified in a real browser by tests/shadcn-controls.spec.ts.
  expect(
    screen.getByRole("combobox", { name: "筛选存储" }),
  ).toBeTruthy();
  expect(
    filterSpaces([personal, team], { group: "all", storage: "s3", query: "" }),
  ).toEqual([personal]);
  expect(
    filterSpaces([personal, team], { group: "all", storage: "", query: "团队" }),
  ).toEqual([team]);
  expect(
    filterSpaces([personal, team], {
      group: "all",
      storage: "",
      query: "",
      teams: [{ id: "team", name: "后期组", tenant_id: "tenant" }],
    }),
  ).toEqual([personal, team]);
});

it("does not request unavailable team APIs from a personal-only server", async () => {
  const { TeamWorkspace } = await import("./Teams");
  const spy = vi.spyOn(globalThis, "fetch");
  render(
    <TeamWorkspace
      enabled={false}
      user={{ id: "user", name: "User" }}
      openDrive={() => {}}
      openKeys={() => {}}
    />,
  );
  expect(
    screen.getByRole("heading", { name: "团队服务尚未启用" }),
  ).toBeTruthy();
  expect(spy).not.toHaveBeenCalled();
  spy.mockRestore();
});
