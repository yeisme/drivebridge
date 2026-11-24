import { describe, expect, it } from "vitest";
import { chooseDefaultDriveSpace } from "./default-space";
import type { Space } from "../api";

const teamSpace: Space = {
  id: "team-1",
  name: "团队素材库",
  team_id: "t1",
  connection_id: "conn-1",
  backend: "s3",
  max_bytes: 100,
  used_bytes: 0,
  reserved_bytes: 0,
};

const personalSpace: Space = {
  id: "personal-1",
  name: "我的个人网盘",
  connection_id: "conn-1",
  backend: "s3",
  max_bytes: 100,
  used_bytes: 0,
  reserved_bytes: 0,
};

describe("chooseDefaultDriveSpace", () => {
  it("prefers an explicitly chosen space when still accessible", () => {
    expect(chooseDefaultDriveSpace([teamSpace, personalSpace], "team-1")?.id).toBe("team-1");
  });

  it("defaults to the first personal space before team spaces", () => {
    expect(chooseDefaultDriveSpace([teamSpace, personalSpace])?.id).toBe("personal-1");
  });

  it("falls back to the first accessible space when only team spaces exist", () => {
    expect(chooseDefaultDriveSpace([teamSpace])?.id).toBe("team-1");
    expect(chooseDefaultDriveSpace([])).toBeUndefined();
  });
});
