import { browseLocation, sortEntries } from "./browse-model";
import { FileDetailsList } from "./FileDetailsList";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Folder,
  Star,
  Columns3,
  SlidersHorizontal,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Btn, IconBtn, Choice, Empty, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { emptySearch } from "./cloud-model";
import type { SearchSpec } from "./cloud-model";
import type { FileRecord } from "./model";
import {
  bookmarkFor,
  findEntries,
  pageEntries,
  propertyState,
  conditionMatches,
  entryUnavailable,
} from "./organization-model";
import type { FindEntry, PropertyCondition } from "./organization-model";
import { displayProperty, parsePropertyInput, typeMatches } from "./properties";
import { FavoriteButton } from "./FavoritesWorkspace";
import { PropertyInput } from "./PropertyWorkspace";
import { FileButton } from "./HoverPreview";
import { FileToolbar } from "./CloudToolbar";
import { fileRef } from "./preview-model";
import { writableNote, isMarkdown } from "./knowledge-model";
import { canEditFile } from "./v4-model";
export function useFind(q: SearchSpec, composing = false, immediate = 0) {
  const { state: s } = useDemo();
  const [found, setFound] = useState<FindEntry[]>([]),
    [pending, setPending] = useState(false);
  const [resultKey, setResultKey] = useState("");
  const lastImmediate = useRef(immediate);
  const generation = useRef(0),
    latest = useRef(s);
  latest.current = s;
  const key = JSON.stringify([s.space, s.actor, q, immediate]);
  useEffect(() => {
    const request = ++generation.current;
    const runNow = lastImmediate.current !== immediate;
    lastImmediate.current = immediate;
    if (composing) {
      setPending(false);
      return;
    }
    setPending(true);
    const timer = setTimeout(
      () => {
        if (request !== generation.current) return;
        setFound(findEntries(latest.current, q));
        setResultKey(key);
        setPending(false);
      },
      runNow ? 0 : 250,
    );
    return () => {
      clearTimeout(timer);
      generation.current++;
    };
  }, [key, composing, immediate]);
  // Recheck access at display time; keep stable ranking while stars/properties change.
  return {
    found: (resultKey === key ? found : []).map((r) => ({
      ...r,
      available:
        entryUnavailable(s, r.ref) || bookmarkFor(s, r.ref)?.invalid || "",
      file: r.file, // Keep the observed version frozen until the next query.
    })),
    pending,
    ready: resultKey === key && !pending && !composing,
  };
}
export function EntryList({
  entries,
  label = "检索结果",
  query = "",
  sort,
  onSort,
}: {
  entries: FindEntry[];
  label?: string;
  query?: string;
  sort?: string;
  onSort?: (value: string) => void;
}) {
  return (
    <FileDetailsList
      entries={entries}
      label={label}
      query={query}
      sort={sort}
      onSort={onSort}
      crossConnections
    />
  );
}
export function PropertyTable({
  files,
  label = "属性表格",
}: {
  files: FileRecord[];
  label?: string;
}) {
  const { state: s, dispatch, open, compact } = useDemo(),
    o = s.organization;
  const [config, setConfig] = useState(false),
    [edit, setEdit] = useState<{ id: string; key: string } | null>(null),
    [revision, setRevision] = useState(0);
  const keys = o.columns.filter((key) =>
    o.definitions[s.space].some((d) => d.key === key),
  );
  const defs = o.definitions[s.space];

  const ordered = useMemo(
    () =>
      [...files].sort((a, b) =>
        o.sortKey === "name"
          ? a.name.localeCompare(b.name, "zh-CN")
          : displayProperty(
              propertyState(s, a).values[o.sortKey],
            ).localeCompare(
              displayProperty(propertyState(s, b).values[o.sortKey]),
              "zh-CN",
              { numeric: true },
            ),
      ),
    [files.map((f) => f.id).join("|"), o.sortKey, revision],
  );
  const frozenGroups = useMemo(() => {
    const members = new Map<string, string[]>();
    for (const f of ordered) {
      const p = propertyState(s, f);
      const v = o.groupBy ? p.values[o.groupBy] : "全部文件";
      const groups =
        p.status !== "ready" && o.groupBy
          ? [p.status === "invalid" ? "解析失败" : "未扫描或过期"]
          : Array.isArray(v) && v.length
            ? v.map(String)
            : [displayProperty(v)];
      for (const group of new Set(groups)) {
        const list = members.get(group) || [];
        list.push(f.id);
        members.set(group, list);
      }
    }
    return members;
  }, [ordered, o.groupBy, revision]);
  const grouped = new Map(
    [...frozenGroups].map(([group, ids]) => [
      group,
      ids
        .map((id) => files.find((f) => f.id === id))
        .filter((f): f is FileRecord => !!f),
    ]),
  );
  const name = (f: FileRecord) => (
    <FileButton
      reference={fileRef(f)}
      variant="ghost"
      className="!h-11 !max-w-full !justify-start !px-0"
      onOpen={() =>
        open({
          type: "preview",
          id: f.id,
          scope: { label, refs: files.map((f) => fileRef(f)) },
        })
      }
    >
      <span className="truncate">{f.name}</span>
    </FileButton>
  );
  const cell = (f: FileRecord, key: string) => {
    const p = propertyState(s, f),
      def = defs.find((d) => d.key === key)!,
      value = p.values[key];
    const invalid =
      p.status !== "ready"
        ? p.status === "invalid"
          ? "解析失败"
          : p.status === "stale"
            ? "过期投影"
            : "未扫描"
        : value !== undefined && !typeMatches(value, def.type)
          ? "类型不一致"
          : "";
    return edit?.id === f.id && edit.key === key ? (
      <PropertyInput
        definition={def}
        value={value}
        onChange={(value) => {
          dispatch({
            type: "property-set",
            id: f.id,
            key,
            value,
            version: f.version,
          });
        }}
      />
    ) : (
      <button
        className="min-h-11 w-full truncate text-left text-xs"
        disabled={
          !!invalid ||
          !canEditFile(s, f) ||
          (isMarkdown(f) && !writableNote(s, f))
        }
        onClick={() => setEdit({ id: f.id, key })}
      >
        {invalid || displayProperty(value)}
      </button>
    );
  };
  return (
    <section aria-label={label} className="min-w-0">
      <div className="flex h-11 items-center gap-2 text-xs">
        <span className="mr-auto">
          {new Set(files.map((f) => f.id)).size} 个文件 · 按文件去重
        </span>
        <Btn
          variant="ghost"
          onClick={() => {
            setEdit(null);
            setRevision((x) => x + 1);
          }}
        >
          刷新分组
        </Btn>
        <Btn variant="ghost" onClick={() => setConfig(true)}>
          <Columns3 size={15} />
          列与分组
        </Btn>
      </div>
      <p className="mb-2 text-[10px] text-muted-foreground">
        编辑后保留当前行；筛选变化请刷新结果。投影仅覆盖已读取或已授权提取范围。
      </p>
      {[...grouped].map(([group, items]) => (
        <div key={group} className="mb-3">
          <h3 className="sticky top-0 z-10 flex h-11 items-center bg-muted px-3 text-xs font-semibold">
            {o.groupBy ? group : "当前页"}
            <span className="ml-auto">{items.length}</span>
          </h3>
          {compact ? (
            <div className="divide-y rounded-b border bg-card">
              {items.map((f) => (
                <div key={f.id} data-property-row={f.id} className="p-3">
                  <div className="flex items-center">
                    <Checkbox
                      aria-label={`选择 ${f.name}`}
                      checked={s.selected.includes(f.id)}
                      onCheckedChange={(v) =>
                        dispatch({
                          type: "select",
                          ids: v
                            ? [...s.selected, f.id]
                            : s.selected.filter((id) => id !== f.id),
                        })
                      }
                    />
                    <div className="min-w-0 flex-1 pl-2">{name(f)}</div>
                  </div>
                  {keys.slice(0, 3).map((key) => (
                    <div
                      key={key}
                      className="grid grid-cols-[80px_minmax(0,1fr)] items-center text-xs"
                    >
                      <span className="text-muted-foreground">
                        {defs.find((d) => d.key === key)?.label}
                      </span>
                      {cell(f, key)}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ) : (
            <div
              className="max-w-full overflow-x-auto rounded-b border"
              data-property-table-scroll
            >
              <table
                className="w-full table-fixed border-collapse text-xs"
                style={{
                  minWidth:
                    300 +
                    keys.reduce((sum, key) => sum + (o.widths[key] || 170), 0),
                }}
              >
                <colgroup>
                  <col style={{ width: 300 }} />
                  {keys.map((key) => (
                    <col key={key} style={{ width: o.widths[key] || 170 }} />
                  ))}
                </colgroup>
                <thead>
                  <tr>
                    <th className="sticky left-0 z-10 h-11 bg-card px-3 text-left">
                      名称
                    </th>
                    {keys.map((key) => (
                      <th
                        key={key}
                        className="relative h-11 border-l bg-card px-3 text-left"
                      >
                        {defs.find((d) => d.key === key)?.label}
                        <input
                          aria-label={`调整列宽 ${key}`}
                          type="range"
                          min={120}
                          max={320}
                          value={o.widths[key] || 170}
                          className="ml-2 w-12"
                          onChange={(e) =>
                            dispatch({
                              type: "property-table",
                              patch: {
                                widths: {
                                  ...o.widths,
                                  [key]: Number(e.target.value),
                                },
                              },
                            })
                          }
                        />
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {items.map((f) => (
                    <tr
                      key={f.id}
                      data-property-row={f.id}
                      className="border-t"
                    >
                      <td className="sticky left-0 z-[1] bg-card px-3">
                        <div className="flex items-center gap-2">
                          <Checkbox
                            aria-label={`选择 ${f.name}`}
                            checked={s.selected.includes(f.id)}
                            onCheckedChange={(v) =>
                              dispatch({
                                type: "select",
                                ids: v
                                  ? [...s.selected, f.id]
                                  : s.selected.filter((id) => id !== f.id),
                              })
                            }
                          />
                          <div className="min-w-0">
                            {name(f)}
                            <p className="truncate pb-2 text-[10px] text-muted-foreground">
                              {
                                s.connections.find(
                                  (c) => c.id === f.connectionId,
                                )?.name
                              }{" "}
                              / {f.folder} · v{f.version}
                            </p>
                          </div>
                        </div>
                      </td>
                      {keys.map((key) => (
                        <td key={key} className="border-l bg-card px-3">
                          {cell(f, key)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ))}
      <Dialog open={config} onOpenChange={setConfig}>
        <DialogContent>
          <DialogTitle>属性视图</DialogTitle>
          <DialogDescription>
            复用文件与目录身份，不复制出另一套文件库。
          </DialogDescription>
          {defs.map((d) => (
            <div key={d.key} className="flex min-h-11 items-center gap-2">
              <label className="flex min-h-11 flex-1 items-center gap-2 text-sm">
                <Checkbox
                  checked={keys.includes(d.key)}
                  onCheckedChange={(v) =>
                    dispatch({
                      type: "property-table",
                      patch: {
                        columns: v
                          ? [...keys, d.key]
                          : keys.filter((key) => key !== d.key),
                      },
                    })
                  }
                />
                {d.label}
              </label>
              {keys.includes(d.key) && (
                <IconBtn
                  label={`前移列 ${d.label}`}
                  onClick={() => {
                    const cols = [...keys],
                      i = cols.indexOf(d.key);
                    if (i > 0) [cols[i - 1], cols[i]] = [cols[i], cols[i - 1]];
                    dispatch({
                      type: "property-table",
                      patch: { columns: cols },
                    });
                  }}
                >
                  <ArrowLeft size={14} />
                </IconBtn>
              )}
            </div>
          ))}
          <Choice
            label="按属性分组"
            value={o.groupBy || "none"}
            onChange={(key) =>
              dispatch({
                type: "property-table",
                patch: { groupBy: key === "none" ? "" : key },
              })
            }
            options={[
              { value: "none", label: "不分组" },
              ...defs.map((d) => ({ value: d.key, label: d.label })),
            ]}
          />
          <Choice
            label="属性排序"
            value={o.sortKey}
            onChange={(sortKey) =>
              dispatch({ type: "property-table", patch: { sortKey } })
            }
            options={[
              { value: "name", label: "名称" },
              ...defs.map((d) => ({ value: d.key, label: d.label })),
            ]}
          />
          <Btn onClick={() => setConfig(false)}>完成</Btn>
        </DialogContent>
      </Dialog>
    </section>
  );
}
export function FindResults({
  query: q,
  composing = false,
  refresh = 0,
  toolbarExtra,
  rangeLabel,
}: {
  query: SearchSpec;
  composing?: boolean;
  refresh?: number;
  toolbarExtra?: React.ReactNode;
  rangeLabel?: string;
}) {
  const { state: s, dispatch } = useDemo();
  const { found, pending, ready } = useFind(
    q,
    composing,
    refresh + s.browse.revision,
  );
  const restored = useRef(-1);
  useLayoutEffect(() => {
    const loc = browseLocation(s),
      main = document.getElementById("main-content");
    if (
      ready &&
      main &&
      loc.surface === "search" &&
      restored.current !== s.browse.revision
    ) {
      main.scrollTop = loc.scroll;
      delete main.dataset.browseRestoring;
      restored.current = s.browse.revision;
    }
  }, [ready, s.browse.revision]);
  const [page, setPage] = useState(1),
    [sort, setSort] = useState("relevance");
  useEffect(() => setPage(1), [JSON.stringify(q)]);
  useEffect(() => {
    const loc = browseLocation(s);
    if (loc.surface === "search") {
      setPage(loc.page);
      setSort(loc.sort);
    }
  }, [s.browse.revision, browseLocation(s).sort]);
  const pageTo = (value: number) => {
    setPage(value);
    dispatch({ type: "browse-checkpoint", page: value });
  };
  const entries = pageEntries(sortEntries(found, sort), page),
    files = entries.flatMap((r) => (r.file && !r.available ? [r.file] : []));
  return (
    <section>
      <FileToolbar
        summary={
          pending
            ? "查询中…"
            : `${found.length} 项${rangeLabel ? ` · ${rangeLabel}` : ""}`
        }
        extra={
          <>
            {toolbarExtra}
            <IconBtn
              label={s.organization.table ? "紧凑列表" : "属性表格"}
              onClick={() =>
                dispatch({
                  type: "property-table",
                  patch: { table: !s.organization.table },
                })
              }
            >
              <Columns3 size={15} />
            </IconBtn>
          </>
        }
        count={found.length}
        shown={files.map((f) => f.id)}
        matched={found.flatMap((r) =>
          r.file && !r.available ? [r.file.id] : [],
        )}
        page={page}
        pages={Math.max(1, Math.ceil(found.length / 50))}
      />

      {s.organization.table ? (
        <>
          <EntryList
            entries={entries.filter((r) => !r.file)}
            label="目录结果"
          />
          <PropertyTable files={files} />
        </>
      ) : (
        <EntryList
          entries={entries}
          query={q.query}
          sort={sort}
          onSort={(value) => {
            setSort(value);
            dispatch({ type: "browse-checkpoint", sort: value });
          }}
        />
      )}
      <div className="flex h-11 items-center justify-end gap-2 text-xs">
        <span>
          {page} / {Math.max(1, Math.ceil(found.length / 50))}
        </span>
        <IconBtn
          label="检索上一页"
          disabled={page === 1}
          onClick={() => pageTo(page - 1)}
        >
          <ArrowLeft size={15} />
        </IconBtn>
        <IconBtn
          label="检索下一页"
          disabled={page * 50 >= found.length}
          onClick={() => pageTo(page + 1)}
        >
          <ArrowRight size={15} />
        </IconBtn>
      </div>
    </section>
  );
}
export function PropertyFilters({
  query: q,
  onChange,
}: {
  query: SearchSpec;
  onChange: (p: Partial<SearchSpec>) => void;
}) {
  const { state: s } = useDemo();
  const defs = s.organization.definitions[s.space],
    [key, setKey] = useState("status"),
    [op, setOp] = useState<PropertyCondition["op"]>("eq"),
    [value, setValue] = useState(""),
    [end, setEnd] = useState(""),
    [error, setError] = useState("");
  return (
    <section className="space-y-3 border-t pt-3">
      <h3 className="text-sm font-medium">收藏、文档标签与属性</h3>
      <label className="flex min-h-11 items-center gap-2 text-xs">
        <Checkbox
          checked={!!q.onlyFavorites}
          onCheckedChange={(v) => onChange({ onlyFavorites: !!v })}
        />
        仅收藏条目（不自动包含子项）
      </label>
      <label className="block text-xs">收藏夹</label>
      <Choice
        className="w-full"
        label="指定收藏夹"
        value={q.favoriteGroup || "all"}
        onChange={(id) =>
          onChange({
            favoriteGroup: id === "all" ? undefined : id,
            onlyFavorites: id !== "all",
          })
        }
        options={[
          { value: "all", label: "所有收藏夹" },
          ...s.organization.groups
            .filter(
              (g) =>
                g.user === s.actor &&
                g.space === s.space &&
                q.connections.includes(g.connectionId),
            )
            .map((g) => ({
              value: g.id,
              label: `${s.connections.find((c) => c.id === g.connectionId)?.name} · ${g.name}`,
            })),
        ]}
      />
      <label className="flex min-h-11 items-center gap-2 text-xs">
        <Checkbox
          checked={!!q.favoriteDirectories}
          onCheckedChange={(v) => onChange({ favoriteDirectories: !!v })}
        />
        在收藏目录内检索（包含后代）
      </label>
      <label className="block text-xs">文档层级标签</label>
      <Input
        aria-label="文档层级标签"
        placeholder="例如 项目/山野"
        value={q.documentTag || ""}
        onChange={(e) =>
          onChange({ documentTag: e.target.value.replace(/^#/, "") })
        }
      />
      <label className="flex min-h-11 items-center gap-2 text-xs">
        <Checkbox
          checked={!!q.exactTag}
          onCheckedChange={(v) => onChange({ exactTag: !!v })}
        />
        仅此标签，不包含子标签
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label className="space-y-1 text-xs">
          <span className="block">属性字段</span>
          <Choice
            className="w-full"
            label="属性条件字段"
            value={key}
            onChange={setKey}
            options={defs.map((d) => ({ value: d.key, label: d.label }))}
          />
        </label>
        <label className="space-y-1 text-xs">
          <span className="block">匹配方式</span>
          <Choice
            className="w-full"
            label="属性条件操作"
            value={op}
            onChange={(v) => setOp(v as typeof op)}
            options={[
              ["eq", "等于"],
              ["ne", "不等于"],
              ["contains", "包含"],
              ["in", "属于列表"],
              ["range", "范围"],
              ["empty", "为空"],
              ["unset", "未设置"],
            ].map(([value, label]) => ({ value, label }))}
          />
        </label>
      </div>
      {!["empty", "unset"].includes(op) && (
        <label className="block space-y-1 text-xs">
          <span>属性条件值</span>
          <Input
            aria-label="属性条件值"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={
              op === "in"
                ? "多个值用逗号分隔"
                : "按字段类型输入，复选框使用 true / false"
            }
          />
        </label>
      )}
      {op === "range" && (
        <label className="block space-y-1 text-xs">
          <span>范围上限</span>
          <Input
            aria-label="属性范围上限"
            value={end}
            onChange={(e) => setEnd(e.target.value)}
          />
        </label>
      )}
      <Btn
        variant="outline"
        onClick={() => {
          try {
            const d = defs.find((d) => d.key === key)!;
            const v =
              op === "in"
                ? value.split(/[,，]/).map((v) => v.trim())
                : parsePropertyInput(value, d.type);
            onChange({
              propertyConditions: [
                ...(q.propertyConditions || []),
                {
                  key,
                  op,
                  value: v,
                  end: d.type === "number" ? Number(end) : end,
                },
              ],
            });
            setError("");
          } catch (e) {
            setError((e as Error).message);
          }
        }}
      >
        追加条件
      </Btn>
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
      {q.propertyConditions?.map((c, i) => (
        <div key={i} className="flex min-h-11 items-center text-xs">
          <span className="min-w-0 flex-1 truncate">
            {c.key} · {c.op} · {displayProperty(c.value)}
          </span>
          <Btn
            variant="ghost"
            onClick={() =>
              onChange({
                propertyConditions: q.propertyConditions?.filter(
                  (_, j) => j !== i,
                ),
              })
            }
          >
            移除条件
          </Btn>
        </div>
      ))}
      <p className="text-xs leading-6 text-muted-foreground">
        0、false、空值与未设置分别匹配。未解析或过期投影不参加属性判断。
      </p>
    </section>
  );
}
export function QuickFind() {
  const { state: s, dispatch, go, close, open } = useDemo();
  const [q, setQ] = useState<SearchSpec>(() => ({
      ...emptySearch(),
      connections: [s.activeConnection],
      directoryId: undefined,
    })),
    [composing, setComposing] = useState(false),
    [run, setRun] = useState(0),
    [selected, setSelected] = useState(0);
  const { found, pending } = useFind(q, composing, run);
  const root = useRef<HTMLDivElement>(null);
  const suggested = q.query
    ? found
    : found
        .filter(
          (r) =>
            bookmarkFor(s, r.ref) || s.organization.recent.includes(r.ref.id),
        )
        .sort(
          (a, b) =>
            (bookmarkFor(s, a.ref)?.pin ?? 999) -
            (bookmarkFor(s, b.ref)?.pin ?? 999),
        );
  const entries = suggested.slice(0, 50);
  useEffect(() => setSelected(0), [q.query, q.connections.join(",")]);
  const launch = (r: FindEntry) => {
    if (r.available) return;
    dispatch({ type: "entry-visit", id: r.ref.id });
    if (r.file) open({ type: "preview", id: r.ref.id, single: true });
    else {
      close();
      dispatch({ type: "directory-open", id: r.ref.id });
    }
  };
  return (
    <div ref={root} className="space-y-3">
      <Input
        autoFocus
        aria-label="命令与文件搜索"
        value={q.query}
        onCompositionStart={() => setComposing(true)}
        onCompositionEnd={() => setComposing(false)}
        onChange={(e) => setQ({ ...q, query: e.target.value })}
        placeholder="查找当前连接的文件或目录"
        onKeyDown={(e) => {
          if (composing || e.nativeEvent.isComposing) return;
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            setSelected((i) =>
              Math.max(
                0,
                Math.min(
                  entries.length - 1,
                  i + (e.key === "ArrowDown" ? 1 : -1),
                ),
              ),
            );
          }
          if (e.key === "Enter") {
            e.preventDefault();
            if (pending) {
              setRun((x) => x + 1);
            } else if (entries[selected]) launch(entries[selected]);
            else setRun((x) => x + 1);
          }
        }}
      />
      <Choice
        label="快速查找范围"
        value={q.connections[0]}
        onChange={(id) => setQ({ ...q, connections: [id] })}
        options={s.connections
          .filter((c) => c.id === "drive" || c.space === s.space)
          .map((c) => ({ value: c.id, label: c.name }))}
      />
      <p role="status" className="text-xs text-muted-foreground">
        {pending
          ? "查询中…"
          : q.query
            ? `${entries.length} 个当前页结果`
            : "置顶、最近访问与收藏"}{" "}
        · ↑↓ 选择，Enter 打开
      </p>
      <div
        role="listbox"
        aria-label="快速查找结果"
        className="max-h-[45dvh] overflow-auto"
      >
        {entries.map((r, i) =>
          r.file && !r.available ? (
            <FileButton
              reference={fileRef(r.file!)}
              variant="ghost"
              onOpen={() => launch(r)}
              role="option"
              aria-selected={i === selected}
              key={r.ref.id}
              disabled={!!r.available}
              className={`flex !h-auto !min-h-16 w-full min-w-0 items-center gap-2 rounded px-3 !justify-start !whitespace-normal text-left ${i === selected ? "bg-accent" : "hover:bg-muted"}`}
            >
              <span className="shrink-0">
                {r.ref.kind === "directory" ? (
                  <Folder size={17} />
                ) : (
                  <Star size={15} />
                )}
              </span>
              <span className="min-w-0">
                <strong className="block truncate text-sm font-medium">
                  {r.name}
                </strong>
                <small className="block truncate text-muted-foreground">
                  {r.path} · {r.available || r.reason}
                </small>
              </span>
            </FileButton>
          ) : (
            <button
              role="option"
              aria-selected={i === selected}
              key={r.ref.id}
              disabled={!!r.available}
              className={`flex min-h-16 w-full min-w-0 items-center gap-2 rounded px-3 text-left ${i === selected ? "bg-accent" : "hover:bg-muted"}`}
              onClick={() => launch(r)}
            >
              <span className="shrink-0">
                {r.ref.kind === "directory" ? (
                  <Folder size={17} />
                ) : (
                  <Star size={15} />
                )}
              </span>
              <span className="min-w-0">
                <strong className="block truncate text-sm font-medium">
                  {r.name}
                </strong>
                <small className="block truncate text-muted-foreground">
                  {r.path} · {r.available || r.reason}
                </small>
              </span>
            </button>
          ),
        )}
      </div>
      <Btn
        variant="outline"
        className="w-full"
        onClick={() => {
          close();
          go("search");
          dispatch({ type: "search-patch", patch: q });
          dispatch({ type: "search-submit" });
        }}
      >
        打开完整检索与筛选
      </Btn>
      <div className="flex flex-wrap gap-1">
        {[
          { page: "tasks", label: "任务中心" },
          { page: "settings", label: "设置" },
          { page: "tags", label: "标签与属性" },
        ]
          .filter((x) => !q.query || x.label.includes(q.query))
          .map((x) => (
            <Btn
              key={x.page}
              variant="ghost"
              onClick={() => {
                close();
                go(x.page as "tasks" | "settings" | "tags");
              }}
            >
              {x.label}
            </Btn>
          ))}
      </div>
    </div>
  );
}
