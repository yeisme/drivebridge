import { useEffect, useRef, useState } from "react";
import { api, bytes, getDataContext, type Space } from "../../api";
import { Button } from "../../components/ui/button";
import { SelectAdapter } from "../../components/SelectAdapter";
import { WorkspacePageHeader } from "../../components/WorkspacePageHeader";
import { ArrowUpRight, Cloud, Folder, Inbox, RefreshCw } from "lucide-react";
import { MediaIcon } from "../../Media";
import { humanFileKind } from "../../lib/file-display";
import { HomeFileText } from "./HomeFileText";
import { HomeCollection } from "./HomeCollection";
import { submissionURL } from "../inbox/submission-route";
import type { components } from "../../generated/personal-workspace";

type Home = components["schemas"]["WorkspaceHomeEnvelope"]["data"];
type FileRef = Home["recent_files"][number];
const sections = ["recent_files", "collections", "pending_counts", "usage"] as const;
const validCount = (value: unknown) =>
  value === null || (typeof value === "number" && Number.isSafeInteger(value) && value >= 0);

function validHome(value: Home, space: string) {
  return (
    value &&
    value.space_id === space &&
    value.sections &&
    sections.every(
      (key) =>
        value.sections[key] &&
        ["ready", "partial", "unavailable"].includes(value.sections[key].state),
    ) &&
    Array.isArray(value.recent_files) &&
    value.recent_files.length <= 6 &&
    value.recent_files.every(
      (file) =>
        file.space_id === space &&
        !!file.file_id &&
        !!file.version_id &&
        typeof file.path === "string" &&
        typeof file.ref === "string" &&
        file.ref.endsWith(`/file/${file.file_id}/version/${file.version_id}`) &&
        file.size_bytes !== null &&
        validCount(file.size_bytes),
    ) &&
    Array.isArray(value.collections) &&
    value.collections.length <= 20 &&
    value.collections.every(
      (item) =>
        !!item.collection_id &&
        typeof item.name === "string" &&
        ["manual", "rule"].includes(item.kind),
    ) &&
    value.pending_counts &&
    validCount(value.pending_counts.submitted) &&
    validCount(value.pending_counts.accepted) &&
    value.usage &&
    Object.values(value.usage).length === 3 &&
    Object.values(value.usage).every(validCount)
  );
}

export function WorkspaceHome({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <ScopedHome key={scope} spaces={spaces.filter((space) => !space.team_id)} />;
}

function ScopedHome({ spaces }: { spaces: Space[] }) {
  const [chosen, setChosen] = useState("");
  const space = spaces.some((item) => item.id === chosen) ? chosen : spaces[0]?.id || "";
  return (
    <section aria-label="个人工作台摘要" className="min-w-0 space-y-5">
      <WorkspacePageHeader
        title="最近使用"
        actions={
          spaces.length ? (
            <div className="flex min-w-0 items-center gap-2 text-[13px] text-muted-foreground">
              <span className="shrink-0">云盘</span>
              <SelectAdapter
                aria-label="摘要空间"
                className="min-h-11 max-w-56 md:min-h-8"
                value={space}
                onChange={(event) => setChosen(event.target.value)}
              >
                {spaces.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </SelectAdapter>
            </div>
          ) : undefined
        }
      />
      {spaces.length ? (
        <HomeData key={space} space={space} />
      ) : (
        <p role="status" className="rounded-2xl border border-border bg-muted/40 p-5 text-sm text-muted-foreground">
          当前范围没有可用的个人空间；团队聚合尚未提供，可从“容量与活动观察”查看原总览。
        </p>
      )}
    </section>
  );
}

function HomeData({ space }: { space: string }) {
  const [home, setHome] = useState<Home | null>(null),
    [state, setState] = useState("loading"),
    [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<FileRef | null>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const [collection, setCollection] = useState<Home["collections"][number] | null>(null);
  const collectionTrigger = useRef<HTMLButtonElement | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const deny = () => {
    controllerRef.current?.abort();
    setHome(null);
    setSelected(null);
    setCollection(null);
    setState("denied");
  };

  useEffect(() => {
    const controller = new AbortController();
    controllerRef.current = controller;
    setHome(null);
    setSelected(null);
    setCollection(null);
    const invalidate = () => {
      controller.abort();
      setHome(null);
      setSelected(null);
      setCollection(null);
      setState("denied");
    };
    const offline = () => {
      controller.abort();
      setHome(null);
      setSelected(null);
      setCollection(null);
      setState("offline");
    };
    window.addEventListener("drivebridge-context", invalidate);
    window.addEventListener("offline", offline);
    if (navigator.onLine) {
      setState("loading");
      api<Home>(
        `/workspace/home?${new URLSearchParams({ space_id: space, recent_limit: "6" })}`,
        "GET",
        undefined,
        {},
        controller.signal,
      )
        .then((value) => {
          if (controller.signal.aborted) return;
          if (!validHome(value, space)) throw new Error("invalid_contract");
          setHome(value);
          setState("ready");
        })
        .catch((error: { status?: number }) => {
          if (!controller.signal.aborted) {
            let next = "error";
            if (error.status === 401 || error.status === 403) next = "denied";
            else if (error.status === 404) next = "unsupported";
            setState(next);
          }
        });
    } else {
      offline();
    }
    return () => {
      controller.abort();
      window.removeEventListener("drivebridge-context", invalidate);
      window.removeEventListener("offline", offline);
    };
  }, [space, attempt]);

  const sectionNote = (key: (typeof sections)[number]) => {
    if (home?.sections[key].state === "unavailable") {
      return <p role="status" className="text-xs text-muted-foreground">此段暂不可用，不代表数据为空。</p>;
    }
    if (home?.sections[key].state === "partial") {
      let detail = "部分固定版本当前不可用";
      if (key === "pending_counts") detail = "仅收件状态，不含其他任务、建议与审批";
      else if (key === "collections") detail = "最多展示最近 20 个集合";
      return <p className="text-xs text-muted-foreground">部分覆盖 · {detail}</p>;
    }
    return null;
  };

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-[13px] text-muted-foreground">快速回到最近打开的文件、常用资料集与 AI 投递成果。</p>
        <Button
          variant="outline"
          className="min-h-11 rounded-lg text-[13px] md:min-h-8"
          disabled={state === "loading" || state === "denied"}
          onClick={() => setAttempt((value) => value + 1)}
        >
          <RefreshCw className="size-3.5" />
          刷新工作台摘要
        </Button>
      </div>
      {state === "loading" && <p role="status" className="text-sm text-muted-foreground">正在加载最近动态…</p>}
      {state === "offline" && (
        <p role="alert" className="rounded-xl border border-border bg-muted/40 p-4 text-sm">
          当前离线，摘要已清除。联网后请手动刷新，不会自动重发。
        </p>
      )}
      {state === "denied" && (
        <p role="alert" className="rounded-xl border border-border bg-muted/40 p-4 text-sm">
          身份或范围已变化，摘要已清除，请重新进入。
        </p>
      )}
      {(state === "error" || state === "unsupported") && (
        <p role="alert" className="rounded-xl border border-border bg-muted/40 p-4 text-sm">
          工作台摘要不可用或服务尚未提供此入口，不代表没有文件。
        </p>
      )}
      {home && (
        <div className="space-y-7">
          <section aria-label="最近文件摘要" className="min-w-0 space-y-3">
            <h3 className="m-0 text-sm font-semibold">最近文件</h3>
            {sectionNote("recent_files")}
            {home.sections.recent_files.state !== "unavailable" &&
              (home.recent_files.length > 0 ? (
                <div className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
                  {home.recent_files.map((file) => {
                    const baseName = file.path.split("/").pop() || file.path;
                    const dirPath = file.path.includes("/") ? file.path.slice(0, file.path.lastIndexOf("/")) : "根目录";
                    const dateStr = Number.isFinite(Date.parse(file.updated_at))
                      ? new Date(file.updated_at).toLocaleDateString("zh-CN")
                      : "未提供";
                    return (
                      <Button
                        key={`${file.file_id}:${file.version_id}`}
                        layout="surface"
                        variant="ghost"
                        type="button"
                        aria-label={`查看固定引用：${file.path}`}
                        onClick={(event) => {
                          trigger.current = event.currentTarget;
                          setSelected(file);
                        }}
                        className="flex h-auto min-h-12 w-full items-center gap-3 px-3.5 py-2.5 text-left transition-colors hover:bg-muted/60"
                      >
                        <span className="shrink-0">
                          <MediaIcon file={file} size={20} />
                        </span>
                        <span className="grid min-w-0 flex-1 gap-0.5 whitespace-normal sm:flex sm:items-center sm:justify-between sm:gap-4">
                          <span className="min-w-0">
                            <strong className="line-clamp-1 break-all text-[13px] font-medium text-foreground" title={file.path}>
                              {baseName}
                            </strong>
                            <span className="line-clamp-1 text-xs text-muted-foreground">{dirPath}</span>
                          </span>
                          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                            {humanFileKind(file)} · {bytes(file.size_bytes)} · {dateStr}
                          </span>
                        </span>
                      </Button>
                    );
                  })}
                </div>
              ) : (
                <div className="rounded-xl border border-dashed border-border bg-muted/20 px-5 py-7 text-center text-[13px] text-muted-foreground">
                  当前观察没有可展示的最近文件。
                </div>
              ))}
          </section>

          <section aria-label="集合摘要" className="space-y-3">
            <h3 className="m-0 text-sm font-semibold">常用图集与资料包</h3>
            {sectionNote("collections")}
            {home.sections.collections.state !== "unavailable" && (
              <>
                {!home.collections.length && (
                  <p className="rounded-xl border border-dashed border-border bg-muted/20 px-5 py-6 text-center text-[13px] text-muted-foreground">
                    当前没有集合。
                  </p>
                )}
                {home.collections.length > 0 && (
                  <ul className="m-0 grid list-none gap-2.5 p-0 sm:grid-cols-2">
                    {home.collections.map((item) => (
                      <li
                        className="flex min-w-0 items-center gap-3 rounded-xl border border-border bg-card p-3 transition-colors hover:border-input hover:bg-muted/30"
                        key={item.collection_id}
                      >
                        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-foreground">
                          <Folder className="size-4" />
                        </span>
                        <div className="min-w-0 flex-1">
                          <Button
                            variant="link"
                            title={item.name}
                            className="h-auto min-h-11 max-w-full justify-start whitespace-normal break-words p-0 text-left text-sm font-semibold text-foreground md:min-h-7"
                            onClick={(event) => {
                              collectionTrigger.current = event.currentTarget;
                              setSelected(null);
                              setCollection(item);
                            }}
                          >
                            <span className="line-clamp-1">{item.name}</span>
                          </Button>
                          <p className="text-xs text-muted-foreground">
                            {item.kind === "manual" ? "手动资料集" : "智能规则集"} · 引用原文件
                          </p>
                        </div>
                        <ArrowUpRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                      </li>
                    ))}
                  </ul>
                )}
              </>
            )}
          </section>

          <div className="grid gap-4 lg:grid-cols-2">
            <section aria-label="收件待办摘要" className="space-y-3 rounded-xl border border-border bg-card p-4">
              <div className="flex items-center gap-2">
                <Inbox className="size-4 text-muted-foreground" />
                <h3 className="m-0 text-sm font-semibold">AI 投递待查收</h3>
              </div>
              {sectionNote("pending_counts")}
              {home.sections.pending_counts.state !== "unavailable" && (
                <ul className="m-0 list-none divide-y divide-border p-0">
                  {(
                    [
                      ["submitted", "待你确认查收", "查看待人工决定"],
                      ["accepted", "已收下待存入目录", "查看已接收待归档"],
                    ] as const
                  ).map(([filter, label, action]) => (
                    <li key={filter} className="flex flex-wrap items-center gap-3 py-2.5 first:pt-1 last:pb-0">
                      <span className="min-w-0 flex-1 text-sm">{label}</span>
                      <span className="rounded-md bg-secondary px-2 py-0.5 text-xs font-semibold text-foreground tabular-nums">
                        {home.pending_counts[filter] ?? "未知"}
                      </span>
                      <Button
                        variant="ghost"
                        className="min-h-11 rounded-lg text-[13px] md:min-h-8"
                        onClick={() => {
                          const context = getDataContext();
                          location.hash = submissionURL(space, filter, context.tenant, context.space);
                        }}
                      >
                        {action}
                        <ArrowUpRight className="size-4" />
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
              <p className="text-xs text-muted-foreground">确认通过后再归档至正式目录。</p>
            </section>

            <section aria-label="逻辑容量摘要" className="space-y-3 rounded-xl border border-border bg-card p-4">
              <div className="flex items-center gap-2">
                <Cloud className="size-4 text-muted-foreground" />
                <h3 className="m-0 text-sm font-semibold">当前网盘配额</h3>
              </div>
              {sectionNote("usage")}
              {home.sections.usage.state !== "unavailable" && (
                <dl className="grid grid-cols-3 gap-2.5 pt-1">
                  {(
                    [
                      ["used_bytes", "已使用"],
                      ["reserved_bytes", "上传预留"],
                      ["quota_bytes", "总配额"],
                    ] as const
                  ).map(([key, label]) => (
                    <div key={key} className="rounded-lg border border-border/70 bg-muted/40 p-2.5">
                      <dt className="text-xs text-muted-foreground">{label}</dt>
                      <dd className="m-0 mt-1 text-sm font-semibold tabular-nums">
                        {home.usage[key] === null ? "未知" : bytes(home.usage[key]!)}
                      </dd>
                    </div>
                  ))}
                </dl>
              )}
              <p className="text-xs text-muted-foreground">统计当前空间逻辑配额。</p>
            </section>
          </div>
        </div>
      )}
      {collection && (
        <HomeCollection
          key={collection.collection_id}
          collection={collection}
          onDenied={deny}
          onOpen={(file, button) => {
            trigger.current = button;
            setSelected(file);
          }}
          close={() => {
            setCollection(null);
            setSelected(null);
            collectionTrigger.current?.focus();
          }}
        />
      )}
      {selected && (
        <FixedReference
          key={`${selected.file_id}:${selected.version_id}`}
          file={selected}
          denied={deny}
          close={() => {
            setSelected(null);
            trigger.current?.focus();
          }}
        />
      )}
    </>
  );
}

export function FixedReference({
  file,
  close,
  denied,
  sourceLabel = "首页",
}: {
  sourceLabel?: "首页" | "集合页";
  file: FileRef;
  close: () => void;
  denied: () => void;
}) {
  const [reading, setReading] = useState(false);
  const [state, setState] = useState("loading");
  const heading = useRef<HTMLHeadingElement>(null);
  const onDenied = useRef(denied);
  onDenied.current = denied;

  useEffect(() => {
    heading.current?.focus();
    const controller = new AbortController();
    api<{ file: { id: string; space_id: string; version_id: string }; ref: string }>(
      `/references?${new URLSearchParams({ ref: file.ref })}`,
      "GET",
      undefined,
      {},
      controller.signal,
    )
      .then((value) => {
        if (controller.signal.aborted) return;
        if (
          value.ref !== file.ref ||
          value.file?.id !== file.file_id ||
          value.file.space_id !== file.space_id ||
          value.file.version_id !== file.version_id
        )
          throw new Error("invalid_contract");
        setState("ready");
      })
      .catch((error: { status?: number }) => {
        if (controller.signal.aborted) return;
        setState("error");
        if (error.status === 401 || error.status === 403) onDenied.current();
      });
    return () => controller.abort();
  }, [file]);

  return (
    <section
      aria-label={`${sourceLabel}固定引用`}
      className="space-y-3 rounded-xl border border-border bg-card p-4"
    >
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h3 ref={heading} tabIndex={-1} className="text-sm font-semibold">
          文件版本详情
        </h3>
        <Button className="min-h-11 rounded-lg md:min-h-8" variant="outline" onClick={close}>
          关闭固定引用
        </Button>
      </header>
      {state === "loading" && <p role="status" className="text-sm text-muted-foreground">正在核对该版本访问权限…</p>}
      {state === "error" && <p role="alert" className="text-sm text-destructive">原固定版本当前不可用，未改查最新版本。</p>}
      {state === "ready" && (
        <>
          <p className="break-all font-medium">{file.path}</p>
          <p className="break-all rounded-lg bg-muted/60 px-2.5 py-1.5 font-mono text-xs text-muted-foreground">{file.ref}</p>
          <Button
            variant="outline"
            className="min-h-11 rounded-lg md:min-h-8"
            aria-expanded={reading}
            onClick={() => setReading((value) => !value)}
          >
            {reading ? "关闭原版本正文" : "读取原版本正文"}
          </Button>
          {reading && (
            <HomeFileText
              label={`${sourceLabel}原版本纯文本`}
              space={file.space_id}
              file={file.file_id}
              version={file.version_id}
              onDenied={() => onDenied.current()}
            />
          )}
        </>
      )}
    </section>
  );
}
