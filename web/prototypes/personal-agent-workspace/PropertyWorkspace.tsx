import { useEffect, useState } from "react";
import { Plus, Settings2, ArrowRight, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Btn, Choice, IconBtn, Pill, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { propertyState, allFileTags } from "./organization-model";
import {
  documentTags,
  displayProperty,
  parsePropertyInput,
  typeMatches,
} from "./properties";
import type {
  PropertyValue,
  PropertyDefinition,
  PropertyType,
} from "./properties";
import type { FileRecord } from "./model";
import { isMarkdown, noteBody, writableNote } from "./knowledge-model";
import { canEditFile, fileUnavailable } from "./v4-model";
export function PropertyInput({
  definition: d,
  value,
  onChange,
  disabled = false,
}: {
  definition: PropertyDefinition;
  value: PropertyValue | undefined;
  onChange: (v: PropertyValue) => void;
  disabled?: boolean;
}) {
  const text = (v: typeof value) =>
    Array.isArray(v) ? v.join(", ") : v == null ? "" : String(v);
  const [draft, setDraft] = useState(text(value)),
    [error, setError] = useState("");
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    if (!focused) setDraft(text(value));
  }, [JSON.stringify(value), focused]);
  if (d.type === "checkbox")
    return (
      <label className="flex min-h-11 items-center gap-2 text-xs">
        <Checkbox
          aria-label={d.label}
          checked={value === true}
          disabled={disabled}
          onCheckedChange={(v) => onChange(!!v)}
        />
        {displayProperty(value)}
      </label>
    );
  return (
    <div className="min-w-0">
      <Input
        aria-label={d.label}
        disabled={disabled}
        value={draft}
        placeholder={value === undefined ? "未设置" : "空值"}
        type={
          d.type === "date"
            ? "date"
            : d.type === "datetime"
              ? "datetime-local"
              : d.type === "number"
                ? "number"
                : "text"
        }
        onFocus={() => setFocused(true)}
        onChange={(e) => {
          setDraft(e.target.value);
          setError("");
        }}
        onBlur={() => {
          setFocused(false);
          if (draft === text(value)) return;
          try {
            onChange(parsePropertyInput(draft, d.type));
          } catch (e) {
            setError((e as Error).message);
          }
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.nativeEvent.isComposing)
            e.currentTarget.blur();
        }}
      />
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
export function PropertyPanel({
  file: f,
  version,
}: {
  file: FileRecord;
  version: number;
}) {
  const { state: s, dispatch, go } = useDemo();
  const p = propertyState(s, f, version),
    defs = s.organization.definitions[s.space];
  const [adding, setAdding] = useState(false),
    [key, setKey] = useState("project");
  const historical = version !== f.version,
    md = isMarkdown(f),
    note = s.knowledge.notes[f.id];
  const disabled =
    historical ||
    !canEditFile(s, f) ||
    (md && (!writableNote(s, f) || !note || note.composing)) ||
    p.status !== "ready";
  const fields = [
    ...new Set([...Object.keys(p.values), ...p.fields.map((f) => f.key)]),
  ];
  const set = (key: string, value: PropertyValue | undefined) =>
    dispatch({ type: "property-set", id: f.id, key, value, version });
  const tags =
    md && note
      ? documentTags(historical ? noteBody(s, f.id, version) : note.draft)
      : [];
  const source = (from = 0) => {
    dispatch({ type: "k-open", id: f.id, mode: "source" });
    window.dispatchEvent(
      new CustomEvent("drivebridge-source-position", {
        detail: { id: f.id, from },
      }),
    );
  };
  return (
    <section className="space-y-3 border-t pt-4" data-file-properties>
      <div className="flex items-center">
        <h3 className="mr-auto font-semibold">属性</h3>
        <IconBtn label="管理属性定义" onClick={() => go("tags")}>
          <Settings2 size={15} />
        </IconBtn>
        <IconBtn
          label="添加属性"
          disabled={disabled}
          onClick={() => setAdding(true)}
        >
          <Plus size={16} />
        </IconBtn>
      </div>
      <p className="text-xs leading-6 text-muted-foreground">
        {md
          ? "写入文首 YAML · 与源码共用草稿和保存状态"
          : "DriveBridge 元数据 · 不改源文件"}
        {historical ? " · 历史版本只读" : ""}
      </p>
      {p.status !== "ready" && (
        <p
          role="alert"
          className="rounded border border-amber-400/40 p-3 text-xs leading-6"
        >
          {p.status === "invalid"
            ? "解析失败：" + p.errors.join("；")
            : p.status === "stale"
              ? "属性投影已过期"
              : "属性尚未扫描"}
          。不能将此状态解释为没有属性。
          {md && (
            <Btn variant="ghost" onClick={() => source()}>
              打开源码修复
            </Btn>
          )}
        </p>
      )}
      {[
        ...new Set(
          fields.map(
            (key) => defs.find((d) => d.key === key)?.group || "未分组",
          ),
        ),
      ].map((group) => (
        <div key={group} className="space-y-2">
          <h4 className="text-xs font-medium text-muted-foreground">{group}</h4>
          {fields
            .filter(
              (key) =>
                (defs.find((d) => d.key === key)?.group || "未分组") === group,
            )
            .map((key) => {
              const field = p.fields.find((f) => f.key === key),
                value = p.values[key],
                d = defs.find((d) => d.key === key) || {
                  key,
                  label: key,
                  type: Array.isArray(value)
                    ? "list"
                    : typeof value === "number"
                      ? "number"
                      : typeof value === "boolean"
                        ? "checkbox"
                        : "text",
                  group,
                };
              const mismatch =
                value !== undefined && !typeMatches(value, d.type);
              return (
                <div key={key} className="space-y-1 rounded border p-2">
                  <div className="flex items-center">
                    <label className="mr-auto text-xs" title={key}>
                      {d.label}
                      <small className="ml-1 text-muted-foreground">
                        {key}
                      </small>
                    </label>
                    <IconBtn
                      label={`移除属性 ${key}`}
                      disabled={disabled || field?.complex || mismatch}
                      onClick={() => set(key, undefined)}
                    >
                      <Trash2 size={13} />
                    </IconBtn>
                  </div>
                  {field?.complex ? (
                    <p className="text-xs">
                      嵌套结构或引用，保留原文。
                      <Btn variant="ghost" onClick={() => source(field.from)}>
                        在源码编辑
                      </Btn>
                    </p>
                  ) : mismatch ? (
                    <p role="alert" className="text-xs text-destructive">
                      类型不一致：定义为 {d.type}，原值 {displayProperty(value)}
                      ；请在源码处理。
                    </p>
                  ) : (
                    <PropertyInput
                      definition={d}
                      value={value}
                      disabled={disabled}
                      onChange={(v) => set(key, v)}
                    />
                  )}
                </div>
              );
            })}
        </div>
      ))}
      {!fields.length && p.status === "ready" && (
        <p className="text-xs text-muted-foreground">
          还没有自定义属性，添加后将写入{md ? "同一文档" : "元数据"}。
        </p>
      )}
      {md && (
        <>
          <div className="flex flex-wrap gap-1">
            {[...new Set(tags.map((t) => t.name.toLocaleLowerCase()))].map(
              (name) => {
                const matches = tags.filter(
                  (t) => t.name.toLocaleLowerCase() === name,
                );
                return (
                  <Btn
                    key={name}
                    variant="outline"
                    className="!h-11 !max-w-full !px-2 text-xs"
                    title={matches
                      .map((t) =>
                        t.source === "inline" ? "正文标签" : "文首标签",
                      )
                      .join("、")}
                    onClick={() => {
                      go("search");
                      dispatch({
                        type: "search-patch",
                        patch: {
                          connections: [f.connectionId || "drive"],
                          directoryId: undefined,
                          query: "",
                          documentTag: name,
                          exactTag: false,
                          tab: "files",
                        },
                      });
                      dispatch({ type: "search-submit" });
                    }}
                  >
                    #{name}
                  </Btn>
                );
              },
            )}
          </div>
          {tags
            .filter((t) => t.source === "inline")
            .map((t, i) => (
              <Btn
                key={i}
                className="!h-11 w-full !justify-start !px-0 text-xs"
                variant="ghost"
                onClick={() => source(t.from)}
              >
                正文 #{t.name} · 源码位置 {t.from}
                <ArrowRight size={13} />
              </Btn>
            ))}
          <Btn
            variant="outline"
            disabled={disabled}
            onClick={() => {
              const prior = p.values.tags;
              set("tags", [
                ...new Set([
                  ...(Array.isArray(prior) ? prior : []),
                  ...allFileTags(s, f)
                    .filter((t) => t.source === "DriveBridge 标签")
                    .map((t) => t.name),
                ]),
              ]);
            }}
          >
            将平台标签写入文档标签
          </Btn>
          <p className="text-xs leading-5 text-muted-foreground">
            只修改文首 tags；正文标签与外部建议保持原样。
          </p>
        </>
      )}
      <p className="text-[10px] text-muted-foreground">
        {p.at} · {p.version === undefined ? "版本未知" : `v${p.version}`}
      </p>
      <Dialog open={adding} onOpenChange={setAdding}>
        <DialogContent>
          <DialogTitle>添加文档属性</DialogTitle>
          <DialogDescription>
            定义在当前空间复用；字段分组只用于展示。
          </DialogDescription>
          <Choice
            label="属性字段"
            value={key}
            onChange={setKey}
            options={defs.map((d) => ({
              value: d.key,
              label: `${d.label} · ${d.type}`,
            }))}
          />
          <Btn
            onClick={() => {
              const d = defs.find((d) => d.key === key)!;
              set(
                key,
                d.type === "list"
                  ? []
                  : d.type === "checkbox"
                    ? false
                    : d.type === "number"
                      ? 0
                      : d.type === "date" || d.type === "datetime"
                        ? null
                        : "",
              );
              setAdding(false);
            }}
          >
            添加属性
          </Btn>
        </DialogContent>
      </Dialog>
    </section>
  );
}
export function PropertyBatchEditor({
  ids,
  planOnly = false,
}: {
  ids: string[];
  planOnly?: boolean;
}) {
  const { state: s, dispatch } = useDemo();
  const defs = s.organization.definitions[s.space];
  const [key, setKey] = useState("status"),
    [value, setValue] = useState<PropertyValue>(""),
    [remove, setRemove] = useState(false);
  const b = s.organization.batch;
  return (
    <div className="space-y-4">
      <div hidden={planOnly}>
        <p className="text-xs leading-6 text-muted-foreground">
          冻结 {ids.length} 个文件及当前版本；Markdown 写回
          YAML，其他文件写入元数据。成功项不会重复执行。
        </p>
        <Choice
          label="批量属性字段"
          value={key}
          onChange={(key) => {
            setKey(key);
            const t = defs.find((d) => d.key === key)!.type;
            setValue(
              t === "list"
                ? []
                : t === "number"
                  ? 0
                  : t === "checkbox"
                    ? false
                    : "",
            );
          }}
          options={defs.map((d) => ({ value: d.key, label: d.label }))}
        />
        <PropertyInput
          definition={defs.find((d) => d.key === key)!}
          value={value}
          onChange={setValue}
          disabled={remove}
        />
        <label className="flex min-h-11 items-center gap-2 text-xs">
          <Checkbox checked={remove} onCheckedChange={(v) => setRemove(!!v)} />
          移除该字段（保留其他字段）
        </label>
        <Btn
          onClick={() =>
            dispatch({
              type: "property-batch",
              ids,
              key,
              value: remove ? undefined : value,
            })
          }
        >
          预览属性差异
        </Btn>
      </div>
      {b && (
        <section className="space-y-3 border-t pt-3" aria-label="属性修改计划">
          <p className="text-xs">
            原操作 {b.id} · {b.items.filter((i) => i.status === "done").length}/
            {b.items.length} 完成
          </p>
          {b.items.map((i) => (
            <article key={i.id} className="rounded border p-3 text-xs">
              <strong>
                {s.files.find((f) => f.id === i.id)?.name} · v{i.version}
              </strong>
              <p className="my-2">
                {
                  {
                    pending: "待确认",
                    done: "成功",
                    failed: "失败",
                    skipped: "跳过",
                  }[i.status]
                }{" "}
                · {i.reason}
              </p>
              <div className="grid gap-2 sm:grid-cols-2">
                <pre
                  className="max-h-32 overflow-auto whitespace-pre-wrap bg-muted p-2"
                  aria-label="修改前"
                >
                  {i.before}
                </pre>
                <pre
                  className="max-h-32 overflow-auto whitespace-pre-wrap bg-accent p-2"
                  aria-label="修改后"
                >
                  {i.after}
                </pre>
              </div>
            </article>
          ))}
          <div className="flex flex-wrap gap-2">
            <Btn
              onClick={() => dispatch({ type: "property-batch-run", id: b.id })}
            >
              {b.confirmed ? "恢复原操作未完成项" : "确认修改属性"}
            </Btn>
            {!b.confirmed && (
              <Btn
                variant="outline"
                onClick={() =>
                  dispatch({
                    type: "property-batch-run",
                    id: b.id,
                    partial: true,
                  })
                }
              >
                演示部分失败
              </Btn>
            )}
          </div>
        </section>
      )}
    </div>
  );
}
export function PropertyDefinitions() {
  const { state: s, dispatch } = useDemo();
  const defs = s.organization.definitions[s.space];
  const [edit, setEdit] = useState<PropertyDefinition | null>(null),
    [migration, setMigration] = useState<PropertyDefinition | null>(null);
  return (
    <section className="mt-8 space-y-3 border-t pt-5" aria-label="属性定义">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">属性定义与显示分组</h2>
        <Btn
          variant="outline"
          onClick={() =>
            setEdit({ key: "", label: "", type: "text", group: "未分组" })
          }
        >
          <Plus size={15} />
          新建属性
        </Btn>
      </div>
      <p className="text-xs leading-6 text-muted-foreground">
        当前空间共享字段类型；修改显示名称不改 YAML
        键。类型不一致保留原值，不自动转换。
      </p>
      <div className="divide-y rounded-lg border bg-card">
        {defs.map((d) => {
          const invalid = s.files
            .filter((f) => f.space === s.space)
            .filter((f) => {
              const p = propertyState(s, f);
              return (
                p.status === "invalid" ||
                (Object.hasOwn(p.values, d.key) &&
                  !typeMatches(p.values[d.key], d.type))
              );
            });
          return (
            <div key={d.key} className="flex min-h-16 items-center gap-3 px-3">
              <div className="min-w-0 flex-1">
                <strong className="text-sm">{d.label}</strong>
                <p className="text-xs text-muted-foreground">
                  {d.key} · {d.type} · {d.group}
                </p>
              </div>
              {invalid.length > 0 && (
                <Btn
                  variant="ghost"
                  onClick={() => {
                    dispatch({ type: "navigate", page: "search" });
                    dispatch({
                      type: "search-patch",
                      patch: {
                        fileIds: invalid.map((f) => f.id),
                        connections: [
                          ...new Set(
                            invalid.map((f) => f.connectionId || "drive"),
                          ),
                        ],
                        query: "",
                      },
                    });
                    dispatch({ type: "search-submit" });
                  }}
                >
                  {invalid.length} 个异常
                </Btn>
              )}
              <Btn variant="ghost" onClick={() => setEdit({ ...d })}>
                编辑定义
              </Btn>
              <Btn variant="ghost" onClick={() => setMigration({ ...d })}>
                迁移字段
              </Btn>
            </div>
          );
        })}
      </div>
      <Dialog open={!!edit} onOpenChange={(v) => !v && setEdit(null)}>
        <DialogContent>
          <DialogTitle>属性定义</DialogTitle>
          <DialogDescription>
            字段键和类型的迁移需逐文件差异确认，此处不会自动改写已有文档。
          </DialogDescription>
          {edit && (
            <>
              <label className="text-xs">
                字段键
                <Input
                  aria-label="属性键"
                  value={edit.key}
                  disabled={defs.some((d) => d.key === edit.key)}
                  onChange={(e) => setEdit({ ...edit, key: e.target.value })}
                />
              </label>
              <label className="text-xs">
                显示名称
                <Input
                  aria-label="属性显示名称"
                  value={edit.label}
                  onChange={(e) => setEdit({ ...edit, label: e.target.value })}
                />
              </label>
              <Choice
                label="属性类型"
                value={edit.type}
                onChange={(type) =>
                  setEdit({ ...edit, type: type as PropertyType })
                }
                options={[
                  "text",
                  "list",
                  "number",
                  "checkbox",
                  "date",
                  "datetime",
                ].map((value) => ({ value, label: value }))}
              />
              <label className="text-xs">
                显示分组
                <Input
                  aria-label="属性显示分组"
                  value={edit.group}
                  onChange={(e) => setEdit({ ...edit, group: e.target.value })}
                />
              </label>
              <label className="text-xs">
                建议选项（逗号分隔）
                <Input
                  value={edit.options?.join(", ") || ""}
                  onChange={(e) =>
                    setEdit({
                      ...edit,
                      options: e.target.value
                        .split(/[,，]/)
                        .map((s) => s.trim())
                        .filter(Boolean),
                    })
                  }
                />
              </label>
              <Btn
                disabled={!edit.key || !edit.label}
                onClick={() => {
                  dispatch({ type: "property-definition", definition: edit });
                  setEdit(null);
                }}
              >
                保存定义
              </Btn>
            </>
          )}
        </DialogContent>
      </Dialog>
      <Dialog open={!!migration} onOpenChange={(v) => !v && setMigration(null)}>
        <DialogContent>
          <DialogTitle>字段迁移与类型转换</DialogTitle>
          <DialogDescription>
            先预览已读取范围的差异，再按固定版本逐项应用。无法转换或字段冲突的文件保留原文。
          </DialogDescription>
          {migration && <PropertyMigration definition={migration} />}
        </DialogContent>
      </Dialog>
    </section>
  );
}

export function PropertyExtraction({ directoryId }: { directoryId: string }) {
  const { state: s, dispatch } = useDemo();
  const [bodyTags, setBodyTags] = useState(false),
    [confirm, setConfirm] = useState(false);
  const scope = s.organization.extraction[directoryId];
  const node = s.storage.nodes.find((n) => n.id === directoryId)!;
  return (
    <section
      className="my-5 space-y-3 rounded-lg border bg-card p-4"
      aria-label="Markdown 属性提取"
    >
      <div className="flex items-center gap-2">
        <h3 className="mr-auto text-sm font-semibold">Markdown 属性提取</h3>
        <Pill>
          {scope
            ? scope.status === "paused"
              ? "已暂停"
              : scope.status === "partial"
                ? "部分覆盖"
                : "已提取样本"
            : "未启用"}
        </Pill>
      </div>
      <p className="text-xs leading-6 text-muted-foreground">
        独立于目录索引与语义分析。范围：
        {s.connections.find((c) => c.id === node.connectionId)?.name} /{" "}
        {node.path}。默认只解析文首 YAML；已打开的文档可在本轮局部解析。
      </p>
      <p className="text-xs text-muted-foreground">
        来源版本与提取时间随条目保留。演示仅解析已有样本；真实扫描方式、费用和覆盖总量未知。
      </p>
      <div className="flex flex-wrap gap-2">
        <Btn
          variant="outline"
          onClick={() => {
            setBodyTags(scope?.bodyTags || false);
            setConfirm(true);
          }}
        >
          配置属性提取范围
        </Btn>
        {scope && (
          <>
            <Btn
              variant="ghost"
              onClick={() =>
                dispatch({
                  type: "property-extract",
                  directoryId,
                  bodyTags: scope.bodyTags,
                  pause: scope.status !== "paused",
                })
              }
            >
              {scope.status === "paused" ? "恢复提取" : "暂停后续提取"}
            </Btn>
            <Btn
              variant="ghost"
              onClick={() => {
                setBodyTags(false);
                setConfirm(true);
              }}
            >
              清除或修改范围
            </Btn>
          </>
        )}
      </div>
      <Dialog open={confirm} onOpenChange={setConfirm}>
        <DialogContent>
          <DialogTitle>确认属性提取</DialogTitle>
          <DialogDescription>
            {node.path} 及子目录。只对当前可访问 Markdown 执行；不扩大外部授权。
          </DialogDescription>
          <label className="flex min-h-11 items-center gap-2 text-sm">
            <Checkbox
              checked={bodyTags}
              onCheckedChange={(v) => setBodyTags(!!v)}
            />
            额外读取正文并提取 #标签
          </label>
          <p className="text-xs leading-6 text-muted-foreground">
            未勾选时只处理文首。解析失败、未扫描与过期数据不会被解释为空值。刷新样本后此授权重置。
          </p>
          <Btn
            onClick={() => {
              dispatch({ type: "property-extract", directoryId, bodyTags });
              setConfirm(false);
            }}
          >
            确认并提取演示样本
          </Btn>
          {scope && (
            <Btn
              variant="ghost"
              onClick={() => {
                dispatch({
                  type: "property-extract",
                  directoryId,
                  bodyTags: false,
                  clear: true,
                });
                setConfirm(false);
              }}
            >
              仅清除属性投影，不删除文件
            </Btn>
          )}
        </DialogContent>
      </Dialog>
    </section>
  );
}
export function DocumentTagCatalog() {
  const { state: s, dispatch, go } = useDemo();
  const tags = new Map<string, Set<string>>();
  const [merge, setMerge] = useState(false),
    [from, setFrom] = useState(""),
    [to, setTo] = useState(""),
    [preview, setPreview] = useState(false);
  for (const f of s.files.filter(
    (f) =>
      f.space === s.space &&
      f.connectionId === s.activeConnection &&
      !f.trashed &&
      !fileUnavailable(s, f),
  )) {
    for (const tag of allFileTags(s, f)) {
      const parts = tag.name.split("/");
      for (let i = 1; i <= parts.length; i++) {
        const key = parts.slice(0, i).join("/"),
          ids = tags.get(key) || new Set();
        ids.add(f.id);
        tags.set(key, ids);
      }
    }
  }
  return (
    <section className="my-5 space-y-2 rounded-lg border bg-card p-4">
      <h2 className="text-sm font-semibold">当前连接的标签层级</h2>
      <p className="text-xs text-muted-foreground">
        已读取文档与正式平台标签 · 父标签包含后代 · 建议不计入
      </p>
      <div className="grid gap-1 sm:grid-cols-2">
        {[...tags]
          .sort(([a], [b]) => a.localeCompare(b))
          .map(([name, ids]) => (
            <Btn
              key={name}
              variant="ghost"
              className="!h-11 !justify-start"
              onClick={() => {
                go("search");
                dispatch({
                  type: "search-patch",
                  patch: {
                    connections: [s.activeConnection],
                    directoryId: undefined,
                    query: "",
                    documentTag: name,
                    exactTag: false,
                    tab: "files",
                  },
                });
                dispatch({ type: "search-submit" });
              }}
            >
              <span className="truncate">#{name}</span>
              <span className="ml-auto text-xs text-muted-foreground">
                {ids.size}
              </span>
            </Btn>
          ))}
      </div>
      <Btn
        variant="outline"
        onClick={() => {
          setMerge(true);
          setPreview(false);
        }}
      >
        合并文首标签
      </Btn>
      <Dialog open={merge} onOpenChange={setMerge}>
        <DialogContent>
          <DialogTitle>合并当前连接的文首标签</DialogTitle>
          <DialogDescription>
            仅修改 tags
            属性及对应保存视图；包含子标签。正文标签保持原文，可从文件属性定位后单独修改。
          </DialogDescription>
          <Input
            aria-label="原文档标签"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            placeholder="原标签"
          />
          <Input
            aria-label="目标文档标签"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            placeholder="合并到"
          />
          <Btn
            disabled={!from.trim() || !to.trim() || from === to}
            onClick={() => {
              dispatch({ type: "document-tag-merge", from, to });
              setPreview(true);
            }}
          >
            预览标签合并
          </Btn>
          {preview && <PropertyBatchEditor ids={[]} planOnly />}
        </DialogContent>
      </Dialog>
    </section>
  );
}

function PropertyMigration({
  definition: d,
}: {
  definition: PropertyDefinition;
}) {
  const { state: s, dispatch } = useDemo();
  const [to, setTo] = useState(d.key),
    [type, setType] = useState(d.type),
    [id, setId] = useState("");
  return (
    <div className="space-y-3">
      <label className="text-xs">
        目标字段键
        <Input
          aria-label="迁移目标字段键"
          value={to}
          onChange={(e) => setTo(e.target.value)}
        />
      </label>
      <Choice
        label="目标属性类型"
        value={type}
        onChange={(v) => setType(v as PropertyType)}
        options={["text", "list", "number", "checkbox", "date", "datetime"].map(
          (value) => ({ value, label: value }),
        )}
      />
      <Btn
        disabled={!to.trim()}
        onClick={() => {
          dispatch({
            type: "property-migrate",
            from: d.key,
            to,
            definition: { ...d, key: to, type },
          });
          setId("preview");
        }}
      >
        预览字段迁移
      </Btn>
      {id && <PropertyBatchEditor ids={[]} planOnly />}
    </div>
  );
}
