import { expect, it } from "vitest";
import { notebookBlockInsert } from "./notebook-blocks";

it("inserts Markdown blocks at the current line without creating a block model", () => {
  const value = "Intro\n/";
  const change = notebookBlockInsert(value, value.length, "- [ ] ");
  expect(value.slice(0, change.from) + change.insert + value.slice(change.to)).toBe("Intro\n- [ ] ");
});
