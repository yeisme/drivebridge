// Search center (云端检索): unified retrieval over exact metadata lists and
// content Top-K candidates.
//
// Exact queries run through the preserved typed file-query surface; content
// discovery runs the frozen content-query contract. The two calibers are
// always grouped separately: Top-K candidates are labeled as candidates, not
// as a complete set. Coverage, missing modalities, index generation and
// observation time are shown as returned — never coerced. Late responses
// from a switched tenant/space context are dropped by revision guard.

import { Search, Files } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { latestRequest } from "../app/latest-request";
import { getDataContext, type Space } from "../api";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Checkbox } from "../components/ui/checkbox";
import { Table } from "../components/ui/table";
import { queryFiles } from "../info-workspace/client";
import type { FileQueryRequest } from "../info-workspace/model";
import { CapabilityGate, flowErrorText } from "./ConsumerFlows";
import { queryContent, saveSearch, type ContentQueryRequest, type FlowCapability } from "./client";
import { acceptQueryResponse, groupSearchResults, type ContentQueryView, type SearchResultGroups } from "./model";

import { SearchFileReference } from "./SearchFileReference";
import { SavedSearches } from "./SavedSearches";

type Mode = "exact" | "discovery";

const MODALITIES = ["text", "image", "video", "audio"];

export function SearchCenter({
  spaces,
  capability,
}: {
  spaces: Space[];
  capability: FlowCapability;
}) {
  const requests = useRef(latestRequest());
  const mounted = useRef(true);
  const criteriaRevision = useRef(0);
  const savePending = useRef(false);
  const [saving, setSaving] = useState(false);
  const [showSaved, setShowSaved] = useState(false);
  const [savedSpaces, setSavedSpaces] = useState<string[] | null>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; requests.current.cancel(); }; }, []);
  const [mode, setMode] = useState<Mode>("exact");
  const [text, setText] = useState("");
  const [modalities, setModalities] = useState<string[]>([]);
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [groups, setGroups] = useState<SearchResultGroups | null>(null);
  const [content, setContent] = useState<ContentQueryView | null>(null);
  const [exactCoverage, setExactCoverage] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [pageCursors, setPageCursors] = useState<(string | undefined)[]>([undefined]);
  const [pageIndex, setPageIndex] = useState(0);
  const [error, setError] = useState<string>("");
  const [saved, setSaved] = useState("");
  const [blocked, setBlocked] = useState<"denied" | "offline" | null>(null);
  const [selected, setSelected] = useState<{ file: string; version: string } | null>(null);
  const resultTrigger = useRef<HTMLButtonElement | null>(null);

  const invalidateQuery = () => {
    setSelected(null);
    criteriaRevision.current++;
    setNextCursor(null); setPageCursors([undefined]); setPageIndex(0);
    requests.current.cancel();
    setState("idle"); setGroups(null); setContent(null); setExactCoverage(null); setError(""); setSaved("");
  };

  useEffect(() => {
    const changed = () => {
      setShowSaved(false); setSavedSpaces(null);
      setSelected(null);
      criteriaRevision.current++;
      setNextCursor(null); setPageCursors([undefined]); setPageIndex(0);
      requests.current.cancel();
      setState("idle"); setGroups(null); setContent(null); setExactCoverage(null);
      setError(""); setSaved(""); setText(""); setModalities([]); setMode("exact"); setBlocked(null);
    };
    const offline = () => { changed(); setBlocked("offline"); };
    window.addEventListener("drivebridge-context", changed);
    window.addEventListener("offline", offline);
    return () => { window.removeEventListener("drivebridge-context", changed); window.removeEventListener("offline", offline); };
  }, []);

  const spaceIds = getDataContext().space ? [getDataContext().space] : spaces.map((space) => space.id);

  const exactQuery = (cursor?: string): FileQueryRequest => ({ mode: "exact", space_ids: savedSpaces ?? spaceIds, filters: [{ field: "path_prefix", op: "contains", value: text }], limit: 50, ...(cursor ? { cursor } : {}) });
  const clearResults = () => { setShowSaved(false); setSelected(null); setGroups(null); setContent(null); setExactCoverage(null); setNextCursor(null); setPageCursors([undefined]); setPageIndex(0); };
  const canRead = () => {
    if (blocked === "denied") return false;
    if (navigator.onLine === false) { requests.current.cancel(); clearResults(); setState("idle"); setBlocked("offline"); return false; }
    setBlocked(null);
    return mode === "exact" || capability === "available";
  };
  const run = async (cursor?: string, targetPage = 0) => {
    if (!canRead()) return;
    if (!text.trim()) {
      setError("请输入检索条件。");
      return;
    }
    setSelected(null);
    const pending = requests.current.start();
    setError("");
    setState("loading");
    // Capture the context revision at query time: a response that resolves
    // after a tenant/space switch is dropped instead of rendered under the
    // new context (the shared api() helper aborts in-flight calls the same
    // way; this guard also covers pollers that already resolved).
    const observedRevision = getDataContext().revision;
    try {
      if (mode === "exact") {
        const exact = await queryFiles(exactQuery(cursor), pending.signal);
        if (!pending.isCurrent() || !acceptQueryResponse(observedRevision, getDataContext().revision)) return;
        setContent(null);
        setExactCoverage(exact.coverage || "unknown");
        setNextCursor(exact.next_cursor);
        setPageIndex(targetPage);
        setPageCursors(current => [...current.slice(0, targetPage), cursor]);
        setGroups(groupSearchResults(exact, null));
      } else {
        const request: ContentQueryRequest = {
          mode: "discovery",
          query: { text },
          space_ids: spaceIds,
          evidence_required: true,
          limit: 20,
        };
        if (modalities.length) request.modalities = modalities;
        const view = await queryContent(request, pending.signal);
        if (!pending.isCurrent() || !acceptQueryResponse(observedRevision, getDataContext().revision)) return;
        setExactCoverage(null);
        setNextCursor(null);
        setContent(view);
        setGroups(groupSearchResults(null, view));
      }
      if (pending.isCurrent()) setState("ready");
    } catch (err) {
      if (!pending.isCurrent() || !acceptQueryResponse(observedRevision, getDataContext().revision)) return;
      setState("error");
      clearResults();
      if ([401, 403].includes((err as { status?: number }).status ?? 0)) { setBlocked("denied"); return; }
      if (err instanceof Error && (err.message === "cursor_expired" || (cursor && (err as Error & { status?: number }).status === 410))) {
        setNextCursor(null); setPageCursors([undefined]); setPageIndex(0);
        setGroups(null); setContent(null); setExactCoverage(null);
        setError("搜索分页已失效，检索条件已保留。请点击检索，从第一页重新查询。");
      } else setError(flowErrorText(err));
    }
  };

  const save = async () => {
    if (savePending.current || mode !== "exact" || !canRead()) return;
    if (!text.trim()) { setError("请输入检索条件。"); return; }
    savePending.current = true;
    setSaving(true); setError(""); setSaved("");
    const criteria = criteriaRevision.current;
    const revision = getDataContext().revision;
    const isCurrent = () => mounted.current && criteria === criteriaRevision.current && revision === getDataContext().revision;
    try {
      await saveSearch({ name: text.slice(0, 60) || "未命名条件", query: exactQuery() });
      if (isCurrent()) setSaved("检索条件已保存；再次使用时会重新授权读取。");
    } catch (err) {
      if (isCurrent()) {
        if ([401, 403].includes((err as { status?: number }).status ?? 0)) { requests.current.cancel(); clearResults(); setState("error"); setBlocked("denied"); }
        else setError("未取得保存回执，结果未知；请先在已保存条件中核对，不要重复保存同一条件。");
      }
    } finally {
      savePending.current = false;
      if (mounted.current) setSaving(false);
    }
  };

  return (
      <section className="flex min-w-0 flex-col gap-4 max-md:[&_button]:min-h-11 max-md:[&_input]:min-h-11" aria-label="云端检索" aria-busy={state === "loading"}>
        {blocked === "denied" && <p role="alert">访问权限已失效，搜索结果已清除。请重新确认账户与空间。</p>}
        {blocked === "offline" && <p role="alert">已离线，搜索结果已清除；联网后请手动重新检索。</p>}
        {error && (
          <div className="banner error" role="alert">
            {error}
          </div>
        )}
        <form onSubmit={event => { event.preventDefault(); if (state !== "loading") void run(); }} className="flex min-w-0 flex-col gap-5 rounded-xl border border-border bg-card p-5 sm:p-6" data-region="query">
          <div className="inline-flex w-fit max-w-full flex-wrap gap-1 rounded-lg bg-muted p-1" role="radiogroup" aria-label="检索口径">
            <Button variant="ghost" type="button" role="radio" aria-checked={mode === "exact"} className="aria-checked:bg-accent aria-checked:font-semibold" onClick={() => { invalidateQuery(); setMode("exact"); }}>
              精确元数据
            </Button>
            <Button variant="ghost" type="button" role="radio" aria-checked={mode === "discovery"} className="aria-checked:bg-accent aria-checked:font-semibold" onClick={() => { invalidateQuery(); setSavedSpaces(null); setMode("discovery"); }}>
              内容发现（Top-K）
            </Button>
          </div>
          {mode === "discovery" && <CapabilityGate capability={capability} action="content_query">{null}</CapabilityGate>}
          <label className="flex flex-col gap-2 text-sm font-medium">{mode === "exact" ? "文件名或路径" : "想查找的内容"}
          <div className="relative"><Search aria-hidden="true" className="pointer-events-none absolute left-3 top-3.5 size-5 text-muted-foreground"/><Input
            className="h-12 pl-11 text-base"
            ref={searchInput}
            type="search"
            value={text}
            onChange={(event) => { invalidateQuery(); setText(event.target.value); }}
            placeholder={mode === "exact" ? "按文件路径中的字面文本检索" : "按内容语义检索，例如：雨天街道的镜头"}
            aria-label="检索条件"
          /></div></label>
          {mode === "discovery" && (
            <fieldset className="cf-modalities">
              <legend>模态</legend>
              {MODALITIES.map((modality) => (
                <label key={modality}>
                  <Checkbox
                    checked={modalities.includes(modality)}
                    onCheckedChange={(checked) => {
                      invalidateQuery();
                      setModalities((current) =>
                        checked ? [...current, modality] : current.filter((item) => item !== modality),
                      );
                    }}
                  />
                  {modality}
                </label>
              ))}
            </fieldset>
          )}
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="default" type="button" onClick={() => void run()} disabled={state === "loading" || blocked === "denied" || (mode === "discovery" && capability !== "available")}>
              {state === "loading" ? "检索中…" : "检索"}
            </Button>
            <Button variant="outline" type="button" disabled={saving || mode !== "exact" || blocked === "denied"} onClick={() => void save()}>
              {saving ? "正在保存…" : "保存条件"}
            </Button>
            <Button type="button" variant="ghost" className="min-h-11 md:min-h-8" disabled={blocked === "denied"} aria-expanded={showSaved} onClick={() => setShowSaved(value => !value)}>{showSaved ? '关闭已保存条件' : '查看已保存条件'}</Button>
          </div>
          {savedSpaces && <p className="m-0 text-xs leading-5 text-muted-foreground">当前使用已保存的 {savedSpaces.length} 个空间；修改文本仍保留此范围。<Button type="button" variant="ghost" className="min-h-11" onClick={() => { invalidateQuery(); setSavedSpaces(null); }}>改用当前空间范围</Button></p>}
          {mode === "discovery" && <p className="m-0 text-xs leading-5 text-muted-foreground">内容发现条件尚不支持保存；不会降级保存为元数据查询。</p>}
          {mode === "exact" && <p className="m-0 text-xs leading-5 text-muted-foreground">查询整个文件路径的字面子串，不查询正文或标签。保存条件只记录查询，不授予文件权限。</p>}
          {saved && (
            <p className="m-0 text-xs leading-5 text-muted-foreground" role="status">
              {saved}
            </p>
          )}
        </form>

        {showSaved && <SavedSearches onSelect={query => { invalidateQuery(); setMode("exact"); setText(query.text); setModalities([]); setSavedSpaces(query.spaces); setShowSaved(false); searchInput.current?.focus(); }} onDenied={() => { requests.current.cancel(); clearResults(); setState("error"); setBlocked("denied"); }}/>}
        {selected && <SearchFileReference key={selected.file + ":" + selected.version} file={selected.file} version={selected.version} close={() => { setSelected(null); resultTrigger.current?.focus(); }} onDenied={() => { requests.current.cancel(); clearResults(); setState("error"); setBlocked("denied"); }}/>}
        {state === "idle" && <div role="status" className="flex min-h-56 flex-col items-center justify-center gap-3 px-5 py-10 text-center"><Files aria-hidden="true" className="size-8 text-muted-foreground"/><h3 className="m-0 text-base font-medium">从一个文件名开始</h3><p className="m-0 max-w-md text-sm leading-6 text-muted-foreground">输入检索条件后开始查询，尚未读取搜索结果。</p><p className="m-0 text-xs text-muted-foreground">只显示你有权访问的文件；打开时会再次核对权限。</p></div>}
        {state === "loading" && <p role="status">正在检索，请等待结果；发送请求不代表已找到文件。</p>}
        {state === "ready" && groups && (
          <div className="cf-results">
            {exactCoverage && <p className="m-0 text-xs leading-5 text-muted-foreground">元数据覆盖：{exactCoverage}{exactCoverage !== "complete" && "；结果覆盖不足或未知，不能据此断言没有文件。"}</p>}
            {groups.exact.length > 0 && (
              <div className="flex min-w-0 flex-col gap-3 rounded-lg border border-border bg-card p-3 text-card-foreground" data-region="exact">
                <h3 className="m-0 text-sm font-semibold">精确列表</h3>
                <Table className="w-full table-fixed [&_td]:whitespace-normal [&_td]:break-all [&_td]:p-2 [&_th]:p-2 [&_code]:text-xs" aria-label="精确元数据结果">
                  <thead>
                    <tr>
                      <th>文件</th>
                      <th>版本</th>
                    </tr>
                  </thead>
                  <tbody>
                    {groups.exact.map((item) => (
                      <tr key={item.file_id}>
                        <td>
                          <code>{item.file_id}</code>
                          <Button variant="ghost" className="mt-1 min-h-11 px-0 text-[13px] md:min-h-8" aria-label={`查看搜索固定版本 ${item.file_id}`} onClick={event => { resultTrigger.current = event.currentTarget; setSelected({ file: item.file_id, version: item.version_id }); }}>查看固定版本</Button>
                        </td>
                        <td>
                          <code>{item.version_id}</code>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              </div>
            )}
            {!content && (nextCursor || pageIndex > 0) && <nav className="cf-actions" aria-label="搜索结果分页">
              <span>第 {pageIndex + 1} 页{nextCursor ? "，后续仍有记录。" : "，已到末页。"}</span>
              {pageIndex > 0 && <Button type="button" variant="outline" onClick={() => void run(pageCursors[pageIndex - 1], pageIndex - 1)}>上一页</Button>}
              {nextCursor && <Button type="button" variant="outline" onClick={() => void run(nextCursor, pageIndex + 1)}>下一页</Button>}
            </nav>}
            {content && (
              <div className="flex min-w-0 flex-col gap-3 rounded-lg border border-border bg-card p-3 text-card-foreground" data-region="candidates">
                <h3 className="m-0 text-sm font-semibold">内容候选{groups.topK ? "（Top-K，不代表全集）" : ""}</h3>
                <p className="cf-chips">
                  <span className="cf-chip">覆盖：{content.coverage}</span>
                  {content.missing.length > 0 && <span className="rounded-full border border-border px-2 py-0.5 text-xs text-foreground">缺失模态：{content.missing.join("、")}</span>}
                  {content.observed_at && <span className="cf-chip">观测于 {content.observed_at}</span>}
                  {content.items[0]?.index_generation && <span className="cf-chip">索引代次 {content.items[0].index_generation}</span>}
                </p>
                <ul className="cf-candidates">
                  {content.items.map((item) => (
                    <li key={item.file_ref + (item.evidence_ref ?? "")}>
                      <div>
                        <code>{item.file_ref}</code>
                        {item.locator && (
                          <span className="cf-locator">
                            {" "}
                            {item.locator.kind === "time_range"
                              ? `${Math.round((item.locator.start_ms ?? 0) / 1000)}s–${Math.round((item.locator.end_ms ?? 0) / 1000)}s`
                              : item.locator.kind}
                          </span>
                        )}
                      </div>
                      <div className="cf-meta">
                        {item.truth_level && <span>可信度 {item.truth_level}</span>}
                        {item.evidence_ref && <span>证据 {item.evidence_ref}</span>}
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {groups.exact.length === 0 && !content && (
              <section className="cf-state" role="status" data-state="empty">
                <p>{exactCoverage === "complete" ? "没有符合条件的结果。" : "当前已覆盖范围未返回结果，请检查覆盖状态后重试。"}</p>
              </section>
            )}
          </div>
        )}
      </section>
  );
}
