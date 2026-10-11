import { documentTags } from "./properties";
import { noteBody, isMarkdown } from "./knowledge-model";
import { PropertyDefinitions, DocumentTagCatalog } from "./PropertyWorkspace";
import { useState } from "react";
import { Check, Merge, Plus, Search, Star, Tag, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { assignedTags, tagImpact } from "./cloud-model";
import type { FileRecord } from "./model";
import {
  Btn,
  Choice,
  Empty,
  Heading,
  IconBtn,
  panel,
  Pill,
  useDemo,
} from "./ui";
export const tagColor: Record<string, string> = {
  pine: "bg-accent text-primary",
  blue: "bg-[var(--info-soft)] text-[var(--info)]",
  amber: "bg-[var(--warning-soft)] text-[var(--warning)]",
  rose: "bg-destructive/10 text-destructive",
  slate: "bg-muted text-muted-foreground",
};
export function FileTags({
  file,
  full = false,
}: {
  file: FileRecord;
  full?: boolean;
}) {
  const { state: s, dispatch } = useDemo();
  const tags = (file.tags || [])
    .filter((a) => s.options.showSuggestions || a.source !== "suggested")
    .flatMap((a) => {
      const tag = s.tags.find((t) => t.id === a.tagId && t.space === s.space);
      return tag
        ? [
            {
              ...tag,
              source: a.source,
              originTag:
                a.source === "manual"
                  ? "DriveBridge 标签"
                  : a.source === "rule"
                    ? "规则标签"
                    : "外部建议",
            },
          ]
        : [];
    });
  if (isMarkdown(file) && s.knowledge.notes[file.id]) {
    const seen = new Set<string>();
    for (const t of documentTags(noteBody(s, file.id))) {
      const key = t.name.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      tags.push({
        id: `document-${key}`,
        name: t.name,
        space: file.space,
        group: "文档",
        color: "pine",
        favorite: false,
        source: "manual",
        originTag: documentTags(noteBody(s, file.id))
          .filter((x) => x.name.toLowerCase() === key)
          .map((x) => (x.source === "inline" ? "正文标签" : "文首标签"))
          .join("、"),
      });
    }
  }
  return (
    <div
      className={
        full
          ? "flex flex-wrap gap-2"
          : "flex h-6 min-w-0 items-center gap-1 overflow-hidden"
      }
      aria-label="文件标签"
    >
      {(full ? tags : tags.slice(0, 2)).map((t) => (
        <span
          key={`${t.id}-${t.source}`}
          className={`inline-flex max-w-36 shrink-0 items-center gap-1 truncate rounded px-1.5 py-0.5 text-xs ${tagColor[t.color] || tagColor.pine}`}
          title={`${t.name} · ${t.originTag}`}
        >
          {t.source === "suggested" ? "建议：" : ""}
          {t.name}
          {full && (
            <small>
              ·{" "}
              {t.source === "rule"
                ? "规则"
                : t.source === "manual"
                  ? "人工"
                  : "建议"}
            </small>
          )}
        </span>
      ))}
      {!full && tags.length > 2 && (
        <span className="text-xs text-muted-foreground">
          +{tags.length - 2}
        </span>
      )}
      {full &&
        tags
          .filter((t) => t.source === "suggested")
          .map((t) => (
            <span className="inline-flex gap-1" key={`review-${t.id}`}>
              <Btn
                variant="outline"
                onClick={() =>
                  dispatch({
                    type: "tag-suggestion",
                    fileId: file.id,
                    tagId: t.id,
                    accept: true,
                  })
                }
              >
                接受 {t.name}
              </Btn>
              <Btn
                variant="ghost"
                onClick={() =>
                  dispatch({
                    type: "tag-suggestion",
                    fileId: file.id,
                    tagId: t.id,
                    accept: false,
                  })
                }
              >
                忽略
              </Btn>
            </span>
          ))}
    </div>
  );
}
export function TagPicker({
  ids,
  onChange,
}: {
  ids: string[];
  onChange: (ids: string[]) => void;
}) {
  const { state: s } = useDemo();
  return (
    <div className="flex flex-wrap gap-2">
      {s.tags
        .filter((t) => t.space === s.space)
        .map((t) => (
          <Btn
            key={t.id}
            variant={ids.includes(t.id) ? "secondary" : "outline"}
            aria-pressed={ids.includes(t.id)}
            className="text-xs"
            onClick={() =>
              onChange(
                ids.includes(t.id)
                  ? ids.filter((id) => id !== t.id)
                  : [...ids, t.id],
              )
            }
          >
            <Tag size={13} />
            {t.name}
          </Btn>
        ))}
    </div>
  );
}
export function TagBatch({ ids }: { ids: string[] }) {
  const { state: s, dispatch, open, close } = useDemo();
  const [tags, setTags] = useState<string[]>([]);
  const [mode, setMode] = useState<"add" | "remove" | "replace">("add");
  const [name, setName] = useState("");
  const files = s.files.filter(
    (f) => ids.includes(f.id) && f.space === s.space && !f.trashed,
  );
  return (
    <div className="space-y-4">
      <p className="text-sm">
        为 {files.length}{" "}
        个文件管理人工标签。追加不会覆盖已有标签，规则和建议层独立保留。
      </p>
      <Choice
        label="标签操作"
        value={mode}
        onChange={(v) => setMode(v as typeof mode)}
        options={[
          { value: "add", label: "追加标签" },
          { value: "remove", label: "移除指定标签" },
          { value: "replace", label: "替换全部人工标签" },
        ]}
        className="w-full"
      />
      <div className="max-h-64 divide-y divide-border overflow-y-auto rounded-xl border border-border px-4">
        {s.tags
          .filter((t) => t.space === s.space)
          .map((t) => {
            const count = files.filter((f) =>
              assignedTags(f).includes(t.id),
            ).length;
            return (
              <div key={t.id} className="flex min-h-12 items-center gap-3">
                <Checkbox
                  aria-label={`使用标签 ${t.name}`}
                  checked={tags.includes(t.id)}
                  onCheckedChange={() =>
                    setTags(
                      tags.includes(t.id)
                        ? tags.filter((id) => id !== t.id)
                        : [...tags, t.id],
                    )
                  }
                />
                <span
                  className={`rounded px-2 py-1 text-xs ${tagColor[t.color]}`}
                >
                  {t.name}
                </span>
                <span className="ml-auto text-xs text-muted-foreground">
                  {count === files.length
                    ? "全部拥有"
                    : count
                      ? `部分拥有 · ${count}/${files.length}`
                      : "尚未添加"}
                </span>
              </div>
            );
          })}
      </div>
      <div className="flex gap-2">
        <Input
          aria-label="新标签名称"
          placeholder="新建一个标签"
          className="h-11"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Btn
          variant="outline"
          disabled={!name.trim()}
          onClick={() => {
            dispatch({ type: "tag-create", name, group: "", color: "pine" });
            setName("");
          }}
        >
          创建
        </Btn>
      </div>
      <div className="flex justify-end gap-2">
        <Btn variant="outline" onClick={close}>
          取消
        </Btn>
        <Btn
          disabled={!files.length || (!tags.length && mode !== "replace")}
          onClick={() => {
            const action = {
              type: "tag-apply" as const,
              ids: files.map((f) => f.id),
              tagIds: tags,
              mode,
              confirmed: true,
            };
            if (mode === "replace")
              open({
                type: "danger",
                title: "替换全部人工标签？",
                text: `将替换 ${files.length} 个文件的人工标签；规则标签与 Agent 建议不变。`,
                action,
              });
            else {
              dispatch(action);
              close();
            }
          }}
        >
          确认更新标签
        </Btn>
      </div>
    </div>
  );
}
export function Tags() {
  const { state: s, dispatch, open } = useDemo();
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState("");
  const [name, setName] = useState("");
  const [group, setGroup] = useState("项目");
  const [color, setColor] = useState("pine");
  const [newGroup, setNewGroup] = useState("");
  const [target, setTarget] = useState("");
  const tags = s.tags.filter(
    (t) => t.space === s.space && t.name.includes(query),
  );
  const current = s.tags.find((t) => t.id === editing && t.space === s.space);
  const impact = current ? tagImpact(s, current.id) : null;
  return (
    <>
      <Heading
        title="标签与属性"
        note="按主题、项目或状态找到文件，文件仍保留在原目录。"
      />
      <div className="mb-5 flex gap-2">
        <Input
          aria-label="搜索标签"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="搜索标签"
          className="h-11"
        />
        <Btn
          onClick={() => {
            setEditing("");
            setName("");
          }}
        >
          新建标签
        </Btn>
      </div>
      <div className="grid gap-3 min-[1100px]:grid-cols-[minmax(0,1fr)_300px]">
        <section className="min-w-0">
          {[
            ...new Set([
              ...s.tagGroups
                .filter((g) => g.space === s.space)
                .map((g) => g.name),
              "",
            ]),
          ].map((g) => (
            <div key={g} className="mb-3">
              <h2 className="mb-2 text-sm font-semibold">{g || "未分组"}</h2>
              <div className="divide-y divide-border rounded-xl border border-border bg-card">
                {tags
                  .filter((t) => t.group === g)
                  .map((t) => (
                    <div
                      key={t.id}
                      className="flex min-h-9 items-center gap-2 px-2 max-[767px]:min-h-12 [@media(pointer:coarse)]:min-h-12"
                    >
                      <Btn
                        variant="ghost"
                        className="min-w-0 flex-1 !justify-start"
                        onClick={() => {
                          dispatch({
                            type: "search-patch",
                            patch: {
                              query: "",
                              tab: "files",
                              tagIds: [t.id],
                              tagMode: "all",
                            },
                          });
                          dispatch({ type: "search-submit" });
                          dispatch({ type: "navigate", page: "search" });
                        }}
                      >
                        <span
                          className={`rounded px-2 py-1 ${tagColor[t.color]}`}
                        >
                          #{t.name}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {tagImpact(s, t.id).files} 个文件
                        </span>
                      </Btn>
                      <IconBtn
                        label={`${t.favorite ? "取消收藏" : "收藏"}标签 ${t.name}`}
                        onClick={() =>
                          dispatch({
                            type: "tag-update",
                            id: t.id,
                            favorite: !t.favorite,
                          })
                        }
                      >
                        <Star
                          size={15}
                          className={
                            t.favorite ? "fill-primary/30 text-primary" : ""
                          }
                        />
                      </IconBtn>
                      <Btn
                        variant="ghost"
                        onClick={() => {
                          setEditing(t.id);
                          setName(t.name);
                          setGroup(t.group);
                          setColor(t.color);
                          setTarget("");
                        }}
                      >
                        管理
                      </Btn>
                    </div>
                  ))}
                {!tags.some((t) => t.group === g) && (
                  <p className="p-4 text-xs text-muted-foreground">暂无标签</p>
                )}
              </div>
            </div>
          ))}
        </section>
        <section className={`${panel} h-fit space-y-3 p-3`}>
          <h2 className="font-semibold">
            {current ? `管理「${current.name}」` : "新建标签"}
          </h2>
          <Input
            aria-label="标签名称"
            className="h-11"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="输入标签名称"
          />
          <Choice
            label="标签分组"
            value={group}
            onChange={setGroup}
            className="w-full"
            options={[
              { value: "", label: "未分组" },
              ...s.tagGroups
                .filter((g) => g.space === s.space)
                .map((g) => ({ value: g.name, label: g.name })),
            ]}
          />
          <Choice
            label="标签颜色"
            value={color}
            onChange={setColor}
            className="w-full"
            options={[
              { value: "pine", label: "松绿" },
              { value: "blue", label: "蓝色" },
              { value: "amber", label: "琥珀" },
              { value: "rose", label: "玫红" },
              { value: "slate", label: "灰色" },
            ]}
          />
          <Btn
            disabled={!name.trim()}
            onClick={() =>
              dispatch(
                current
                  ? { type: "tag-update", id: current.id, name, group, color }
                  : { type: "tag-create", name, group, color },
              )
            }
          >
            {current ? "保存标签" : "创建标签"}
          </Btn>
          <div className="border-t border-border pt-4">
            <p className="mb-2 text-xs text-muted-foreground">新建分组</p>
            <div className="flex gap-2">
              <Input
                aria-label="新分组名称"
                className="h-11"
                value={newGroup}
                onChange={(e) => setNewGroup(e.target.value)}
              />
              <IconBtn
                label="创建标签分组"
                onClick={() => {
                  dispatch({ type: "tag-group", name: newGroup });
                  setNewGroup("");
                }}
              >
                <Plus size={16} />
              </IconBtn>
            </div>
          </div>
          {current && impact && (
            <div className="space-y-3 border-t border-border pt-4">
              <p className="text-sm">
                影响 {impact.files} 个文件、{impact.views}{" "}
                个保存视图。文件不会删除。
              </p>
              <Choice
                label="合并到标签"
                value={target}
                onChange={setTarget}
                className="w-full"
                options={[
                  { value: "", label: "选择目标标签" },
                  ...s.tags
                    .filter((t) => t.space === s.space && t.id !== current.id)
                    .map((t) => ({ value: t.id, label: t.name })),
                ]}
              />
              <Btn
                variant="outline"
                disabled={!target}
                onClick={() =>
                  open({
                    type: "danger",
                    title: "合并标签？",
                    text: `将更新 ${impact.files} 个文件和 ${impact.views} 个保存视图，合并后保留目标标签。`,
                    action: { type: "tag-merge", from: current.id, to: target },
                  })
                }
              >
                <Merge size={15} />
                合并
              </Btn>
              <Btn
                variant="ghost"
                onClick={() =>
                  open({
                    type: "danger",
                    title: "删除这个标签？",
                    text: `从 ${impact.files} 个文件及 ${impact.views} 个视图移除标签。视图范围可能扩大，请复查；文件保留。`,
                    action: { type: "tag-delete", id: current.id },
                  })
                }
              >
                <Trash2 size={15} />
                删除标签
              </Btn>
            </div>
          )}
        </section>
      </div>
      <DocumentTagCatalog />
      <PropertyDefinitions />
    </>
  );
}
