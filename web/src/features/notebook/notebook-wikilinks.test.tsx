import { afterEach, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { parseWikiReference } from "./notebook-wikilinks";
import { NotebookRenderer } from "./notebook-renderer";
afterEach(cleanup);

it("recognizes aliases and explicit heading/block references without resolving titles", () => {
  expect(parseWikiReference("Note#Heading|Alias")).toEqual({ target: "Note", anchor: "Heading", kind: "heading", label: "Alias", embed: false });
  expect(parseWikiReference("Note#^block", true)).toEqual({ target: "Note", anchor: "block", kind: "block", label: "Note#^block", embed: true });
  expect(parseWikiReference("#Local")).toMatchObject({ target: "", anchor: "Local", kind: "heading" });
  expect(parseWikiReference("#^")).toBeUndefined();
  expect(parseWikiReference("")).toBeUndefined();
  expect(parseWikiReference("a\nb")).toBeUndefined();
  expect(parseWikiReference("[nested]")).toBeUndefined();
});

it("preserves malformed bracket runs and multiline text", () => {
  const { container } = render(<NotebookRenderer body={'[[[nested]]]\n\n[[extra]]]\n\n[[line\nbreak]]'}/>);
  expect(container.textContent).not.toContain('未解析引用');
  expect(container.textContent).toContain('[[[nested]]]');
  expect(container.textContent).toContain('[[extra]]]');
});

it("navigates local heading aliases inside only the active renderer", () => {
  const body = '# 中文 标题\n\n[[#中文 标题|跳转]]';
  render(<><NotebookRenderer body={body}/><NotebookRenderer body={body}/></>);
  const route = location.hash;
  fireEvent.click(screen.getAllByRole('button', { name: '跳转' })[1]);
  expect(document.activeElement).toBe(screen.getAllByRole('heading', { name: '中文 标题' })[1]);
  expect(location.hash).toBe(route);
});

it("navigates explicit block anchors without promoting external targets", () => {
  const { container } = render(<NotebookRenderer body={'A paragraph. ^block-one\n\n[[#^block-one|定位区块]]\n\n[[Other#^block-one|其他笔记]]'}/>);
  fireEvent.click(screen.getByRole('button', { name: '定位区块' }));
  expect(document.activeElement?.textContent).toBe('A paragraph.');
  expect(container.querySelector('[data-notebook-anchor="^block-one"]')).toBe(document.activeElement);
  expect(screen.getByText('其他笔记（未解析引用）')).toBeTruthy();
});

it("keeps missing and ambiguous local references unresolved", () => {
  render(<NotebookRenderer body={'# Same\n\n# Same\n\n[[#Same|重复标题]] [[#missing|缺失标题]]\n\nFirst ^duplicate\n\nSecond ^duplicate\n\n[[#^duplicate|重复块]]'}/>);
  for (const label of ['重复标题', '缺失标题', '重复块']) {
    expect(screen.queryByRole('button', { name: label })).toBeNull();
    expect(screen.getByText(`${label}（未解析引用）`)).toBeTruthy();
  }
});

it("keeps block markers in code and links literal and does not expand local embeds", () => {
  const { container } = render(<NotebookRenderer body={'`Code ^literal`\n\n```md\nBlock ^fenced\n```\n\n[Link ^linked](https://example.com)\n\nReal ^real\n\n[[#^literal|代码]] [[#^fenced|围栏]] [[#^linked|链接]] ![[#^real|嵌入]]'}/>);
  for (const label of ['代码', '围栏', '链接', '嵌入']) expect(screen.getByText(`${label}（未解析引用）`)).toBeTruthy();
  expect(container.querySelectorAll('[data-notebook-anchor]')).toHaveLength(1);
  expect(container.querySelector('code')?.textContent).toBe('Code ^literal');
});

it("expands collapsed sections when following a local block reference", () => {
  const { container } = render(<NotebookRenderer body={'[[#^inside|展开区块]]\n\n# Parent\n\n## Child\n\nBody ^inside'}/>);
  const sections = Array.from(container.querySelectorAll('details'));
  sections.forEach(section => { section.open = false; });
  fireEvent.click(screen.getByRole('button', { name: '展开区块' }));
  expect(sections.every(section => section.open)).toBe(true);
  expect(document.activeElement?.textContent).toBe('Body');
});

it("marks unresolved references without interpreting code or ordinary link text", () => {
  const { container } = render(<NotebookRenderer body={'[[Note|Alias]] ![[Other#^block]]\n\n`[[code]]`\n\n```md\n[[fenced]]\n```\n\n[[[invalid]]\n\n[ordinary [[label]]](https://example.com)'}/>);
  expect(screen.getByText('Alias（未解析引用）')).toBeTruthy();
  expect(screen.getByText('Other#^block（未解析引用）')).toBeTruthy();
  expect(container.querySelector('code')?.textContent).toBe('[[code]]');
  expect(container.querySelector('pre code')?.textContent).toContain('[[fenced]]');
  expect(screen.getByRole('link', { name: 'ordinary [[label]]' })).toBeTruthy();
  expect(container.querySelector('img')).toBeNull();
  expect(screen.getByText('[[[invalid]]')).toBeTruthy();
});
