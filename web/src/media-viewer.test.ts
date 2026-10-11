import { describe, expect, it } from "vitest";
import {
  clampZoom,
  displayToNatural,
  drawStrokes,
  fitScale,
  formatDuration,
  formatExposure,
  formatGps,
  naturalToDisplay,
  orientationLabel,
  panForZoom,
  parseJpegExif,
  parsePngHeader,
  pngColorName,
  textStats,
  type ViewerStroke,
} from "./media-viewer";

const BOX = { left: 100, top: 50, width: 300, height: 200 };
const STAGE = { left: 0, top: 0, width: 600, height: 400 };
const NATURAL = { width: 900, height: 600 };

describe("display/natural coordinate mapping", () => {
  const cases = [
    { zoom: 1, rotDeg: 0, flipH: false, flipV: false },
    { zoom: 0.4, rotDeg: 0, flipH: false, flipV: false },
    { zoom: 1, rotDeg: 90, flipH: false, flipV: false },
    { zoom: 1, rotDeg: 180, flipH: false, flipV: false },
    { zoom: 1, rotDeg: 270, flipH: false, flipV: false },
    { zoom: 1, rotDeg: 90, flipH: true, flipV: false },
    { zoom: 1.6, rotDeg: 270, flipH: true, flipV: true },
  ];
  for (const geometry of cases) {
    it(`roundtrips rot ${geometry.rotDeg} zoom ${geometry.zoom}`, () => {
      const g = { rect: BOX, natural: NATURAL, ...geometry };
      const point = { x: 234, y: 121 };
      const natural = displayToNatural(point.x, point.y, g);
      const display = naturalToDisplay(natural, g);
      expect(display.x).toBeCloseTo(point.x, 6);
      expect(display.y).toBeCloseTo(point.y, 6);
    });
  }
  it("maps natural center to rect center", () => {
    const display = naturalToDisplay(
      { x: NATURAL.width / 2, y: NATURAL.height / 2 },
      { rect: BOX, natural: NATURAL, zoom: 1.5, rotDeg: 90, flipH: true, flipV: false },
    );
    expect(display.x).toBeCloseTo(BOX.left + BOX.width / 2, 6);
    expect(display.y).toBeCloseTo(BOX.top + BOX.height / 2, 6);
  });
});

describe("panForZoom", () => {
  it("keeps the cursor-anchored natural point fixed", () => {
    const zoom = 0.5;
    const nextZoom = 0.8;
    const cursor = { x: 180, y: 160 };
    const g = { rect: BOX, natural: NATURAL, zoom, rotDeg: 90, flipH: true, flipV: false };
    const pan = panForZoom({
      rect: BOX,
      stage: STAGE,
      natural: NATURAL,
      zoom,
      nextZoom,
      rotDeg: 90,
      flipH: true,
      flipV: false,
      cursor,
    });
    const after = {
      ...BOX,
      left: STAGE.left + STAGE.width / 2 + pan.x - BOX.width / 2,
      top: STAGE.top + STAGE.height / 2 + pan.y - BOX.height / 2,
    };
    const point = displayToNatural(cursor.x, cursor.y, { ...g, rect: after, zoom: nextZoom });
    const before = displayToNatural(cursor.x, cursor.y, g);
    expect(point.x).toBeCloseTo(before.x, 6);
    expect(point.y).toBeCloseTo(before.y, 6);
  });
  it("centers when zooming at the stage center without pan", () => {
    const cursor = { x: STAGE.left + STAGE.width / 2, y: STAGE.top + STAGE.height / 2 };
    const pan = panForZoom({
      rect: { left: 150, top: 100, width: 300, height: 200 },
      stage: STAGE,
      natural: NATURAL,
      zoom: 1,
      nextZoom: 2,
      rotDeg: 0,
      flipH: false,
      flipV: false,
      cursor,
    });
    expect(pan.x).toBeCloseTo(0, 6);
    expect(pan.y).toBeCloseTo(0, 6);
  });
});

describe("fitScale", () => {
  it("contains within the box and never upscales", () => {
    expect(fitScale(NATURAL, { width: 450, height: 300 })).toBe(0.5);
    expect(fitScale({ width: 100, height: 50 }, { width: 800, height: 600 })).toBe(1);
    expect(fitScale({ width: 0, height: 0 }, { width: 800, height: 600 })).toBe(1);
  });
});

describe("clampZoom", () => {
  it("bounds the zoom factor", () => {
    expect(clampZoom(0.01)).toBe(0.2);
    expect(clampZoom(99)).toBe(8);
    expect(clampZoom(1.5)).toBe(1.5);
  });
});

interface FakeCtxCalls extends Record<string, unknown> {
  calls: [string, ...unknown[]][];
}
function fakeContext(): CanvasRenderingContext2D & FakeCtxCalls {
  const calls: [string, ...unknown[]][] = [];
  const impl: Record<string, unknown> = {
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    stroke: () => {},
    rect: () => {},
    fill: () => {},
    closePath: () => {},
  };
  const handler: ProxyHandler<CanvasRenderingContext2D & FakeCtxCalls> = {
    get(_target, property: string) {
      if (property === "calls") return calls;
      if (property in impl)
        return (...args: unknown[]) => {
          calls.push([property, ...args]);
        };
      return undefined;
    },
    set(_target, property: string, value) {
      calls.push([property as string, value]);
      return true;
    },
  };
  return new Proxy({ calls } as CanvasRenderingContext2D & FakeCtxCalls, handler);
}

describe("drawStrokes", () => {
  it("draws pen polylines with the stroke width", () => {
    const ctx = fakeContext();
    const stroke: ViewerStroke = {
      tool: "pen",
      color: "#e5484d",
      width: 6,
      points: [
        { x: 10, y: 20 },
        { x: 40, y: 60 },
        { x: 80, y: 30 },
      ],
    };
    drawStrokes(ctx, [stroke]);
    const names = ctx.calls.map(([name]) => name);
    expect(names).toContain("lineWidth");
    expect(names.filter((name) => name === "lineTo")).toHaveLength(2);
    expect(names).toContain("stroke");
  });
  it("draws rectangles between the two corner points", () => {
    const ctx = fakeContext();
    drawStrokes(ctx, [
      {
        tool: "rect",
        color: "#0090ff",
        width: 3,
        points: [
          { x: 100, y: 50 },
          { x: 20, y: 90 },
        ],
      },
    ]);
    const rect = ctx.calls.find(([name]) => name === "rect");
    expect(rect).toEqual(["rect", 20, 50, 80, 40]);
  });
  it("skips single-point strokes", () => {
    const ctx = fakeContext();
    drawStrokes(ctx, [
      { tool: "pen", color: "#fff", width: 3, points: [{ x: 1, y: 2 }] },
    ]);
    expect(ctx.calls.map(([name]) => name)).toEqual(["lineCap", "lineJoin"]);
  });
});

function ascii(value: string): number[] {
  return [...value].map((char) => char.charCodeAt(0));
}
function buildExifJpeg(): ArrayBuffer {
  // Little-endian TIFF with a trailing data area; offsets relative to TIFF start.
  const IFD0 = 8;
  const ifd0Count = 5;
  const EXIF_IFD = IFD0 + 2 + ifd0Count * 12 + 4;
  const exifCount = 5;
  const GPS_IFD = EXIF_IFD + 2 + exifCount * 12 + 4;
  const gpsCount = 4;
  const DATA = GPS_IFD + 2 + gpsCount * 12 + 4;
  const at = {
    make: DATA,
    model: DATA + 8,
    latRef: DATA + 16,
    lonRef: DATA + 18,
    exposure: DATA + 20,
    fNumber: DATA + 28,
    focal: DATA + 36,
    date: DATA + 44,
    latitude: DATA + 64,
    longitude: DATA + 88,
  };
  const u16 = (value: number) => [value & 255, (value >> 8) & 255];
  const u32 = (value: number) => [
    value & 255,
    (value >> 8) & 255,
    (value >> 16) & 255,
    (value >>> 24) & 255,
  ];
  const rational = (numerator: number, denominator: number) => [
    ...u32(numerator),
    ...u32(denominator),
  ];
  const entry = (
    tag: number,
    type: number,
    count: number,
    value: number[],
  ) => [...u16(tag), ...u16(type), ...u32(count), ...value];
  const ifd = (count: number, entries: number[][]) => [
    ...u16(count),
    ...entries.flat(),
    0,
    0,
    0,
    0,
  ];
  const tiff: number[] = [
    ...ascii("II"),
    ...u16(42),
    ...u32(IFD0),
    ...ifd(ifd0Count, [
      entry(0x010f, 2, 8, u32(at.make)),
      entry(0x0110, 2, 8, u32(at.model)),
      entry(0x0112, 3, 1, [...u16(6), 0, 0]),
      entry(0x8769, 4, 1, u32(EXIF_IFD)),
      entry(0x8825, 4, 1, u32(GPS_IFD)),
    ]),
    ...ifd(exifCount, [
      entry(0x829a, 5, 1, u32(at.exposure)),
      entry(0x829d, 5, 1, u32(at.fNumber)),
      entry(0x8827, 3, 1, [...u16(200), 0, 0]),
      entry(0x9003, 2, 20, u32(at.date)),
      entry(0x920a, 5, 1, u32(at.focal)),
    ]),
    ...ifd(gpsCount, [
      entry(0x0001, 2, 2, [...ascii("N"), 0, 0, 0]),
      entry(0x0002, 5, 3, u32(at.latitude)),
      entry(0x0003, 2, 2, [...ascii("W"), 0, 0, 0]),
      entry(0x0004, 5, 3, u32(at.longitude)),
    ]),
    ...ascii("TestCam\0"),
    ...ascii("Model X\0"),
    ...ascii("N\0"),
    ...ascii("W\0"),
    ...rational(1, 250),
    ...rational(28, 10),
    ...rational(50, 1),
    ...ascii("2026:09:19 10:00:00\0"),
    ...rational(37, 1),
    ...rational(0, 1),
    ...rational(0, 1),
    ...rational(122, 1),
    ...rational(0, 1),
    ...rational(0, 1),
  ];
  const app1Body = [...ascii("Exif"), 0, 0, ...tiff];
  const bytes = [
    0xff, 0xd8,
    0xff, 0xe1,
    (app1Body.length + 2) >> 8,
    (app1Body.length + 2) & 255,
    ...app1Body,
    0xff, 0xd9,
  ];
  return new Uint8Array(bytes).buffer;
}

describe("parseJpegExif", () => {
  it("reads camera, exposure and GPS fields", () => {
    const exif = parseJpegExif(buildExifJpeg());
    expect(exif.make).toBe("TestCam");
    expect(exif.model).toBe("Model X");
    expect(exif.orientation).toBe(6);
    expect(exif.exposureTime).toBeCloseTo(1 / 250, 6);
    expect(exif.fNumber).toBeCloseTo(2.8, 6);
    expect(exif.iso).toBe(200);
    expect(exif.dateTimeOriginal).toBe("2026:09:19 10:00:00");
    expect(exif.focalLength).toBe(50);
    expect(exif.gps?.latitude).toBeCloseTo(37, 6);
    expect(exif.gps?.longitude).toBeCloseTo(-122, 6);
  });
  it("ignores non-JPEG buffers", () => {
    expect(parseJpegExif(new Uint8Array([0, 1, 2, 3]).buffer)).toEqual({});
  });
});

describe("parsePngHeader", () => {
  it("reads IHDR dimensions and depth", () => {
    const bytes = [
      0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
      0, 0, 0, 13,
      0x49, 0x48, 0x44, 0x52,
      0, 0, 0, 2,
      0, 0, 0, 3,
      8, 6, 0, 0, 0,
      0, 0, 0, 0,
    ];
    const header = parsePngHeader(new Uint8Array(bytes).buffer);
    expect(header).toEqual({ width: 2, height: 3, bitDepth: 8, colorType: 6 });
    expect(pngColorName(6)).toBe("RGBA");
  });
  it("rejects other data", () => {
    expect(parsePngHeader(new Uint8Array(64).buffer)).toBeNull();
  });
});

describe("formatting helpers", () => {
  it("formats exposure, duration, GPS and orientation", () => {
    expect(formatExposure(1 / 250)).toBe("1/250 s");
    expect(formatExposure(2)).toBe("2 s");
    expect(formatDuration(90)).toBe("1:30");
    expect(formatDuration(3723)).toBe("1:02:03");
    expect(formatGps(37.5, "N", "S")).toBe("37° 30′ 0″ N");
    expect(formatGps(-122.25, "E", "W")).toBe("122° 15′ 0″ W");
    expect(orientationLabel(6)).toBe("顺时针 90°");
    expect(orientationLabel(undefined)).toBeUndefined();
  });
  it("counts text lines and characters", () => {
    expect(textStats("a\nb\nc")).toEqual({ lines: 3, chars: 5 });
    expect(textStats("")).toEqual({ lines: 0, chars: 0 });
  });
});
