import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { NotebookDirectory } from "./NotebookDirectory";
import { noteKey } from "./notebook-state";
afterEach(cleanup);

it("groups only supplied paths and preserves fixed identity on open and pin", () => {
  const ref = { sourceId: "source", objectId: "note", revision: "one" };
  const onOpen = vi.fn(), onPin = vi.fn();
  const { container } = render(<NotebookDirectory entries={[{ ref, path: "folder/note.md", title: "note.md" }]} activeKey={noteKey(ref)} onOpen={onOpen} onPin={onPin}/>);
  expect(container.querySelector("summary")?.textContent).toBe("folder");
  const button = screen.getByRole("button", { name: "note.md" });
  expect(button.getAttribute("aria-current")).toBe("page");
  fireEvent.click(button);
  fireEvent.doubleClick(button);
  expect(onOpen).toHaveBeenCalledWith(ref);
  expect(onPin).toHaveBeenCalledWith(ref);
});

it("labels missing loaded entries without implying an empty vault", () => {
  render(<NotebookDirectory entries={[]} onOpen={vi.fn()} onPin={vi.fn()}/>);
  expect(screen.getByRole("status").textContent).toBe("当前来源没有已加载笔记。");
});
