import { useEffect, useState } from "react";
import { Folder, ArrowLeft, ArrowRight } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Btn, IconBtn, useDemo } from "./ui";
import { largeCatalogPage } from "./large-catalog";
import { entryRef } from "./organization-model";
import { FavoriteButton } from "./FavoritesWorkspace";
import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";
import { FileToolbar } from "./CloudToolbar";
export function LargeCatalog() {
  const { state: s, dispatch, open } = useDemo();
  const [parent, setParent] = useState<number | null>(null),
    [page, setPage] = useState(1),
    [input, setInput] = useState(""),
    [query, setQuery] = useState(""),
    [composing, setComposing] = useState(false);
  useEffect(() => {
    if (composing) return;
    const t = setTimeout(() => {
      setQuery(input);
      setPage(1);
    }, 250);
    return () => clearTimeout(t);
  }, [input, composing]);
  const data = largeCatalogPage(s, parent, page, query),
    key = [s.space, s.activeConnection, parent, page, query].join(":");
  useEffect(() => {
    dispatch({
      type: "large-load",
      files: data.files,
      directories: data.directories,
    });
  }, [key]);
  const files = data.files
    .map((f) => s.files.find((x) => x.id === f.id))
    .filter(Boolean) as typeof data.files;
  return (
    <section
      className="flex h-full min-h-0 flex-col p-3"
      aria-label="万级目录演示"
    >
      <header className="flex h-11 shrink-0 items-center gap-2">
        <Btn
          variant="ghost"
          onClick={() => dispatch({ type: "large-mode", value: false })}
        >
          <ArrowLeft size={14} />
          返回原目录
        </Btn>
        <h1 className="min-w-0 flex-1 truncate text-sm font-semibold">
          {data.parent?.name || "规模演示根目录"}
        </h1>
      </header>
      <form
        className="flex h-11 shrink-0 gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (!composing) {
            setQuery(input);
            setPage(1);
          }
        }}
      >
        <Input
          aria-label="规模场景检索"
          value={input}
          onCompositionStart={() => setComposing(true)}
          onCompositionEnd={() => setComposing(false)}
          onChange={(e) => setInput(e.target.value)}
          placeholder="检索 10,000 个模拟文件，例如 00999"
        />
        <Btn type="submit">检索</Btn>
      </form>
      <p className="py-2 text-xs text-muted-foreground">
        1,000 个目录 × 10 个文件 · 仅物化本次页 · 不代表真实 Provider 性能
      </p>
      <FileToolbar
        count={data.total}
        shown={files.map((f) => f.id)}
        page={page}
        pages={Math.max(1, Math.ceil(data.total / 50))}
      />
      <div
        className="min-h-0 flex-1 overflow-auto rounded-lg border bg-card"
        data-large-list
      >
        {files.length ? (
          files.map((f) => (
            <article
              key={f.id}
              data-large-entry
              className="flex h-16 items-center gap-2 border-b px-3"
            >
              <FileButton
                reference={fileRef(f)}
                variant="ghost"
                className="min-w-0 flex-1 !justify-start"
                onOpen={() =>
                  open({
                    type: "preview",
                    id: f.id,
                    scope: {
                      label: "规模演示 · 当前页",
                      refs: files.map((f) => fileRef(f)),
                    },
                  })
                }
              >
                <span className="truncate">{f.name}</span>
              </FileButton>
              <FavoriteButton reference={entryRef(s, f.id)!} label={f.name} />
            </article>
          ))
        ) : !query && parent === null ? (
          data.directories.map((n) => (
            <div
              key={n.id}
              data-large-entry
              className="flex h-14 items-center gap-2 border-b px-3"
            >
              <button
                className="flex h-11 min-w-0 flex-1 items-center gap-2 text-left text-sm"
                onClick={() => {
                  setParent(Number(n.id.split(":d").at(-1)));
                  setPage(1);
                }}
              >
                <Folder size={18} />
                {n.name}
              </button>
              {entryRef(s, n.id, "directory") && (
                <FavoriteButton
                  reference={entryRef(s, n.id, "directory")!}
                  label={n.name}
                />
              )}
            </div>
          ))
        ) : (
          <p className="p-4 text-xs">
            {data.total ? "加载本页…" : "模拟范围没有匹配条目"}
          </p>
        )}
      </div>
      <footer className="flex h-11 shrink-0 items-center justify-end gap-2 text-xs">
        {parent !== null && (
          <Btn
            variant="ghost"
            onClick={() => {
              setParent(null);
              setInput("");
              setQuery("");
              setPage(1);
            }}
          >
            返回规模根目录
          </Btn>
        )}
        <span className="mr-auto">
          第 {page} 页 · {data.total} 条
        </span>
        <IconBtn
          label="规模上一页"
          disabled={page === 1}
          onClick={() => setPage(page - 1)}
        >
          <ArrowLeft size={16} />
        </IconBtn>
        <IconBtn
          label="规模下一页"
          disabled={page * 50 >= data.total}
          onClick={() => setPage(page + 1)}
        >
          <ArrowRight size={16} />
        </IconBtn>
      </footer>
    </section>
  );
}
