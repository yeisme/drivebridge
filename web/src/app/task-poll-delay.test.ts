import { expect, it } from "vitest";
import { taskPollDelay } from "./task-poll-delay";
it("uses idle cadence only when all observed operations are terminal", () => {
  expect(taskPollDelay([])).toBe(30000);
  expect(taskPollDelay([{ status: "completed" }, { status: "cancelled" }])).toBe(30000);
  for (const status of ["running", "failed", "interrupted", "unknown", "future-status"]) {
    expect(taskPollDelay([{ status: "completed" }, { status }])).toBe(2000);
  }
});
