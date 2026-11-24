export interface BoxRect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

export function normalizeMarqueeBox(
  startX: number,
  startY: number,
  endX: number,
  endY: number,
): BoxRect {
  return {
    left: Math.min(startX, endX),
    top: Math.min(startY, endY),
    right: Math.max(startX, endX),
    bottom: Math.max(startY, endY),
  };
}

export function marqueeIntersects(a: BoxRect, b: BoxRect): boolean {
  return a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
}

export function mergeSelectionIds(
  baseIds: readonly string[],
  hitIds: readonly string[],
  additive: boolean,
): string[] {
  if (!additive) return [...new Set(hitIds)];
  const merged = new Set(baseIds);
  for (const id of hitIds) merged.add(id);
  return [...merged];
}
