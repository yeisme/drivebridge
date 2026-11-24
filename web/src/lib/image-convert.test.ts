import { describe, expect, it } from "vitest";
import { buildConvertedFileName, computeTargetDimensions } from "./image-convert";

describe("image-convert helpers", () => {
  it("computes scaled dimensions accurately", () => {
    expect(computeTargetDimensions(3840, 2160, "100")).toEqual({ width: 3840, height: 2160 });
    expect(computeTargetDimensions(3840, 2160, "50")).toEqual({ width: 1920, height: 1080 });
    expect(computeTargetDimensions(3840, 2160, "max1920")).toEqual({ width: 1920, height: 1080 });
    expect(computeTargetDimensions(1200, 800, "max1920")).toEqual({ width: 1200, height: 800 });
  });

  it("builds non-conflicting converted filenames without overwriting the original", () => {
    expect(buildConvertedFileName("Forza Horizon 5 2025_10_8 0_55_36.jxr", "png")).toBe(
      "Forza Horizon 5 2025_10_8 0_55_36.png",
    );
    expect(
      buildConvertedFileName("shots/Forza Horizon 5.jxr", "jpeg", ["shots/Forza Horizon 5.jpg"]),
    ).toBe("Forza Horizon 5-2.jpg");
    expect(buildConvertedFileName("cover.png", "png")).toBe("cover-converted.png");
    expect(buildConvertedFileName("cover.png", "png", ["cover-converted.png"])).toBe(
      "cover-converted-2.png",
    );
  });
});
