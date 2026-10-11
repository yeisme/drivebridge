import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NotebookWorkspace, type NotebookDocumentContext } from "./NotebookWorkspace";
import { NotebookDocument } from "./NotebookDocument";
import { EditorView } from "@codemirror/view";
import { undo } from "@codemirror/commands";

const scope = { actor: "a", space: "space", source: "source" };
const entries = [{ title: "笔记甲", ref: { sourceId: "source", objectId: "a", revision: "1" } }, { title: "笔记乙", ref: { sourceId: "source", objectId: "b", revision: "1" } }];
const document = ({ draft, onDraft, ref, pane }: NotebookDocumentContext) => <textarea aria-label={`正文-${pane}`} value={draft ?? ref.objectId} onChange={event => onDraft(event.target.value)}/>;
beforeEach(() => { vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({ width: 1400, left: 0 } as DOMRect); });
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); });
async function open(title: string) {
  fireEvent.click(screen.getByRole("button", { name: "快速打开" }));
  fireEvent.click(screen.getByRole("button", { name: title }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
}

it("preserves actual CodeMirror undo when switching away from a dirty tab", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={({ ref, draft, onDraft }) =>
    <NotebookDocument identity={ref} body={ref.objectId} draft={draft} historical={false} editable onDraft={onDraft}/>}/>);
  await open("笔记甲");
  fireEvent.click(screen.getByRole("button", { name: /^源码$/ }));
  const dom = view.container.querySelector<HTMLElement>(".cm-editor")!;
  const editor = EditorView.findFromDOM(dom)!;
  act(() => { editor.dispatch({ changes: { from: 1, insert: " edited" } }); });
  await open("笔记乙");
  fireEvent.click(screen.getByRole("tab", { name: /笔记甲/ }));
  expect(EditorView.findFromDOM(dom)).toBe(editor);
  expect(editor.state.doc.toString()).toBe("a edited");
  act(() => { expect(undo(editor)).toBe(true); });
  expect(editor.state.doc.toString()).toBe("a");
});

it("retains a pinned tab's DOM across switches and removes it on revocation", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.doubleClick(screen.getByRole("tab", { name: "笔记甲" }));
  const original = screen.getByLabelText("正文-primary");
  await open("笔记乙");
  expect(original.isConnected).toBe(true);
  expect(original.closest("[hidden]")).not.toBeNull();
  fireEvent.click(screen.getByRole("tab", { name: /笔记甲/ }));
  expect(original.closest("[hidden]")).toBeNull();
  view.rerender(<NotebookWorkspace scope={scope} entries={[entries[1]]} renderDocument={document}/>);
  expect(original.isConnected).toBe(false);
});

it("temporarily hides sidebars in focus mode and restores focus without changing preferences", async () => {
  render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  const stored = localStorage.length;
  fireEvent.click(screen.getByRole("button", { name: "进入专注模式" }));
  expect(screen.queryByRole("complementary", { name: "笔记目录侧栏" })).toBeNull();
  expect(screen.queryByRole("complementary", { name: "笔记信息侧栏" })).toBeNull();
  fireEvent.keyDown(screen.getByLabelText("正文-primary"), { key: "Escape" });
  expect(screen.getByRole("button", { name: "退出专注模式" })).toBeTruthy();
  fireEvent.keyDown(screen.getByRole("tab", { name: "笔记甲" }), { key: "Escape" });
  expect(screen.getByRole("complementary", { name: "笔记目录侧栏" })).toBeTruthy();
  expect(screen.getByRole("complementary", { name: "笔记信息侧栏" })).toBeTruthy();
  expect(window.document.activeElement).toBe(screen.getByRole("button", { name: "进入专注模式" }));
  expect(localStorage.length).toBe(stored);
});

it("applies reading preferences and restores them only for the same scope", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  fireEvent.click(screen.getByRole("button", { name: "命令面板" }));
  fireEvent.click(screen.getByRole("button", { name: "增大阅读字号" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  fireEvent.click(screen.getByRole("button", { name: "命令面板" }));
  fireEvent.click(screen.getByRole("button", { name: "阅读行宽：宽" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  const workspace = screen.getByLabelText("笔记工作台");
  expect(workspace.style.getPropertyValue("--notebook-font-size")).toBe("17px");
  expect(workspace.style.getPropertyValue("--notebook-line-width")).toBe("880px");
  view.unmount();
  const restored = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  expect(screen.getByLabelText("笔记工作台").style.getPropertyValue("--notebook-font-size")).toBe("17px");
  restored.rerender(<NotebookWorkspace scope={{ ...scope, actor: "other" }} entries={entries} renderDocument={document}/>);
  expect(screen.getByLabelText("笔记工作台").style.getPropertyValue("--notebook-font-size")).toBe("16px");
});

it("bounds appearance controls and keeps them scoped without changing a draft", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.change(screen.getByLabelText("正文-primary"), { target: { value: "未提交的正文" } });
  fireEvent.click(screen.getByRole("button", { name: "阅读设置" }));
  const increase = await screen.findByRole("button", { name: "增大阅读字号" });
  for (let count = 0; count < 12; count++) fireEvent.click(increase);
  expect((increase as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByLabelText("当前阅读字号").textContent).toBe("24");
  fireEvent.click(screen.getByRole("button", { name: "阅读行宽：窄" }));
  expect(screen.getByLabelText("笔记工作台").style.getPropertyValue("--notebook-line-width")).toBe("640px");
  expect((screen.getByLabelText("正文-primary") as HTMLTextAreaElement).value).toBe("未提交的正文");
  for (let index = 0; index < localStorage.length; index++) expect(localStorage.getItem(localStorage.key(index)!)).not.toContain("未提交的正文");
  view.rerender(<NotebookWorkspace scope={{ ...scope, actor: "another" }} entries={entries} renderDocument={document}/>);
  expect(screen.queryByRole("dialog", { name: "阅读设置" })).toBeNull();
  expect(screen.getByLabelText("笔记工作台").style.getPropertyValue("--notebook-font-size")).toBe("16px");
});

it("opens notes, protects dirty drafts and shares them across panes", async () => {
  render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.change(screen.getByLabelText("正文-primary"), { target: { value: "私有草稿" } });
  fireEvent.click(screen.getByRole("button", { name: "切换分屏" }));
  expect((screen.getByLabelText("正文-secondary") as HTMLTextAreaElement).value).toBe("私有草稿");
  fireEvent.click(screen.getByRole("button", { name: "关闭 笔记甲" }));
  expect(screen.getByRole("alertdialog")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "取消" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect((screen.getByLabelText("正文-primary") as HTMLTextAreaElement).value).toBe("私有草稿");
  await open("笔记乙");
  expect(screen.getAllByRole("tab")).toHaveLength(2);
});

it("pins and unpins via commands and protects dirty closes from the same panel", async () => {
  render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.click(screen.getByRole("button", { name: "命令面板" }));
  fireEvent.click(screen.getByRole("button", { name: "固定当前笔记" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("tab", { name: /笔记甲/ }).querySelector('[aria-label="已固定"]')).not.toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "命令面板" }));
  fireEvent.click(screen.getByRole("button", { name: "取消固定当前笔记" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("tab", { name: /笔记甲/ }).querySelector('[aria-label="已固定"]')).toBeNull();
  fireEvent.change(screen.getByLabelText("正文-primary"), { target: { value: "keep draft" } });
  fireEvent.click(screen.getByRole("button", { name: "命令面板" }));
  expect((screen.getByRole("button", { name: "取消固定当前笔记" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "关闭当前笔记" }));
  expect(screen.getByRole("alertdialog")).toBeTruthy();
  expect(screen.getByRole("tab", { name: /笔记甲/, hidden: true })).toBeTruthy();
});

it("opens a loaded source note from the directory", async () => {
  render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  expect(screen.getByRole("complementary", { name: "笔记目录侧栏" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "笔记目录" }));
  expect(screen.queryByRole("navigation", { name: "笔记目录列表" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "笔记目录" }));
  expect(screen.getByRole("navigation", { name: "笔记目录列表" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "笔记乙" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("tab", { name: "笔记乙" })).toBeTruthy();
});

it("supports arrow navigation with focus on the selected tab", async () => {
  render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.doubleClick(screen.getByRole("tab", { name: "笔记甲" }));
  await open("笔记乙");
  fireEvent.keyDown(screen.getByRole("tab", { name: "笔记乙" }), { key: "ArrowLeft" });
  const first = screen.getByRole("tab", { name: /笔记甲/ });
  expect(first.getAttribute("aria-selected")).toBe("true");
  expect(window.document.activeElement).toBe(first);
});

it("clears old identity state synchronously when the actor changes", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.change(screen.getByLabelText("正文-primary"), { target: { value: "private" } });
  view.rerender(<NotebookWorkspace scope={{ ...scope, actor: "other" }} entries={entries} renderDocument={document}/>);
  expect(screen.queryByRole("tab")).toBeNull();
  expect(screen.queryByLabelText("正文-primary")).toBeNull();
});

it("purges revoked drafts so reauthorization cannot resurrect sensitive content", async () => {
  const view = render(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  fireEvent.change(screen.getByLabelText("正文-primary"), { target: { value: "private" } });
  view.rerender(<NotebookWorkspace scope={scope} entries={[]} renderDocument={document}/>);
  expect(screen.queryByLabelText("正文-primary")).toBeNull();
  view.rerender(<NotebookWorkspace scope={scope} entries={entries} renderDocument={document}/>);
  await open("笔记甲");
  expect((screen.getByLabelText("正文-primary") as HTMLTextAreaElement).value).toBe("a");
});
