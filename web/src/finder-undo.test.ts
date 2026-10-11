import { describe, expect, it } from "vitest";
import { moveCount, moveNotice, type LastMove } from "./finder-undo";

const move: LastMove = {
  space_id: "s",
  files: [
    { id: "1", from: "a.txt", to: "b/a.txt", version_id: "v" },
  ],
  folders: [{ from: "资料", to: "备份/资料" }],
};

describe("last move", () => {
  it("counts files and folders", () => {
    expect(moveCount(move)).toBe(2);
    expect(moveNotice(move)).toBe("已移动 2 项");
  });
});
