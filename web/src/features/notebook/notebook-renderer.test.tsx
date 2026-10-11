import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NotebookRenderer, notebookAnchors, safeNotebookLink } from "./notebook-renderer";
afterEach(cleanup);

it("renders GFM without executing raw HTML or fetching external images", () => {
  const { container } = render(<NotebookRenderer body={'# 标题\n\n<script>alert(1)</script>\n\n<img src="https://tracker.invalid/pixel">\n\n![外部图片](https://tracker.invalid/image.png)\n\n|列|值|\n|--|--|\n|a|b|\n\n- [x] 完成'}/>);
  expect(container.querySelector("script")).toBeNull();
  expect(container.querySelector("img")).toBeNull();
  expect(screen.getByText("图片未自动加载：外部图片")).toBeTruthy();
  expect(screen.getByRole("table")).toBeTruthy();
  expect((screen.getByRole("checkbox") as HTMLInputElement).disabled).toBe(true);
});

it("wraps lists without changing ordered starts, task state, or nested items", () => {
  const { container } = render(<NotebookRenderer body={'3. Third\n4. Fourth\n\n- [x] Done\n- Parent\n  - Child'}/>);
  expect(container.querySelector('ol')?.getAttribute('start')).toBe('3');
  expect(container.querySelectorAll('details')).toHaveLength(3);
  expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);
  const child = screen.getByText('Child');
  expect(child.closest('li')).not.toBeNull();
  expect(child.closest('details')?.querySelector('summary')?.textContent).toBe('列表（1 项）');
  expect(screen.queryByRole('navigation', { name: '文档大纲' })).toBeNull();
});

it("assigns deterministic unique anchors even when headings contain suffixes", () => {
  const body = "# Hello\n# Hello\n# Hello-1\n# Hello\n# 中文 标题";
  const view = render(<NotebookRenderer body={body}/>);
  const ids = () => screen.getAllByRole("heading").map(node => node.dataset.notebookAnchor);
  expect(ids()).toEqual(["hello", "hello-1", "hello-1-1", "hello-2", "中文-标题"]);
  view.rerender(<NotebookRenderer body={body}/>);
  expect(new Set(ids()).size).toBe(5);
  expect(notebookAnchors({ type: "root", children: [{ type: "heading", depth: 2, children: [{ type: "text", value: "Outline" }] }] })).toEqual([{ anchor: "outline", title: "Outline", depth: 2 }]);
});

it("keeps the outline collapsed until requested without hiding the document", () => {
  render(<NotebookRenderer body={"# Heading\n\nBody"}/>);
  const toggle = screen.getByRole("button", { name: "文档大纲" });
  expect(toggle.getAttribute("aria-expanded")).toBe("false");
  expect(screen.queryByRole("navigation", { name: "文档大纲" })).toBeNull();
  expect(screen.getByRole("heading", { name: "Heading" })).toBeTruthy();
  fireEvent.click(toggle);
  expect(toggle.getAttribute("aria-expanded")).toBe("true");
  expect(screen.getByRole("navigation", { name: "文档大纲" })).toBeTruthy();
  fireEvent.click(toggle);
  expect(screen.queryByRole("navigation", { name: "文档大纲" })).toBeNull();
});

it("scopes outline navigation to its own pane without rewriting the app route", () => {
  const { container } = render(<><NotebookRenderer body="# Shared"/><NotebookRenderer body="# Shared"/></>);
  const headings = screen.getAllByRole("heading", { name: "Shared" });
  expect(headings[0].id).not.toBe(headings[1].id);
  const route = location.hash;
  for (const toggle of screen.getAllByRole("button", { name: "文档大纲" })) fireEvent.click(toggle);
  fireEvent.click(screen.getAllByRole("button", { name: "Shared" })[1]);
  expect(document.activeElement).toBe(headings[1]);
  expect(location.hash).toBe(route);
  expect(container.querySelectorAll('nav[aria-label="文档大纲"]')).toHaveLength(2);
});

it("groups nested sections and expands collapsed ancestors for outline navigation", () => {
  const { container } = render(<NotebookRenderer body={'# Parent\n\nparent body\n\n## Child\n\nchild body\n\n# Sibling\n\nsibling body\n\n```md\n# Not a heading\n```'}/>);
  const sections = container.querySelectorAll("details");
  expect(sections).toHaveLength(3);
  expect(sections[0].contains(sections[1])).toBe(true);
  expect(sections[0].contains(sections[2])).toBe(false);
  sections[0].open = false;
  sections[1].open = false;
  fireEvent.click(screen.getByRole("button", { name: "文档大纲" }));
  fireEvent.click(screen.getByRole("button", { name: /^Child$/ }));
  expect(sections[0].open).toBe(true);
  expect(sections[1].open).toBe(true);
  expect(document.activeElement?.textContent).toBe("Child");
  expect(container.querySelectorAll("summary")).toHaveLength(3);
});

it("blocks unsafe and unresolved paths while routing fragment clicks", () => {
  for (const url of ["javascript:alert(1)", "data:text/html,bad", "file:///secret", "//tracker.invalid", "../private"]) expect(safeNotebookLink(url)).toBeUndefined();
  const onAnchor = vi.fn();
  const { container } = render(<NotebookRenderer body="[bad](javascript:alert%281%29) [good](https://example.com) [jump](#intro)" onAnchor={onAnchor}/>);
  expect(container.querySelector('a[href^="javascript"]')).toBeNull();
  expect(screen.getByRole("link", { name: "good" }).getAttribute("rel")).toBe("noopener noreferrer");
  fireEvent.click(screen.getByRole("button", { name: "jump" }));
  expect(onAnchor).toHaveBeenCalledWith("intro");
});

it("renders formulas while rejecting trusted HTML and external math resources", async () => {
  const { container } = render(<NotebookRenderer body={'Inline $E=mc^2$\n\n$$\\frac{a}{b}$$\n\n$\\href{javascript:alert(1)}{click}$\n\n$\\includegraphics{https://tracker.invalid/pixel}$'}/>);
  await waitFor(() => expect(container.querySelectorAll('.katex')).toHaveLength(4));
  expect(container.querySelector('math')).not.toBeNull();
  expect(container.querySelector('a, img, script, iframe')).toBeNull();
  expect(container.querySelector('.katex-html')).not.toBeNull();
});

it("preserves formula source beyond the document rendering budget", async () => {
  const { container } = render(<NotebookRenderer body={Array.from({ length: 201 }, (_, index) => `$x_${index}$`).join('\n\n')}/>);
  await waitFor(() => expect(container.querySelectorAll('.katex')).toHaveLength(200));
  expect(container.querySelector('code[title="公式超出渲染预算，保留源码"]')?.textContent).toBe('x_200');
});
