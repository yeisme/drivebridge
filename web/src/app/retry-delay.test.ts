import { expect, it } from "vitest";
import { retryAfterMs, readRetryDelay } from "./retry-delay";

it("parses seconds and HTTP dates without treating invalid values as delays", () => {
  const now = Date.parse("2026-01-01T00:00:00Z");
  expect(retryAfterMs("60", now)).toBe(60000);
  expect(retryAfterMs("Thu, 01 Jan 2026 00:01:00 GMT", now)).toBe(60000);
  expect(retryAfterMs(null, now)).toBeUndefined();
  expect(retryAfterMs("not-a-date", now)).toBeUndefined();
  expect(retryAfterMs("Wed, 31 Dec 2025 00:00:00 GMT", now)).toBeUndefined();
});
it.each(["-1", "+1", "1.5", ".5", "Infinity", "9".repeat(400)])("rejects malformed or overflowing delay %s", value => {
  expect(retryAfterMs(value, 0)).toBeUndefined();
});
it("backs off with bounded positive jitter and honors longer server delays", () => {
  expect([1,2,3,4,5,6].map(n => readRetryDelay(n, 0, 0))).toEqual([2000,4000,8000,16000,30000,30000]);
  expect(readRetryDelay(1, 0, 0.5)).toBe(2100);
  expect(readRetryDelay(1, 60000, 0.5)).toBe(60000);
});
