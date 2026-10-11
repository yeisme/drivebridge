import { PropertyExtraction } from "./PropertyWorkspace";
import { useEffect, useState } from "react";
import { ArrowLeft, Database, Folder, Play, RefreshCw } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Btn, Choice, Empty, Pill, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { DirectoryTree } from "./CloudExplorer";
import {
  directoryFiles,
  directoryAvailable,
  rootNode,
  bytes,
} from "./storage-model";
import { IndexSettings } from "./SearchWorkspace";
const labels: Record<string, string> = {
  running: "扫描中",
  queued: "排队中",
  complete: "完整",
  partial: "部分完成",
  failed: "失败",
  unknown: "结果未知",
  cancelled: "已取消",
};
export function StorageIndexes() {
  const { state: s, dispatch, open } = useDemo();
  const st = s.storage;
  const [edit, setEdit] = useState(false),
    [content, setContent] = useState(false),
    [demo, setDemo] = useState(false);
  const node = st.nodes.find((n) => n.id === st.inspect && n.space === s.space);
  const scope = node ? st.scopes[node.id] : undefined;
  const run = st.runs.find((r) => r.id === st.run && r.space === s.space);
  const c = s.connections.find(
    (c) => c.id === (node?.connectionId || run?.connectionId),
  );
  const [exclusions, setExclusions] = useState("私密,凭据"),
    [budget, setBudget] = useState(10000),
    [policy, setPolicy] = useState<"manual" | "changes" | "daily">("manual");
  useEffect(() => {
    setExclusions(scope?.exclusions.join(",") || "私密,凭据");
    setBudget(scope?.budget || 10000);
    setPolicy(scope?.policy || "manual");
  }, [node?.id, scope?.generation]);
  const choose = (id: string) =>
    dispatch({ type: "storage-page", tab: "indexes", directoryId: id });
  if (s.actor !== "yeisme")
    return (
      <Empty
        title="没有索引管理权限"
        text="共享文件可从共享给我进入；加入团队不会授予整个存储的扫描权限。"
      />
    );
  if (
    run &&
    (directoryAvailable(
      s,
      st.nodes.find((n) => n.id === run.directoryId),
    ) ||
      c?.revision !== run.revision)
  )
    return (
      <Empty
        title="运行记录的访问范围已变化"
        text={`保留原操作 ${run.id}；恢复授权并核对范围后查看明细。`}
        action="返回索引总览"
        onAction={() => dispatch({ type: "storage-page", tab: "indexes" })}
      />
    );
  if (run)
    return (
      <section aria-label="扫描运行详情">
        <Btn variant="ghost" onClick={() => choose(run.directoryId)}>
          <ArrowLeft size={15} />
          返回索引范围
        </Btn>
        <div className="my-4 flex flex-wrap items-center gap-3">
          <h2 className="text-lg font-semibold">{run.id}</h2>
          <Pill tone={run.status === "complete" ? "green" : "amber"}>
            {labels[run.status]}
          </Pill>
          <span className="text-xs text-muted-foreground">
            {c?.name} · 目录元数据 · 演示
          </span>
        </div>
        <div className="rounded-xl border bg-card p-4 text-sm leading-8">
          <p>范围：{st.nodes.find((n) => n.id === run.directoryId)?.path}</p>
          <p>
            已观察 {run.observed.length} 项 · 失败 {run.failed.length} 项 · 总量{" "}
            {run.total ?? "未知"}
          </p>
          <p>
            观测时间：{run.observedAt || "尚未取得"} · 范围代次 {run.generation}
          </p>
          <p>
            {run.checkpoint
              ? "此演示连接支持检查点；查询后可恢复原任务。"
              : "此连接没有检查点续扫能力；需查询后明确重新扫描。"}
          </p>
          <p className="text-xs text-muted-foreground">
            部分扫描不推导删除。未完成结果不会覆盖最近完整记录。
          </p>
        </div>
        <div className="my-3 flex flex-wrap gap-2">
          <Btn onClick={() => dispatch({ type: "catalog-query", id: run.id })}>
            查询原操作
          </Btn>
          <Btn
            variant="outline"
            disabled={
              !run.checked || !["partial", "failed"].includes(run.status)
            }
            onClick={() =>
              run.checkpoint
                ? dispatch({ type: "catalog-resume", id: run.id })
                : dispatch({
                    type: "catalog-start",
                    directoryId: run.directoryId,
                  })
            }
          >
            {run.checkpoint ? "续扫原任务" : "重新扫描"}
          </Btn>
          <Btn variant="ghost" onClick={() => setDemo(true)}>
            演示扫描反馈
          </Btn>
        </div>
        <h3 className="my-3 text-sm font-semibold">逐项记录</h3>
        <div className="rounded-lg border bg-card">
          {[...run.observed, ...run.failed].map((id) => {
            const f = s.files.find((f) => f.id === id),
              n = st.nodes.find((n) => n.id === id);
            return (
              <div
                key={id}
                className="flex min-h-11 items-center gap-3 border-b px-3 text-xs"
              >
                <span className="min-w-0 flex-1 truncate">
                  {f?.name || n?.name || "条目已不可用"}
                </span>
                <Pill tone={run.failed.includes(id) ? "amber" : "green"}>
                  {run.failed.includes(id) ? "读取中断" : "已观察"}
                </Pill>
              </div>
            );
          })}
          {!run.observed.length && !run.failed.length && (
            <p className="p-4 text-xs text-muted-foreground">
              等待原操作反馈，尚无逐项结果。
            </p>
          )}
        </div>
        <Dialog open={demo} onOpenChange={setDemo}>
          <DialogContent>
            <DialogTitle>演示扫描反馈</DialogTitle>
            <DialogDescription>
              只修改当前模拟运行，不接触真实网盘。
            </DialogDescription>
            {(["complete", "partial", "failed", "unknown"] as const).map(
              (result) => (
                <Btn
                  key={result}
                  disabled={run.status !== "running"}
                  onClick={() => {
                    dispatch({
                      type: "catalog-complete",
                      id: run.id,
                      generation: run.generation,
                      result,
                    });
                    setDemo(false);
                  }}
                >
                  {labels[result]}
                </Btn>
              ),
            )}
          </DialogContent>
        </Dialog>
      </section>
    );
  if (!node)
    return (
      <section aria-label="索引总览">
        <div className="mb-4 flex items-center gap-2">
          <Database size={18} />
          <h2 className="font-semibold">目录与内容索引</h2>
        </div>
        <p className="mb-4 text-xs leading-6 text-muted-foreground">
          先确认连接根目录及排除项，再建立元数据索引。正文分析独立授权；容量观测另行统计。
        </p>
        <div className="overflow-hidden rounded-xl border bg-card">
          {st.nodes
            .filter((n) => n.space === s.space && n.parentId === null)
            .map((n) => {
              const sc = st.scopes[n.id],
                r = st.runs.find((r) => r.id === sc?.lastRun),
                conn = s.connections.find((c) => c.id === n.connectionId)!;
              return (
                <button
                  key={n.id}
                  onClick={() => choose(n.id)}
                  data-compact-status
                  className="flex min-h-12 w-full min-w-0 items-center gap-2 border-b px-2 py-1 text-left hover:bg-accent/30"
                >
                  <Folder size={20} className="shrink-0 text-primary" />
                  <span className="min-w-0 flex-1">
                    <strong className="block truncate text-sm">
                      {conn.name}
                    </strong>
                    <span className="text-xs text-muted-foreground">
                      {n.path} · {r?.observedAt || "尚未扫描"}·{" "}
                      {r
                        ? `${r.observed.length} 已观察 · ${r.failed.length} 失败`
                        : "覆盖未知"}
                    </span>
                  </span>
                  <Pill tone={r?.status === "complete" ? "green" : "amber"}>
                    {!sc?.confirmed
                      ? "未启用"
                      : sc.paused
                        ? "已暂停"
                        : r
                          ? labels[r.status]
                          : "待扫描"}
                  </Pill>
                </button>
              );
            })}
        </div>
      </section>
    );
  const last = st.runs.find((r) => r.id === scope?.lastRun),
    complete = st.runs.find((r) => r.id === scope?.lastComplete);
  const error = directoryAvailable(s, node);
  return (
    <section aria-label="索引范围详情">
      <div className="mb-3 flex h-11 items-center gap-2">
        <Btn
          variant="ghost"
          onClick={() => dispatch({ type: "storage-page", tab: "indexes" })}
        >
          <ArrowLeft size={14} />
          全部索引
        </Btn>
        <span className="min-w-0 flex-1 truncate text-sm font-medium">
          {c?.name} / {node.path}
        </span>
      </div>
      <div className="grid min-h-[440px] gap-4 min-[1000px]:grid-cols-[240px_minmax(0,1fr)]">
        <aside className="hidden max-h-[560px] overflow-auto rounded-lg border bg-card min-[1000px]:block">
          <DirectoryTree onChoose={choose} selected={node.id} />
        </aside>
        <div className="min-w-0">
          <PropertyExtraction key={node.id} directoryId={node.id} />
          <div className="mb-4 flex gap-1">
            <Btn
              aria-pressed={!content}
              variant={!content ? "secondary" : "ghost"}
              onClick={() => setContent(false)}
            >
              目录索引
            </Btn>
            <Btn
              aria-pressed={content}
              variant={content ? "secondary" : "ghost"}
              onClick={() => setContent(true)}
            >
              内容索引
            </Btn>
          </div>
          {content ? (
            <IndexSettings key={node.id} />
          ) : (
            <>
              <div className="rounded-lg border bg-card p-4 text-sm leading-8">
                <p>
                  状态：
                  {scope?.paused
                    ? "后续更新已暂停"
                    : last
                      ? labels[last.status]
                      : "未启用"}
                </p>
                <p>
                  已观察：{last?.observed.length ?? "未知"} · 总条目：
                  {last?.total ?? "未知"}
                </p>
                <p>
                  最近完整扫描：{complete?.id || "尚未取得"}
                  {last && complete && last.id !== complete.id
                    ? " · 重建中沿用旧结果"
                    : ""}
                </p>
                <p>
                  排除：{scope?.exclusions.join("、") || "尚未确认"} · 预算：
                  {scope?.budget ?? "待确认"}
                </p>
                <p>
                  更新策略：
                  {scope?.policy === "daily"
                    ? "定时扫描（演示）"
                    : scope?.policy === "changes"
                      ? "变化游标（演示）"
                      : "手动"}
                </p>
                <p className="text-xs text-muted-foreground">
                  {c?.kind === "s3"
                    ? "对象前缀扫描；无原生变化游标。"
                    : "变化能力需逐连接验证；此处为交互演示。"}{" "}
                  不读取正文、不替代文件权限。
                </p>
                {error && <p className="text-destructive">{error}</p>}
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <Btn onClick={() => setEdit(true)}>配置扫描范围</Btn>
                <Btn
                  variant="outline"
                  disabled={!scope?.confirmed || !!error || scope.paused}
                  onClick={() =>
                    dispatch({ type: "catalog-start", directoryId: node.id })
                  }
                >
                  刷新目录索引
                </Btn>
                <Btn
                  variant="outline"
                  disabled={!scope?.confirmed}
                  onClick={() =>
                    dispatch({ type: "catalog-pause", directoryId: node.id })
                  }
                >
                  {scope?.paused ? "恢复后续更新" : "暂停后续更新"}
                </Btn>
                <Btn
                  variant="ghost"
                  disabled={!scope?.confirmed}
                  onClick={() =>
                    open({
                      type: "danger",
                      title: "清除这个范围的目录索引？",
                      text: "只清除索引结果与后续扫描配置。原文件、标签和运行记录保留。",
                      action: { type: "catalog-clear", directoryId: node.id },
                    })
                  }
                >
                  清除索引
                </Btn>
              </div>
              <h3 className="mb-2 mt-6 text-sm font-semibold">运行记录</h3>
              {st.runs
                .filter((r) => r.directoryId === node.id)
                .map((r) => (
                  <Btn
                    key={r.id}
                    variant="ghost"
                    className="!h-12 !w-full !justify-between border-b"
                    onClick={() =>
                      dispatch({
                        type: "storage-page",
                        tab: "indexes",
                        directoryId: node.id,
                        runId: r.id,
                      })
                    }
                  >
                    {r.id}
                    <span>
                      {labels[r.status]} · {r.observed.length} 项
                    </span>
                  </Btn>
                ))}
            </>
          )}
        </div>
      </div>
      <Dialog open={edit} onOpenChange={setEdit}>
        <DialogContent>
          <DialogTitle>确认扫描范围</DialogTitle>
          <DialogDescription>
            {c?.name} / {node.path}{" "}
            及子目录；仅名称、层级、类型、大小和时间。浏览器样本演示，无真实扫描费用。
          </DialogDescription>
          <label className="text-xs">
            排除路径或名称
            <Input
              aria-label="扫描排除项"
              value={exclusions}
              onChange={(e) => setExclusions(e.target.value)}
            />
          </label>
          <label className="text-xs">
            条目预算
            <Input
              aria-label="扫描条目预算"
              type="number"
              min={1}
              max={100000}
              value={budget}
              onChange={(e) => setBudget(Number(e.target.value))}
            />
          </label>
          <Choice
            label="扫描更新策略"
            value={policy}
            onChange={(v) => setPolicy(v as typeof policy)}
            options={[
              { value: "manual", label: "手动更新" },
              { value: "daily", label: "定时扫描（演示）" },
              ...(c?.kind !== "s3"
                ? [{ value: "changes", label: "变化游标（演示）" }]
                : []),
            ]}
          />
          <Btn
            disabled={!!error || !Number.isFinite(budget) || budget < 1}
            onClick={() => {
              dispatch({
                type: "catalog-configure",
                directoryId: node.id,
                exclusions: exclusions
                  .split(/[,，]/)
                  .map((x) => x.trim())
                  .filter(Boolean),
                budget,
                policy,
              });
              dispatch({ type: "catalog-start", directoryId: node.id });
              setEdit(false);
            }}
          >
            确认范围并扫描
          </Btn>
        </DialogContent>
      </Dialog>
    </section>
  );
}
export function StorageUsage() {
  const { state: s, dispatch } = useDemo();
  const [connection, setConnection] = useState(
      s.activeConnection === "all" ? "drive" : s.activeConnection,
    ),
    [list, setList] = useState(false);
  const root = rootNode(s, connection);
  const files = root ? directoryFiles(s, root.id, true) : [];
  const dirs = s.storage.nodes.filter((n) => n.parentId === root?.id);
  const c = s.connections.find((c) => c.id === connection);
  const total = files.reduce((n, f) => n + bytes(f.size), 0);
  const byType = [...new Set(files.map((f) => f.kind))].map((kind) => ({
    kind,
    size: files
      .filter((f) => f.kind === kind)
      .reduce((n, f) => n + bytes(f.size), 0),
  }));
  const error = directoryAvailable(s, root);
  const observations = s.storage.usage.filter(
    (x) =>
      x.connectionId === connection &&
      x.space === s.space &&
      x.actor === s.actor &&
      x.revision === c?.revision,
  );
  if (error)
    return (
      <Empty
        title="容量观测不可用"
        text={error}
        action="连接说明"
        onAction={() =>
          dispatch({ type: "connection-inspect", id: connection })
        }
      />
    );
  return (
    <section aria-label="存储用量">
      <div className="mb-4 flex flex-wrap gap-2">
        <Choice
          label="用量连接范围"
          value={connection}
          onChange={setConnection}
          options={s.connections
            .filter((c) => c.id === "drive" || c.space === s.space)
            .map((c) => ({ value: c.id, label: c.name }))}
        />
        <Btn variant="ghost" onClick={() => setList(!list)}>
          {list ? "图表视图" : "列表视图"}
        </Btn>
      </div>
      <p className="mb-4 text-xs leading-6 text-muted-foreground">
        {c?.name} · 当前授权样本范围 ·
        观测时间：本轮演示。外部全盘容量未知；目录观测与托管逻辑容量不相加。
      </p>
      <div className="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ["当前文件", `${(total / 1e6).toFixed(1)} MB`],
          ["历史版本", "未知"],
          [
            "回收站",
            `${s.files.filter((f) => f.trashed && f.connectionId === connection).length} 项 · 容量未知`,
          ],
          ["上传预留", "未知"],
        ].map(([label, value]) => (
          <div key={label} className="rounded-lg border bg-card p-4">
            <p className="text-xs text-muted-foreground">{label}</p>
            <p className="mt-2 text-lg font-semibold">{value}</p>
          </div>
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-xl border bg-card p-4">
          <h2 className="mb-3 text-sm font-semibold">
            目录占用 · 当前授权范围
          </h2>
          {dirs.map((n) => {
            const size = directoryFiles(s, n.id, true).reduce(
              (sum, f) => sum + bytes(f.size),
              0,
            );
            return (
              <button
                key={n.id}
                className="block min-h-14 w-full py-2 text-left text-xs"
                onClick={() => dispatch({ type: "directory-open", id: n.id })}
              >
                <span className="flex justify-between gap-2">
                  <span>{n.name}</span>
                  <span>{(size / 1e6).toFixed(1)} MB</span>
                </span>
                {!list && (
                  <span className="mt-2 block h-2 rounded bg-muted">
                    <span
                      className="block h-2 rounded bg-primary/70"
                      style={{ width: `${total ? (size / total) * 100 : 0}%` }}
                    />
                  </span>
                )}
              </button>
            );
          })}
          {!dirs.length && (
            <p className="text-xs text-muted-foreground">
              当前根目录没有子目录观测。
            </p>
          )}
        </div>
        <div className="rounded-xl border bg-card p-4">
          <h2 className="mb-3 text-sm font-semibold">文件类型分布</h2>
          {byType.map(({ kind, size }) => (
            <button
              key={kind}
              className="block min-h-14 w-full py-2 text-left text-xs"
              onClick={() => {
                dispatch({
                  type: "search-patch",
                  patch: {
                    connections: [connection],
                    directoryId: root?.id,
                    kind,
                    tab: "files",
                    folder: "全部文件",
                  },
                });
                dispatch({ type: "search-submit" });
                dispatch({ type: "navigate", page: "search" });
              }}
            >
              <span className="flex justify-between">
                <span>
                  {
                    (
                      {
                        document: "文档",
                        image: "图片",
                        video: "视频",
                        audio: "音频",
                        sheet: "表格",
                      } as Record<string, string>
                    )[kind]
                  }
                </span>
                <span>{(size / 1e6).toFixed(1)} MB</span>
              </span>
              {!list && (
                <span className="mt-2 block h-2 rounded bg-muted">
                  <span
                    className="block h-2 rounded bg-primary/50"
                    style={{ width: `${total ? (size / total) * 100 : 0}%` }}
                  />
                </span>
              )}
            </button>
          ))}
        </div>
      </div>
      <div className="mt-4 rounded-xl border bg-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-sm font-semibold">用量变化</h2>
          <Btn
            variant="outline"
            onClick={() =>
              dispatch({ type: "usage-observe", connectionId: connection })
            }
          >
            记录样本观测
          </Btn>
        </div>
        {observations.length < 2 ? (
          <p className="mt-2 text-xs text-muted-foreground">
            尚无可比较的历史容量观测；不补造增长曲线，目录扫描次数不等于容量采样。
          </p>
        ) : (
          <div className="mt-3 space-y-2" aria-label="容量观测趋势">
            {observations.map((x, i) => (
              <div key={i} className="flex min-h-11 items-center gap-3 text-xs">
                <span className="w-20 shrink-0">观测 {i + 1}</span>
                {!list && (
                  <span className="min-w-0 flex-1 rounded bg-muted">
                    <span
                      className="block h-3 rounded bg-primary/60"
                      style={{
                        width: `${(x.bytes / Math.max(1, ...observations.map((x) => x.bytes))) * 100}%`,
                      }}
                    />
                  </span>
                )}
                <span className="shrink-0">
                  {(x.bytes / 1e6).toFixed(1)} MB · {x.count} 项
                </span>
                <time
                  title={x.at}
                  className="hidden text-muted-foreground sm:block"
                >
                  {new Date(x.at).toLocaleTimeString("zh-CN")}
                </time>
              </div>
            ))}
            <p className="text-xs text-muted-foreground">
              变化：
              {(
                (observations.at(-1)!.bytes - observations[0].bytes) /
                1e6
              ).toFixed(1)}{" "}
              MB · 相同连接、身份与授权修订的样本观测。
            </p>
          </div>
        )}
      </div>
    </section>
  );
}
