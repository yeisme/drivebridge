import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { EditorView } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { undo } from "@codemirror/commands";
import { NotebookDocument } from "./NotebookDocument";
const identity = { sourceId: "source", objectId: "note", revision: "one" };
afterEach(cleanup);
function editor(container: HTMLElement) { return EditorView.findFromDOM(container.querySelector(".cm-editor") as HTMLElement)!; }

it("keeps the same editor and undo history through all three modes", () => {
  const onDraft = vi.fn();
  const view = render(<NotebookDocument identity={identity} body="# Original" historical={false} editable onDraft={onDraft}/>);
  const source = editor(view.container);
  fireEvent.click(screen.getByRole("button", { name: /^源码$/ }));
  source.dispatch({ changes: { from: source.state.doc.length, insert: " change" } });
  expect(onDraft).toHaveBeenLastCalledWith("# Original change");
  fireEvent.click(screen.getByRole("button", { name: "实时预览" }));
  fireEvent.click(screen.getByRole("button", { name: /^阅读$/ }));
  expect(editor(view.container)).toBe(source);
  expect(undo(source)).toBe(true);
  expect(source.state.doc.toString()).toBe("# Original");
});

it("starts from and reports the per-revision remembered mode", () => {
  const onModeChange = vi.fn();
  render(<NotebookDocument identity={identity} body="# Original" historical={false} editable initialMode="source" onModeChange={onModeChange} onDraft={vi.fn()}/>);
  expect(screen.getByRole("button", { name: /^源码$/ }).getAttribute("aria-pressed")).toBe("true");
  fireEvent.click(screen.getByRole("button", { name: "实时预览" }));
  expect(onModeChange).toHaveBeenCalledWith("live");
});

it("offers a Markdown-native block menu without replacing the editor model", () => {
  const view = render(<NotebookDocument identity={identity} body="/" historical={false} editable initialMode="source" onDraft={vi.fn()}/>);
  fireEvent.click(screen.getByRole("button", { name: "插入块" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "任务" }));
  expect(editor(view.container).state.doc.toString()).toContain("- [ ] ");
});

it("preserves disclosure state across modes but not across revisions", () => {
  const props = { identity, body: "# Heading\n\nBody", historical: false, editable: false, onDraft: vi.fn() };
  const view = render(<NotebookDocument {...props}/>);
  const section = view.container.querySelector("details")!;
  section.open = false;
  fireEvent.click(screen.getByRole("button", { name: /^源码$/ }));
  fireEvent.click(screen.getByRole("button", { name: /^阅读$/ }));
  expect(view.container.querySelector("details")).toBe(section);
  expect(section.open).toBe(false);
  view.rerender(<NotebookDocument {...props} identity={{ ...identity, revision: "two" }}/>);
  expect(view.container.querySelector("details")).not.toBe(section);
  expect(view.container.querySelector("details")!.open).toBe(true);
});

it("treats historical and oversized UTF-8 documents as read-only", () => {
  const view = render(<NotebookDocument identity={identity} body="old" historical editable onDraft={vi.fn()}/>);
  expect(editor(view.container).state.facet(EditorState.readOnly)).toBe(true);
  expect(screen.getByRole("status").textContent).toBe("历史版本只读");
  view.rerender(<NotebookDocument identity={identity} body={"中".repeat(22000)} historical={false} editable onDraft={vi.fn()}/>);
  expect(editor(view.container).state.facet(EditorState.readOnly)).toBe(true);
  expect(screen.getByRole("status").textContent).toContain("64KiB");
});

it("updates a peer draft without echoing it and resets history on revision changes", () => {
  const onDraft = vi.fn();
  const view = render(<NotebookDocument identity={identity} body="first" historical={false} editable onDraft={onDraft}/>);
  const first = editor(view.container);
  view.rerender(<NotebookDocument identity={identity} body="first" draft="peer" historical={false} editable onDraft={onDraft}/>);
  expect(first.state.doc.toString()).toBe("peer");
  expect(onDraft).not.toHaveBeenCalled();
  expect(undo(first)).toBe(false);
  view.rerender(<NotebookDocument identity={{ ...identity, revision: "two" }} body="second" historical={false} editable onDraft={onDraft}/>);
  expect(editor(view.container)).not.toBe(first);
  expect(editor(view.container).state.doc.toString()).toBe("second");
});
