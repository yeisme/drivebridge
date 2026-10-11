import { lazy, Suspense, useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { ChevronDown, ListTree } from "lucide-react";
import { Button } from "../../components/ui/button";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import type { Options } from "react-markdown";
import { notebookWikiLinks, type WikiReference } from "./notebook-wikilinks";

const NotebookDiagram = lazy(() => import("./NotebookDiagram").then(module => ({ default: module.NotebookDiagram })));

interface MarkdownNode {
  type: string;
  value?: string;
  depth?: number;
  children?: MarkdownNode[];
  url?: string;
  ordered?: boolean;
  lang?: string;
  data?: { hName?: string; hProperties?: Record<string, unknown> };
}
export interface NotebookHeading { anchor: string; title: string; depth: number }
function plainText(node: MarkdownNode): string {
  return node.value ?? node.children?.map(plainText).join("") ?? "";
}

// Applied to the same parse tree used by the renderer; no second regex parser.
export function notebookAnchors(tree: MarkdownNode): NotebookHeading[] {
  const used = new Set<string>();
  const headings: NotebookHeading[] = [];
  function visit(node: MarkdownNode) {
    if (node.type === "heading") {
      const title = plainText(node);
      const base = title.normalize("NFKC").toLocaleLowerCase().replace(/[^\p{L}\p{N}_-]+/gu, "-").replace(/^-|-$/g, "") || "section";
      let anchor = base;
      let suffix = 1;
      while (used.has(anchor)) anchor = `${base}-${suffix++}`;
      used.add(anchor);
      node.data = { ...node.data, hProperties: { ...node.data?.hProperties, id: anchor } };
      headings.push({ anchor, title, depth: node.depth ?? 1 });
    }
    node.children?.forEach(visit);
  }
  visit(tree);
  return headings;
}
// Nest sections according to heading depth, leaving code and list contents
// untouched. Native details provides keyboard disclosure without DOM mutation.
export function notebookSections(nodes: MarkdownNode[]): MarkdownNode[] {
  const result: MarkdownNode[] = [];
  const stack: { depth: number; children: MarkdownNode[] }[] = [];
  for (const node of nodes) {
    if (node.type === "heading") {
      const depth = node.depth ?? 1;
      while (stack.length && stack[stack.length - 1].depth >= depth) stack.pop();
      const children: MarkdownNode[] = [{ type: "paragraph", data: { hName: "summary" }, children: [node] }];
      const section: MarkdownNode = {
        type: "blockquote", data: { hName: "details", hProperties: { open: true, "data-section-anchor": node.data?.hProperties?.id } }, children,
      };
      (stack.at(-1)?.children ?? result).push(section);
      stack.push({ depth, children });
    } else (stack.at(-1)?.children ?? result).push(node);
  }
  return result;
}

function foldLists(node: MarkdownNode) {
  if (!node.children) return;
  node.children = node.children.map(child => {
    foldLists(child);
    if (child.type !== "list") return child;
    return {
      type: "blockquote", data: { hName: "details", hProperties: { open: true } },
      children: [
        { type: "paragraph", data: { hName: "summary" }, children: [{ type: "text", value: `列表（${child.children?.length ?? 0} 项）` }] },
        child,
      ],
    };
  });
}

// Resolve only same-document references against the renderer's own AST.
// Cross-document identities and permissions remain the source owner's concern.
function resolveLocalReferences(tree: MarkdownNode, headings: NotebookHeading[]) {
  const blocks = new Map<string, MarkdownNode[]>();
  const walk = (node: MarkdownNode, visit: (node: MarkdownNode) => void) => {
    visit(node);
    node.children?.forEach(child => walk(child, visit));
  };
  walk(tree, node => {
    const last = node.children?.at(-1);
    if (node.type !== "paragraph" || last?.type !== "text" || last.data) return;
    const match = last.value?.match(/(?:^|\s)\^([A-Za-z0-9-]+)\s*$/);
    if (match) blocks.set(match[1], [...(blocks.get(match[1]) ?? []), node]);
  });
  for (const [id, nodes] of blocks) {
    if (nodes.length !== 1) continue;
    const node = nodes[0];
    const last = node.children!.at(-1)!;
    last.value = last.value!.replace(/(?:^|\s)\^[A-Za-z0-9-]+\s*$/, "").trimEnd();
    node.data = { ...node.data, hProperties: { ...node.data?.hProperties, "data-notebook-anchor": `^${id}`, tabIndex: -1 } };
  }
  walk(tree, node => {
    const raw = node.data?.hProperties?.["data-wiki-reference"];
    if (typeof raw !== "string") return;
    const ref = JSON.parse(raw) as WikiReference;
    if (ref.target || ref.embed || !ref.anchor) return;
    let anchor: string | undefined;
    if (ref.kind === "block" && blocks.get(ref.anchor)?.length === 1) anchor = `^${ref.anchor}`;
    if (ref.kind === "heading") {
      const matching = headings.filter(heading => heading.title === ref.anchor);
      if (matching.length === 1) anchor = matching[0].anchor;
      else if (!matching.length) anchor = headings.find(heading => heading.anchor === ref.anchor)?.anchor;
    }
    if (!anchor) return;
    node.type = "link";
    node.url = `#${encodeURIComponent(anchor)}`;
    node.children = [{ type: "text", value: ref.label }];
    delete node.value;
    delete node.data;
  });
}

function anchorPlugin() {
  return (tree: MarkdownNode) => {
    let diagramCount = 0, mathCount = 0, mathBytes = 0;
    const diagrams = (node: MarkdownNode) => {
      if (node.type === "math" || node.type === "inlineMath") {
        const value = node.value ?? "";
        mathBytes += value.length;
        if (++mathCount > 200 || value.length > 4096 || mathBytes > 32768) {
          const block = node.type === "math";
          node.type = block ? "code" : "inlineCode";
          node.data = { hProperties: { title: "公式超出渲染预算，保留源码" } };
        }
      }
      if (node.type === "code" && node.lang === "mermaid" && ++diagramCount <= 8) {
        node.data = { hName: "div", hProperties: { "data-notebook-diagram": node.value ?? "" } };
        node.type = "paragraph"; node.children = []; delete node.value; delete node.lang;
      }
      node.children?.forEach(diagrams);
    };
    diagrams(tree);
    const headings = notebookAnchors(tree);
    resolveLocalReferences(tree, headings);
    foldLists(tree);
    if (!headings.length) return;
    tree.children = notebookSections(tree.children ?? []);
    tree.children.unshift({
      type: "list", ordered: false,
      data: { hName: "nav", hProperties: { "aria-label": "文档大纲" } },
      children: headings.map(heading => ({
        type: "listItem", children: [{ type: "paragraph", children: [{
          type: "link", url: `#${encodeURIComponent(heading.anchor)}`,
          children: [{ type: "text", value: heading.title || "未命名标题" }],
        }] }],
      })),
    });
  };
}

export function safeNotebookLink(url: string): string | undefined {
  if (url.startsWith("#")) return url;
  try {
    const parsed = new URL(url);
    return ["https:", "http:", "mailto:"].includes(parsed.protocol) ? parsed.href : undefined;
  } catch { return undefined; }
}

function NotebookOutline({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  return <div className="mb-3 text-[13px]">
    <Button type="button" variant="ghost" className="min-h-11 gap-2 px-2 text-muted-foreground md:min-h-8" aria-expanded={open} aria-controls={id} onClick={() => setOpen(value => !value)}>
      <ListTree className="size-4"/>文档大纲<ChevronDown className={open ? "size-4 rotate-180" : "size-4"}/>
    </Button>
    <nav id={id} aria-label="文档大纲" hidden={!open} className="mt-1 rounded-md border border-border p-2"><ol className="m-0">{children}</ol></nav>
  </div>;
}

export function NotebookRenderer({ body, onAnchor }: { body: string; onAnchor?: (anchor: string) => void }) {
  const root = useRef<HTMLDivElement>(null);
  const prefix = useId();
  const [mathPlugins, setMathPlugins] = useState<Options["rehypePlugins"]>([]);
  useEffect(() => {
    if (!body.includes("$")) return;
    let active = true;
    void Promise.all([import("rehype-katex"), import("katex/dist/katex.min.css")]).then(([module]) => {
      if (active) setMathPlugins([[module.default, { trust: false, strict: "error", throwOnError: false, maxExpand: 1000, maxSize: 20 }]]);
    }).catch(() => { /* The original math source stays visible if loading fails. */ });
    return () => { active = false; };
  }, [body]);
  // Memoizing the element avoids reparsing while a pane divider is dragged.
  return useMemo(() => {
    const heading = (Tag: "h1" | "h2" | "h3" | "h4" | "h5" | "h6") => ({ id, children }: { id?: string; children?: React.ReactNode }) =>
      <Tag id={`${prefix}-${id}`} data-notebook-anchor={id} tabIndex={-1}>{children}</Tag>;
    return <div ref={root} style={{ fontSize: "var(--notebook-font-size, 16px)", maxWidth: "var(--notebook-line-width, 720px)", lineHeight: 1.75 }} className="mx-auto min-w-0 break-words [&_pre]:my-5 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:bg-muted [&_pre]:p-4 [&_pre]:text-sm [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-[0.9em] [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_table]:my-5 [&_table]:block [&_table]:max-w-full [&_table]:overflow-x-auto [&_table]:border-collapse [&_th]:border [&_th]:bg-muted/50 [&_th]:text-left [&_th]:font-medium [&_td]:border [&_th]:px-3 [&_th]:py-2 [&_td]:px-3 [&_td]:py-2 [&_h1]:text-[2em] [&_h1]:font-bold [&_h1]:leading-tight [&_h1]:tracking-tight [&_h2]:text-[1.5em] [&_h2]:font-semibold [&_h2]:leading-snug [&_h3]:text-[1.25em] [&_h3]:font-semibold [&_h4]:font-semibold [&_p]:my-3 [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1 [&_blockquote]:my-5 [&_blockquote]:border-l-[3px] [&_blockquote]:border-foreground/30 [&_blockquote]:pl-4 [&_blockquote]:text-muted-foreground [&_hr]:my-8 [&_hr]:border-border [&_a]:underline [&_a]:decoration-border [&_a]:underline-offset-4 [&_a:hover]:decoration-foreground [&_[data-notebook-anchor]]:scroll-mt-16">
    <Markdown remarkPlugins={[remarkGfm, remarkMath, notebookWikiLinks, anchorPlugin]} rehypePlugins={mathPlugins} skipHtml components={{
      div: ({ node, children, ...props }) => {
        const source = node?.properties?.dataNotebookDiagram ?? node?.properties?.["data-notebook-diagram"];
        return typeof source === "string"
          ? <Suspense fallback={<pre className="overflow-x-auto">{source}</pre>}><NotebookDiagram source={source}/></Suspense>
          : <div {...props}>{children}</div>;
      },
      span: ({ node, children, ...props }) => (node?.properties?.dataWikiReference ?? node?.properties?.["data-wiki-reference"])
        ? <span className="text-muted-foreground" title="当前来源尚未提供引用解析能力">{children}（未解析引用）</span>
        : <span {...props}>{children}</span>,
      details: ({ children, ...props }) => <details open data-section-anchor={(props.node?.properties?.dataSectionAnchor ?? props.node?.properties?.["data-section-anchor"]) as string | undefined} className="my-4 min-w-0 [&_details[data-section-anchor]]:mt-7">{children}</details>,
      summary: ({ children }) => <summary className="min-h-11 cursor-pointer rounded-sm py-2 marker:text-muted-foreground/50 focus-visible:outline-2 focus-visible:outline-ring [&_h1]:inline [&_h2]:inline [&_h3]:inline [&_h4]:inline [&_h5]:inline [&_h6]:inline">{children}</summary>,
      nav: ({ children }) => <NotebookOutline>{children}</NotebookOutline>,
      h1: heading("h1"), h2: heading("h2"), h3: heading("h3"), h4: heading("h4"), h5: heading("h5"), h6: heading("h6"),
      img: ({ alt }) => <span role="note" className="text-muted-foreground">图片未自动加载：{alt || "未命名图片"}</span>,
      a: ({ href, children }) => {
        const safe = href ? safeNotebookLink(href) : undefined;
        if (!safe) return <span>{children}<span className="text-muted-foreground">（链接不可用）</span></span>;
        if (safe.startsWith("#")) return <Button type="button" variant="link" className="h-auto min-h-11 whitespace-normal text-left" onClick={() => {
          let anchor = safe.slice(1);
          try { anchor = decodeURIComponent(anchor); } catch { /* Malformed anchors remain unresolved. */ }
          const target = Array.from(root.current?.querySelectorAll<HTMLElement>("[data-notebook-anchor]") ?? []).find(node => node.dataset.notebookAnchor === anchor);
          let section = target?.closest("details");
          while (section && root.current?.contains(section)) { section.open = true; section = section.parentElement?.closest("details") ?? null; }
          target?.scrollIntoView?.({ block: "start", behavior: "instant" });
          target?.focus({ preventScroll: true });
          onAnchor?.(anchor);
        }}>{children}</Button>;
        return <a href={safe} rel="noopener noreferrer" target="_blank">{children}</a>;
      },
    }}>{body}</Markdown>
  </div>;
  }, [body, onAnchor, prefix, mathPlugins]);
}
