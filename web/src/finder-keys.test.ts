import { describe, expect, it } from "vitest";
import {
  finderCommand,
  isTypingTarget,
  typeaheadIndex,
} from "./finder-keys";

const event = (key: string, extra: Partial<KeyboardEvent> = {}) =>
  ({
    key,
    metaKey: false,
    ctrlKey: false,
    shiftKey: false,
    target: document.body,
    ...extra,
  }) as KeyboardEvent;

describe("finder commands", () => {
  it("maps copy paste and delete outside inputs", () => {
    expect(finderCommand(event("c", { ctrlKey: true }))).toBe("copy");
    expect(finderCommand(event("v", { ctrlKey: true }))).toBe("paste");
    expect(finderCommand(event("Delete"))).toBe("trash");
    expect(finderCommand(event("z", { ctrlKey: true }))).toBe("undo");
    expect(finderCommand(event("F2"))).toBe("rename");
  });
  it("uses left and right arrows for folder navigation", () => {
    expect(finderCommand(event("ArrowRight"))).toBe("open");
    expect(finderCommand(event("ArrowLeft"))).toBe("goParent");
    expect(finderCommand(event("ArrowDown"))).toBe("focusNext");
  });
  it("jumps to names that start with the typed buffer", () => {
    const names = ["资料", "笔记.txt", "空白.txt", "项目.md"];
    expect(typeaheadIndex(names, "笔", 0)).toBe(1);
    expect(typeaheadIndex(names, "b", 0)).toBe(-1);
    expect(typeaheadIndex(names, "空空", 1)).toBe(2);
  });
  it("does not steal from text fields", () => {
    const input = document.createElement("input");
    expect(isTypingTarget(input)).toBe(true);
    expect(finderCommand(event("c", { ctrlKey: true, target: input }))).toBe(
      null,
    );
  });
});
