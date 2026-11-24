import { describe, expect, it } from "vitest";
import { marqueeIntersects, mergeSelectionIds, normalizeMarqueeBox } from "./marquee";

describe("marquee geometry", () => {
  it("normalizes drag coordinates regardless of direction and detects hit intersections", () => {
    const box = normalizeMarqueeBox(220, 180, 40, 60);
    expect(box).toEqual({ left: 40, top: 60, right: 220, bottom: 180 });
    expect(marqueeIntersects(box, { left: 100, top: 100, right: 160, bottom: 140 })).toBe(true);
    expect(marqueeIntersects(box, { left: 230, top: 100, right: 300, bottom: 140 })).toBe(false);
  });

  it("replaces or merges hit item ids cleanly", () => {
    expect(mergeSelectionIds(["a"], ["b", "c"], false)).toEqual(["b", "c"]);
    expect(mergeSelectionIds(["a", "b"], ["b", "c"], true)).toEqual(["a", "b", "c"]);
  });
});
