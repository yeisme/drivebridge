import type { Space } from "./api";

export function capacityOf(space: Pick<Space, "used_bytes" | "reserved_bytes" | "max_bytes">) {
  const valid = (n: unknown): n is number => typeof n === "number" && Number.isFinite(n) && n >= 0;
  const used = valid(space.used_bytes) ? space.used_bytes : null;
  const reserved = valid(space.reserved_bytes) ? space.reserved_bytes : null;
  const quota = valid(space.max_bytes) && space.max_bytes > 0 ? space.max_bytes : null;
  const occupied = used !== null && reserved !== null ? used + reserved : null;
  const free = quota !== null && occupied !== null ? Math.max(0, quota - occupied) : null;
  const percent = quota !== null && occupied !== null ? occupied / quota * 100 : null;
  return { used, reserved, quota, occupied, free, percent,
    overflow: quota !== null && occupied !== null ? Math.max(0, occupied - quota) : 0 };
}
export function percentText(value: number) {
  return value > 0 && value < 0.1 ? "<0.1%" : `${Math.round(value * 10) / 10}%`;
}
