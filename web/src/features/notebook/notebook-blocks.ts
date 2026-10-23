export interface NotebookBlockCommand {
  id: string;
  label: string;
  snippet: string;
}

export const notebookBlockCommands: readonly NotebookBlockCommand[] = [
  { id: "heading", label: "标题", snippet: "## " },
  { id: "bullet", label: "项目列表", snippet: "- " },
  { id: "task", label: "任务", snippet: "- [ ] " },
  { id: "quote", label: "引用", snippet: "> " },
  { id: "callout", label: "Callout", snippet: "> [!NOTE]\n> " },
  { id: "code", label: "代码块", snippet: "```\n\n```" },
  { id: "table", label: "表格", snippet: "| 列 1 | 列 2 |\n| --- | --- |\n|  |  |" },
  { id: "math", label: "公式", snippet: "$$\n\n$$" },
  { id: "mermaid", label: "Mermaid 图表", snippet: "```mermaid\ngraph TD\n  A[开始] --> B[下一步]\n```" },
  { id: "wikilink", label: "双链", snippet: "[[笔记标题]]" },
  { id: "attachment", label: "附件引用", snippet: "![[附件名称]]" },
];

export function notebookBlockInsert(value: string, cursor: number, snippet: string): { from: number; to: number; insert: string } {
  const lineStart = value.lastIndexOf("\n", Math.max(0, cursor - 1)) + 1;
  const current = value.slice(lineStart, cursor);
  if (current.trim() === "/") return { from: lineStart, to: cursor, insert: snippet };
  return { from: lineStart, to: cursor, insert: `${current.trim() ? "\n" : ""}${snippet}` };
}
