import { expect, it } from "vitest";
import { listCommand, stepIndex } from "./page-list";

it("wraps list movement", () => {
  expect(stepIndex(0, -1, 3)).toBe(2);
  expect(stepIndex(2, 1, 3)).toBe(0);
  expect(stepIndex(1, 1, 3)).toBe(2);
});

it("maps list keys outside fields", () => {
  expect(listCommand({ key: "ArrowDown", target: document.body })).toBe("next");
  expect(listCommand({ key: "Enter", target: document.body })).toBe("open");
  expect(listCommand({ key: "Delete", target: document.body })).toBe("cancel");
  const input = document.createElement("input");
  expect(listCommand({ key: "ArrowDown", target: input })).toBeNull();
});
