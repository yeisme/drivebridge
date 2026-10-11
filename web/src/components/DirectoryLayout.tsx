import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { ArrowRight, LayoutGrid, List } from "lucide-react";
import { Button } from "./ui/button";
import { AppSheet } from "./AppSheet";

function query() { return new URLSearchParams(location.hash.split("?")[1] || ""); }
function replaceQuery(key: string, value?: string) {
  const params = query();
  if (value) params.set(key, value); else params.delete(key);
  const route = location.hash.startsWith("#/") ? location.hash.split("?")[0] : "#/drives";
  history.replaceState(history.state, "", location.pathname + location.search + route + (params.size ? "?" + params : ""));
}
export function useDirectoryQuery(key: string, fallback = "") {
  const [value, setValue] = useState(() => query().get(key) || fallback);
  useEffect(() => {
    const restore = () => setValue(query().get(key) || fallback);
    window.addEventListener("hashchange", restore);
    return () => window.removeEventListener("hashchange", restore);
  }, [key, fallback]);
  return [value, (next: string) => { setValue(next); replaceQuery(key, next === fallback ? undefined : next); }] as const;
}
export type DirectoryItem = {
  id: string; title: string; subtitle: string; icon: ReactNode; meta: ReactNode;
  detail: ReactNode; openLabel: string; open: () => void;
};
const scrollPositions = new Map<string, number>();

export function DirectoryLayout({ items, cards, label }: { items: DirectoryItem[]; cards: ReactNode; label: string }) {
  const [layout, setLayout] = useDirectoryQuery("layout", "list");
  const [selected, setSelected] = useState<string | undefined>(() => query().get("selected") ?? undefined);
  const [sheet, setSheet] = useState(false);
  const [wide, setWide] = useState(false);
  const root = useRef<HTMLDivElement>(null), list = useRef<HTMLDivElement>(null);
  const scrollKey = location.hash.split("?")[0] + ":" + (query().get("tenant") || "") + ":" + (query().get("q") || "") + ":" + (query().get("group") || "");
  const current = selected === undefined ? items[0] : items.find(item => item.id === selected);
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => setWide(entry.contentRect.width >= 880));
    if (root.current) observer.observe(root.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    const restore = () => { setSelected(query().get("selected") ?? undefined); setSheet(false); };
    window.addEventListener("hashchange", restore);
    return () => window.removeEventListener("hashchange", restore);
  }, []);
  useEffect(() => {
    if (selected && !items.some(item => item.id === selected)) { setSelected(""); setSheet(false); replaceQuery("selected"); }
  }, [items, selected]);
  useLayoutEffect(() => { if (list.current) list.current.scrollTop = scrollPositions.get(scrollKey) || 0; }, [scrollKey, layout]);
  const select = (item: DirectoryItem) => { setSelected(item.id); replaceQuery("selected", item.id); if (!wide) setSheet(true); };
  const open = (item: DirectoryItem) => { replaceQuery("selected", item.id); item.open(); };
  const detail = current ? <div className="space-y-4 break-words">
    <h2 className="text-base font-semibold">{current.title}</h2>
    <p className="text-sm text-muted-foreground">{current.subtitle}</p>
    {current.detail}
    <Button type="button" onClick={() => open(current)}>进入所选{label}<ArrowRight /></Button>
  </div> : <p className="p-4 text-sm text-muted-foreground">选择{label}查看详情</p>;
  return <div ref={root} className="directory-layout min-w-0">
    <div className="mb-3 flex justify-end gap-1" role="group" aria-label={`${label}目录视图`}>
      <Button type="button" variant={layout !== "cards" ? "secondary" : "ghost"} aria-pressed={layout !== "cards"} onClick={() => {setLayout("list");setSheet(false);}}><List />列表详情</Button>
      <Button type="button" variant={layout === "cards" ? "secondary" : "ghost"} aria-pressed={layout === "cards"} onClick={() => {setLayout("cards");setSheet(false);}}><LayoutGrid />卡片视图</Button>
    </div>
    {layout === "cards" ? cards : <div className="grid min-w-0 gap-4" style={{gridTemplateColumns: wide ? "minmax(0,1fr) 320px" : "minmax(0,1fr)"}}>
      <div ref={list} className="max-h-[calc(100dvh-16rem)] min-w-0 overflow-auto rounded-xl border border-border bg-card" aria-label={`${label}列表`} onScroll={e => scrollPositions.set(scrollKey, e.currentTarget.scrollTop)}>
        {items.map(item => <div key={item.id} className="flex min-h-16 items-center gap-2 border-b border-border px-4 py-2 last:border-0" data-selected={current?.id === item.id} style={current?.id === item.id ? {background:"var(--ui-accent)"} : undefined}>
          <Button layout="surface" type="button" className="directory-select flex min-w-0 flex-1 items-center gap-3 text-left" aria-label={`查看${label}详情 ${item.title}`} aria-pressed={current?.id === item.id}
            onClick={() => select(item)} onDoubleClick={() => open(item)} onKeyDown={e => {if(e.key === "Enter"){e.preventDefault();open(item);}}}>
            <span className="shrink-0 text-muted-foreground">{item.icon}</span><span className="min-w-0 flex-1"><strong className="block truncate text-sm" title={item.title}>{item.title}</strong><small className="block truncate text-xs text-muted-foreground" title={item.subtitle}>{item.subtitle}</small></span>
            <span className="hidden shrink-0 text-right text-xs tabular-nums text-muted-foreground sm:block">{item.meta}</span>
          </Button>
          <Button type="button" variant="ghost" size="icon" aria-label={item.openLabel} title={item.openLabel} onClick={() => open(item)}><ArrowRight /></Button>
        </div>)}
        {!items.length && <p className="p-4 text-sm text-muted-foreground">没有匹配的{label}，请调整筛选条件。</p>}
      </div>
      {wide && <aside className="min-w-0 rounded-xl border border-border bg-card p-5" aria-label={`${label}详情`}>{detail}</aside>}
      <AppSheet title={`${label}详情`} open={!wide && sheet && !!current} onOpenChange={setSheet}>{detail}</AppSheet>
    </div>}
  </div>;
}
