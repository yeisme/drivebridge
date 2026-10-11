import { expect, it } from "vitest";
import { filterShares, shareStatus, type ShareRow } from "./Sharing";

const rows: ShareRow[] = [
  { id: "1", name: "分镜包", mode: "fixed", audience: "link", path: "boards", expires_at: "2099-01-01", revoked: false, allow_download: true },
  { id: "2", name: "旧链接", mode: "fixed", audience: "link", path: "old", expires_at: "2001-01-01", revoked: false, allow_download: false },
  { id: "3", name: "已撤", mode: "live", audience: "users", path: "x", expires_at: "2099-01-01", revoked: true, allow_download: true },
];

it("classifies and filters shares", () => {
  expect(shareStatus(rows[0])).toBe("有效");
  expect(filterShares(rows, "", "有效").map((r) => r.id)).toEqual(["1"]);
  expect(filterShares(rows, "旧", "all").map((r) => r.id)).toEqual(["2"]);
});

it("keeps permanent shares active", () => {
  expect(shareStatus({ id:"permanent", name:"永久", mode:"fixed", audience:"link", path:"x", expires_at:"", revoked:false, allow_download:true, is_permanent:true })).toBe("永久有效");
});
