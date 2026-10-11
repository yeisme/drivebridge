import { bodyWithoutProperties } from "./properties";
import { createContext, useContext } from "react";
import type { Components } from "react-markdown";
import Markdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import { FileButton } from "./HoverPreview";
import { FileArt, useDemo } from "./ui";
import type { FileRecord } from "./model";
import {
  parseNoteLinks,
  resolveNoteLink,
  markdownTasks,
} from "./knowledge-model";
import { fileRef } from "./preview-model";
export function linkedMarkdown(body: string) {
  let output = body;
  for (const link of parseNoteLinks(body).reverse()) {
    const url = `note:${encodeURIComponent(link.target + (link.anchor ? "#" + link.anchor : ""))}`;
    output =
      output.slice(0, link.from) +
      `${link.embed ? "!" : ""}[${link.label.replace(/[\[\]]/g, "")}](${url})` +
      output.slice(link.to);
  }
  return output;
}
export function NoteReference({
  source,
  target,
  label,
  embed = false,
}: {
  source: FileRecord;
  target: string;
  label: string;
  embed?: boolean;
}) {
  const { state: s, open } = useDemo();
  let decoded = target.replace(/^note:/, "");
  try {
    decoded = decodeURIComponent(decoded);
  } catch {}
  const [path, anchor = ""] = decoded.split("#");
  const resolved = resolveNoteLink(s, source, {
    target: path,
    anchor,
    label,
    embed,
    raw: "",
    from: 0,
    to: 0,
  });
  const f = resolved.file;
  if (!f || resolved.status !== "可访问")
    return (
      <span
        className="inline-block rounded border border-dashed border-border px-2 py-1 text-xs text-muted-foreground"
        title={resolved.status}
      >
        {label || path} · {resolved.status}
      </span>
    );
  return (
    <FileButton
      reference={fileRef(f)}
      onOpen={() =>
        open({
          type: "preview",
          id: f.id,
          version: f.version,
          single: true,
          anchor,
        })
      }
      variant="ghost"
      className={
        embed
          ? "!my-3 !h-auto !w-full !max-w-lg !flex-col !items-stretch !whitespace-normal !p-0 overflow-hidden rounded-lg border border-border text-left"
          : "!inline-flex !h-auto !min-h-0 max-[767px]:!min-h-11 !min-w-0 !whitespace-normal !px-0.5 !py-0 text-primary underline decoration-primary/35 underline-offset-4"
      }
    >
      {embed && (
        <span className="block h-36 overflow-hidden bg-muted [&>div]:h-full">
          <FileArt file={f} version={f.version} />
        </span>
      )}
      <span className={embed ? "block p-3 text-sm" : ""}>
        {label || f.name}
        {embed && (
          <small className="mt-1 block text-muted-foreground">
            {f.size} · 跟随最新 v{f.version} ·{" "}
            {s.connections.find((c) => c.id === f.connectionId)?.name ||
              "来源未知"}
            {["audio", "video"].includes(f.kind) && " · 点击预览，不自动播放"}
          </small>
        )}
      </span>
    </FileButton>
  );
}
const MarkdownContext = createContext<{
  file: FileRecord;
  tasks: ReturnType<typeof markdownTasks>;
  onTaskToggle?: (index: number, checked: boolean) => void;
}>(null!);
function numberTaskInputs() {
  return (tree: unknown) => {
    type Node = {
      tagName?: string;
      properties?: Record<string, unknown>;
      children?: Node[];
    };
    let index = 0;
    const visit = (node: Node) => {
      if (node.tagName === "input" && node.properties?.type === "checkbox")
        node.properties["data-task-index"] = index++;
      node.children?.forEach(visit);
    };
    visit(tree as Node);
  };
}
const markdownComponents: Components = {
  input: function TaskInput({ checked, node }) {
    const { tasks, onTaskToggle } = useContext(MarkdownContext);
    const index = Number(node?.properties["data-task-index"] ?? 0);
    return (
      <label className="inline-flex min-h-11 min-w-11 items-center justify-center align-middle">
        <input
          type="checkbox"
          checked={!!checked}
          disabled={!onTaskToggle}
          aria-label={tasks[index]?.label || `任务 ${index + 1}`}
          onChange={(e) => onTaskToggle?.(index, e.target.checked)}
        />
      </label>
    );
  },
  p: ({ children }) => <div className="my-3">{children}</div>,
  a: function Link({ href, children }) {
    const { file } = useContext(MarkdownContext);
    return href?.startsWith("note:") ? (
      <NoteReference source={file} target={href} label={String(children)} />
    ) : (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-primary underline"
      >
        {children}
      </a>
    );
  },
  img: function Image({ src, alt }) {
    const { file } = useContext(MarkdownContext);
    return typeof src === "string" && src.startsWith("note:") ? (
      <NoteReference source={file} target={src} label={alt || ""} embed />
    ) : (
      <span className="block rounded-lg border border-border bg-muted p-5 text-xs">
        外部图片：{alt || "未命名"} · 演示不加载远端内容
      </span>
    );
  },
  h1: ({ children }) => <h1 id={String(children)}>{children}</h1>,
  h2: ({ children }) => <h2 id={String(children)}>{children}</h2>,
  h3: ({ children }) => <h3 id={String(children)}>{children}</h3>,
};
export function MarkdownBody({
  body,
  file,
  onTaskToggle,
}: {
  body: string;
  file: FileRecord;
  onTaskToggle?: (index: number, checked: boolean) => void;
}) {
  return (
    <MarkdownContext.Provider
      value={{ file, tasks: markdownTasks(body), onTaskToggle }}
    >
      <article
        data-markdown-reading
        className="mx-auto w-full max-w-[760px] break-words text-[15px] leading-8 text-foreground [&_h1]:mb-6 [&_h1]:mt-3 [&_h1]:text-3xl [&_h1]:font-semibold [&_h2]:mb-3 [&_h2]:mt-9 [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:mt-6 [&_h3]:font-semibold [&_blockquote]:my-5 [&_blockquote]:border-l-2 [&_blockquote]:border-primary [&_blockquote]:pl-4 [&_blockquote]:text-muted-foreground [&_ul]:my-4 [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:list-decimal [&_ol]:pl-6 [&_pre]:my-5 [&_pre]:overflow-auto [&_pre]:rounded-lg [&_pre]:bg-muted [&_pre]:p-4 [&_code]:break-all [&_code]:font-mono [&_code]:text-[13px] [&_table]:my-4 [&_table]:w-full [&_td]:border [&_td]:border-border [&_td]:p-2 [&_th]:border [&_th]:border-border [&_th]:bg-muted [&_th]:p-2 [&_input]:accent-[var(--primary)]"
      >
        <Markdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[numberTaskInputs]}
          skipHtml
          urlTransform={(url) =>
            url.startsWith("note:") ? url : defaultUrlTransform(url)
          }
          components={markdownComponents}
        >
          {linkedMarkdown(bodyWithoutProperties(body))}
        </Markdown>
      </article>
    </MarkdownContext.Provider>
  );
}
