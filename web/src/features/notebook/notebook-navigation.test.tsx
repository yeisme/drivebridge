import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { NotebookNavigator, notebookShortcut } from "./notebook-navigation";

afterEach(cleanup);

describe("notebook navigation", () => {
  it("reserves notebook shortcuts without intercepting global search", () => {
    expect(notebookShortcut(new KeyboardEvent("keydown", { key: "p", ctrlKey: true }))).toBe("commands");
    expect(notebookShortcut(new KeyboardEvent("keydown", { key: "o", metaKey: true }))).toBe("open");
    expect(notebookShortcut(new KeyboardEvent("keydown", { key: "Tab", ctrlKey: true, shiftKey: true }))).toBe("previous-tab");
    for (const key of ["k", "f", "s"]) expect(notebookShortcut(new KeyboardEvent("keydown", { key, ctrlKey: true }))).toBeUndefined();
    expect(notebookShortcut(new KeyboardEvent("keydown", { key: "p", ctrlKey: true, isComposing: true }))).toBeUndefined();
  });

  it("yields to text inputs, editors, and already-handled events", () => {
    render(<div><input aria-label="field"/><div contentEditable aria-label="editor"/><div className="cm-editor" data-testid="cm"><span>code</span></div></div>);
    for (const target of [screen.getByLabelText("field"), screen.getByLabelText("editor"), screen.getByText("code")]) {
      const event = new KeyboardEvent("keydown", { key: "p", ctrlKey: true, bubbles: true });
      target.dispatchEvent(event);
      expect(notebookShortcut(event)).toBeUndefined();
    }
    const event = new KeyboardEvent("keydown", { key: "o", ctrlKey: true, cancelable: true });
    event.preventDefault();
    expect(notebookShortcut(event)).toBeUndefined();
  });

  it("searches only supplied entries in the current source and opens a fixed identity", () => {
    const ref = { sourceId: "one", objectId: "note", revision: "fixed" };
    const onOpen = vi.fn(), close = vi.fn();
    render(<NotebookNavigator mode="open" sourceId="one" commands={[]} entries={[{ ref, title: "旅行笔记" }, { ref: { ...ref, sourceId: "other" }, title: "不可见笔记" }]} onOpen={onOpen} close={close}/>);
    expect(screen.queryByText("不可见笔记")).toBeNull();
    fireEvent.change(screen.getByLabelText("搜索笔记标题"), { target: { value: "旅行" } });
    fireEvent.click(screen.getByRole("button", { name: "旅行笔记" }));
    expect(onOpen).toHaveBeenCalledWith(ref);
    expect(close).toHaveBeenCalledOnce();
  });

  it("navigates enabled commands and leaves IME confirmation untouched", () => {
    const run = vi.fn(), close = vi.fn();
    render(<NotebookNavigator mode="commands" sourceId="one" entries={[]} commands={[
      { id: "disabled", label: "不可用", disabled: true, run },
      { id: "first", label: "第一项", run }, { id: "second", label: "第二项", run },
    ]} onOpen={vi.fn()} close={close}/>);
    const input = screen.getByLabelText("搜索命令");
    input.focus();
    fireEvent.keyDown(input, { key: "ArrowDown", isComposing: true });
    expect(document.activeElement).toBe(input);
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    expect(run).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: "ArrowDown" });
    const first = screen.getByRole("button", { name: "第一项" });
    expect(document.activeElement).toBe(first);
    fireEvent.keyDown(first, { key: "ArrowUp" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "第二项" }));
    input.focus();
    fireEvent.keyDown(input, { key: "Enter" });
    expect(run).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });

  it("keeps unsupported commands disabled and exposes an empty search state", () => {
    const run = vi.fn();
    render(<NotebookNavigator mode="commands" sourceId="one" entries={[]} commands={[{ id: "save", label: "保存", disabled: true, run }]} onOpen={vi.fn()} close={vi.fn()}/>);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(run).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("搜索命令"), { target: { value: "missing" } });
    expect(screen.getByRole("status").textContent).toBe("没有匹配结果");
  });
});
