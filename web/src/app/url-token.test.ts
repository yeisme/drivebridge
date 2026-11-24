import { describe, expect, it } from "vitest";
import {
  buildDirectTokenInviteUrl,
  extractUrlLoginToken,
  formatInviteShareText,
  parsePastedInviteOrKey,
} from "./url-token";

describe("url-token helpers", () => {
  it("extracts ?token= from search or hash query and strips only secret params", () => {
    expect(
      extractUrlLoginToken({
        pathname: "/",
        search: "?token=dbu_secret123&utm=invite",
        hash: "#/files",
      }),
    ).toEqual({
      token: "dbu_secret123",
      cleanUrl: "/?utm=invite#/files",
    });

    expect(
      extractUrlLoginToken({
        pathname: "/",
        search: "",
        hash: "#/inbox?token=dbu_hash456&submission_state=submitted",
      }),
    ).toEqual({
      token: "dbu_hash456",
      cleanUrl: "/#/inbox?submission_state=submitted",
    });

    expect(
      extractUrlLoginToken({
        pathname: "/",
        search: "",
        hash: "#token=dbu_frag789",
      }),
    ).toEqual({
      token: "dbu_frag789",
      cleanUrl: "/",
    });

    expect(
      extractUrlLoginToken({
        pathname: "/",
        search: "?view=drive",
        hash: "#/files",
      }),
    ).toBeNull();
  });

  it("parses raw keys, full ?token= URLs, and Baidu-Netdisk-style invite copy", () => {
    const token = `dbu_${"a".repeat(96)}`;
    const url = buildDirectTokenInviteUrl("http://10.10.1.101:18786", token);
    const inviteCopy = formatInviteShareText({
      name: "外部协作者",
      url,
      token,
      oneTime: true,
    });

    expect(parsePastedInviteOrKey(token)).toBe(token);
    expect(parsePastedInviteOrKey(url)).toBe(token);
    expect(parsePastedInviteOrKey(inviteCopy)).toBe(token);
  });
});
